"""Inspect a sealed upload manifest before pairing Apple assets within a directory."""
import tempfile
from pathlib import Path, PurePosixPath
from zipfile import ZipFile
from sqlalchemy import select
from app.entities import UploadItem, UploadBatch, User, PhotoMedia
from app.infrastructure.media import asset_path, encode_live
from app.services.base import BusinessError

COVERS = {'jpg', 'jpeg', 'heic', 'heif'}
CANDIDATES = COVERS | {'mov'}


def materialize(service, item, destination):
    from app.services.uploads import CheckedStream
    def copy(stream):
        checked = CheckedStream(stream, item.size, service.storage.root, service.infra.config['UPLOAD_DISK_RESERVE_BYTES'])
        with destination.open('wb') as out:
            while chunk := checked.read(1024 * 1024):
                out.write(chunk)
    if item.source_id:
        with ZipFile(service.source(item.source_id)) as archive:
            entry = archive.infolist()[item.entry_index]
            with archive.open(entry) as stream:
                copy(stream)
    else:
        with service.source(item.id).open('rb') as stream:
            copy(stream)


def process_import(service, item_id):
    from app.services.uploads import now, suffix, TERMINAL
    item = service.session.get(UploadItem, item_id)
    if suffix(item.path) == 'zip':
        return False
    batch = service.session.get(UploadBatch, item.batch_id)
    uid = batch.user_id
    staging = service.storage.root / '.staging'
    staging.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(dir=staging) as directory:
        work = Path(directory)
        original = work / ('source.' + suffix(item.path))
        materialize(service, item, original)
        if item.media_metadata is None and suffix(item.path) in CANDIDATES:
            metadata = service.infra.metadata.read(original)
            identifier = metadata.get('ContentIdentifier')
            # File type is checked too; a renamed arbitrary file is never a Live component.
            file_type = str(metadata.get('FileType', '')).upper()
            valid = file_type in ({'MOV'} if suffix(item.path) == 'mov' else {'JPEG', 'HEIC', 'HEIF'})
            pairing = {'identifier': identifier if valid and isinstance(identifier, str) and 0 < len(identifier) <= 255 else None}
            with service.atomic(uid):
                item = service.session.get(UploadItem, item_id, populate_existing=True)
                if item.state != 'processing':
                    return True
                item.media_metadata = pairing
        identifier = (item.media_metadata or {}).get('identifier')
        from app.services.photo import PhotoService
        photos = PhotoService(service.session, service.infra)
        if identifier:
            siblings = service.session.scalars(select(UploadItem).where(UploadItem.batch_id == batch.id, UploadItem.kind == 'upload')).all()
            parent = PurePosixPath(item.path).parent
            candidates = [i for i in siblings if PurePosixPath(i.path).parent == parent and suffix(i.path) in CANDIDATES]
            # A pending archive can add candidates; never pair before its manifest is known.
            unsettled_zip = any(suffix(i.path) == 'zip' and i.state not in TERMINAL for i in siblings)
            incomplete = not batch.sealed or unsettled_zip or any(i.state not in TERMINAL and i.media_metadata is None for i in candidates)
            if incomplete:
                with service.atomic(uid):
                    item = service.session.get(UploadItem, item_id, populate_existing=True)
                    if item.state == 'processing':
                        item.state, item.updated_at = 'waiting', now()
                return True
            matches = [i for i in candidates if (i.media_metadata or {}).get('identifier') == identifier]
            covers = [i for i in matches if suffix(i.path) in COVERS]
            movies = [i for i in matches if suffix(i.path) == 'mov']
            if len(covers) == len(movies) == 1 and all(i.state in ('ready', 'waiting', 'processing') for i in matches):
                cover, movie = covers[0], movies[0]
                # The caller holds a batch lock; either component can finish the pair.
                if item.id != cover.id:
                    original = work / ('cover.' + suffix(cover.path))
                    materialize(service, cover, original)
                movie_path = work / 'live.mov'
                materialize(service, movie, movie_path)
                with service.atomic(uid):
                    cover = service.session.get(UploadItem, cover.id, populate_existing=True)
                    movie = service.session.get(UploadItem, movie.id, populate_existing=True)
                    batch = service.batch(uid, batch.id)
                    if batch.cancelled or any(i.state not in ('ready', 'waiting', 'processing') for i in (cover, movie)):
                        return True
                    cover.pair_id, movie.pair_id = movie.id, cover.id
                    cover.state = movie.state = 'processing'
                    cover.updated_at = movie.updated_at = now()
                preview = work / 'live.mp4'
                failure = None
                service.session.rollback()
                try:
                    encode_live(movie_path, preview, timeout=service.infra.config['MEDIA_PROCESS_TIMEOUT'])
                    if preview.stat().st_size > service.infra.config['MAX_CONTENT_LENGTH']:
                        raise BusinessError('file_too_large')
                except BusinessError as error:
                    failure = str(error.value)
                prepared = photos.prepare(original, work)
                with service.atomic(uid) as files:
                    cover = service.session.get(UploadItem, cover.id, populate_existing=True)
                    movie = service.session.get(UploadItem, movie.id, populate_existing=True)
                    batch = service.batch(uid, batch.id)
                    if batch.cancelled or any(i.state not in ('ready', 'waiting', 'processing') for i in (cover, movie)):
                        return True
                    ns_id = service.folder(batch, cover.path)
                    with original.open('rb') as stream:
                        service.install(cover, uid, ns_id, stream, files, prepared)
                    user = service.session.get(User, uid)
                    from app.entities import Photo
                    photo = service.session.get(Photo, cover.photo_id)
                    with movie_path.open('rb') as stream:
                        files.install(stream, asset_path(service.storage, user.uuid, photo, 'live_original'))
                    if not failure:
                        with preview.open('rb') as stream:
                            files.install(stream, asset_path(service.storage, user.uuid, photo, 'live_preview'))
                    media = service.session.get(PhotoMedia, photo.photo_id)
                    media.live, media.live_status, media.live_error = True, ('failed' if failure else 'ready'), failure
                    cover.pair_id, movie.pair_id = movie.id, cover.id
                    movie.photo_id, movie.ns_id, movie.state, movie.error = cover.photo_id, ns_id, 'done', None
                    cover.updated_at = movie.updated_at = batch.updated_at = now()
                return True
        # No reliable unique counterpart: import exactly as a normal file.
        prepared = photos.prepare(original, work)
        with service.atomic(uid) as files:
            item = service.session.get(UploadItem, item_id, populate_existing=True)
            batch = service.batch(uid, item.batch_id)
            if batch.cancelled or item.state != 'processing':
                return True
            ns_id = service.folder(batch, item.path)
            with original.open('rb') as stream:
                service.install(item, uid, ns_id, stream, files, prepared)
            item.updated_at = batch.updated_at = now()
        return True
