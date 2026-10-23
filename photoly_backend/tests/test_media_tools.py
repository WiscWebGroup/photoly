import io
import json
import socket
import struct
import subprocess
from uuid import uuid4
from zipfile import ZipFile
import pytest
from PIL import Image
from sqlalchemy import select, func
from app.entities import Photo, PhotoExtra, PhotoMedia, Setting, User, TagPhoto, GalleryPhoto
from app.extensions import db
from app.infrastructure.media import encode_image, image_info, asset_path
from app.infrastructure.remote_image import public_target, download_image
from app.services.base import BusinessError
from app.services.photo import PhotoService
from tests.test_uploads_storage import api, batch, receive, drain, results, zip_bytes


def submit(world, kind='convert', **options):
    bid = str(uuid4())
    response = api(world, '/photo/' + ('convert' if kind == 'convert' else 'onlineDownload'), {'id': bid, **options})
    assert response['msgCode'] == 200, response
    return bid


@pytest.mark.parametrize('source', ['JPEG', 'PNG', 'GIF'])
@pytest.mark.parametrize('target', ['jpeg', 'png', 'gif'])
def test_conversion_formats(tmp_path, source, target):
    path, out = tmp_path / 'input', tmp_path / 'output'
    Image.new('RGB', (8, 12), 'red').save(path, source)
    encode_image(path, out, target)
    with Image.open(out) as image:
        assert image.format == target.upper() and image.size == (8, 12)


def test_transparency_orientation_exif_and_animation(tmp_path):
    source, output = tmp_path / 'input.png', tmp_path / 'out'
    exif = Image.Exif(); exif[274] = 6; exif[306] = '2024:02:29 12:00:00'
    Image.new('RGBA', (8,12), (0,0,0,0)).save(source, exif=exif)
    from app.infrastructure.storage import LocalStorage
    storage=LocalStorage(tmp_path/'media')
    with storage.mutation() as files:
        storage.thumbnail(source, output, files)
    with Image.open(output) as thumbnail:
        assert thumbnail.size == (12,8)
    encode_image(source, output, 'jpeg')
    with Image.open(output) as image:
        assert image.size == (12,8) and image.getpixel((0,0)) == (255,255,255)
        assert image.getexif()[274] == 1 and image.getexif()[306] == exif[306]
    encode_image(source, output, 'png')
    with Image.open(output) as image:
        assert image.getpixel((0,0))[3] == 0
    frames = [Image.new('RGB',(10,10), color) for color in ('red','blue','green')]
    frames[0].save(source, 'GIF', save_all=True, append_images=frames[1:], duration=[80,120,200], loop=3)
    encode_image(source, output, 'gif')
    with Image.open(output) as image:
        assert image.n_frames == 3 and image.info['loop'] == 3
        durations = []
        for i in range(3):
            image.seek(i); durations.append(image.info['duration'])
        assert durations == [80,120,200]
    encode_image(source, output, 'png')
    with Image.open(output) as image:
        assert not getattr(image, 'is_animated', False) and image.getpixel((0,0))[:3] == (255,0,0)


def test_conversion_task_idempotency_folder_metadata_private(world):
    key = str(uuid4()); payload = {'id': key, 'photoId': 1, 'format': 'png'}
    assert api(world, '/photo/convert', payload)['t'] == key
    assert api(world, '/photo/convert', payload)['t'] == key
    assert api(world, '/photo/convert', {**payload, 'format': 'gif'})['msgCode'] == 409
    drain(world)
    row = results(world,key)[0]
    assert row['state'] == 'done', row
    assert row['nsId'] == 3
    with world.app.app_context():
        photo = db.session.get(Photo,row['photoId'])
        assert photo.photo_name == 'sunset (converted)' and photo.visibility == 0
        extra = db.session.scalar(select(PhotoExtra).where(PhotoExtra.f_photo_id == photo.photo_id))
        assert extra.extra_photo_taken_date.year == 2024
        assert not db.session.scalar(select(TagPhoto).where(TagPhoto.photo_id == photo.photo_id))
        assert not db.session.scalar(select(GalleryPhoto).where(GalleryPhoto.photo_id == photo.photo_id))
    second = submit(world, photoId=1, format='png'); drain(world)
    with world.app.app_context():
        assert db.session.get(Photo,results(world,second)[0]['photoId']).photo_name == 'sunset (converted) (1)'
    assert world.client.post('/photo/convert', json={'id':str(uuid4()),'photoId':1,'format':'png'},headers=world.other).json['msgCode'] == 404


@pytest.mark.parametrize('change', ['delete','replace','quota','folder'])
def test_conversion_rechecks_before_save(world, monkeypatch, change):
    from app.services import media
    real = media.convert_bounded
    bid = submit(world, photoId=1, format='png')
    def convert(*args):
        real(*args)
        if change == 'delete':
            PhotoService(db.session,world.app.extensions['photoly']).delete(2,1)
        elif change == 'replace':
            photo = db.session.get(Photo,1); photo.content_version += 1; db.session.commit()
        elif change == 'quota':
            user = db.session.get(User,2); user.storage_override = True; user.max_storage_bytes = 1; db.session.commit()
        else:
            from app.services.namespace import NamespaceService
            # Move the source away, then delete the submission-time destination.
            PhotoService(db.session,world.app.extensions['photoly']).move(2,1,2)
            NamespaceService(db.session,world.app.extensions['photoly']).delete(2,3)
    monkeypatch.setattr(media,'convert_bounded',convert)
    drain(world)
    row=results(world,bid)[0]
    assert row['state']=='failed', row
    assert row['error'] == {'delete':'source_changed','replace':'source_changed','quota':'storage_quota_exceeded','folder':'destination_deleted'}[change]


def enable(world, value=1):
    assert api(world,f'/admin/setGeneralSetting?name=OnlineDownload&val={value}',admin=True)['msgCode']==200


def test_online_gates_and_save(world,monkeypatch):
    from app.services import media
    assert api(world,'/photo/mediaCapabilities',method='get')['t']['onlineDownload'] is False
    payload={'id':str(uuid4()),'url':'https://example.com/a.png','format':'jpeg','nsId':2}
    assert api(world,'/photo/onlineDownload',payload)['t']=='online_download_disabled'
    enable(world)
    monkeypatch.setattr(media,'download_bounded',lambda url,path,config:path.write_bytes(world.jpeg))
    bid=submit(world,'online',url=payload['url'],format='png',nsId=3); drain(world)
    assert results(world,bid)[0]['state']=='done'
    bid=submit(world,'online',url=payload['url'],format='png',nsId=3)
    enable(world,0); drain(world)
    assert results(world,bid)[0]['error']=='online_download_disabled'
    enable(world)
    def disabled_during_download(url,path,config):
        path.write_bytes(world.jpeg)
        setting=db.session.scalar(select(Setting).where(Setting.setting_name=='OnlineDownload'))
        setting.setting_value='0'; db.session.commit()
    monkeypatch.setattr(media,'download_bounded',disabled_during_download)
    bid=submit(world,'online',url=payload['url'],format='png',nsId=3); drain(world)
    assert results(world,bid)[0]['error']=='online_download_disabled'


@pytest.mark.parametrize('address',['127.0.0.1','10.0.0.1','169.254.169.254','192.168.1.1','0.0.0.0','::1','fc00::1','fe80::1','::ffff:127.0.0.1','2002:7f00:1::','224.0.0.1'])
def test_non_public_dns_rejected(monkeypatch,address):
    monkeypatch.setattr(socket,'getaddrinfo',lambda *a,**k:[(0,0,0,'',(address,80))])
    with pytest.raises(BusinessError,match='non_public'):
        public_target('https://example.com/image')


@pytest.mark.parametrize('url',['file:///etc/passwd','ftp://example.com/a','http://user:pass@example.com/a','http://example.com\\@127.0.0.1/a','http://example.com/\r\nHost:localhost'])
def test_unsafe_urls_rejected(url):
    with pytest.raises(BusinessError): public_target(url)


def test_pinned_connection_redirect_and_size(monkeypatch,tmp_path):
    from app.infrastructure import remote_image as remote
    addresses = ['93.184.216.34','127.0.0.1']
    monkeypatch.setattr(socket,'getaddrinfo',lambda *a,**k:[(0,0,0,'',(addresses.pop(0),443))])
    calls=[]
    class Response:
        status=302; headers={'Location':'https://example.com/next'}
        def close(self): pass
    class Pool:
        def __init__(self,*args,**kwargs): calls.append((args,kwargs))
        def urlopen(self,*args,**kwargs): return Response()
        def close(self): pass
    monkeypatch.setattr(remote.urllib3,'HTTPSConnectionPool',Pool)
    config={'ONLINE_DOWNLOAD_TIMEOUT':60,'ONLINE_DOWNLOAD_CONNECT_TIMEOUT':5,'ONLINE_DOWNLOAD_MAX_BYTES':8,'UPLOAD_DISK_RESERVE_BYTES':0}
    with pytest.raises(BusinessError,match='non_public'): download_image('https://example.com/a',tmp_path/'out',config)
    assert calls[0][0][0]=='93.184.216.34'
    assert calls[0][1]['server_hostname']=='example.com' and calls[0][1]['assert_hostname']=='example.com'
    monkeypatch.setattr(socket,'getaddrinfo',lambda *a,**k:[(0,0,0,'',('93.184.216.34',443))])
    Response.status=200; Response.headers={'Content-Length':'9'}
    with pytest.raises(BusinessError,match='too_large'): download_image('https://example.com/a',tmp_path/'out',config)


@pytest.fixture
def apple_pair(tmp_path):
    """Small real JPEG MakerApple IFD + MOV QuickTime metadata, decoded by ExifTool."""
    identifier=str(uuid4())
    # Apple maker-note offsets are relative to its own header.
    value=identifier.encode()+b'\0'
    maker=b'Apple iOS\0'+b'\x00\x01MM'+struct.pack('>H',1)+struct.pack('>HHII',17,2,len(value),32)+b'\0'*4+value
    exif=Image.Exif(); exif[271]='Apple'; exif[272]='iPhone'; exif[34665]={37500:maker,36867:'2024:02:29 12:00:00'}
    still=tmp_path/'cover.jpg'; movie=tmp_path/'clip.mov'
    Image.new('RGB',(64,48),'coral').save(still,exif=exif)
    subprocess.run(['ffmpeg','-v','error','-f','lavfi','-i','testsrc=size=64x48:rate=10','-t','0.5','-c:v','libx264','-pix_fmt','yuv420p','-threads','1',str(movie)],check=True)
    subprocess.run(['exiftool','-overwrite_original',f'-Keys:ContentIdentifier={identifier}',str(movie)],check=True,capture_output=True)
    for path in (still,movie):
        metadata=json.loads(subprocess.check_output(['exiftool','-j','-n',str(path)]))[0]
        assert metadata.get('ContentIdentifier') == identifier, metadata
    return still.read_bytes(),movie.read_bytes()


@pytest.mark.parametrize('movie_first',[False,True])
def test_live_pair_waits_and_exports_as_one(world,apple_pair,movie_first):
    still,movie=apple_pair
    entries=[('Trip/a.jpg',still),('Trip/different-name.mov',movie)]
    if movie_first: entries.reverse()
    bid,rows=batch(world,entries)
    receive(world,bid,rows[0],entries[0][1]); drain(world)
    assert results(world,bid)[0]['state'] in ('waiting','pending')
    receive(world,bid,rows[1],entries[1][1]); drain(world)
    out=results(world,bid)
    assert {r['state'] for r in out}=={'done'},out
    assert len({r['photoId'] for r in out})==1
    pid=out[0]['photoId']
    info=api(world,f'/photo/mediaInfo?photoId={pid}',method='get')['t']
    assert info['live'] and info['liveStatus']=='ready'
    response=world.client.get(f'/photo/renderLive/token-alice?photoId={pid}',headers={'Range':'bytes=0-15'})
    assert response.status_code==206 and response.mimetype=='video/mp4'
    archive=world.client.get(f'/photo/downloadLive/token-alice?photoId={pid}')
    with ZipFile(io.BytesIO(archive.data)) as zipfile:
        assert sorted(zipfile.namelist())==['a.jpg','a.mov']
    assert api(world,'/photo/getDateSummary',method='get')['t']['totalCount']==2
    with world.app.app_context():
        photo=db.session.get(Photo,pid); media=db.session.get(PhotoMedia,pid)
        assert media.live
        storage=world.app.extensions['photoly'].storage
        paths=[asset_path(storage,'user-alice',photo,role) for role in ('live_original','live_preview')]
        assert all(p.exists() for p in paths)
        from werkzeug.datastructures import FileStorage
        PhotoService(db.session,world.app.extensions['photoly']).replace(2,pid,FileStorage(io.BytesIO(world.jpeg),filename='edited.jpg'))
        assert db.session.get(PhotoMedia,pid).live and all(p.exists() for p in paths)
        assert paths[0].read_bytes() == movie
        PhotoService(db.session,world.app.extensions['photoly']).move(2,pid,2)
        PhotoService(db.session,world.app.extensions['photoly']).delete(2,pid)
        assert not any(p.exists() for p in paths)
        assert db.session.get(User,2).used_bytes==storage.usage('user-alice')[0]


def test_heic_preview_preserves_original(world,tmp_path):
    from pillow_heif import register_heif_opener
    register_heif_opener()
    path=tmp_path/'cover.heic'; Image.new('RGB',(64,48),'blue').save(path,'HEIF')
    content=path.read_bytes(); bid,rows=batch(world,[('cover.heic',content)])
    receive(world,bid,rows[0],content); drain(world)
    row=results(world,bid)[0]; assert row['state']=='done',row
    pid=row['photoId']
    assert world.client.get(f'/photo/render/token-alice?photoId={pid}').mimetype=='image/jpeg'
    assert world.client.get(f'/photo/renderThumbnail/token-alice?photoId={pid}').mimetype=='image/jpeg'
    assert world.client.get(f'/photo/download/token-alice?photoId={pid}').data==content
    api(world,'/photo/update',{'photoId':pid,'visibility':2})
    link=api(world,f'/photo/getPath?photoId={pid}',method='get')['t']
    from urllib.parse import urlsplit
    shared=urlsplit(link)
    response=world.client.get(shared.path+'?'+shared.query)
    assert response.status_code==200 and response.mimetype=='image/jpeg'


@pytest.mark.parametrize('variant',['zip','missing','ambiguous','different-folder'])
def test_live_pairing_scope(world,apple_pair,variant):
    still,movie=apple_pair
    entries=[('a.jpg',still),('other.mov',movie)]
    if variant=='zip': entries=[('Trip.zip',zip_bytes(entries))]
    if variant=='missing': entries=entries[:1]
    if variant=='ambiguous': entries.append(('duplicate.jpg',still))
    if variant=='different-folder': entries[1]=('elsewhere/other.mov',movie)
    bid,rows=batch(world,entries)
    for row,(_,data) in zip(rows,entries): receive(world,bid,row,data)
    drain(world)
    out=[r for r in results(world,bid) if r['state']!='expanded']
    assert {r['state'] for r in out}=={'done'},out
    assert len({r['photoId'] for r in out})==(1 if variant=='zip' else len(entries))


def test_live_encoding_failure_keeps_cover_and_retry_restores_playback(world,apple_pair,monkeypatch):
    import app.services.live_import as importer
    still,movie=apple_pair
    def fail(*args,**kwargs): raise BusinessError('video_conversion_failed')
    monkeypatch.setattr(importer,'encode_live',fail)
    bid,rows=batch(world,[('cover.jpg',still),('movie.mov',movie)])
    for row,content in zip(rows,(still,movie)): receive(world,bid,row,content)
    drain(world)
    out=results(world,bid); assert {r['state'] for r in out}=={'done'}
    pid=out[0]['photoId']
    assert api(world,f'/photo/mediaInfo?photoId={pid}',method='get')['t']['liveStatus']=='failed'
    assert world.client.get(f'/photo/render/token-alice?photoId={pid}').status_code==200
    retry=str(uuid4())
    assert api(world,'/photo/retryLivePreview',{'id':retry,'photoId':pid})['t']==retry
    drain(world)
    assert results(world,retry)[0]['state']=='done'
    assert api(world,f'/photo/mediaInfo?photoId={pid}',method='get')['t']['liveStatus']=='ready'
    # Conversion from Live uses the paired movie; a GIF has multiple actual frames.
    converted=submit(world,photoId=pid,format='gif'); drain(world)
    row=results(world,converted)[0]; assert row['state']=='done',row
    data=world.client.get(f'/photo/render/token-alice?photoId={row["photoId"]}').data
    with Image.open(io.BytesIO(data)) as image: assert image.n_frames > 1
    folder_zip=world.client.get(f'/namespace/downloadNs/token-alice?nsId={out[0]["nsId"]}')
    assert folder_zip.status_code==200
    with ZipFile(io.BytesIO(folder_zip.data)) as archive:
        assert any(name.endswith('movie.mov') or name.endswith('cover.mov') for name in archive.namelist())


def test_live_wait_cancel_and_expiry(world,apple_pair):
    from datetime import timedelta
    from app.entities import UploadBatch,UploadItem
    from app.services.uploads import UploadService,now
    still,movie=apple_pair
    bid,rows=batch(world,[('cover.jpg',still),('movie.mov',movie)])
    receive(world,bid,rows[0],still); drain(world)
    api(world,f'/uploads/{bid}/cancel')
    assert {r['state'] for r in results(world,bid)}=={'cancelled'}
    api(world,f'/uploads/{bid}/retry')
    drain(world)
    with world.app.app_context():
        batch_row=db.session.get(UploadBatch,bid); batch_row.updated_at=now()-timedelta(hours=25); db.session.commit()
        UploadService(db.session,world.app.extensions['photoly']).cleanup()
    drain(world)
    out=results(world,bid)
    assert sorted(r['state'] for r in out)==['done','failed'],out
    pid=next(r['photoId'] for r in out if r['state']=='done')
    assert api(world,f'/photo/mediaInfo?photoId={pid}',method='get')['t']['live'] is False


def test_cancel_during_conversion_and_retry_is_single_copy(world,monkeypatch):
    from app.services import media
    from app.services.uploads import UploadService
    real=media.convert_bounded
    bid=submit(world,photoId=1,format='png')
    def cancel(*args):
        real(*args)
        UploadService(db.session,world.app.extensions['photoly']).cancel(2,bid)
    monkeypatch.setattr(media,'convert_bounded',cancel)
    drain(world)
    assert results(world,bid)[0]['state']=='cancelled'
    monkeypatch.setattr(media,'convert_bounded',real)
    api(world,f'/uploads/{bid}/retry'); drain(world)
    assert results(world,bid)[0]['state']=='done'
    pid=results(world,bid)[0]['photoId']
    api(world,f'/uploads/{bid}/retry'); drain(world)
    assert results(world,bid)[0]['photoId']==pid


def test_online_invalid_image_leaves_no_files(world,monkeypatch):
    from app.services import media
    enable(world)
    monkeypatch.setattr(media,'download_bounded',lambda url,path,config:path.write_bytes(b'<html>not an image</html>'))
    with world.app.app_context():
        before=world.app.extensions['photoly'].storage.usage('user-alice')[0]
    bid=submit(world,'online',url='https://example.com/fake.png',format='png',nsId=2); drain(world)
    assert results(world,bid)[0]['error']=='invalid_image'
    with world.app.app_context():
        storage=world.app.extensions['photoly'].storage
        assert storage.usage('user-alice')[0]==before
        assert not list((storage.root/'.staging').iterdir())


def test_remote_stream_bound_redirect_limit_and_timeout(monkeypatch,tmp_path):
    from app.infrastructure import remote_image as remote
    monkeypatch.setattr(socket,'getaddrinfo',lambda *a,**k:[(0,0,0,'',('93.184.216.34',80))])
    class Response:
        status=200; headers={}
        def read1(self,*a,**k): return b'12345'
        def close(self): pass
    class Pool:
        def __init__(self,*a,**k): pass
        def urlopen(self,*a,**k): return Response()
        def close(self): pass
    monkeypatch.setattr(remote.urllib3,'HTTPConnectionPool',Pool)
    config={'ONLINE_DOWNLOAD_TIMEOUT':60,'ONLINE_DOWNLOAD_CONNECT_TIMEOUT':5,'ONLINE_DOWNLOAD_MAX_BYTES':8,'UPLOAD_DISK_RESERVE_BYTES':0}
    with pytest.raises(BusinessError,match='download_too_large'): download_image('http://example.com/a',tmp_path/'out',config)
    Response.status=302; Response.headers={'Location':'/again'}
    with pytest.raises(BusinessError,match='too_many_redirects'): download_image('http://example.com/a',tmp_path/'out',config)
    config['ONLINE_DOWNLOAD_TIMEOUT']=-1
    with pytest.raises(BusinessError,match='download_timeout'): download_image('http://example.com/a',tmp_path/'out',config)


def test_media_migration_upgrade_from_uploads_preserves_records(world,migration_engine):
    from alembic import command
    from app.infrastructure.schema import migration_config
    from sqlalchemy import text
    with migration_engine.begin() as connection:
        config=migration_config(); config.attributes['connection']=connection
        command.downgrade(config,'0006_uploads_storage')
        assert connection.scalar(text('SELECT count(*) FROM photo')) == 2
        command.upgrade(config,'head')
        assert connection.scalar(text("SELECT setting_value FROM setting WHERE setting_name='OnlineDownload'")) == '0'
        connection.execute(text("UPDATE setting SET setting_value='1' WHERE setting_name='OnlineDownload'"))
        command.upgrade(config,'head')
        assert connection.scalar(text("SELECT setting_value FROM setting WHERE setting_name='OnlineDownload'")) == '1'
        assert connection.scalar(text('SELECT count(*) FROM photo')) == 2


def test_cancel_between_pair_inspection_and_claim_does_not_resurrect_items(world,apple_pair,monkeypatch):
    from app.services import live_import
    still,movie=apple_pair
    bid,rows=batch(world,[('cover.jpg',still),('movie.mov',movie)])
    for row,content in zip(rows,(still,movie)): receive(world,bid,row,content)
    real=live_import.materialize
    def cancel(service,item,destination):
        real(service,item,destination)
        if destination.name == 'live.mov': service.cancel(2,bid)
    monkeypatch.setattr(live_import,'materialize',cancel)
    drain(world)
    assert {row['state'] for row in results(world,bid)}=={'cancelled'}
