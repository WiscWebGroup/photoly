"""Decode uploads before storing a predictable, square JPEG avatar."""
import io
import warnings
from PIL import Image, ImageOps, UnidentifiedImageError
from app.services.base import BusinessError

MAX_AVATAR_BYTES = 20 * 1024 * 1024
MAX_AVATAR_PIXELS = 40_000_000
AVATAR_SIZE = 512


def avatar_jpeg(stream):
    data = stream.read(MAX_AVATAR_BYTES + 1)
    if not data or len(data) > MAX_AVATAR_BYTES:
        raise BusinessError(-1)
    try:
        with warnings.catch_warnings():
            warnings.simplefilter("error", Image.DecompressionBombWarning)
            with Image.open(io.BytesIO(data)) as source:
                if source.width * source.height > MAX_AVATAR_PIXELS:
                    raise BusinessError(-1)
                source.seek(0)
                image = ImageOps.exif_transpose(source).convert("RGBA")
                square = ImageOps.fit(image, (AVATAR_SIZE, AVATAR_SIZE), Image.Resampling.LANCZOS)
                background = Image.new("RGB", square.size, "white")
                background.paste(square, mask=square.getchannel("A"))
                output = io.BytesIO()
                background.save(output, "JPEG", quality=92)
                output.seek(0)
                return output
    except (UnidentifiedImageError, OSError, ValueError, Image.DecompressionBombError,
            Image.DecompressionBombWarning) as error:
        raise BusinessError(-1) from error
