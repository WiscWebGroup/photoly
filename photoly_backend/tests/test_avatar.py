import io
import pytest
from PIL import Image
from app.services.avatar import avatar_jpeg
from app.services.base import BusinessError


def image_bytes(size=(800, 400), mode='RGB', color='coral', **kwargs):
    output = io.BytesIO()
    Image.new(mode, size, color).save(output, 'PNG', **kwargs)
    return output.getvalue()


@pytest.mark.parametrize('size', [(800, 400), (400, 800), (1, 1), (512, 512)])
def test_square_jpeg(size):
    with Image.open(avatar_jpeg(io.BytesIO(image_bytes(size)))) as saved:
        assert saved.size == (512, 512)
        assert saved.format == 'JPEG'
        assert saved.mode == 'RGB'
        # Corners retain image content, rather than a circular mask.
        assert saved.getpixel((0, 0))[0] > 240
        assert saved.getpixel((0, 0))[1] < 150


def test_transparency_is_white_and_orientation_is_applied():
    with Image.open(avatar_jpeg(io.BytesIO(image_bytes(mode='RGBA', color=(0, 0, 0, 0))))) as saved:
        assert saved.getpixel((256, 256)) == (255, 255, 255)
    source = Image.new('RGB', (80, 80), 'red')
    source.paste('blue', (0, 40, 80, 80))
    exif = Image.Exif()
    exif[274] = 6
    output = io.BytesIO()
    source.save(output, 'PNG', exif=exif)
    with Image.open(avatar_jpeg(io.BytesIO(output.getvalue()))) as saved:
        assert saved.getpixel((50, 256))[2] > 240
        assert saved.getpixel((460, 256))[0] > 240


def test_rejects_invalid_and_oversized_uploads(monkeypatch):
    for data in [b'', b'not an image']:
        with pytest.raises(BusinessError):
            avatar_jpeg(io.BytesIO(data))
    monkeypatch.setattr('app.services.avatar.MAX_AVATAR_PIXELS', 10)
    with pytest.raises(BusinessError):
        avatar_jpeg(io.BytesIO(image_bytes()))
    monkeypatch.setattr('app.services.avatar.MAX_AVATAR_BYTES', 10)
    with pytest.raises(BusinessError):
        avatar_jpeg(io.BytesIO(b'x' * 11))


def test_upload_normalizes_and_failed_upload_keeps_previous_avatar(world):
    def upload(data):
        return world.client.post('/user/updateAvatar', headers=world.headers,
                                 data={'file': (io.BytesIO(data), 'avatar.png')})
    assert upload(image_bytes()).json['msgCode'] == 200
    before = world.client.get('/user/getAvatar/token-alice')
    assert before.mimetype == 'image/jpeg'
    with Image.open(io.BytesIO(before.data)) as saved:
        assert saved.size == (512, 512)
        assert saved.format == 'JPEG'
    assert upload(b'bad image').json['msgCode'] == 400
    assert world.client.get('/user/getAvatar/token-alice').data == before.data
