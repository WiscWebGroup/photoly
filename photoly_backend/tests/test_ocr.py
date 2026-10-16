import io
from pathlib import Path
import subprocess
import time
from concurrent.futures import ThreadPoolExecutor
from threading import Event
import pytest
from PIL import Image, ImageDraw
from sqlalchemy import text
from app.extensions import db
from app.entities import Photo, PhotoOCR
from app.ocr.worker import prepare, recognize, OCRError
from app.services.ocr import OCRService, processing_slot, run_worker
from app.services.base import BusinessError
from tests.openapi_assertions import assert_documented_response

PAYLOAD = {'photoId': 1, 'contentVersion': 1, 'revision': 0}


def get(world):
    return world.client.get('/photo/ocr?photoId=1', headers=world.headers)


def save(world, value='Edited 中文 日本語', **versions):
    return world.client.put('/photo/ocr', headers=world.headers, json={**PAYLOAD, 'text': value, **versions})


def run(world, **versions):
    return world.client.post('/photo/ocr/recognize', headers=world.headers, json={**PAYLOAD, **versions})


def test_persistence_revision_empty_text_and_owner_isolation(world):
    initial = get(world)
    assert initial.headers['Cache-Control'] == 'private, no-store'
    assert initial.json['t']['text'] is None
    assert initial.json['t']['revision'] == 0
    assert_documented_response('GET', '/photo/ocr', initial)
    saved = save(world)
    assert saved.json['msgCode'] == 200
    assert_documented_response('PUT', '/photo/ocr', saved)
    assert get(world).json['t']['text'] == 'Edited 中文 日本語'
    assert save(world, 'stale').json['msgCode'] == 409
    assert save(world, '', revision=1).json['t']['text'] == ''
    for method, url, kwargs in [('get', '/photo/ocr?photoId=1', {}), ('put', '/photo/ocr', {'json': {**PAYLOAD, 'text': 'attack'}}),
                                ('post', '/photo/ocr/recognize', {'json': PAYLOAD})]:
        assert getattr(world.client, method)(url, headers=world.other, **kwargs).json['msgCode'] == 404
        assert getattr(world.client, method)(url, **kwargs).json['msgCode'] == 403
    assert get(world).json['t']['text'] == ''


@pytest.mark.parametrize('value', [None, 42, 'x' * 100001, '\x00'])
def test_invalid_text(world, value):
    assert save(world, value).json['msgCode'] == 400


def test_recognition_independent_of_ai_and_metadata_changes(world, monkeypatch):
    monkeypatch.setattr('app.services.ocr.run_worker', lambda *args: {'text': 'Recognized\n文字', 'engineVersion': 'tesseract 5 test'})
    response = run(world)
    assert response.json['msgCode'] == 200
    assert_documented_response('POST', '/photo/ocr/recognize', response)
    world.client.post('/photo/update', headers=world.headers, json={'photoId': 1, 'photoName': 'renamed'})
    world.client.post('/photo/changeNamespace', headers=world.headers, data={'photoId': 1, 'nsId': 2})
    assert get(world).json['t']['text'] == 'Recognized\n文字'
    assert run(world).json['msgCode'] == 409
    assert run(world, revision=1).json['t']['revision'] == 2


def test_overwrite_and_delete_clear_text(world):
    save(world)
    assert world.client.post('/photo/replace', headers=world.headers,
                            data={'photoId': 1, 'file': (io.BytesIO(world.jpeg), 'new.jpg')}).json['msgCode'] == 200
    assert get(world).json['t']['text'] is None
    assert get(world).json['t']['contentVersion'] == 2
    assert save(world, 'old content').json['msgCode'] == 409
    assert save(world, contentVersion=2).json['msgCode'] == 200
    world.client.post('/photo/delete', headers=world.headers, data={'photoId': 1})
    with world.app.app_context():
        assert db.session.get(PhotoOCR, 1) is None


@pytest.mark.parametrize('action', ['edit', 'replace', 'delete'])
def test_late_recognition_cannot_overwrite_new_state(world, monkeypatch, action):
    ready, done = Event(), Event()
    snapshots = []
    def worker(source, directory, deadline):
        snapshots.append((source, source.read_bytes()))
        ready.set()
        assert done.wait(10)
        return {'text': 'late', 'engineVersion': 'tesseract test'}
    monkeypatch.setattr('app.services.ocr.run_worker', worker)
    def recognize_request():
        with world.app.test_client() as client:
            return client.post('/photo/ocr/recognize', headers=world.headers, json=PAYLOAD).json
    with ThreadPoolExecutor(1) as pool:
        future = pool.submit(recognize_request)
        assert ready.wait(10)
        try:
            if action == 'edit':
                assert save(world, 'new edit').json['msgCode'] == 200
            elif action == 'replace':
                world.client.post('/photo/replace', headers=world.headers,
                                  data={'photoId': 1, 'file': (io.BytesIO(world.jpeg), 'new.jpg')})
            else:
                world.client.post('/photo/delete', headers=world.headers, data={'photoId': 1})
        finally:
            done.set()
        assert future.result()['msgCode'] == (404 if action == 'delete' else 409)
    assert snapshots[0][1] == world.jpeg
    assert not snapshots[0][0].parent.exists()
    if action == 'edit':
        assert get(world).json['t']['text'] == 'new edit'


def test_busy_global_lock_and_failure_cleanup(world, monkeypatch):
    with world.app.app_context():
        with processing_slot(db.engine):
            assert run(world).json['t']['error'] == 'ocr_busy'
    paths = []
    def failed(source, directory, deadline):
        paths.append(directory)
        raise BusinessError({'error': 'ocr_timeout'}, 504)
    save(world, 'keep me')
    monkeypatch.setattr('app.services.ocr.run_worker', failed)
    assert run(world, revision=1).json['msgCode'] == 504
    assert get(world).json['t']['text'] == 'keep me'
    assert not paths[0].exists()
    with world.app.app_context(), processing_slot(db.engine):
        pass  # Failed operation released its slot.


def test_limits_invalid_format_and_missing_file(world, monkeypatch):
    assert world.client.get('/photo/ocr?photoId=2', headers=world.headers).json['t']['error'] == 'unsupported_format'
    monkeypatch.setattr('app.services.ocr.MAX_BYTES', 5)
    assert run(world).json['t']['error'] == 'image_too_large'
    with world.app.app_context():
        photo = db.session.get(Photo, 1)
        world.app.extensions['photoly'].storage.media('user-alice', photo).unlink()
    assert run(world).json['t']['error'] == 'image_missing'


def test_prepare_orientation_transparency_resize_and_first_frame(tmp_path):
    source, output = tmp_path / 'input.png', tmp_path / 'prepared.png'
    exif = Image.Exif(); exif[274] = 6
    Image.new('RGBA', (100, 50), (0, 0, 0, 0)).save(source, exif=exif)
    prepare(source, output)
    with Image.open(output) as result:
        assert result.size == (50, 100) and result.mode == 'RGB'
        assert result.getpixel((0, 0)) == (255, 255, 255)
    Image.new('RGB', (5000, 1000)).save(source)
    prepare(source, output)
    with Image.open(output) as result:
        assert result.size == (4096, 819)
    source = tmp_path / 'animated.gif'
    Image.new('RGB', (20, 20), 'red').save(source, save_all=True, append_images=[Image.new('RGB', (20, 20), 'blue')])
    prepare(source, output)
    with Image.open(output) as result:
        assert result.getpixel((0, 0)) == (255, 0, 0)


def test_worker_error_mapping_and_real_cpu_recognition(tmp_path):
    source = tmp_path / 'input.png'
    image = Image.new('RGB', (1200, 240), 'white')
    ImageDraw.Draw(image).text((40, 60), 'HELLO PHOTOLY 2026', fill='black', font_size=65)
    image.save(source)
    result = run_worker(source, tmp_path, time.monotonic() + 45)
    assert 'PHOTOLY' in result['text'] and result['engineVersion'].startswith('tesseract 5')
    source.write_bytes(b'broken')
    with pytest.raises(BusinessError) as error:
        run_worker(source, tmp_path, time.monotonic() + 45)
    assert error.value.value['error'] == 'invalid_image'


def test_missing_engine_languages_and_timeout(tmp_path, monkeypatch):
    def missing(*args, **kwargs): raise FileNotFoundError()
    monkeypatch.setattr('app.ocr.worker.subprocess.run', missing)
    with pytest.raises(OCRError, match='engine_unavailable'):
        recognize(tmp_path / 'in', tmp_path)
    monkeypatch.setattr('app.ocr.worker.subprocess.run', lambda *args, **kwargs: subprocess.CompletedProcess([], 0, 'eng\n'))
    with pytest.raises(OCRError, match='languages_unavailable'):
        recognize(tmp_path / 'in', tmp_path)


def test_whole_process_group_is_killed_on_deadline(tmp_path, monkeypatch):
    original = subprocess.Popen
    def slow(*args, **kwargs):
        return original(['python', '-c', 'import time; time.sleep(60)'], **kwargs)
    monkeypatch.setattr('app.services.ocr.subprocess.Popen', slow)
    start = time.monotonic()
    with pytest.raises(BusinessError) as error:
        run_worker(tmp_path / 'in', tmp_path, start + 0.1)
    assert error.value.code == 504 and time.monotonic() - start < 5


def test_ocr_migration_roundtrip(world, migration_engine):
    from alembic import command
    from app.infrastructure.schema import migration_config
    with migration_engine.begin() as connection:
        config = migration_config(); config.attributes['connection'] = connection
        command.downgrade(config, '0003_video_vectors')
        assert connection.execute(text("SELECT count(*) FROM photo")).scalar() == 2
        command.upgrade(config, 'head')
    assert get(world).json['t']['revision'] == 0


@pytest.mark.parametrize('name,expected', [
    ('simplified', '图片文字识别测试\n欢迎使用照片管理系统'),
    ('traditional', '圖片文字識別測試\n歡迎使用照片管理系統'),
    ('english', 'Photoly Text Recognition\nHello World 2026'),
    ('japanese', '写真の文字を読み取ります\nこんにちは 日本語のテスト'),
])
def test_real_language_fixtures(tmp_path, name, expected):
    source = Path(__file__).parent / 'fixtures' / 'ocr' / (name + '.png')
    actual = run_worker(source, tmp_path, time.monotonic() + 45)['text']
    assert ''.join(actual.split()) == ''.join(expected.split())


def test_real_mixed_language_fixture_preserves_multiple_scripts(tmp_path):
    source = Path(__file__).parent / 'fixtures' / 'ocr' / 'mixed.png'
    actual = run_worker(source, tmp_path, time.monotonic() + 45)['text']
    # Mixed Simplified/Traditional text on one line is imperfect; do not assert perfect OCR.
    assert 'Photoly' in actual and '日本語' in actual and '圖片' in actual


def test_pixel_limit_checked_before_decoding(tmp_path, monkeypatch):
    source = tmp_path / 'input.png'
    Image.new('RGB', (20, 20)).save(source)
    monkeypatch.setattr('app.ocr.worker.MAX_PIXELS', 100)
    with pytest.raises(OCRError, match='image_too_large'):
        prepare(source, tmp_path / 'out.png')


def test_real_upload_recognition_edit_and_reopen(world):
    source = Path(__file__).parent / 'fixtures' / 'ocr' / 'english.png'
    with source.open('rb') as stream:
        response = world.client.post('/photo/replace', headers=world.headers,
                                     data={'photoId': 1, 'file': (stream, 'text.png')})
    assert response.json['msgCode'] == 200
    result = run(world, contentVersion=2)
    assert result.json['msgCode'] == 200
    assert result.json['t']['text'] == 'Photoly Text Recognition\nHello World 2026'
    assert save(world, 'Corrected text', contentVersion=2, revision=1).json['msgCode'] == 200
    assert get(world).json['t']['text'] == 'Corrected text'


def test_upload_lock_wait_respects_recognition_deadline(world, monkeypatch):
    monkeypatch.setattr('app.services.ocr.TOTAL_SECONDS', 0.1)
    with world.app.app_context(), db.engine.connect() as holder:
        holder.execute(text('SELECT user_id FROM "user" WHERE user_id=2 FOR UPDATE'))
        start = time.monotonic()
        assert run(world).json['t']['error'] == 'ocr_timeout'
        assert time.monotonic() - start < 2
        holder.rollback()
