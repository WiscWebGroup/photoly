"""Owned media OCR with optimistic editing and a database-wide process slot."""
from contextlib import contextmanager
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
from sqlalchemy import select, text
from sqlalchemy.exc import DBAPIError
from app.entities import Photo, PhotoOCR, User
from app.services.base import BaseService, BusinessError
from app.ocr import LANGUAGES, MAX_BYTES, MAX_TEXT, TOTAL_SECONDS
from app.services.photo import IMAGE_FORMATS

# Session advisory lock: separate autocommit connection, no long-running DB transaction.
OCR_LOCK = 74831004


def fail(reason, code=400):
    raise BusinessError({'error': reason}, code)


@contextmanager
def processing_slot(engine):
    with engine.connect().execution_options(isolation_level='AUTOCOMMIT') as connection:
        acquired = connection.execute(text('SELECT pg_try_advisory_lock(:key)'), {'key': OCR_LOCK}).scalar()
        if not acquired:
            fail('ocr_busy', 429)
        try:
            yield
        finally:
            connection.execute(text('SELECT pg_advisory_unlock(:key)'), {'key': OCR_LOCK})


def run_worker(source, directory, deadline):
    remaining = deadline - time.monotonic()
    if remaining <= 0:
        fail('ocr_timeout', 504)
    process = subprocess.Popen([sys.executable, '-m', 'app.ocr.worker', str(source), str(directory)],
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
    try:
        stdout, _ = process.communicate(timeout=remaining)
    except BaseException:
        # Kill the entire group, including Tesseract if decoding or OCR hangs.
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        process.communicate()
        if sys.exc_info()[0] is subprocess.TimeoutExpired:
            fail('ocr_timeout', 504)
        raise
    try:
        result = json.loads(stdout) if process.returncode == 0 else {}
    except (ValueError, UnicodeError):
        result = {}
    if 'error' in result:
        reason = result['error']
        fail(reason, 504 if reason == 'ocr_timeout' else 503 if reason.endswith('unavailable') else 400)
    if not isinstance(result.get('text'), str) or len(result['text']) > MAX_TEXT or '\x00' in result['text']:
        fail('recognition_failed')
    return result


class OCRService(BaseService):
    def photo(self, uid, photo_id, lock=False, deadline=None):
        if type(photo_id) is not int or photo_id < 1:
            fail('invalid_request')
        if lock:
            # An upload may hold this user lock. Waiting must not escape the OCR deadline.
            remaining = (deadline - time.monotonic()) if deadline is not None else 5
            if remaining <= 0:
                fail('ocr_timeout', 504)
            self.session.execute(text("SELECT set_config('lock_timeout', :value, true)"),
                                 {'value': f'{max(1, int(remaining * 1000))}ms'})
            try:
                self.session.scalar(select(User).where(User.user_id == uid).with_for_update())
            except DBAPIError as error:
                if getattr(error.orig, 'sqlstate', None) == '55P03':
                    self.session.rollback()
                    fail('ocr_timeout' if deadline is not None else 'ocr_busy', 504 if deadline is not None else 429)
                raise
        photo = self.session.scalar(select(Photo).where(Photo.photo_id == photo_id, Photo.user_id == uid)
                                    .execution_options(populate_existing=True))
        if photo is None:
            fail('photo_not_found', 404)
        if photo.format.lower() not in IMAGE_FORMATS:
            fail('unsupported_format')
        return photo

    def response(self, photo, row):
        return {'photoId': photo.photo_id, 'contentVersion': photo.content_version,
                'revision': row.revision if row else 0, 'text': row.text if row else None,
                'languages': LANGUAGES, 'engineVersion': row.engine_version if row else None,
                'updatedAt': row.updated_at.isoformat() if row else None}

    def get(self, uid, photo_id):
        photo = self.photo(uid, photo_id)
        row = self.session.get(PhotoOCR, photo_id)
        return self.response(photo, row if row and row.content_version == photo.content_version else None)

    def validate(self, photo, data):
        if type(data.get('contentVersion')) is not int or type(data.get('revision')) is not int:
            fail('invalid_request')
        row = self.session.get(PhotoOCR, photo.photo_id, populate_existing=True)
        if photo.content_version != data['contentVersion'] or (row.revision if row else 0) != data['revision']:
            fail('ocr_conflict', 409)
        return row

    def save(self, uid, data, recognized=None, deadline=None):
        value = recognized['text'] if recognized is not None else data.get('text')
        if not isinstance(value, str) or len(value) > MAX_TEXT or '\x00' in value:
            fail('invalid_text')
        try:
            photo = self.photo(uid, data.get('photoId'), lock=True, deadline=deadline)
            row = self.validate(photo, data)
            if row is None:
                row = PhotoOCR(photo_id=photo.photo_id, revision=0)
                self.session.add(row)
            row.content_version, row.text, row.languages = photo.content_version, value, LANGUAGES
            if recognized is not None:
                row.engine_version = recognized['engineVersion']
            row.revision += 1
            row.updated_at = datetime.now(timezone.utc)
            self.session.flush()
            result = self.response(photo, row)
            self.session.commit()
            return result
        except BaseException:
            self.session.rollback()
            raise

    def recognize(self, uid, data):
        deadline = time.monotonic() + TOTAL_SECONDS
        # Authenticate ownership before competing for the global processing slot.
        self.photo(uid, data.get('photoId'))
        self.session.rollback()
        with processing_slot(self.session.get_bind()), tempfile.TemporaryDirectory(prefix='photoly-ocr-') as work:
            directory = Path(work)
            source = directory / 'source'
            try:
                photo = self.photo(uid, data.get('photoId'), lock=True, deadline=deadline)
                self.validate(photo, data)
                user = self.session.get(User, uid)
                # Writers replace inodes under the same user lock. This descriptor remains stable.
                stream = self.storage.media(user.uuid, photo).open('rb')
            except OSError:
                fail('image_missing', 404)
            finally:
                self.session.rollback()
            with stream, source.open('wb') as output:
                count = 0
                while chunk := stream.read(1024 * 1024):
                    count += len(chunk)
                    if count > MAX_BYTES:
                        fail('image_too_large', 413)
                    if time.monotonic() >= deadline:
                        fail('ocr_timeout', 504)
                    output.write(chunk)
            recognized = run_worker(source, directory, deadline)
            if time.monotonic() >= deadline:
                fail('ocr_timeout', 504)
            return self.save(uid, data, recognized, deadline=deadline)
