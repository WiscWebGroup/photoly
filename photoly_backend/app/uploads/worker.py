"""A process-held PostgreSQL advisory lock makes interrupted work reclaimable immediately."""
import json
import logging
import time
from datetime import datetime, timezone
from sqlalchemy import select, text
from app import create_app
from app.entities import UploadItem
from app.extensions import db
from app.services.uploads import UploadService

log = logging.getLogger(__name__)


def tick(app):
    service = UploadService(db.session, app.extensions['photoly'])
    candidates = list(db.session.scalars(select(UploadItem.id).where(
        UploadItem.state.in_(['ready','processing'])).order_by(UploadItem.updated_at, UploadItem.id).limit(100)))
    db.session.rollback()
    for item_id in candidates:
        # A dedicated connection retains the session lock across service commits.
        with db.engine.connect() as lock:
            acquired = lock.scalar(text('SELECT pg_try_advisory_lock(hashtextextended(:id,74831004))'), {'id': item_id})
            lock.commit()
            if not acquired:
                continue
            try:
                service.process(item_id)
            finally:
                lock.execute(text('SELECT pg_advisory_unlock(hashtextextended(:id,74831004))'), {'id': item_id})
                lock.commit()
        return True
    return False


def main():
    logging.basicConfig(level=logging.INFO)
    app = create_app()
    cleanup_at = 0
    while True:
        busy = False
        with app.app_context():
            try:
                service = UploadService(db.session, app.extensions['photoly'])
                heartbeat = service.storage.root / '.upload-worker.json'
                heartbeat.write_text(json.dumps({'updatedAt': datetime.now(timezone.utc).isoformat()}))
                busy = tick(app)
                if time.monotonic() >= cleanup_at:
                    service.cleanup()
                    cleanup_at = time.monotonic() + 300
            except Exception:
                db.session.rollback()
                log.exception('Upload worker cycle failed')
        if not busy:
            time.sleep(1)


if __name__ == '__main__':
    main()
