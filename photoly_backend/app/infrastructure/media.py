"""Bounded image encoding and browser-compatible derivatives. Originals are never rewritten."""
import mimetypes
import subprocess
import warnings
from pathlib import Path
from PIL import Image, ImageOps, UnidentifiedImageError
from pillow_heif import register_heif_opener
from app.services.base import BusinessError

register_heif_opener(thumbnails=False)
TARGETS = {'jpeg': 'JPEG', 'png': 'PNG', 'gif': 'GIF'}
DECODABLE = {'jpg', 'jpeg', 'jfif', 'png', 'gif', 'webp', 'bmp', 'tif', 'tiff', 'ico', 'heic', 'heif'}
MAX_PIXELS = 50_000_000
MAX_ANIMATION_PIXELS = 200_000_000
ASSET_SUFFIXES = {'preview': '_preview.jpg', 'live_original': '_live.mov', 'live_preview': '_live.mp4'}


def asset_path(storage, user_uuid, photo, role):
    return storage.path(user_uuid, photo.photo_uuid + ASSET_SUFFIXES[role])


def mime(path):
    try:
        with Image.open(path) as image:
            return Image.MIME.get(image.format, 'application/octet-stream')
    except (OSError, ValueError, Image.DecompressionBombError):
        return mimetypes.guess_type(str(path))[0] or 'application/octet-stream'


def image_info(path):
    try:
        with warnings.catch_warnings():
            warnings.simplefilter('error', Image.DecompressionBombWarning)
            with Image.open(path) as source:
                if source.width * source.height > MAX_PIXELS:
                    raise BusinessError('image_too_many_pixels', 413)
                return {'animated': bool(getattr(source, 'is_animated', False)), 'format': source.format}
    except (UnidentifiedImageError, OSError, ValueError, Image.DecompressionBombError, Image.DecompressionBombWarning) as error:
        raise BusinessError('invalid_image', 415) from error


def encode_image(source_path, output_path, target):
    if target not in TARGETS:
        raise BusinessError('unsupported_target_format')
    image_info(source_path)
    try:
        with Image.open(source_path) as source:
            animated = target == 'gif' and bool(getattr(source, 'is_animated', False))
            count = getattr(source, 'n_frames', 1) if animated else 1
            if count > 1000 or count * source.width * source.height > MAX_ANIMATION_PIXELS:
                raise BusinessError('animation_too_large', 413)
            frames, durations = [], []
            for index in range(count):
                source.seek(index)
                frame = ImageOps.exif_transpose(source).convert('RGBA')
                if target == 'jpeg':
                    background = Image.new('RGBA', frame.size, 'white')
                    background.alpha_composite(frame)
                    frame = background.convert('RGB')
                frames.append(frame)
                durations.append(source.info.get('duration', 100))
            options = {}
            if target in ('jpeg', 'png'):
                exif = source.getexif()
                if exif:
                    exif[274] = 1  # Pixels have already been oriented.
                    options['exif'] = exif.tobytes()
                if source.info.get('icc_profile'):
                    options['icc_profile'] = source.info['icc_profile']
            if target == 'jpeg':
                options['quality'] = 90
            elif target == 'gif':
                options.update(save_all=animated, append_images=frames[1:], duration=durations if animated else durations[0], disposal=2)
                if 'loop' in source.info:
                    options['loop'] = source.info['loop']
            frames[0].save(output_path, format=TARGETS[target], **options)
    except (UnidentifiedImageError, OSError, ValueError, Image.DecompressionBombError) as error:
        raise BusinessError('conversion_failed') from error


def run_encoder(arguments, timeout=60):
    try:
        result = subprocess.run(['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-y',
                                 '-threads', '2', *arguments], capture_output=True, timeout=timeout)
        if result.returncode:
            raise BusinessError('video_conversion_failed')
    except (OSError, subprocess.TimeoutExpired) as error:
        raise BusinessError('video_conversion_failed') from error


def encode_live(source, output, gif=False, timeout=60):
    # Only local files are accepted, including by demuxers that can reference external resources.
    args = ['-protocol_whitelist', 'file,pipe', '-i', str(source)]
    if gif:
        args += ['-filter_complex_threads', '1', '-filter_complex',
                 'fps=15,scale=1280:-2:force_original_aspect_ratio=decrease,split[a][b];[a]palettegen[p];[b][p]paletteuse',
                 '-an', '-loop', '0']
    else:
        args += ['-vf', 'scale=trunc(iw/2)*2:trunc(ih/2)*2', '-c:v', 'libx264', '-threads', '2',
                 '-pix_fmt', 'yuv420p', '-c:a', 'aac', '-movflags', '+faststart']
    run_encoder([*args, str(output)], timeout)


def convert_bounded(source, output, target, timeout=60):
    """Decode in a child process so a pathological file cannot monopolize the worker."""
    import sys
    try:
        result = subprocess.run([sys.executable, '-m', 'app.infrastructure.media', str(source), str(output), target],
                                capture_output=True, timeout=timeout)
    except subprocess.TimeoutExpired as error:
        raise BusinessError('conversion_timeout') from error
    if result.returncode:
        reason = result.stdout.decode(errors='replace').strip()
        raise BusinessError(reason if reason in ('invalid_image', 'animation_too_large', 'image_too_many_pixels') else 'conversion_failed')


if __name__ == '__main__':
    import sys
    import resource
    resource.setrlimit(resource.RLIMIT_AS, (2 * 1024**3, 2 * 1024**3))
    try:
        encode_image(Path(sys.argv[1]), Path(sys.argv[2]), sys.argv[3])
    except BusinessError as error:
        print(error.value)
        sys.exit(1)
