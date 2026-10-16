"""Disposable process bounds both Pillow decoding and Tesseract execution."""
import json
import os
from pathlib import Path
import subprocess
import sys
import warnings
from PIL import Image, ImageOps, UnidentifiedImageError
from . import LANGUAGES, MAX_PIXELS, MAX_TEXT, ENGINE_SECONDS, FORMATS


class OCRError(Exception):
    pass


def prepare(source, destination):
    with warnings.catch_warnings():
        warnings.simplefilter('error', Image.DecompressionBombWarning)
        with Image.open(source) as image:
            if image.format not in FORMATS:
                raise OCRError('unsupported_format')
            if image.width * image.height > MAX_PIXELS:
                raise OCRError('image_too_large')
            image.seek(0)
            oriented = ImageOps.exif_transpose(image)
            oriented.thumbnail((4096, 4096), Image.Resampling.LANCZOS)
            rgba = oriented.convert('RGBA')
            white = Image.new('RGBA', rgba.size, 'white')
            white.alpha_composite(rgba)
            white.convert('RGB').save(destination, 'PNG')


def recognize(source, directory):
    env = {**os.environ, 'OMP_THREAD_LIMIT': '1'}
    try:
        version = subprocess.run(['tesseract', '--version'], capture_output=True, text=True,
                                 timeout=3, check=True, env=env).stdout.splitlines()[0]
        languages = subprocess.run(['tesseract', '--list-langs'], capture_output=True, text=True,
                                   timeout=3, check=True, env=env).stdout.splitlines()
    except FileNotFoundError as error:
        raise OCRError('engine_unavailable') from error
    if not set(LANGUAGES.split('+')).issubset({line.strip() for line in languages}):
        raise OCRError('languages_unavailable')
    prepared = directory / 'prepared.png'
    prepare(source, prepared)
    output = directory / 'recognized'
    subprocess.run(['tesseract', str(prepared), str(output), '-l', LANGUAGES,
                    '--oem', '1', '--psm', '3'], capture_output=True, timeout=ENGINE_SECONDS,
                   check=True, env=env)
    # Bound the result even for pathological inputs. Never silently truncate saved text.
    with output.with_suffix('.txt').open(encoding='utf-8') as stream:
        text = stream.read(MAX_TEXT + 1).replace('\r\n', '\n').rstrip('\n\f')
    if len(text) > MAX_TEXT:
        raise OCRError('text_too_long')
    return {'text': text, 'engineVersion': version}


def main():
    try:
        result = recognize(Path(sys.argv[1]), Path(sys.argv[2]))
    except OCRError as error:
        result = {'error': str(error)}
    except subprocess.TimeoutExpired:
        result = {'error': 'ocr_timeout'}
    except (Image.DecompressionBombError, Image.DecompressionBombWarning):
        result = {'error': 'image_too_large'}
    except (UnidentifiedImageError, OSError, ValueError):
        result = {'error': 'invalid_image'}
    except subprocess.CalledProcessError:
        result = {'error': 'recognition_failed'}
    print(json.dumps(result, ensure_ascii=False))


if __name__ == '__main__':
    main()
