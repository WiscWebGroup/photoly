from datetime import datetime, timedelta, timezone
from types import SimpleNamespace
import hashlib
import json
from pathlib import Path
import pytest
from sqlalchemy import select, func
from app.extensions import db
from app.entities import User, Photo, Tag, Gallery, GalleryPhoto, TagPhoto
from app.entities.smart import SmartPreference, SmartMedia, SmartItem, SmartMember
from app.ai.state import control, settings
from app.smart.metadata import device_name, capture_time, coordinates
from app.smart.geography import Gazetteer, DATA
from app.services.smart import SmartService, eligibility, manual_member, dismiss, rename
from app.smart.organize import devices, anchor_step, missing_step, item_step, member_step


def test_device_and_time_rules():
    assert device_name('Apple', 'iPhone 13') == 'iPhone 13'
    assert device_name('SONY', 'Sony Sony ILCE-7M4') == 'Sony ILCE-7M4'
    assert device_name('Tamron', None) is None
    assert device_name('', '  28-200mm   F2.8-5.6  ') == '28-200mm F2.8-5.6'
    assert coordinates({'GPSLatitude': 'nan', 'GPSLongitude': 3}) == (None, None)
    assert coordinates({'GPSLatitude': 91, 'GPSLongitude': 0}) == (None, None)
    assert capture_time({'ModifyDate': '2026:01:01 10:00:00'}) == (None, {})
    stamp, info = capture_time({'DateTimeOriginal': '2026:10:01 10:00:00', 'OffsetTimeOriginal': '+08:00', 'CreateDate': '2025:01:01 00:00:00'})
    assert stamp == datetime(2026, 10, 1, 2, tzinfo=timezone.utc)
    assert info['timezoneSource'] == 'explicit'
    stamp, info = capture_time({'CreateDate': '2026:10:01 10:00:00'}, 'Asia/Hong_Kong')
    assert stamp.hour == 2 and info['timezoneSource'] == 'city'
    assert capture_time({'CreateDate': '0000:00:00 00:00:00'}) == (None, {})


def test_offline_city_snapshot():
    manifest = json.loads((DATA.parent / 'manifest.json').read_text())
    assert hashlib.sha256(DATA.read_bytes()).hexdigest() == manifest['databaseSha256']
    city = Gazetteer()
    assert city.nearest(22.55, 114.06)['name'] == 'Shenzhen'
    assert city.nearest(22.28, 114.16)['timezone'] == 'Asia/Hong_Kong'
    assert city.nearest(0, -140) is None


def enable(session, uid=2):
    session.get(User, uid).ai_search_enabled = True
    control(session).desired = {**settings(session), 'enabled': True, 'smart_tag_enabled': True, 'scheduler': 'running'}
    pref = SmartPreference(user_id=uid, enabled=True, revision=1, completed_revision=0)
    session.add(pref)
    session.flush()
    return pref


def facts(session, index, hour, gps=True, city='Hong Kong'):
    photo = Photo(photo_name=f'photo-{index}', user_id=2, format='jpg', content_version=1)
    session.add(photo)
    session.flush()
    taken = datetime(2026, 10, 1, tzinfo=timezone.utc) + timedelta(hours=hour)
    media = SmartMedia(photo_id=photo.photo_id, user_id=2, content_version=1, captured_at=taken,
        latitude=22.3 if gps else None, longitude=114.1 if gps else None,
        facts={'camera': 'iPhone 13', 'localDate': taken.date().isoformat(), 'timezoneSource': 'explicit',
               'city': {'name': city, 'timezone': 'Asia/Hong_Kong'} if gps else None})
    session.add(media)
    session.flush()
    return media


def organize(session, pref, run):
    cp = {'phase': 'anchors', 'run': run, 'revision': pref.revision}
    steps = {'anchors': anchor_step, 'missing': missing_step, 'items': item_step, 'members': member_step}
    for _ in range(500):
        steps[cp['phase']](session, pref, cp)
        session.flush()
        if not cp: return
    pytest.fail('Organization did not terminate')


def test_gates_and_user_isolation(app, world):
    with app.app_context():
        pref = enable(db.session)
        assert eligibility(db.session, 2, True) is None
        for global_ai, smart, grant, opt in __import__('itertools').product((False, True), repeat=4):
            control(db.session).desired = {**settings(db.session), 'enabled': global_ai, 'smart_tag_enabled': smart}
            db.session.get(User, 2).ai_search_enabled = grant
            pref.enabled = opt
            assert (eligibility(db.session, 2) is None) == all((global_ai, smart, grant, opt))
        assert SmartService(db.session).dismissed(3)['items'] == []
        with pytest.raises(Exception, match='smart_item_not_found'):
            SmartService(db.session).restore(3, 999)


def test_device_tags_do_not_merge_manual_and_preserve_choices(app, world):
    with app.app_context():
        enable(db.session)
        media = facts(db.session, 0, 0)
        db.session.add(Tag(user_id=2, tag_name='iPhone 13'))
        devices(db.session, media)
        db.session.flush()
        item = db.session.scalar(select(SmartItem))
        assert db.session.scalar(select(func.count()).select_from(Tag).where(Tag.tag_name == 'iPhone 13')) == 2
        tag = db.session.get(Tag, item.tag_id)
        tag.tag_name = 'My phone'
        rename(db.session, Tag, tag)
        manual_member(db.session, Tag, tag.tag_id, media.photo_id, False)
        devices(db.session, media)
        assert tag.tag_name == 'My phone'
        assert not db.session.scalar(select(TagPhoto).where(TagPhoto.tag_id == tag.tag_id))
        dismiss(db.session, Tag, tag)
        db.session.delete(tag)
        db.session.flush()
        devices(db.session, media)
        assert item.tag_id is None and item.suppressed


def test_trips_continuity_gap_minimum_and_dismissal(app, world):
    with app.app_context():
        pref = enable(db.session)
        media = [facts(db.session, i, i*12, city='Hong Kong' if i < 3 else 'Shenzhen') for i in range(5)]
        organize(db.session, pref, 'run-1')
        item = db.session.scalar(select(SmartItem).where(SmartItem.source == 'trip'))
        assert item.ga_id and 'Hong Kong and Shenzhen' in item.name
        assert db.session.scalar(select(func.count()).select_from(GalleryPhoto).where(GalleryPhoto.ga_id == item.ga_id)) == 5
        gallery = db.session.get(Gallery, item.ga_id)
        dismiss(db.session, Gallery, gallery)
        db.session.delete(gallery)
        db.session.flush()
        # Late upload changes the group's start, but overlap keeps the dismissal.
        facts(db.session, 6, -6)
        organize(db.session, pref, 'run-2')
        assert all(x.ga_id is None for x in db.session.scalars(select(SmartItem)))
        SmartService(db.session).restore(2, item.item_id)
        organize(db.session, pref, 'run-3')
        assert item.ga_id
        for i in range(5): facts(db.session, i+10, 120+i)
        organize(db.session, pref, 'run-4')
        assert db.session.scalar(select(func.count()).select_from(SmartItem).where(SmartItem.ga_id.is_not(None))) == 2


@pytest.mark.parametrize('provider', ['siglip2', 'qwen3_vl'])
def test_video_missing_gps_reuses_document_vectors_and_threshold(app, world, provider):
    import numpy as np
    from app.services.ai_admin import AIAdmin
    from app.entities.ai import EmbeddingProfile
    from tests.test_ai import ready
    from tests.test_video_vectors import video_ready
    with app.app_context():
        pref = enable(db.session)
        pid = AIAdmin(db.session).create_profile(provider)['profileId']
        pref.profile_id = pid
        dim = db.session.get(EmbeddingProfile, pid).dimension
        unit = np.zeros(dim, dtype=np.float32); unit[0] = 1
        other = np.zeros(dim, dtype=np.float32); other[1] = 1
        anchors = [facts(db.session, i, i) for i in range(2)]
        for media in anchors: ready(db.session, pid, media.photo_id, unit)
        matches = [facts(db.session, i+10, 1, gps=False) for i in range(3)]
        for media in matches:
            db.session.get(Photo, media.photo_id).format = 'mp4'
            video_ready(db.session, pid, media.photo_id, [other, unit, other])
        bad = facts(db.session, 50, 1, gps=False)
        ready(db.session, pid, bad.photo_id, other)
        distant = facts(db.session, 51, 48, gps=False)
        ready(db.session, pid, distant.photo_id, unit)
        organize(db.session, pref, 'video-run')
        gallery = db.session.scalar(select(SmartItem).where(SmartItem.ga_id.is_not(None)))
        assert gallery and gallery.basis['gpsCount'] == 2 and gallery.basis['mediaCount'] == 5
        ids = set(db.session.scalars(select(GalleryPhoto.photo_id).where(GalleryPhoto.ga_id == gallery.ga_id)))
        assert ids == {x.photo_id for x in anchors + matches}
        # No GPS means no standalone gallery, even with identical vectors.
        for media in anchors: media.latitude = media.longitude = None
        organize(db.session, pref, 'no-gps')
        assert not db.session.scalar(select(SmartItem).where(SmartItem.ga_id.is_not(None)))


def test_api_contract_opt_in_restore_disabled_and_ownership(world):
    from tests.openapi_assertions import assert_documented_response
    client = world.client
    response = client.get('/ai/smart/settings', headers=world.headers)
    assert response.json['t']['enabled'] is False
    assert_documented_response('GET', '/ai/smart/settings', response)
    assert client.patch('/ai/smart/settings', json={'enabled': 'true'}, headers=world.headers).status_code == 400
    for method, path, body in [('get','/ai/smart/status',None), ('post','/ai/smart/refresh',None), ('get','/ai/smart/dismissed',None), ('patch','/ai/smart/settings',{'enabled':True})]:
        assert getattr(client, method)(path, json=body).status_code == 401
        response = getattr(client, method)(path, json=body, headers=world.headers)
        assert response.status_code == 200
        assert_documented_response(method.upper(), path, response)
    with world.app.app_context():
        media = facts(db.session, 1, 0)
        devices(db.session, media)
        item = db.session.scalar(select(SmartItem))
        tag_id, item_id = item.tag_id, item.item_id
        db.session.commit()
    legacy = client.get('/tag/getAll', headers=world.headers).json['t']
    assert all('smart' not in tag for tag in legacy)
    response = client.get('/tag/getAll?includeSmart=true', headers=world.headers)
    assert_documented_response('GET', '/tag/getAll', response)
    assert any(tag['smart'] and tag['smart']['source'] == 'camera' for tag in response.json['t'])
    assert client.post('/tag/delete', query_string={'tagId': tag_id}, headers=world.headers).json['msgCode'] == 200
    assert client.post(f'/ai/smart/items/{item_id}/restore', headers=world.other).status_code == 404
    assert client.get('/ai/smart/dismissed', headers=world.other).json['t']['items'] == []
    # Restore remains useful when the administrator has disabled automatic processing.
    assert client.post(f'/ai/smart/items/{item_id}/restore', headers=world.headers).status_code == 200
    assert any(tag['smart'] for tag in client.get('/tag/getAll?includeSmart=true', headers=world.headers).json['t'])


def test_replacement_and_delete_invalidate_automatic_links_but_keep_pins(app, world):
    from sqlalchemy import update, delete
    with app.app_context():
        enable(db.session)
        media = facts(db.session, 1, 0)
        devices(db.session, media)
        item = db.session.scalar(select(SmartItem))
        photo_id = media.photo_id
        db.session.execute(update(Photo).where(Photo.photo_id == photo_id).values(content_version=2))
        db.session.expire_all()
        assert db.session.get(SmartMedia, photo_id) is None
        assert not db.session.scalar(select(TagPhoto).where(TagPhoto.photo_id == photo_id))
        manual_member(db.session, Tag, item.tag_id, photo_id, True)
        from app.services.smart import sync_link
        member = db.session.get(SmartMember, (item.item_id, photo_id), populate_existing=True)
        sync_link(db.session, item, member)
        db.session.execute(update(Photo).where(Photo.photo_id == photo_id).values(content_version=3))
        assert db.session.scalar(select(TagPhoto).where(TagPhoto.photo_id == photo_id))
        db.session.execute(delete(Photo).where(Photo.photo_id == photo_id))
        assert not db.session.scalar(select(SmartMember).where(SmartMember.photo_id == photo_id))


def test_scheduler_batches_offline_and_fences_changes(world, monkeypatch):
    from sqlalchemy.orm import sessionmaker
    from app.scheduler.photo_embedding_scheduler import PhotoEmbeddingScheduler
    from app.scheduler import job_repository as jobs
    from app.entities.ai import AIJob
    from app.ai.types import AIError
    from app.ai.state import utcnow
    with world.app.app_context():
        enable(db.session)
        for i in range(28):
            db.session.add(Photo(user_id=2, photo_name=f'media-{i}', format='jpg', content_version=1))
        db.session.commit()
        sessions = sessionmaker(db.engine, expire_on_commit=False)
    class Offline:
        def status(self): raise AIError('runtime_offline', 503)
        def encode(self, *a, **k): pytest.fail('Smart Tag must not run inference')
    scheduler = PhotoEmbeddingScheduler(sessions, Offline(), world.app.extensions['photoly'].storage)
    monkeypatch.setattr('app.scheduler.smart_tag_scheduler.system_usage', lambda **kw: {'sampled_at': utcnow().isoformat(), 'cpu_percent': 0, 'memory_available': 16*1024**3, 'disk_free': 20*1024**3})
    monkeypatch.setattr(scheduler.smart, 'read_media', lambda *a: {'facts': {'camera': 'iPhone 13'}, 'captured_at': None, 'latitude': None, 'longitude': None})
    scheduler.tick()
    with sessions() as s:
        assert s.scalar(select(func.count()).select_from(SmartMedia)) == 25
        assert s.scalar(select(func.count()).select_from(SmartItem)) == 1
    scheduler.tick()
    with sessions() as s:
        assert s.scalar(select(func.count()).select_from(SmartMedia)) == 30
        assert not s.get(SmartPreference, 2).checkpoint
    # Revoke permission while EXIF is being read. The late result must not be saved.
    with sessions.begin() as s:
        photo = Photo(user_id=2, photo_name='late', format='jpg', content_version=1)
        s.add(photo); s.flush(); pid = photo.photo_id
    scheduler.smart.next_check = 0
    def revoke(*args):
        with sessions.begin() as s: s.get(User, 2).ai_search_enabled = False
        return {'facts': {'camera': 'Late camera'}}
    monkeypatch.setattr(scheduler.smart, 'read_media', revoke)
    scheduler.tick()
    with sessions() as s: assert s.get(SmartMedia, pid) is None


def test_real_exif_reader_pins_original_file(world, tmp_path):
    import subprocess
    from sqlalchemy.orm import sessionmaker
    from app.scheduler.photo_embedding_scheduler import PhotoEmbeddingScheduler
    with world.app.app_context():
        scheduler = PhotoEmbeddingScheduler(sessionmaker(db.engine, expire_on_commit=False), storage=world.app.extensions['photoly'].storage)
        photo, user = db.session.get(Photo, 1), db.session.get(User, 2)
        path = scheduler.storage.media(user.uuid, photo)
        subprocess.run(['exiftool', '-overwrite_original', '-Make=Apple', '-Model=iPhone 13', '-DateTimeOriginal=2026:10:01 10:00:00', str(path)], check=True, capture_output=True)
        values = scheduler.smart.read_media(photo, user, 5)
        assert values['facts']['camera'] == 'iPhone 13'
        assert values['captured_at'] is not None


def test_gap_and_span_boundaries_and_suppressed_split_merge(app, world):
    with app.app_context():
        pref = enable(db.session)
        # Exactly 36 hours remains continuous, but no group extends past 14 days.
        media = [facts(db.session, i, i*24) for i in range(20)]
        organize(db.session, pref, 'initial')
        items = db.session.scalars(select(SmartItem).where(SmartItem.ga_id.is_not(None)).order_by(SmartItem.item_id)).all()
        assert len(items) == 2
        assert [x.basis['gpsCount'] for x in items] == [15, 5]
        first = items[0]
        gallery = db.session.get(Gallery, first.ga_id)
        gallery.ga_name, gallery.cover_id, gallery.cover_color = 'Our trip', 5, '#123456'
        rename(db.session, Gallery, gallery)
        manual_member(db.session, Gallery, gallery.ga_id, media[1].photo_id, False)
        dismiss(db.session, Gallery, gallery)
        db.session.delete(gallery); db.session.flush()
        # Remove bridging metadata: the two new groups inherit suppression and edits.
        for m in media[5:8]: m.captured_at = None
        organize(db.session, pref, 'split')
        inherited = db.session.scalars(select(SmartItem).where(SmartItem.custom_name == 'Our trip')).all()
        assert len(inherited) >= 2 and all(x.suppressed and x.ga_id is None for x in inherited)
        assert all(x.cover['cover_id'] == 5 for x in inherited)
        SmartService(db.session).restore(2, first.item_id)
        organize(db.session, pref, 'restored')
        for item in db.session.scalars(select(SmartItem).where(SmartItem.custom_name == 'Our trip', SmartItem.ga_id.is_not(None))):
            assert not db.session.scalar(select(GalleryPhoto).where(GalleryPhoto.ga_id == item.ga_id, GalleryPhoto.photo_id == media[1].photo_id))
        for i in range(5, 8): media[i].captured_at = datetime(2026, 10, 1, tzinfo=timezone.utc)+timedelta(hours=i*24)
        organize(db.session, pref, 'merged')
        assert db.session.scalar(select(func.count()).select_from(SmartItem).where(SmartItem.ga_id.is_not(None))) == 2


@pytest.mark.parametrize('gap,expected', [(36, 1), (36.001, 2)])
def test_exact_gap_boundary(app, world, gap, expected):
    with app.app_context():
        pref = enable(db.session)
        for i in range(5): facts(db.session, i, i)
        for i in range(5): facts(db.session, i+5, 4+gap+i)
        organize(db.session, pref, 'gap')
        assert db.session.scalar(select(func.count()).select_from(SmartItem).where(SmartItem.ga_id.is_not(None))) == expected


def test_missing_gps_requires_margin_and_limits_twenty_anchors(app, world, monkeypatch):
    import numpy as np
    with app.app_context():
        pref = enable(db.session)
        stamp = datetime(2026,10,1,tzinfo=timezone.utc)
        a = SmartItem(user_id=2, stable_key='a', source='trip', name='A', run='margin', basis={'start':stamp.isoformat(),'end':stamp.isoformat()})
        b = SmartItem(user_id=2, stable_key='b', source='trip', name='B', run='margin', basis={'start':stamp.isoformat(),'end':stamp.isoformat()})
        db.session.add_all([a,b]); db.session.flush()
        for i in range(30):
            m = facts(db.session, i, 0)
            db.session.add(SmartMember(item_id=a.item_id if i%2 else b.item_id, photo_id=m.photo_id, content_version=1, automatic=True, anchor=True, run='margin'))
        candidate = facts(db.session, 50, 0, gps=False)
        calls = []
        def vector(s, pid, profile):
            calls.append(pid)
            return np.array([[1., 0.]])
        monkeypatch.setattr('app.smart.organize.vectors', vector)
        cp = {'run':'margin', 'cursor':0, 'phase':'missing'}
        missing_step(db.session, pref, cp)
        assert len(calls) == 21  # One source plus at most twenty anchored media.
        assert not db.session.scalar(select(SmartMember).where(SmartMember.photo_id == candidate.photo_id))


def test_pause_resources_finite_retries_and_manual_refresh(world, monkeypatch):
    from sqlalchemy.orm import sessionmaker
    from app.scheduler.photo_embedding_scheduler import PhotoEmbeddingScheduler
    from app.ai.state import utcnow
    with world.app.app_context():
        enable(db.session); db.session.commit()
        sessions = sessionmaker(db.engine, expire_on_commit=False)
    scheduler = PhotoEmbeddingScheduler(sessions, storage=world.app.extensions['photoly'].storage)
    resources = {'sampled_at':utcnow().isoformat(), 'cpu_percent':0, 'memory_available':0, 'disk_free':20*1024**3}
    monkeypatch.setattr('app.scheduler.smart_tag_scheduler.system_usage', lambda **kw: resources)
    calls = []
    def broken(*args):
        calls.append(1)
        raise ValueError('broken EXIF')
    monkeypatch.setattr(scheduler.smart, 'read_media', broken)
    scheduler.smart.tick()
    assert not calls
    resources['memory_available'] = 16*1024**3
    with sessions.begin() as s: control(s).desired = {**settings(s), 'maintenance':True}
    scheduler.smart.tick()
    assert not calls
    with sessions.begin() as s: control(s).desired = {**settings(s), 'maintenance':False}
    for _ in range(3): scheduler.smart.tick()
    assert len(calls) == 6  # Two original fixture media, three attempts each.
    for _ in range(3): scheduler.smart.tick()
    assert len(calls) == 6
    with sessions.begin() as s:
        assert SmartService(s).status(2)['failed'] == 2
        SmartService(s).refresh(2)
    scheduler.smart.next_check = 0
    scheduler.smart.user_cursor = 0
    scheduler.smart.tick()
    assert len(calls) > 6


def test_large_library_has_bounded_batches(world, monkeypatch, tmp_path):
    import gc
    import time
    import psutil
    from sqlalchemy import insert
    from sqlalchemy.orm import sessionmaker
    from app.scheduler.photo_embedding_scheduler import PhotoEmbeddingScheduler
    from app.ai.state import utcnow
    with world.app.app_context():
        enable(db.session)
        for start in range(0, 10000, 500):
            db.session.execute(insert(Photo), [{'user_id':2, 'photo_name':f'bulk-{i}', 'format':'jpg', 'content_version':1} for i in range(start,start+500)])
        db.session.commit()
        sessions = sessionmaker(db.engine, expire_on_commit=False)
    scheduler = PhotoEmbeddingScheduler(sessions, storage=world.app.extensions['photoly'].storage)
    monkeypatch.setattr('app.scheduler.smart_tag_scheduler.system_usage', lambda **kw: {'sampled_at':utcnow().isoformat(),'cpu_percent':0,'memory_available':16*1024**3,'disk_free':20*1024**3})
    monkeypatch.setattr(scheduler.smart, 'read_media', lambda *a: {'facts': {'camera':'iPhone 13'}, 'captured_at':None, 'latitude':None, 'longitude':None})
    gc.collect(); before = psutil.Process().memory_info().rss
    elapsed, peak = [], before
    for i in range(10):
        start = time.perf_counter(); scheduler.smart.tick(); elapsed.append(time.perf_counter()-start)
        peak = max(peak, psutil.Process().memory_info().rss)
        with sessions() as s:
            assert s.scalar(select(func.count()).select_from(SmartMedia)) <= (i+1)*25
    report = {'libraryMedia':10002, 'batches':10, 'batchMediaLimit':25, 'maxBatchSeconds':max(elapsed), 'meanBatchSeconds':sum(elapsed)/len(elapsed), 'rssGrowthBytes':peak-before}
    print('SMART_BENCHMARK ' + json.dumps(report))
    assert max(elapsed) < 6
    assert peak-before < 32*1024**2


def test_profile_change_and_expired_lease_restart_safely(world, monkeypatch):
    from sqlalchemy.orm import sessionmaker
    from app.scheduler.photo_embedding_scheduler import PhotoEmbeddingScheduler
    from app.scheduler import job_repository as jobs
    from app.services.ai_admin import AIAdmin
    from app.entities.ai import AIJob
    from app.ai.state import utcnow
    with world.app.app_context():
        pref = enable(db.session)
        first = AIAdmin(db.session).create_profile('siglip2')['profileId']
        second = AIAdmin(db.session).create_profile('qwen3_vl')['profileId']
        pref.profile_id = first
        pref.checkpoint = {'phase':'missing', 'run':'outdated', 'cursor':999, 'revision':1}
        control(db.session).desired = {**settings(db.session), 'search_profile_id':second}
        db.session.commit()
        sessions = sessionmaker(db.engine, expire_on_commit=False)
    scheduler = PhotoEmbeddingScheduler(sessions, storage=world.app.extensions['photoly'].storage)
    with sessions.begin() as s:
        scheduler.smart.reconcile(s)
        task = jobs.claim(s, 'old-worker', kinds=('smart',))
        job = s.get(AIJob, task['id'])
        job.lease_expires_at = utcnow()-timedelta(seconds=1)
    with sessions.begin() as s:
        assert scheduler.smart.fence(s, task) == (None, None)
        fresh = jobs.claim(s, 'new-worker', kinds=('smart',))
        assert fresh['token'] != task['token']
    with sessions.begin() as s:
        pref, job = scheduler.smart.fence(s, fresh)
        assert pref.profile_id == second and pref.checkpoint == {}
        assert job.status == 'running'


def test_manual_pin_inheritance_does_not_disappear_when_already_automatic(app, world):
    from app.smart.organize import inherit
    with app.app_context():
        enable(db.session)
        media = facts(db.session, 1, 0)
        parent = SmartItem(user_id=2, stable_key='parent', source='trip', name='A')
        child = SmartItem(user_id=2, stable_key='child', source='trip', name='B')
        db.session.add_all([parent, child]); db.session.flush()
        db.session.add_all([SmartMember(item_id=parent.item_id, photo_id=media.photo_id, content_version=1, manual=1),
            SmartMember(item_id=child.item_id, photo_id=media.photo_id, content_version=1, automatic=True, manual=0)])
        db.session.flush()
        inherit(db.session, parent, child)
        assert db.session.get(SmartMember, (child.item_id, media.photo_id), populate_existing=True).manual == 1


def test_round_robin_keeps_embedding_work_in_same_cycle(world, monkeypatch):
    from sqlalchemy.orm import sessionmaker
    from app.scheduler.photo_embedding_scheduler import PhotoEmbeddingScheduler
    from app.scheduler import job_repository as jobs
    from app.services.ai_admin import AIAdmin
    from app.entities.ai import EmbeddingProfile, AIJob
    from tests.test_ai import FakeRuntime
    with world.app.app_context():
        enable(db.session)
        pid = AIAdmin(db.session).create_profile('siglip2')['profileId']
        profile = db.session.get(EmbeddingProfile, pid)
        profile.index_status = 'ready'
        control(db.session).desired = {**settings(db.session), 'loaded_profile_id':pid, 'search_profile_id':pid, 'runtime':'loaded', 'background_interval_seconds':0}
        runtime = FakeRuntime(dict(profile.spec))
        db.session.commit()
        sessions = sessionmaker(db.engine, expire_on_commit=False)
    scheduler = PhotoEmbeddingScheduler(sessions, runtime, world.app.extensions['photoly'].storage)
    monkeypatch.setattr('app.scheduler.smart_tag_scheduler.system_usage', lambda **kw: runtime.status()['resources'])
    monkeypatch.setattr(scheduler.smart, 'read_media', lambda *a: {'facts':{},'captured_at':None,'latitude':None,'longitude':None})
    embedded = []
    monkeypatch.setattr(scheduler, 'embed', lambda task: embedded.append(task))
    scheduler.tick()
    assert len(embedded) == 1
    with sessions() as s: assert s.scalar(select(func.count()).select_from(SmartMedia)) == 2
    with sessions.begin() as s:
        # Two queued users rotate instead of permanently favoring the oldest job.
        s.add_all([AIJob(kind='smart', payload={'user_id':2}, priority=0, dedup_key='rotation:2'), AIJob(kind='smart', payload={'user_id':3}, priority=0, dedup_key='rotation:3')])
    with sessions.begin() as s:
        task = jobs.claim(s, 'round-robin', kinds=('smart',), last_user=2)
        assert task['payload']['user_id'] == 3


def test_three_users_do_not_starve_the_last_user(app, world):
    from app.entities.ai import AIJob
    from app.scheduler import job_repository as jobs
    with app.app_context():
        for uid in (1,2,3): db.session.add(AIJob(kind='smart', payload={'user_id':uid}, dedup_key=f'rotation:{uid}'))
        db.session.flush()
        last, seen = 0, []
        for _ in range(6):
            task = jobs.claim(db.session, 'rotation', kinds=('smart',), last_user=last)
            last = task['payload']['user_id']; seen.append(last)
            db.session.get(AIJob, task['id']).status = 'queued'
        assert seen == [1,2,3,1,2,3]
