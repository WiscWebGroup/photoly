"""Durable, idempotent imports. Browser transmission and server processing are separate."""
import hashlib
import os
import shutil
import stat
from datetime import datetime, timezone, timedelta
from pathlib import PurePosixPath
from uuid import UUID, uuid4, uuid5
from zipfile import ZipFile, BadZipFile
from werkzeug.datastructures import FileStorage
from sqlalchemy import select, func, delete
from app.entities import UploadBatch, UploadItem, Namespace, User
from app.services.base import BaseService, BusinessError, valid_name
from app.services.photo import PhotoService, IMAGE_FORMATS, VIDEO_FORMATS

TERMINAL = ('done', 'failed', 'skipped', 'cancelled', 'expanded')
MEDIA = IMAGE_FORMATS | VIDEO_FORMATS


def now():
    return datetime.now(timezone.utc)


def identifier(value):
    try:
        return str(UUID(value))
    except (ValueError, TypeError, AttributeError):
        raise BusinessError('invalid_upload_id')


def relative_path(value):
    if not isinstance(value, str) or len(value) > 4096 or '\\' in value or value.startswith('/'):
        raise BusinessError('unsafe_path')
    parts = value.split('/')
    for part in parts:
        valid_name(part)
        if ':' in part:
            raise BusinessError('unsafe_path')
    return '/'.join(parts)


def suffix(path):
    return PurePosixPath(path).suffix[1:].lower()


def item_json(item):
    return {'id': item.id, 'batchId': item.batch_id, 'path': item.path, 'size': item.size,
            'state': item.state, 'error': item.error, 'photoId': item.photo_id,
            'nsId': item.ns_id, 'sourceId': item.source_id}


class CheckedStream:
    def __init__(self, stream, expected, root, reserve):
        self.stream, self.expected, self.root, self.reserve = stream, expected, root, reserve
        self.count = 0

    def read(self, length):
        chunk = self.stream.read(length)
        self.count += len(chunk)
        if self.count > self.expected:
            raise BusinessError('file_size_mismatch')
        if chunk and shutil.disk_usage(self.root).free < len(chunk) + self.reserve:
            raise BusinessError('insufficient_disk_space', 507)
        if not chunk and self.count != self.expected:
            raise BusinessError('file_size_mismatch')
        return chunk


class UploadService(BaseService):
    def source(self, item_id):
        return self.storage.root / '.uploads' / identifier(item_id)

    def batch(self, uid, batch_id):
        batch = self.session.scalar(select(UploadBatch).where(UploadBatch.id == identifier(batch_id))
                                    .execution_options(populate_existing=True))
        if batch is None or batch.user_id != uid:
            raise BusinessError('upload_not_found', 404)
        return batch

    def create(self, uid, data):
        batch_id = identifier(data.get('id'))
        with self.atomic(uid):
            existing = self.session.get(UploadBatch, batch_id)
            if existing:
                self.batch(uid, batch_id)
                return batch_id
            self.owned(Namespace, data.get('nsId'), uid)
            self.session.add(UploadBatch(id=batch_id, user_id=uid, ns_id=data['nsId'],
                name=valid_name(data.get('name')), created_at=now(), updated_at=now(), folders={}))
        return batch_id

    def register(self, uid, batch_id, entries):
        if not isinstance(entries, list) or not 1 <= len(entries) <= 500:
            raise BusinessError('invalid_manifest_page')
        with self.atomic(uid):
            batch = self.batch(uid, batch_id)
            if batch.cancelled or batch.sealed:
                raise BusinessError('upload_closed', 409)
            for data in entries:
                if not isinstance(data, dict):
                    raise BusinessError('invalid_entry')
                key, path, size = identifier(data.get('id')), relative_path(data.get('path')), data.get('size')
                if type(size) is not int or size < 0 or size > 9007199254740991:
                    raise BusinessError('invalid_size')
                old = self.session.get(UploadItem, key)
                if old:
                    if old.batch_id != batch.id or old.path != path or old.size != size:
                        raise BusinessError('upload_id_conflict', 409)
                    continue
                state, error = 'pending', None
                if suffix(path) not in MEDIA | {'zip'}:
                    state, error = 'skipped', 'unsupported_file_type'
                elif size > self.infra.config['MAX_CONTENT_LENGTH']:
                    state, error = 'failed', 'file_too_large'
                self.session.add(UploadItem(id=key, batch_id=batch.id, path=path, size=size,
                                           state=state, error=error, updated_at=now()))
                self.session.flush()
            batch.updated_at = now()
        return 1

    def receive(self, uid, batch_id, item_id, stream):
        batch = self.batch(uid, batch_id)
        item = self.session.get(UploadItem, identifier(item_id))
        if item is None or item.batch_id != batch.id or item.source_id:
            raise BusinessError('upload_not_found', 404)
        if item.state != 'pending':
            return item_json(item)
        if batch.cancelled:
            raise BusinessError('upload_cancelled', 409)
        expected = item.size
        # Do not hold a user/DB lock while bytes travel over the network.
        self.session.rollback()
        directory = self.storage.root / '.uploads'
        directory.mkdir(parents=True, exist_ok=True)
        temporary = directory / (str(uuid4()) + '.part')
        try:
            checked = CheckedStream(stream, expected, self.storage.root, self.infra.config['UPLOAD_DISK_RESERVE_BYTES'])
            digest = hashlib.sha256()
            with temporary.open('xb') as output:
                while chunk := checked.read(1024 * 1024):
                    digest.update(chunk)
                    output.write(chunk)
            with self.atomic(uid) as files:
                batch = self.batch(uid, batch_id)
                item = self.session.get(UploadItem, item_id)
                if batch.cancelled or item is None:
                    raise BusinessError('upload_cancelled', 409)
                if item.state == 'pending':
                    if item.content_sha256 is not None and item.content_sha256 != digest.hexdigest():
                        raise BusinessError('source_changed', 409)
                    item.content_sha256 = digest.hexdigest()
                    # Filesystem compensation covers a failed DB commit.
                    destination = self.source(item.id)
                    files.remove(destination)
                    os.replace(temporary, destination)
                    files.created.append(destination)
                    item.state, item.error, item.updated_at = 'ready', None, now()
                    batch.updated_at = now()
            return item_json(item)
        finally:
            temporary.unlink(missing_ok=True)

    def finish(self, uid, batch_id):
        with self.atomic(uid):
            batch = self.batch(uid, batch_id)
            batch.sealed, batch.updated_at = True, now()
        return 1

    def listing(self, uid, page=1, batch_id=None):
        if type(page) is not int or page < 1:
            raise BusinessError('invalid_page')
        if batch_id:
            batch = self.batch(uid, batch_id)
            query = select(UploadItem).where(UploadItem.batch_id == batch.id)
            count = self.session.scalar(select(func.count()).select_from(query.subquery()))
            items = self.session.scalars(query.order_by(UploadItem.path, UploadItem.id).offset((page-1)*50).limit(50))
            return {'items': [item_json(i) for i in items], 'total': count}
        query = select(UploadBatch).where(UploadBatch.user_id == uid)
        total = self.session.scalar(select(func.count()).select_from(query.subquery()))
        batches = self.session.scalars(query.order_by(UploadBatch.created_at.desc(), UploadBatch.id).offset((page-1)*20).limit(20)).all()
        counts = {}
        if batches:
            for bid, state, count, size in self.session.execute(select(UploadItem.batch_id, UploadItem.state, func.count(), func.sum(UploadItem.size))
                    .where(UploadItem.batch_id.in_([b.id for b in batches])).group_by(UploadItem.batch_id, UploadItem.state)):
                counts.setdefault(bid, {})[state] = {'count': count, 'bytes': size}
        active_count = self.session.scalar(select(func.count()).select_from(UploadItem).join(UploadBatch)
            .where(UploadBatch.user_id == uid, UploadItem.state.in_(['pending','ready','processing'])))
        failed_count = self.session.scalar(select(func.count()).select_from(UploadItem).join(UploadBatch)
            .where(UploadBatch.user_id == uid, UploadItem.state == 'failed'))
        return {'activeCount': active_count, 'failedCount': failed_count, 'batches': [{'id': b.id, 'name': b.name, 'nsId': b.ns_id, 'sealed': b.sealed,
                             'cancelled': b.cancelled, 'createdAt': b.created_at.isoformat(),
                             'counts': counts.get(b.id, {})} for b in batches], 'total': total,
                'maxFileBytes': self.infra.config['MAX_CONTENT_LENGTH']}

    def cancel(self, uid, batch_id):
        with self.atomic(uid):
            batch = self.batch(uid, batch_id)
            batch.cancelled, batch.sealed, batch.updated_at = True, True, now()
            self.session.query(UploadItem).filter(UploadItem.batch_id == batch.id,
                UploadItem.state.in_(['pending', 'ready', 'processing'])).update(
                    {'state': 'cancelled', 'updated_at': now()}, synchronize_session=False)
        return 1

    def retry(self, uid, batch_id):
        with self.atomic(uid):
            batch = self.batch(uid, batch_id)
            batch.cancelled, batch.updated_at = False, now()
            for item in self.session.scalars(select(UploadItem).where(UploadItem.batch_id == batch.id,
                    UploadItem.state.in_(['failed', 'cancelled']))):
                if item.error == 'file_too_large':
                    continue
                available = self.source(item.source_id or item.id).is_file()
                item.state = 'ready' if available else 'pending'
                item.error, item.updated_at = (None if available else 'source_expired'), now()
                if not available and item.source_id:
                    source = self.session.get(UploadItem, item.source_id)
                    source.state, source.error, source.updated_at = 'pending', 'source_expired', now()
        return 1

    def folder(self, batch, path):
        self.owned(Namespace, batch.ns_id, batch.user_id, 'destination_deleted', 404)
        mapping = dict(batch.folders)
        parent = batch.ns_id
        parts = path.split('/')[:-1]
        for index, name in enumerate(parts):
            key = '/'.join(parts[:index+1])
            if key in mapping:
                self.owned(Namespace, mapping[key], batch.user_id, 'destination_deleted', 404)
                parent = mapping[key]
                continue
            candidate, number = name, 1
            # Never merge into existing folders, including a new name collision after deletion.
            while self.session.scalar(select(Namespace.ns_id).where(Namespace.user_id == batch.user_id,
                    Namespace.ns_parent_id == parent, Namespace.ns_name == candidate)) is not None:
                tail = f' ({number})'
                candidate, number = name[:255-len(tail)] + tail, number + 1
            folder = Namespace(ns_name=candidate, ns_parent_id=parent, user_id=batch.user_id)
            self.session.add(folder)
            self.session.flush()
            parent = mapping[key] = folder.ns_id
        batch.folders = mapping
        return parent

    def expand(self, item, batch):
        with ZipFile(self.source(item.id)) as archive:
            entries, size = archive.infolist(), 0
            for entry in entries:
                relative_path(entry.filename.rstrip('/'))
                mode = entry.external_attr >> 16
                if entry.flag_bits & 1 or stat.S_ISLNK(mode) or (stat.S_IFMT(mode) not in (0, stat.S_IFREG, stat.S_IFDIR)):
                    raise BusinessError('unsafe_or_encrypted_zip')
                size += entry.file_size
                if size > self.infra.config['UPLOAD_ZIP_MAX_BYTES']:
                    raise BusinessError('zip_expansion_limit')
            if shutil.disk_usage(self.storage.root).free < min(size, self.infra.config['MAX_CONTENT_LENGTH']) + self.infra.config['UPLOAD_DISK_RESERVE_BYTES']:
                raise BusinessError('insufficient_disk_space', 507)
            prefix = str(PurePosixPath(item.path).with_suffix(''))
            for index, entry in enumerate(entries):
                if entry.is_dir():
                    continue
                key = str(uuid5(UUID(item.id), str(index)))
                path = relative_path(prefix + '/' + entry.filename)
                state, error = 'ready', None
                if suffix(path) not in MEDIA:
                    state, error = 'skipped', 'unsupported_file_type'
                elif entry.file_size > self.infra.config['MAX_CONTENT_LENGTH']:
                    state, error = 'failed', 'file_too_large'
                existing = self.session.get(UploadItem, key)
                if existing is None:
                    self.session.add(UploadItem(id=key, batch_id=batch.id, path=path, size=entry.file_size,
                        state=state, error=error, source_id=item.id, entry_index=index, updated_at=now()))
                elif existing.state == 'pending':
                    existing.state, existing.error, existing.updated_at = state, error, now()
                if index % 500 == 0:
                    self.session.flush()
            item.state, item.error = 'expanded', None

    def process(self, item_id):
        item = self.session.get(UploadItem, item_id)
        if item is None or item.state not in ('ready', 'processing'):
            return
        batch = self.session.get(UploadBatch, item.batch_id)
        uid = batch.user_id
        with self.atomic(uid):
            item = self.session.get(UploadItem, item_id, populate_existing=True)
            batch = self.batch(uid, item.batch_id)
            if batch.cancelled or item.state not in ('ready', 'processing'):
                return
            item.state, item.updated_at = 'processing', now()
        try:
            with self.atomic(uid) as files:
                item = self.session.get(UploadItem, item_id, populate_existing=True)
                batch = self.batch(uid, item.batch_id)
                if batch.cancelled or item.state != 'processing':
                    return
                if suffix(item.path) == 'zip' and item.source_id is None:
                    self.expand(item, batch)
                else:
                    ns_id = self.folder(batch, item.path)
                    if item.source_id:
                        with ZipFile(self.source(item.source_id)) as archive:
                            entry = archive.infolist()[item.entry_index]
                            with archive.open(entry) as stream:
                                self.install(item, uid, ns_id, stream, files)
                    else:
                        with self.source(item.id).open('rb') as stream:
                            self.install(item, uid, ns_id, stream, files)
                item.updated_at = batch.updated_at = now()
        except Exception as error:
            self.session.rollback()
            with self.atomic(uid):
                item = self.session.get(UploadItem, item_id, populate_existing=True)
                if item is not None and item.state == 'processing':
                    item.state = 'failed'
                    item.error = str(error.value) if isinstance(error, BusinessError) else (
                        'invalid_zip' if isinstance(error, BadZipFile) else 'processing_failed')
                    item.updated_at = now()
            if not isinstance(error, (BusinessError, BadZipFile)):
                import logging
                logging.getLogger(__name__).exception('Import failed: %s', item_id)

    def install(self, item, uid, ns_id, stream, files):
        checked = CheckedStream(stream, item.size, self.storage.root, self.infra.config['UPLOAD_DISK_RESERVE_BYTES'])
        photo = PhotoService(self.session, self.infra).install_one(self.session.get(User, uid),
            FileStorage(checked, filename=PurePosixPath(item.path).name), {'nsId': ns_id}, files)
        item.photo_id, item.ns_id, item.state, item.error = photo.photo_id, ns_id, 'done', None

    def cleanup(self):
        cutoff = now() - timedelta(hours=self.infra.config['UPLOAD_TEMP_HOURS'])
        history = now() - timedelta(days=self.infra.config['UPLOAD_HISTORY_DAYS'])
        # Acquire the same per-user lock as receiving/processing before expiring rows.
        ids = list(self.session.execute(select(UploadBatch.id, UploadBatch.user_id).where(UploadBatch.updated_at < cutoff)))
        for bid, uid in ids:
            with self.atomic(uid) as files:
                batch = self.session.get(UploadBatch, bid)
                if batch is None or batch.updated_at >= cutoff:
                    continue
                active = self.session.scalar(select(func.count()).select_from(UploadItem).where(
                    UploadItem.batch_id == bid, UploadItem.state.in_(['ready','processing'])))
                if active:
                    continue
                self.session.query(UploadItem).filter(UploadItem.batch_id == bid, UploadItem.state == 'pending').update(
                    {'state': 'failed', 'error': 'transmission_expired', 'updated_at': now()}, synchronize_session=False)
                batch.sealed = True
                for key in self.session.scalars(select(UploadItem.id).where(UploadItem.batch_id == bid, UploadItem.source_id.is_(None))):
                    files.remove(self.source(key))
                if batch.updated_at < history:
                    self.session.delete(batch)
        directory = self.storage.root / '.uploads'
        if directory.exists():
            for path in directory.iterdir():
                if not path.is_file() or datetime.fromtimestamp(path.stat().st_mtime, timezone.utc) >= cutoff:
                    continue
                if path.suffix == '.part' or self.session.get(UploadItem, path.name) is None:
                    path.unlink(missing_ok=True)
        self.session.rollback()
