"""Bounded local-file video sampling; only decoded frames leave the media service."""
import hashlib
import io
import json
import math
import os
from pathlib import Path
import subprocess
import tempfile
import time
from PIL import Image
from .types import AIError

VIDEO_EXTENSIONS = frozenset({'mp4', 'avi', 'wmv', 'mpeg', 'm4v', 'mov', 'flv', 'asf', 'f4v', 'rmvb', 'vob', 'rm'})
SAMPLING_VERSION = 1
SAMPLE_FRACTIONS = (0.05, 0.25, 0.5, 0.75, 0.95)
MAX_FRAMES = len(SAMPLE_FRACTIONS)
EXTRACT_SECONDS = 30
MAX_FRAME_EDGE = 1280
# Reject playlist/image demuxers: uploaded media must not reference other local files.
DEMUXERS = "mov,avi,asf,mpeg,mpegts,flv,rm"


def is_video(format):
    return (format or '').lower() in VIDEO_EXTENSIONS


def max_video_bytes():
    from flask import current_app, has_app_context
    return int(current_app.config['MAX_CONTENT_LENGTH'] if has_app_context()
               else os.getenv('MAX_CONTENT_LENGTH', 600 * 1024 * 1024))


def run_media(command, deadline):
    remaining = deadline - time.monotonic()
    if remaining <= 0:
        raise AIError('video_decode_timeout', 504)
    try:
        result = subprocess.run(command, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                stderr=subprocess.PIPE, timeout=remaining, check=False)
    except OSError as error:
        raise AIError('video_decoder_unavailable', 503) from error
    except subprocess.TimeoutExpired as error:
        raise AIError('video_decode_timeout', 504) from error
    if result.returncode:
        raise AIError('invalid_video', 415)
    return result.stdout


def extract_frames(path, *, deadline=None):
    deadline = min(deadline if deadline is not None else math.inf, time.monotonic() + EXTRACT_SECONDS)
    raw = run_media(['ffprobe', '-v', 'error', '-protocol_whitelist', 'file,pipe', '-format_whitelist', DEMUXERS,
                     '-show_entries', 'stream=index,codec_type,width,height,duration,nb_frames,avg_frame_rate:stream_disposition=attached_pic:format=duration',
                     '-of', 'json', str(path)], deadline)
    try:
        meta = json.loads(raw)
        stream = next((s for s in meta.get('streams', []) if s.get('codec_type') == 'video'
                       and not s.get('disposition', {}).get('attached_pic')), None)
        if stream is None:
            raise AIError('video_stream_missing', 415)
        duration_value = stream.get('duration')
        if duration_value in (None, '', 'N/A'):
            duration_value = meta.get('format', {}).get('duration', 0)
        duration = float(duration_value)
        width, height = int(stream['width']), int(stream['height'])
        if not math.isfinite(duration) or duration <= 0 or width <= 0 or height <= 0:
            raise ValueError()
        if width * height > 40_000_000:
            raise AIError('video_frame_too_large', 413)
        frame_count = int(stream.get('nb_frames', 0)) if str(stream.get('nb_frames', '')).isdigit() else 0
    except (ValueError, TypeError, KeyError) as error:
        raise AIError('invalid_video', 415) from error
    scale = f'scale=w=min({MAX_FRAME_EDGE}\\,iw):h=min({MAX_FRAME_EDGE}\\,ih):force_original_aspect_ratio=decrease'
    frames = []
    short = 0 < frame_count <= MAX_FRAMES
    times = []
    if short or (frame_count == 0 and duration < 1):
        raw_times = run_media(['ffprobe', '-v', 'error', '-protocol_whitelist', 'file,pipe', '-format_whitelist', DEMUXERS,
            '-select_streams', str(stream['index']), '-read_intervals', '%+#32',
            '-show_entries', 'frame=best_effort_timestamp_time', '-of', 'json', str(path)], deadline)
        try:
            times = [float(f['best_effort_timestamp_time']) for f in json.loads(raw_times).get('frames', [])]
        except (KeyError, ValueError, TypeError) as error:
            raise AIError('invalid_video', 415) from error
        short = short or 0 < len(times) <= MAX_FRAMES

    def read_frame(output, stamp):
        try:
            with Image.open(output) as source:
                frame = source.convert('RGB')
                encoded = io.BytesIO()
                frame.save(encoded, format='PNG')
        except (OSError, ValueError) as error:
            raise AIError('invalid_video', 415) from error
        frames.append({'timestamp_ms': round(stamp * 1000), 'data': encoded.getvalue()})

    with tempfile.TemporaryDirectory(prefix='photoly-frames-') as folder:
        base = ['ffmpeg', '-v', 'error', '-nostdin', '-threads', '2', '-protocol_whitelist', 'file,pipe', '-format_whitelist', DEMUXERS]
        output_args = ['-map', f'0:{stream["index"]}', '-an', '-sn', '-dn', '-vf', scale, '-threads', '1', '-y']
        # Decode short clips in one pass without fps duplication. This also handles
        # unknown frame counts and irregular frame timestamps in very short VFR clips.
        if short:
            run_media(base + ['-i', str(path)] + output_args + ['-vsync', '0', '-frames:v', str(MAX_FRAMES), str(Path(folder) / '%02d.png')], deadline)
            outputs = sorted(Path(folder).glob('*.png'))
            for i, output in enumerate(outputs):
                stamp = max(0, times[i]-times[0]) if i < len(times) else duration*i/max(1,len(outputs))
                read_frame(output, stamp)
        else:
            for fraction in SAMPLE_FRACTIONS:
                target = duration * fraction
                output = Path(folder) / 'frame.png'
                output.unlink(missing_ok=True)
                run_media(base + ['-ss', f'{target:.6f}', '-i', str(path)] + output_args + ['-frames:v', '1', str(output)], deadline)
                if not output.exists():
                    # A VFR tail can end before the container duration. Keep actual
                    # decoded samples; never create a replacement duplicate frame.
                    continue
                read_frame(output, target)
    if not frames:
        raise AIError('video_frame_missing', 415)
    return frames


def snapshot_video(stream, *, deadline=None, expected=None):
    digest = hashlib.sha256()
    size = 0
    limit = max_video_bytes()
    with tempfile.TemporaryDirectory(prefix='photoly-video-') as folder:
        path = Path(folder) / 'source'
        with path.open('wb') as output:
            while chunk := stream.read(1024 * 1024):
                if deadline is not None and time.monotonic() >= deadline:
                    raise AIError('query_timeout', 504)
                size += len(chunk)
                if size > limit:
                    raise AIError('video_too_large', 413)
                digest.update(chunk)
                output.write(chunk)
        if not size:
            raise AIError('invalid_video', 415)
        value = digest.hexdigest()
        if expected is not None and value != expected:
            raise AIError('content_hash_mismatch', 409)
        return {'hash': value, 'frames': extract_frames(path, deadline=deadline), 'sampling_version': SAMPLING_VERSION}
