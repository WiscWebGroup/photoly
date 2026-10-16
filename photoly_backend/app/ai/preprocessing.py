import hashlib
import io
import warnings
from PIL import Image, ImageOps, UnidentifiedImageError
from .types import AIError

MAX_BYTES = 20 * 1024 * 1024
MAX_PIXELS = 40_000_000
# JPEG files with an MPF container are reported as MPO by Pillow.
# They follow the same first-frame policy as other multi-frame images.
FORMATS = {"MPO", "JPEG", "PNG", "WEBP", "BMP", "TIFF", "GIF", "ICO"}
EXTENSIONS = {"jpg", "jpeg", "jfif", "png", "webp", "bmp", "tif", "tiff", "gif", "ico"}


def read_bounded(stream):
    data = stream.read(MAX_BYTES + 1)
    if len(data) > MAX_BYTES:
        raise AIError("image_too_large", 413)
    if not data:
        raise AIError("empty_image")
    return data


def decode_image(data):
    if len(data) > MAX_BYTES:
        raise AIError("image_too_large", 413)
    try:
        with warnings.catch_warnings():
            warnings.simplefilter("error", Image.DecompressionBombWarning)
            with Image.open(io.BytesIO(data)) as source:
                if source.format not in FORMATS:
                    raise AIError("unsupported_image", 415)
                if source.width * source.height > MAX_PIXELS:
                    raise AIError("image_too_many_pixels", 413)
                source.seek(0)
                image = ImageOps.exif_transpose(source).convert("RGBA")
                background = Image.new("RGBA", image.size, "white")
                background.alpha_composite(image)
                return background.convert("RGB")
    except (UnidentifiedImageError, OSError, ValueError, Image.DecompressionBombError, Image.DecompressionBombWarning) as error:
        raise AIError("invalid_image", 415) from error


def sha256(data):
    return hashlib.sha256(data).hexdigest()
