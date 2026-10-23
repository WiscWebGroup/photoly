"""Originals can be landscape pixels while their displayed orientation is portrait."""
import hashlib
import pytest
from PIL import Image
from app.entities import Photo, User
from app.extensions import db
from app.infrastructure.storage import LocalStorage
from app.services.photo import PhotoService

# Distinct quadrants catch mirrors and 180-degree rotations as well as width/height swaps.
def fixture(path, orientation):
    image = Image.new('RGB', (80, 40))
    for box, color in [((0,0,40,20),'red'),((40,0,80,20),'green'),
                       ((0,20,40,40),'blue'),((40,20,80,40),'yellow')]:
        image.paste(color, box)
    exif = Image.Exif()
    exif[274] = orientation
    image.save(path, 'PNG', exif=exif)
    return image


@pytest.mark.parametrize('orientation,operation', [
    (2, Image.Transpose.FLIP_LEFT_RIGHT), (3, Image.Transpose.ROTATE_180),
    (4, Image.Transpose.FLIP_TOP_BOTTOM), (5, Image.Transpose.TRANSPOSE),
    (6, Image.Transpose.ROTATE_270), (7, Image.Transpose.TRANSVERSE),
    (8, Image.Transpose.ROTATE_90),
])
def test_thumbnail_normalizes_all_exif_orientations(tmp_path, orientation, operation):
    source, destination = tmp_path/'original.png', tmp_path/'thumbnail.png'
    raw = fixture(source, orientation)
    storage = LocalStorage(tmp_path)
    original_hash = hashlib.sha256(source.read_bytes()).digest()
    with storage.mutation() as files:
        storage.thumbnail(source, destination, files)
    with Image.open(destination) as thumbnail:
        expected = raw.transpose(operation)
        assert thumbnail.size == expected.size
        assert thumbnail.tobytes() == expected.tobytes()
        assert thumbnail.getexif().get(274, 1) == 1
    assert hashlib.sha256(source.read_bytes()).digest() == original_hash


@pytest.mark.parametrize('orientation', [6,8])
def test_upload_preparation_orients_portrait_before_resizing(world, tmp_path, orientation):
    source = tmp_path/'original.png'
    fixture(source, orientation)
    with world.app.app_context():
        prepared = PhotoService(db.session,world.app.extensions['photoly']).prepare(source,tmp_path)
    with Image.open(prepared['thumbnail']) as thumbnail:
        assert thumbnail.size == (40,80)
        assert thumbnail.getexif().get(274,1) == 1


def test_repair_legacy_thumbnail_preserves_original_and_updates_usage(world):
    with world.app.app_context():
        storage = world.app.extensions['photoly'].storage
        photo, user = db.session.get(Photo,1), db.session.get(User,2)
        original, thumbnail = storage.media(user.uuid,photo), storage.media(user.uuid,photo,True)
        # Deliberately generate the old, unrotated preview without the EXIF marker.
        image = Image.new('RGB',(800,400),'coral')
        exif = Image.Exif(); exif[274]=8
        image.save(original,'JPEG',exif=exif)
        image.resize((400,200)).save(thumbnail,'JPEG')
        user.used_bytes = storage.usage(user.uuid)[0]
        db.session.commit()
        original_bytes, before = original.read_bytes(), thumbnail.read_bytes()
        before_usage = user.used_bytes
    runner = world.app.test_cli_runner()
    audit = runner.invoke(args=['repair-thumbnail-orientation'])
    assert audit.exit_code == 0, audit.output
    assert 'rebuilt: 0' in audit.output
    assert thumbnail.read_bytes() == before
    repair = runner.invoke(args=['repair-thumbnail-orientation','--apply'])
    assert repair.exit_code == 0, repair.output
    assert 'rebuilt: 1' in repair.output
    assert original.read_bytes() == original_bytes
    with Image.open(thumbnail) as image:
        assert image.size == (200,400)
        assert image.getexif().get(274,1) == 1
    with world.app.app_context():
        user=db.session.get(User,2)
        assert user.used_bytes == before_usage + thumbnail.stat().st_size - len(before)
    again = runner.invoke(args=['repair-thumbnail-orientation','--apply'])
    assert again.exit_code == 0
    with Image.open(thumbnail) as image:
        assert image.size == (200,400)  # Never double-rotate an existing preview.
