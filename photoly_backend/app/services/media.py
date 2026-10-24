"""Owner-scoped media resources and asynchronous derivative jobs."""
import shutil
import tempfile
from pathlib import Path
from uuid import UUID, uuid5
from sqlalchemy import select
from werkzeug.datastructures import FileStorage
from app.entities import Photo, PhotoExtra, PhotoMedia, Namespace, User, UploadBatch, UploadItem
from app.infrastructure.media import TARGETS, DECODABLE, asset_path, image_info, convert_bounded, encode_live
from app.infrastructure.remote_image import download_bounded
from app.services.base import BaseService, BusinessError


class MediaService(BaseService):
    def capabilities(self):
        return {'onlineDownload': self.setting('OnlineDownload', '0') == '1', 'convertFormats': list(TARGETS)}

    def require_online(self):
        if not self.capabilities()['onlineDownload']:
            raise BusinessError('online_download_disabled', 403)

    def info(self, uid, photo_id):
        photo = self.owned(Photo, photo_id, uid, 'photo_not_found', 404)
        media = self.session.get(PhotoMedia, photo_id)
        animated = media.animated if media else False
        if media is None and photo.format.lower() in DECODABLE:
            try:
                animated = image_info(self.storage.media(self.session.get(User, uid).uuid, photo))['animated']
            except BusinessError:
                pass
        return {'photoId': photo_id, 'animated': animated, 'live': bool(media and media.live),
                'liveStatus': media.live_status if media else None, 'liveError': media.live_error if media else None,
                'convertFormats': list(TARGETS) if photo.format.lower() in DECODABLE else [], 'nsId': photo.ns_id}

    def submit(self, uid, data, kind):
        from app.services.uploads import identifier, now
        key = identifier(data.get('id'))
        resource_id = data.get('nsId' if kind == 'online' else 'photoId')
        if type(resource_id) is not int or resource_id <= 0:
            raise BusinessError('invalid_resource_id')
        target = data.get('format')
        if kind != 'live_preview' and target not in TARGETS:
            raise BusinessError('unsupported_target_format')
        # Options are compared on retries; IDs cannot be reused for a different request.
        request = {k: data.get(k) for k in (('url', 'nsId', 'format') if kind == 'online' else ('photoId', 'format'))}
        with self.atomic(uid):
            existing = self.session.get(UploadBatch, key)
            if existing:
                if existing.user_id != uid:
                    raise BusinessError('upload_id_conflict', 409)
                previous = self.session.scalar(select(UploadItem).where(UploadItem.batch_id == key))
                if previous is None or previous.kind != kind or (previous.options or {}).get('request') != request:
                    raise BusinessError('upload_id_conflict', 409)
                return key
            options = {'request': request, 'format': target}
            if kind == 'online':
                self.require_online()
                from urllib.parse import urlsplit
                url = data.get('url')
                if not isinstance(url, str) or len(url) > 8192 or any(ord(c) < 33 for c in url) or '\\' in url:
                    raise BusinessError('invalid_image_url')
                try:
                    parsed = urlsplit(url)
                    if parsed.scheme not in ('http', 'https') or not parsed.hostname or parsed.username is not None or parsed.password is not None:
                        raise ValueError()
                except (ValueError, TypeError, AttributeError):
                    raise BusinessError('invalid_image_url')
                options['url'] = url
                ns_id, name = data.get('nsId'), 'Online Download'
            else:
                photo = self.owned(Photo, data.get('photoId'), uid, 'photo_not_found', 404)
                if photo.format.lower() not in DECODABLE:
                    raise BusinessError('unsupported_image')
                if kind == 'live_preview':
                    media = self.session.get(PhotoMedia, photo.photo_id)
                    if not media or not media.live:
                        raise BusinessError('not_live_photo')
                options.update(photoId=photo.photo_id, version=photo.content_version)
                ns_id, name = photo.ns_id, ('Format Convert' if kind == 'convert' else 'Live preview')
            self.owned(Namespace, ns_id, uid, 'destination_deleted', 404)
            self.session.add(UploadBatch(id=key, user_id=uid, ns_id=ns_id, name=name, sealed=True,
                created_at=now(), updated_at=now(), folders={}))
            self.session.flush()
            self.session.add(UploadItem(id=str(uuid5(UUID(key), kind)), batch_id=key, path=name,
                size=0, state='ready', kind=kind, options=options, updated_at=now()))
        return key

    def checked_source(self, uid, options):
        photo = self.owned(Photo, options['photoId'], uid, 'source_changed', 409)
        if photo.content_version != options['version']:
            raise BusinessError('source_changed', 409)
        return photo

    def process(self, item_id):
        from app.services.uploads import UploadService, now
        from app.services.photo import PhotoService
        uploads = UploadService(self.session, self.infra)
        item = self.session.get(UploadItem, item_id)
        batch = self.session.get(UploadBatch, item.batch_id)
        uid, kind, options = batch.user_id, item.kind, dict(item.options)
        target = options.get('format')
        staging = self.storage.root / '.staging'
        staging.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=staging) as directory:
            work = Path(directory)
            source, output = work / 'source', work / ('output.' + ('mp4' if kind == 'live_preview' else target))
            original_id = None
            if kind == 'online':
                self.require_online()
                self.session.rollback()
                download_bounded(options['url'], source, self.infra.config)
                name = 'Downloaded image'
            else:
                photo = self.checked_source(uid, options)
                original_id, name = photo.photo_id, photo.photo_name
                user = self.session.get(User, uid)
                media = self.session.get(PhotoMedia, photo.photo_id)
                use_video = bool(media and media.live and (target == 'gif' or kind == 'live_preview'))
                path = asset_path(self.storage, user.uuid, photo, 'live_original') if use_video else self.storage.media(user.uuid, photo)
                # Open before releasing the transaction: replacement/deletion cannot change the open inode.
                try:
                    with path.open('rb') as stream, source.open('wb') as out:
                        shutil.copyfileobj(stream, out)
                except FileNotFoundError:
                    raise BusinessError('source_changed', 409)
                self.session.rollback()
                if use_video:
                    encode_live(source, output, gif=target == 'gif', timeout=self.infra.config['MEDIA_PROCESS_TIMEOUT'])
            if not output.exists():
                convert_bounded(source, output, target, self.infra.config['MEDIA_PROCESS_TIMEOUT'])
            if output.stat().st_size > self.infra.config['MAX_CONTENT_LENGTH']:
                raise BusinessError('file_too_large', 413)
            prepared = PhotoService(self.session, self.infra).prepare(output, work) if kind != 'live_preview' else None
            with self.atomic(uid) as files:
                item = self.session.get(UploadItem, item_id, populate_existing=True)
                batch = uploads.batch(uid, item.batch_id)
                if batch.cancelled or item.state != 'processing':
                    return
                if kind == 'online':
                    self.require_online()
                else:
                    original = self.checked_source(uid, options)
                self.owned(Namespace, batch.ns_id, uid, 'destination_deleted', 404)
                user = self.session.get(User, uid)
                if kind == 'live_preview':
                    media = self.session.get(PhotoMedia, original_id)
                    if not media or not media.live:
                        raise BusinessError('source_changed', 409)
                    with output.open('rb') as stream:
                        files.install(stream, asset_path(self.storage, user.uuid, original, 'live_preview'))
                    media.live_status, media.live_error = 'ready', None
                    photo = original
                else:
                    stem = name[:230] + (' (converted)' if kind == 'convert' else '')
                    candidate, number = stem, 1
                    while self.session.scalar(select(Photo.photo_id).where(Photo.user_id == uid, Photo.ns_id == batch.ns_id,
                            Photo.photo_name == candidate, Photo.format == target)) is not None:
                        candidate, number = f'{stem} ({number})', number + 1
                    with output.open('rb') as stream:
                        photo = PhotoService(self.session, self.infra).install_one(user,
                            FileStorage(stream, filename=candidate + '.' + target), {'nsId': batch.ns_id}, files, prepared)
                    if original_id:
                        self.session.flush()
                        old = self.session.scalar(select(PhotoExtra).where(PhotoExtra.f_photo_id == original_id))
                        new = self.session.scalar(select(PhotoExtra).where(PhotoExtra.f_photo_id == photo.photo_id))
                        if old and new:
                            for column in PhotoExtra.__table__.columns:
                                if column.name.startswith('extra_photo_') and not column.primary_key:
                                    setattr(new, column.name, getattr(old, column.name))
                item.photo_id, item.ns_id, item.size, item.state, item.error = photo.photo_id, photo.ns_id, output.stat().st_size, 'done', None
                item.updated_at = batch.updated_at = now()

    def live_path(self, uid, photo_id):
        photo = self.owned(Photo, photo_id, uid, None, 404)
        media = self.session.get(PhotoMedia, photo_id)
        if not media or not media.live or media.live_status != 'ready':
            raise BusinessError(None, 404)
        path = asset_path(self.storage, self.session.get(User, uid).uuid, photo, 'live_preview')
        if not path.is_file():
            raise BusinessError(None, 404)
        return path

    def live_archive(self, uid, photo_id):
        photo = self.owned(Photo, photo_id, uid, None, 404)
        media = self.session.get(PhotoMedia, photo_id)
        if not media or not media.live:
            raise BusinessError(None, 404)
        user = self.session.get(User, uid)
        return self.storage.archive([(self.storage.media(user.uuid, photo), photo.photo_name + '.' + photo.format, photo_id),
            (asset_path(self.storage, user.uuid, photo, 'live_original'), photo.photo_name + '.mov', photo_id)])
