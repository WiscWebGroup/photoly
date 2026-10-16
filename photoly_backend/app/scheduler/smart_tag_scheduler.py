"""Cooperative Smart Tag scheduler, owned by PhotoEmbeddingScheduler's process/lease."""
import json
import os
import subprocess
import time
from uuid import uuid4
from sqlalchemy import select, or_
from sqlalchemy.dialects.postgresql import insert
from app.entities import Photo, User
from app.entities.ai import AIJob
from app.entities.smart import SmartPreference, SmartMedia
from app.ai.state import control, settings, utcnow
from app.services.smart import eligibility
from app.smart.geography import Gazetteer
from app.smart.metadata import extract
from app.smart.organize import devices, anchor_step, missing_step, item_step, member_step
from app.util.sys_usage_util import system_usage
from .resource_policy import ResourcePolicy
from . import job_repository as jobs

BATCH_SIZE = 25
BUDGET_SECONDS = 5


class SmartTagScheduler:
    def __init__(self, parent):
        self.parent = parent
        self.policy = ResourcePolicy()
        self.gazetteer = Gazetteer()
        self.next_check = 0
        self.last_user = 0
        self.user_cursor = 0

    def reconcile(self, session):
        cfg = settings(session)
        if not cfg['enabled'] or not cfg['smart_tag_enabled']:
            return
        # Round robin discovery. Checkpoints survive process restarts and disabled periods.
        query = select(SmartPreference).join(User).where(SmartPreference.enabled.is_(True), User.ai_search_enabled.is_(True))
        prefs = session.scalars(query.where(SmartPreference.user_id > self.user_cursor).order_by(SmartPreference.user_id).limit(25)).all()
        if not prefs and self.user_cursor:
            prefs = session.scalars(query.order_by(SmartPreference.user_id).limit(25)).all()
        self.user_cursor = prefs[-1].user_id if prefs else 0
        for pref in prefs:
            if pref.profile_id != cfg['search_profile_id'] or pref.revision > pref.completed_revision or pref.checkpoint:
                session.execute(insert(AIJob).values(kind='smart', dedup_key=f'smart:{pref.user_id}',
                    payload={'user_id': pref.user_id}).on_conflict_do_nothing())

    def tick(self):
        with self.parent.sessions.begin() as session:
            cfg = settings(session)
            if time.monotonic() >= self.next_check:
                self.reconcile(session)
                self.next_check = time.monotonic() + 60
            reason = ('ai_disabled' if not cfg['enabled'] else 'smart_tag_disabled' if not cfg['smart_tag_enabled'] else
                'maintenance' if cfg['maintenance'] else 'scheduler_' + cfg['scheduler'] if cfg['scheduler'] != 'running' else None)
            if not reason:
                decision = self.policy.decide(system_usage(path=str(self.parent.storage.root)), 'cpu', cfg)
                reason = ', '.join(decision.reasons) if not decision.allowed else None
            observed = control(session, 'smart')
            observed.observed = {'reason': reason, 'batchSize': BATCH_SIZE, 'budgetSeconds': BUDGET_SECONDS}
            observed.heartbeat_at = utcnow()
            if reason:
                return
            task = jobs.claim(session, self.parent.owner, kinds=('smart',), last_user=self.last_user)
        if task:
            self.last_user = task['payload']['user_id']
            try:
                self.batch(task)
            except Exception as error:
                with self.parent.sessions.begin() as session:
                    job = jobs.current_job(session, task)
                    if job:
                        job.attempts += 1
                        job.status = 'failed' if job.attempts >= 3 else 'queued'
                        job.result = {'reason': 'smart_processing_failed', 'type': type(error).__name__}
                        if job.status == 'failed':
                            pref = session.get(SmartPreference, self.last_user)
                            pref.reason = 'smart_processing_failed'
                            pref.completed_revision = pref.revision
                            pref.checkpoint = {}
                raise

    def fence(self, session, task):
        uid = task['payload']['user_id']
        session.scalar(select(User).where(User.user_id == uid).with_for_update())
        control(session, lock=True)
        job = jobs.current_job(session, task)
        pref = session.get(SmartPreference, uid)
        if not job:
            return None, None
        if eligibility(session, uid, True):
            jobs.finish(session, task, 'cancelled', {'reason': 'eligibility_changed'})
            return None, None
        current_profile = settings(session)['search_profile_id']
        if pref.profile_id != current_profile:
            pref.profile_id = current_profile
            pref.revision += 1
            pref.checkpoint = {}
        return pref, job

    def read_media(self, photo, user, seconds):
        # An open descriptor pins the original inode through concurrent upload replacement.
        path = self.parent.storage.media(user.uuid, photo)
        with path.open('rb') as stream:
            fdpath = f'/dev/fd/{stream.fileno()}'
            process = subprocess.run([os.getenv('EXIFTOOL_PATH', 'exiftool'), '-json', '-n', '-Make', '-Model', '-LensMake', '-LensModel', '-GPSLatitude', '-GPSLongitude',
                '-SubSecDateTimeOriginal', '-DateTimeOriginal', '-CreationDate', '-CreateDate', '-MediaCreateDate',
                '-OffsetTimeOriginal', '-OffsetTimeDigitized', '-Error', fdpath],
                pass_fds=(stream.fileno(),), capture_output=True, text=True, timeout=max(.1, seconds), check=True)
        values = json.loads(process.stdout)
        if not values or values[0].get('Error'):
            raise ValueError('metadata_decode_failed')
        return extract(values[0], self.gazetteer, os.getenv('TIMEZONE', 'America/New_York'))

    def batch(self, task):
        deadline = time.monotonic() + BUDGET_SECONDS
        for _ in range(BATCH_SIZE):
            if time.monotonic() >= deadline:
                break
            metadata = None
            with self.parent.sessions.begin() as session:
                pref, job = self.fence(session, task)
                if not pref:
                    return
                cp = dict(pref.checkpoint) or {'phase': 'metadata', 'run': str(uuid4()), 'revision': pref.revision}
                phase = cp['phase']
                if phase == 'metadata':
                    photo = session.scalar(select(Photo).outerjoin(SmartMedia, SmartMedia.photo_id == Photo.photo_id).where(Photo.user_id == pref.user_id,
                        or_(SmartMedia.photo_id.is_(None), SmartMedia.content_version != Photo.content_version,
                            (SmartMedia.error.is_not(None) & (SmartMedia.attempts < 3)))).order_by(Photo.photo_id).limit(1))
                    if photo:
                        # Copy ORM values before leaving the short transaction.
                        metadata = (photo, session.get(User, pref.user_id), photo.content_version)
                        session.expunge(photo)
                        session.expunge(metadata[1])
                    else:
                        cp.update(phase='anchors')
                else:
                    {'anchors': anchor_step, 'missing': missing_step, 'items': item_step, 'members': member_step}[phase](session, pref, cp)
                pref.checkpoint = cp
                pref.updated_at = utcnow()
                if not cp:
                    jobs.finish(session, task)
                    return
            if metadata:
                photo, user, version = metadata
                error, values = None, {}
                try:
                    values = self.read_media(photo, user, deadline-time.monotonic())
                except subprocess.TimeoutExpired:
                    error = 'metadata_timeout'
                except FileNotFoundError:
                    error = 'metadata_dependency_or_file_missing'
                except (OSError, ValueError, subprocess.CalledProcessError):
                    error = 'metadata_decode_failed'
                with self.parent.sessions.begin() as session:
                    pref, job = self.fence(session, task)
                    current = session.get(Photo, photo.photo_id)
                    if not pref:
                        return
                    if current and current.content_version == version:
                        media = session.get(SmartMedia, photo.photo_id)
                        attempts = media.attempts if media and media.content_version == version else 0
                        if media is None:
                            media = SmartMedia(photo_id=photo.photo_id, user_id=user.user_id, content_version=version)
                            session.add(media)
                        media.content_version, media.error, media.attempts = version, error, attempts + 1 if error else 0
                        for key, value in values.items(): setattr(media, key, value)
                        session.flush()
                        if not error: devices(session, media)
        with self.parent.sessions.begin() as session:
            pref, job = self.fence(session, task)
            if job:
                job.status = 'queued'
                job.result = {'phase': pref.checkpoint.get('phase'), 'reason': pref.reason}
