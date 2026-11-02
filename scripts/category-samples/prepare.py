#!/usr/bin/env python3
"""Download attributed samples and prepare 30 logical media items; no library writes.
Requires ffmpeg and exiftool, or --docker-image with those tools installed.
"""
import argparse
import hashlib
import json
import pathlib
import shutil
import struct
import subprocess
import urllib.request
import urllib.error
import time
import uuid

HERE = pathlib.Path(__file__).resolve().parent
LIMIT = 300 * 1024 * 1024

def sha(path):
    h = hashlib.sha256()
    with open(path, 'rb') as f:
        for b in iter(lambda: f.read(1024 * 1024), b''): h.update(b)
    return h.hexdigest()

def maker_note(path, *, identifier=None):
    """Explicitly synthetic Apple MakerNote for parser/pairing fixtures."""
    if identifier:
        payload = identifier.encode() + b'\0'
        entry = struct.pack('>HHII', 0x11, 2, len(payload), 32)
    else:
        payload = b''
        entry = struct.pack('>HHII', 0x14, 9, 1, 2)
    mn = b'Apple iOS\0\0\0MM' + struct.pack('>H', 1) + entry + b'\0' * 4 + payload
    tiff = (b'II*\0' + struct.pack('<I', 8) + struct.pack('<H', 1)
            + struct.pack('<HHII', 0x8769, 4, 1, 26) + b'\0' * 4
            + struct.pack('<H', 1) + struct.pack('<HHII', 0x927c, 7, len(mn), 44)
            + b'\0' * 4 + mn)
    data = path.read_bytes()
    if data[:2] != b'\xff\xd8': raise ValueError('JPEG required')
    exif = b'Exif\0\0' + tiff
    path.write_bytes(data[:2] + b'\xff\xe1' + struct.pack('>H', len(exif)+2) + exif + data[2:])

def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--output', required=True, type=pathlib.Path)
    ap.add_argument('--docker-image', help='Use media tools from this existing Docker image')
    ap.add_argument('--record-source-hashes', action='store_true', help='Maintainer: pin verified downloads in sources.json')
    args = ap.parse_args()
    root = args.output.resolve(); root.mkdir(parents=True, exist_ok=True)
    cache = root / 'sources'; cache.mkdir(exist_ok=True)
    media = root / 'media'; media.mkdir(exist_ok=True)
    manifest = json.loads((HERE / 'sources.json').read_text())
    def tool(name, *argv):
        converted = [str(a).replace(str(root), '/samples') if args.docker_image else str(a) for a in argv]
        cmd = (['docker', 'run', '--rm', '-v', f'{root}:/samples', '--entrypoint', name, args.docker_image]
               if args.docker_image else [name])
        subprocess.run(cmd + converted, check=True, stdout=subprocess.DEVNULL)
    def ff(*argv): tool('ffmpeg', '-nostdin', '-v', 'error', '-y', '-threads', '2', *argv)
    sources = {}
    for src in manifest['sources']:
        ext = pathlib.Path(src['downloadUrl']).suffix
        dest = cache / (src['id'] + ext)
        if not dest.exists():
            req = urllib.request.Request(src['downloadUrl'], headers={'User-Agent': 'PhotolySamples/1.0 (attributed open-source test fixtures)'})
            temp = dest.with_suffix('.part')
            try:
                time.sleep(2)
                for attempt in range(4):
                    try:
                        response = urllib.request.urlopen(req, timeout=60)
                        break
                    except urllib.error.HTTPError as e:
                        if e.code not in (429, 503) or attempt == 3: raise
                        delay = max(20 * (attempt + 1), int(e.headers.get('Retry-After', '0')))
                        print(f'Rate limited; retrying in {delay}s', flush=True)
                        time.sleep(delay)
                with response as r, temp.open('wb') as out:
                    used = 0
                    while chunk := r.read(1024 * 1024):
                        used += len(chunk)
                        if used > LIMIT: raise ValueError('source exceeds sample budget')
                        out.write(chunk)
                temp.replace(dest)
            finally: temp.unlink(missing_ok=True)
        digest = sha(dest)
        if src.get('sha256') and src['sha256'] != digest: raise ValueError(f'Changed source: {src["id"]}')
        src['sha256'] = digest
        sources[src['id']] = dest
        print('Source verified:', src['id'], flush=True)
    if args.record_source_hashes:
        (HERE / 'sources.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n')
    entries = []
    def add(category, source, files, changes, synthetic=False):
        entries.append(dict(id=f'{category}-{1+sum(e["category"]==category for e in entries)}', category=category,
                            sourceId=source, synthetic=synthetic, modifications=changes,
                            files=[dict(name=p.name, sha256=sha(p), size=p.stat().st_size) for p in files]))
    for i in range(1, 6):
        key = f'panorama-{i}'; out = media / f'Panorama_{i:02}.jpg'
        ff('-i', sources[key], '-frames:v', '1', '-vf', 'scale=min(2400\\,iw):-1', out)
        if i == 5: tool('exiftool', '-overwrite_original', '-ProjectionType=equirectangular', '-UsePanoramaViewer=True', out)
        add('panorama', key, [out], 'Resized JPEG; sample 5 retains a 2:1 360-degree projection and declares GPano metadata.')
        key = f'screenshot-{i}'; out = media / (f'Screenshot_Sample_{i:02}.png' if i <= 3 else f'IMG_Sample_{i:02}.png')
        ff('-i', sources[key], '-frames:v', '1', out)
        if i > 3: tool('exiftool', '-overwrite_original', '-UserComment=Screenshot', out)
        add('screenshot', key, [out], 'PNG conversion; filename signal for 1–3, EXIF UserComment for 4–5.')
        key = f'portrait-{i}'; out = media / f'Portrait_{i:02}_SYNTHETIC_METADATA_TEST.jpg'
        ff('-i', sources[key], '-frames:v', '1', '-vf', 'scale=min(1200\\,iw):-1', '-map_metadata', '-1', out)
        maker_note(out)
        add('portrait', key, [out], 'Resized JPEG; synthetic Apple ImageCaptureType=2. NOT an original camera portrait-mode capture or depth fixture.', True)
    for i, at in enumerate([2, 6, 10, 14, 18], 1):
        key = 'movie-1'; source = sources[key]
        video = media / f'Video_{i:02}_Blender_Trailer.mp4'
        ff('-ss', str(at), '-i', source, '-t', '3', '-an', '-vf', 'scale=960:-2', '-c:v', 'libx264', '-pix_fmt', 'yuv420p', '-movflags', '+faststart', video)
        add('video', key, [video], f'Three-second silent excerpt at {at}s; resized to 960px.')
        still = media / f'Live_{i:02}_SYNTHETIC_PAIR.jpg'; motion = still.with_suffix('.mov')
        ident = str(uuid.uuid5(uuid.NAMESPACE_URL, f'photoly-category-live-sample-v1-{i}'))
        ff('-ss', str(at + 1), '-i', source, '-frames:v', '1', '-vf', 'scale=960:-2', still)
        maker_note(still, identifier=ident)
        ff('-ss', str(at), '-i', source, '-t', '3', '-an', '-vf', 'scale=960:-2', '-c:v', 'libx264', '-pix_fmt', 'yuv420p', '-metadata', f'com.apple.quicktime.content.identifier={ident}', '-movflags', '+use_metadata_tags', motion)
        add('live', key, [still, motion], f'Synthetic Photoly still/MOV pair from trailer at {at}s with matching content identifiers. Not an iPhone original; native Apple timed still-image track is not simulated.', True)
        ext = ['gif', 'png', 'webp', 'gif', 'png'][i-1]; animated = media / f'Animation_{i:02}.{ext}'
        opts = ['-f', 'apng', '-plays', '0'] if ext == 'png' else ['-loop', '0']
        ff('-ss', str(at), '-i', source, '-t', '2', '-an', '-vf', 'fps=10,scale=480:-2', *opts, animated)
        add('animated', key, [animated], f'Two-second animation at {at}s, 10 fps, resized to 480px; {"APNG" if ext == "png" else ext.upper()}.')
    size = sum(p.stat().st_size for p in root.rglob('*') if p.is_file())
    if size > LIMIT: raise ValueError(f'Sample workspace exceeds 300 MiB: {size}')
    result = dict(version=1, sources=manifest['sources'], items=entries)
    (root / 'manifest.json').write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
    print(f'Prepared {len(entries)} logical media; {size/1024/1024:.1f} MiB including sources')

if __name__ == '__main__': main()
