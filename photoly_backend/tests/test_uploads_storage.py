import io
import json
import stat
from concurrent.futures import ThreadPoolExecutor
from datetime import timedelta
from uuid import uuid4
from zipfile import ZipFile, ZipInfo
import pytest
from PIL import Image
from sqlalchemy import select, func, text
from app.entities import User, UserGroup, Namespace, Photo, UploadItem, UploadBatch
from app.extensions import db
from app.services.uploads import UploadService, now
from app.services.quota import QuotaService
from app.uploads.worker import tick
from tests.openapi_assertions import assert_documented_response


def api(world, path, data=None, method='post', admin=False):
    response = world.client.open(path, method=method, json=data, headers=world.admin if admin else world.headers)
    assert response.status_code == 200, response.text
    return response.json


def batch(world, entries, name='Import'):
    bid = str(uuid4())
    assert api(world, '/uploads', {'id': bid, 'nsId': 2, 'name': name})['t'] == bid
    rows = [{'id': str(uuid4()), 'path': path, 'size': len(content)} for path, content in entries]
    for offset in range(0,len(rows),500):
        assert api(world, f'/uploads/{bid}/items', {'items': rows[offset:offset+500]})['msgCode'] == 200
    assert api(world, f'/uploads/{bid}/finish')['msgCode'] == 200
    return bid, rows


def receive(world, bid, row, content):
    return world.client.put(f'/uploads/{bid}/items/{row["id"]}/content', data=content,
                           content_type='application/octet-stream', headers=world.headers).json


def drain(world):
    with world.app.app_context():
        while tick(world.app):
            pass


def results(world, bid):
    rows, page = [], 1
    while True:
        data = api(world, f'/uploads/{bid}/items?page={page}', method='get')['t']
        rows += data['items']
        if len(rows) >= data['total']:
            return rows
        page += 1


def zip_bytes(entries):
    output = io.BytesIO()
    with ZipFile(output, 'w', allowZip64=True) as archive:
        for path, data in entries:
            with archive.open(path, 'w', force_zip64=True) as stream:
                stream.write(data)
    return output.getvalue()


@pytest.mark.parametrize('count', [16, 100, 1000])
def test_large_batch_is_not_truncated(world, monkeypatch, count):
    # Exercise the actual transmission/worker path with tiny real images, avoiding
    # 1000 subprocess metadata calls that are independently covered elsewhere.
    output = io.BytesIO()
    Image.new('RGB',(1,1),'red').save(output,'JPEG')
    content = output.getvalue()
    monkeypatch.setattr(world.app.extensions['photoly'].metadata, 'read', lambda path: {})
    entries = [(f'旅行/嵌套/{n}.JPG', content) for n in range(count)]
    bid, rows = batch(world, entries)
    for row in rows:
        assert receive(world, bid, row, content)['msgCode'] == 200
    drain(world)
    imported = results(world, bid)
    assert len(imported) == count and {r['state'] for r in imported} == {'done'}
    assert len({r['photoId'] for r in imported}) == count
    assert len({r['nsId'] for r in imported}) == 1
    with world.app.app_context():
        service = UploadService(db.session, world.app.extensions['photoly'])
        user = db.session.get(User,2)
        assert user.used_bytes == service.storage.usage(user.uuid)[0]


def test_zip64_nested_folders_skips_and_idempotency(world):
    content = zip_bytes([('旅行/a.jpg',world.jpeg),('旅行/b.jpg',world.jpeg),('notes.txt',b'notes'),('nested.zip',b'zip')])
    bid, rows = batch(world,[('Trip.zip',content)])
    assert receive(world,bid,rows[0],content)['t']['state'] == 'ready'
    assert receive(world,bid,rows[0],content)['t']['state'] == 'ready'
    drain(world)
    out = results(world,bid)
    assert sorted(r['state'] for r in out) == ['done','done','expanded','skipped','skipped']
    assert {r['error'] for r in out if r['state']=='skipped'} == {'unsupported_file_type'}
    assert api(world,f'/uploads/{bid}/retry')['msgCode'] == 200
    drain(world)
    assert results(world,bid) == out
    with world.app.app_context():
        assert db.session.scalar(select(func.count()).select_from(Photo).where(Photo.user_id==2)) == 4
        assert len(db.session.scalars(select(Namespace).where(Namespace.user_id==2,Namespace.ns_name=='Trip')).all()) == 1


def test_folder_name_collision_does_not_merge(world):
    bid, rows = batch(world,[('Travel/new.jpg',world.jpeg)])
    receive(world,bid,rows[0],world.jpeg)
    drain(world)
    with world.app.app_context():
        item = db.session.get(UploadItem,rows[0]['id'])
        folder = db.session.get(Namespace,item.ns_id)
        assert folder.ns_name == 'Travel (1)' and folder.ns_parent_id == 2


@pytest.mark.parametrize('path', ['../escape.jpg','/escape.jpg','folder/../../escape.jpg','C:/escape.jpg','folder\\escape.jpg'])
def test_unsafe_zip_rejected_without_import(world,path):
    content=zip_bytes([('good.jpg',world.jpeg),(path,world.jpeg)])
    bid,rows=batch(world,[('bad.zip',content)])
    receive(world,bid,rows[0],content)
    drain(world)
    assert results(world,bid)[0]['state']=='failed'
    with world.app.app_context():
        assert db.session.scalar(select(func.count()).select_from(Photo))==2


def test_zip_symlink_and_expansion_budget(world):
    output=io.BytesIO()
    with ZipFile(output,'w') as archive:
        info=ZipInfo('link.jpg'); info.create_system=3; info.external_attr=(stat.S_IFLNK|0o777)<<16
        archive.writestr(info,'../../secret')
    bid,rows=batch(world,[('links.zip',output.getvalue())])
    receive(world,bid,rows[0],output.getvalue()); drain(world)
    assert results(world,bid)[0]['error']=='unsafe_or_encrypted_zip'
    world.app.config['UPLOAD_ZIP_MAX_BYTES']=10
    content=zip_bytes([('a.jpg',world.jpeg)])
    bid,rows=batch(world,[('large.zip',content)])
    receive(world,bid,rows[0],content); drain(world)
    assert results(world,bid)[0]['error']=='zip_expansion_limit'


def test_partial_failure_cancel_retry_and_restart(world):
    bid,rows=batch(world,[('a.jpg',world.jpeg),('bad.jpg',b'bad'),('later.jpg',world.jpeg)])
    receive(world,bid,rows[0],world.jpeg);receive(world,bid,rows[1],b'bad')
    # Emulate a worker that died after committing the processing state.
    with world.app.app_context():
        db.session.get(UploadItem,rows[0]['id']).state='processing';db.session.commit()
    drain(world)
    assert {r['state'] for r in results(world,bid)}=={'done','failed','pending'}
    api(world,f'/uploads/{bid}/cancel')
    assert receive(world,bid,rows[2],world.jpeg)['t']['state']=='cancelled'
    api(world,f'/uploads/{bid}/retry')
    receive(world,bid,rows[2],world.jpeg);drain(world)
    assert sorted(r['state'] for r in results(world,bid))==['done','done','failed']


def test_ownership_size_boundaries_and_deleted_destination(world):
    bid,rows=batch(world,[('a.jpg',world.jpeg)])
    for method,path,data in [('GET',f'/uploads/{bid}/items',None),('POST',f'/uploads/{bid}/retry',None),('POST',f'/uploads/{bid}/cancel',None)]:
        assert world.client.open(path,method=method,json=data,headers=world.other).json['msgCode']!=200
    assert receive(world,bid,rows[0],b'short')['t']=='file_size_mismatch'
    world.app.config['UPLOAD_DISK_RESERVE_BYTES']=10**20
    assert receive(world,bid,rows[0],world.jpeg)['t']=='insufficient_disk_space'
    world.app.config['UPLOAD_DISK_RESERVE_BYTES']=0
    assert receive(world,bid,rows[0],world.jpeg)['msgCode']==200
    with world.app.app_context():
        db.session.get(UploadBatch,bid).ns_id=None;db.session.commit()
    drain(world)
    assert results(world,bid)[0]['error']=='destination_deleted'


def test_request_limit_does_not_limit_batch_total(world):
    world.app.config['MAX_CONTENT_LENGTH']=len(world.jpeg)
    bid,rows=batch(world,[('a.jpg',world.jpeg),('b.jpg',world.jpeg)])
    assert receive(world,bid,rows[0],world.jpeg)['msgCode']==200
    assert receive(world,bid,rows[1],world.jpeg)['msgCode']==200
    oversized=world.client.put(f'/uploads/{bid}/items/{rows[0]["id"]}/content',data=world.jpeg+b'x',headers=world.headers)
    # Repeated content need not be read, but a fresh over-limit request must fail.
    bid2,rows2=batch(world,[('c.jpg',world.jpeg)])
    oversized=world.client.put(f'/uploads/{bid2}/items/{rows2[0]["id"]}/content',data=world.jpeg+b'x',headers=world.headers)
    assert oversized.status_code==413


def test_default_group_overrides_reassignment_and_permissions(world):
    get=lambda: api(world,'/user/getStorage',method='get')['t']
    assert get()['capacityBytes']==50_000_000_000
    created=api(world,'/admin/user-groups',{'name':'Family','maxStorageBytes':1000},admin=True)
    assert created['msgCode']==200
    gid=created['t']
    assert api(world,'/admin/users/2/storage',{'groupId':gid,'storageOverride':False,'maxStorageBytes':None},method='put',admin=True)['msgCode']==200
    assert get()['overQuota'] and get()['capacityBytes']==1000
    api(world,'/admin/users/2/storage',{'groupId':gid,'storageOverride':True,'maxStorageBytes':None},method='put',admin=True)
    assert get()['capacityBytes'] is None and get()['quotaSource']=='user'
    assert api(world,'/admin/user-groups/1',{'reassignTo':gid},method='delete',admin=True)['msgCode']!=200
    assert api(world,f'/admin/user-groups/{gid}',{'reassignTo':1},method='delete',admin=True)['msgCode']==200
    assert get()['groupId']==1 and get()['capacityBytes'] is None
    assert api(world,'/admin/user-groups',{'name':'No','maxStorageBytes':1})['msgCode']!=200
    assert api(world,'/admin/users/2/storage',{'groupId':1,'storageOverride':True,'maxStorageBytes':-1},method='put',admin=True)['msgCode']!=200
    response=world.client.get('/admin/user-groups',headers=world.admin)
    assert_documented_response('GET','/admin/user-groups',response)


def test_quota_all_ingress_and_delete_releases_usage(world):
    usage=api(world,'/user/getStorage',method='get')['t']['usedBytes']
    api(world,'/admin/users/2/storage',{'groupId':1,'storageOverride':True,'maxStorageBytes':usage},method='put',admin=True)
    for route, data in [('/photo/insert',{'photo':json.dumps({'nsId':2})}),('/cred/uploadPhoto/api-alice',{'photo':json.dumps({'nsId':2})}),('/user/updateAvatar',{})]:
        response=world.client.post(route,headers=world.headers,data={**data,'file':(io.BytesIO(world.jpeg),'x.jpg')})
        if route.startswith('/cred'):
            assert not response.data or response.json in (None,-1)
        else: assert response.json['msgCode']==413
    bid,rows=batch(world,[('no.jpg',world.jpeg)])
    receive(world,bid,rows[0],world.jpeg);drain(world)
    assert results(world,bid)[0]['error']=='storage_quota_exceeded'
    assert api(world,'/user/getStorage',method='get')['t']['usedBytes']==usage
    world.client.post('/photo/delete?photoId=1',headers=world.headers)
    assert api(world,'/user/getStorage',method='get')['t']['usedBytes']<usage


def test_concurrent_quota_admits_only_one_file(world):
    usage=api(world,'/user/getStorage',method='get')['t']['usedBytes']
    # Opaque video bytes have no thumbnail, so the exact increase is known.
    api(world,'/admin/users/2/storage',{'groupId':1,'storageOverride':True,'maxStorageBytes':usage+len(world.video)},method='put',admin=True)
    def send(_):
        with world.app.test_client() as client:
            return client.post('/photo/insert',headers=world.headers,data={'photo':json.dumps({'nsId':2}),
                'file':(io.BytesIO(world.video),'x.mp4')}).json['msgCode']
    with ThreadPoolExecutor(2) as pool:
        assert sorted(pool.map(send,range(2)))==[200,413]
    assert api(world,'/user/getStorage',method='get')['t']['usedBytes']==usage+len(world.video)


def test_cleanup_and_reconcile_are_repeatable(world):
    bid,rows=batch(world,[('pending.jpg',world.jpeg),('received.jpg',world.jpeg)])
    receive(world,bid,rows[1],world.jpeg);drain(world)
    with world.app.app_context():
        service=UploadService(db.session,world.app.extensions['photoly'])
        db.session.get(UploadBatch,bid).updated_at=now()-timedelta(days=2);db.session.commit()
        service.cleanup();service.cleanup()
        assert not service.source(rows[1]['id']).exists()
        assert db.session.get(UploadItem,rows[0]['id']).error=='transmission_expired'
        db.session.get(UploadBatch,bid).updated_at=now()-timedelta(days=31);db.session.commit()
        service.cleanup()
        assert db.session.get(UploadBatch,bid) is None
    result=world.app.test_cli_runner().invoke(args=['reconcile-storage','--all-users'])
    assert result.exit_code==0, result.output


def test_worker_claim_lock_skips_owned_item(world):
    bid,rows=batch(world,[('a.jpg',world.jpeg)])
    receive(world,bid,rows[0],world.jpeg)
    with world.app.app_context():
        with db.engine.connect() as lock:
            lock.execute(text('SELECT pg_advisory_lock(hashtextextended(:id,74831004))'),{'id':bid});lock.commit()
            assert tick(world.app) is False
            lock.execute(text('SELECT pg_advisory_unlock(hashtextextended(:id,74831004))'),{'id':bid});lock.commit()
        assert tick(world.app) is True
    assert results(world,bid)[0]['state']=='done'


def test_expired_zip_can_be_reselected_without_duplicate_imports(world):
    content=zip_bytes([('good.jpg',world.jpeg),('broken.jpg',b'bad')])
    bid,rows=batch(world,[('Archive.zip',content)])
    receive(world,bid,rows[0],content);drain(world)
    with world.app.app_context():
        service=UploadService(db.session,world.app.extensions['photoly'])
        service.source(rows[0]['id']).unlink()
    api(world,f'/uploads/{bid}/retry')
    out=results(world,bid)
    assert next(r for r in out if r['id']==rows[0]['id'])['state']=='pending'
    assert receive(world,bid,rows[0],b'x'*len(content))['t']=='source_changed'
    assert receive(world,bid,rows[0],content)['msgCode']==200
    drain(world)
    out=results(world,bid)
    assert sorted(r['state'] for r in out)==['done','expanded','failed']
    with world.app.app_context():
        assert db.session.scalar(select(func.count()).select_from(Photo))==3


def test_concurrent_workers_share_the_same_imported_folder(world):
    bid,rows=batch(world,[('New/first.jpg',world.jpeg),('New/second.jpg',world.jpeg)])
    for row in rows: receive(world,bid,row,world.jpeg)
    from threading import Barrier
    barrier=Barrier(2)
    def process(row):
        with world.app.app_context():
            service=UploadService(db.session,world.app.extensions['photoly'])
            # Populate both identity maps before either worker acquires the user lock.
            service.batch(2,bid)
            barrier.wait(timeout=5)
            service.process(row['id'])
    with ThreadPoolExecutor(2) as pool:
        list(pool.map(process,rows))
    out=results(world,bid)
    assert {r['state'] for r in out}=={'done'}
    assert len({r['nsId'] for r in out})==1


@pytest.mark.parametrize('archive', [False, True])
def test_complete_media_directory_layout_and_duplicate_filenames(world, archive):
    from app.services.namespace import NamespaceService
    files = [('旅行/北京/故宫/same.jpg', world.jpeg), ('旅行/上海/same.jpg', world.jpeg),
             ('旅行/仅文档/readme.txt', b'unsupported')]
    entries = [('Archive.zip', zip_bytes(files))] if archive else files
    bid, rows = batch(world, entries)
    for row, (_, content) in zip(rows, entries):
        if row['path'].endswith('.txt'):
            continue
        assert receive(world, bid, row, content)['msgCode'] == 200
    drain(world)
    imported = [item for item in results(world, bid) if item['state'] == 'done']
    assert len(imported) == 2
    with world.app.app_context():
        service = NamespaceService(db.session, world.app.extensions['photoly'])
        paths = {tuple(node['nsName'] for node in service.trace(2, item['nsId'])[1:]) for item in imported}
        prefix = ('Archive',) if archive else ()
        assert paths == {prefix + ('旅行', '北京', '故宫'), prefix + ('旅行', '上海')}
        assert db.session.scalar(select(func.count()).select_from(Namespace).where(
            Namespace.user_id == 2, Namespace.ns_name == '仅文档')) == 0
        photos = [db.session.get(Photo, item['photoId']) for item in imported]
        assert {photo.photo_name for photo in photos} == {'same'}
        assert len({photo.ns_id for photo in photos}) == 2
    # A second import gets one new top folder; it must not merge either subtree.
    second, rows = batch(world, entries)
    for row, (_, content) in zip(rows, entries):
        if not row['path'].endswith('.txt'):
            receive(world, second, row, content)
    drain(world)
    with world.app.app_context():
        service = NamespaceService(db.session, world.app.extensions['photoly'])
        top_names = {service.trace(2, item['nsId'])[1]['nsName']
                     for item in results(world, second) if item['state'] == 'done'}
        assert top_names == {'Archive (1)' if archive else '旅行 (1)'}


def test_clear_activity_keeps_photos_idempotency_and_active_batches(world):
    done, rows = batch(world, [('a.jpg', world.jpeg)])
    active, _ = batch(world, [('pending.jpg', world.jpeg)])
    with world.app.app_context():
        item = db.session.get(UploadItem, rows[0]['id'])
        item.state, item.photo_id = 'done', 1
        db.session.commit()
    assert api(world, f'/uploads/{active}', method='delete')['msgCode'] == 409
    denied = world.client.delete(f'/uploads/{done}', headers=world.other)
    assert denied.json['msgCode'] == 404
    cleared = world.client.delete('/uploads', headers=world.headers)
    assert_documented_response('DELETE', '/uploads', cleared)
    assert cleared.json['t'] == [done]
    assert api(world, f'/uploads/{done}', method='delete')['t'] == []
    listed = api(world, '/uploads', method='get')['t']
    assert listed['total'] == 1 and listed['batches'][0]['id'] == active
    assert api(world, '/uploads', {'id':done, 'nsId':2, 'name':'Again'})['t'] == done
    with world.app.app_context():
        assert db.session.get(Photo, 1) is not None
        assert db.session.get(UploadItem, rows[0]['id']).photo_id == 1


def test_failure_acknowledgement_persists_and_new_failures_notify(world):
    bid, rows = batch(world, [('bad.jpg', world.jpeg)])
    with world.app.app_context():
        item = db.session.get(UploadItem, rows[0]['id'])
        item.state, item.error = 'failed', 'processing_failed'
        db.session.commit()
    assert api(world, '/uploads', method='get')['t']['failedCount'] == 1
    response = world.client.post('/uploads/acknowledge-failures', headers=world.headers)
    assert_documented_response('POST', '/uploads/acknowledge-failures', response)
    assert response.json['msgCode'] == 200
    listed = api(world, '/uploads', method='get')['t']
    assert listed['failedCount'] == 0 and listed['batches'][0]['counts']['failed']['count'] == 1
    with world.app.app_context():
        item = db.session.get(UploadItem, rows[0]['id'])
        item.updated_at = now() + timedelta(seconds=1)
        db.session.commit()
    assert api(world, '/uploads', method='get')['t']['failedCount'] == 1
    assert api(world, f'/uploads/{bid}', method='delete')['t'] == [bid]
    assert api(world, '/uploads', method='get')['t']['failedCount'] == 0
    assert api(world, f'/uploads/{bid}/retry')['msgCode'] == 200
    assert api(world, '/uploads', method='get')['t']['total'] == 1
