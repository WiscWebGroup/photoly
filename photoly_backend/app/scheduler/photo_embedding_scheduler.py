import logging
import os
import threading
import time
from datetime import timedelta
from uuid import uuid4
from sqlalchemy import create_engine, select, update, delete, func
from sqlalchemy.orm import sessionmaker
from app.config import database_url
from app.entities import User, Photo
from app.entities.ai import AIJob, AIControlState, EmbeddingProfile, PhotoEmbeddingState as State, VideoFrameEmbedding
from app.ai.state import control, settings, utcnow
from app.ai.hooks import enqueue_photo
from app.ai.snapshot import snapshot_photo
from app.ai.video import VIDEO_EXTENSIONS, SAMPLING_VERSION
from app.ai.types import AIError
from app.ai.limits import runtime_config
from app.ai.runtime_client import RuntimeClient
from app.infrastructure.storage import LocalStorage
from app.services.ai_admin import coverage
from .scheduler_interface import SchedulerInterface
from .resource_policy import ResourcePolicy
from . import job_repository as jobs

log = logging.getLogger(__name__)


class Heartbeat:
    def __init__(self, scheduler, task=None):
        self.scheduler, self.task = scheduler, task
        self.stop_event, self.lost = threading.Event(), threading.Event()
    def __enter__(self):
        def run():
            while not self.stop_event.wait(2):
                try:
                    with self.scheduler.sessions.begin() as session:
                        if not self.scheduler.lead(session) or (self.task is not None and not jobs.renew(session,self.task)):
                            self.lost.set()
                            return
                        cfg = settings(session)
                    try:
                        # A new Pause also interrupts a legacy synchronous load.
                        observed = (self.scheduler.managed_runtime(cfg) if cfg["managed_lifecycle"]
                                    else self.scheduler.runtime.status())
                        self.scheduler.publish(observed)
                    except AIError:
                        continue
                    if self.task is not None:
                        with self.scheduler.sessions.begin() as session:
                            job = jobs.current_job(session, self.task)
                            if job and job.kind == "operation":
                                job.result = {**job.result, "step": observed.get("state", "unknown")}
                except Exception:
                    self.lost.set()
                    return
        self.thread = threading.Thread(target=run,daemon=True)
        self.thread.start()
        return self
    def __exit__(self,*args):
        self.stop_event.set()
        self.thread.join(timeout=2)


class PhotoEmbeddingScheduler(SchedulerInterface):
    def __init__(self, sessions=None, runtime=None, storage=None):
        self.sessions = sessions or sessionmaker(create_engine(database_url(),pool_pre_ping=True),expire_on_commit=False)
        self.runtime = runtime or RuntimeClient()
        self.storage = storage or LocalStorage(os.getenv("UPLOAD_FOLDER","/photoly"))
        self.owner, self.leader_token = str(uuid4()), None
        self.policy, self.last_user = ResourcePolicy(), None
        self.user_cursor = 0
        from .smart_tag_scheduler import SmartTagScheduler
        self.smart = SmartTagScheduler(self)

    def lead(self, session):
        row = control(session,"scheduler",lock=True)
        now = utcnow()
        if row.lease_expires_at and row.lease_expires_at>now and row.lease_owner!=self.owner:
            return False
        if row.lease_owner != self.owner or not row.lease_expires_at or row.lease_expires_at<=now:
            self.leader_token = str(uuid4())
        row.lease_owner, row.lease_token = self.owner, self.leader_token
        row.lease_expires_at, row.heartbeat_at = now+timedelta(seconds=120), now
        return True

    def reconcile(self, session):
        cfg = settings(session)
        jobs.recover(session)
        if not cfg["enabled"]:
            return
        users=session.scalars(select(User.user_id).where(User.ai_search_enabled.is_(True),User.user_id>self.user_cursor,
            select(Photo.photo_id).where(Photo.user_id==User.user_id).exists()).order_by(User.user_id).limit(4)).all()
        self.user_cursor=users[-1] if users else 0
        for pid in {cfg["search_profile_id"],cfg["building_profile_id"]}-{None}:
            for uid in users:
                photos=session.scalars(select(Photo).outerjoin(State,(State.photo_id==Photo.photo_id)&(State.profile_id==pid))
                    .where(Photo.user_id==uid,
                        (State.state_id.is_(None)) | (State.content_version!=Photo.content_version) |
                        (func.lower(Photo.format).in_(VIDEO_EXTENSIONS) & (State.sampling_version.is_(None) | (State.sampling_version!=SAMPLING_VERSION))) |
                        ((State.status=="pending") & ~select(AIJob.job_id).where(AIJob.photo_id==Photo.photo_id,AIJob.profile_id==pid,
                            AIJob.content_version==Photo.content_version,AIJob.status.in_(["queued","running"])).exists()))
                    .order_by(Photo.photo_id).limit(25)).all()
                for photo in photos:enqueue_photo(session,photo,pid)
        # Bounded cleanup leaves operations for audit; never deletes in-flight requests.
        old = select(AIJob.job_id).where(AIJob.kind!="operation",AIJob.status.in_(["succeeded","cancelled"]),
            AIJob.finished_at<utcnow()-timedelta(days=7)).order_by(AIJob.job_id).limit(1000)
        session.execute(delete(AIJob).where(AIJob.job_id.in_(old)))

    def publish(self, observed, reason=None):
        with self.sessions.begin() as session:
            row = control(session,"scheduler",lock=True)
            if row.lease_owner != self.owner or row.lease_token!=self.leader_token:
                return
            row.heartbeat_at = utcnow()
            row.observed = {"state":settings(session)["scheduler"], "reason":reason}
            runtime_row = control(session,"runtime")
            runtime_row.heartbeat_at = utcnow()
            runtime_row.observed = observed

    def operate(self, task):
        with self.sessions.begin() as session:
            row = control(session,lock=True)
            command = task["payload"]["command"]
            expected = settings(session)["lifecycle_id"] if command.startswith("ai.") else row.generation
            if task["payload"]["generation"] != expected:
                jobs.finish(session,task,"cancelled",{"reason":"superseded"})
                return
            cfg, command = settings(session), task["payload"]["command"]
            profile = session.get(EmbeddingProfile,task["profile_id"]) if task["profile_id"] else None
            spec = dict(profile.spec) if profile else None
        if command.startswith("model."):
            with Heartbeat(self,task) as heartbeat:
                observed = self.runtime.control(command.split(".")[1],spec,runtime_config(cfg),
                    task["payload"]["generation"],str(task["id"]))
                if heartbeat.lost.is_set():
                    return
            self.publish(observed)
        elif command in ("backfill","retry_failed","rebuild"):
            with self.sessions.begin() as session:
                job = jobs.current_job(session,task)
                if not job:
                    return
                cursor = job.payload.get("cursor",0)
                query = select(Photo).join(User).where(User.ai_search_enabled.is_(True),Photo.photo_id>cursor)
                if command=="retry_failed":
                    query = query.join(State,State.photo_id==Photo.photo_id).where(State.profile_id==task["profile_id"],State.status=="failed")
                photos = session.scalars(query.order_by(Photo.photo_id).limit(100)).all()
                for photo in photos:
                    if command!="backfill" or not session.scalar(select(State.state_id).where(State.photo_id==photo.photo_id,
                        State.profile_id==task["profile_id"],State.content_version==photo.content_version,State.status=="ready")):
                        enqueue_photo(session,photo,task["profile_id"])
                if photos:
                    job.payload = {**job.payload,"cursor":photos[-1].photo_id}
                    job.status, job.reserved_bytes = "queued",0
                    return
        elif command=="activate":
            observed = self.runtime.status()
            with self.sessions.begin() as session:
                row = control(session,lock=True)
                profile = session.get(EmbeddingProfile,task["profile_id"])
                counts = coverage(session,task["profile_id"])
                if (row.generation!=task["payload"]["generation"] or profile.index_status!="ready" or
                    observed["state"]!="ready" or observed["fingerprint"]!=profile.fingerprint or counts["pending"] or counts["failed"]):
                    jobs.finish(session,task,"failed",{"reason":"activation_not_ready","coverage":counts})
                    return
                row.desired = {**settings(session),"search_profile_id":profile.profile_id,"building_profile_id":None,"maintenance":False}
                jobs.finish(session,task,result={"coverage":counts})
                return
        elif command=="purge":
            with self.sessions.begin() as session:
                job = jobs.current_job(session,task)
                if not job:
                    return
                from sqlalchemy.dialects.postgresql import insert
                session.execute(insert(AIJob).values(kind="purge",profile_id=task["profile_id"],dedup_key=f"purge:{task['profile_id']}",
                    payload={"operation_id":task["id"],"generation":task["payload"]["generation"]}).on_conflict_do_nothing())
                job.result = {"step":"waiting_for_maintenance"}
                job.lease_expires_at = utcnow()+timedelta(minutes=15)
                return
        with self.sessions.begin() as session:
            jobs.finish(session,task)

    def embed(self, task):
        with Heartbeat(self,task) as heartbeat:
            with self.sessions() as session:
                snapshot = snapshot_photo(session,self.storage,task["photo_id"],version=task["version"])
                profile = session.get(EmbeddingProfile,task["profile_id"])
                spec = dict(profile.spec)
                session.rollback()
            self.last_user = snapshot["user_id"]
            start = time.monotonic()
            vectors = []
            frames = snapshot.get("frames", [{"data": snapshot.get("data")}])
            for frame in frames:
                if heartbeat.lost.is_set():
                    return
                with self.sessions.begin() as session:
                    if not jobs.current_job(session, task):
                        return
                result = self.runtime.encode(spec,image=frame["data"],role="document",background=True)
                if result["fingerprint"]!=spec["fingerprint"]:
                    raise AIError("profile_mismatch",409)
                vectors.append(result["vector"])
            if heartbeat.lost.is_set():
                return
            with self.sessions.begin() as session:
                leader = session.get(AIControlState,"scheduler")
                if leader.lease_token!=self.leader_token or leader.lease_expires_at<=utcnow():
                    return
                jobs.commit_embedding(session,task,snapshot,vectors if "frames" in snapshot else vectors[0],int((time.monotonic()-start)*1000))

    def managed_runtime(self, cfg):
        observed = self.runtime.status()
        pid = cfg["loaded_profile_id"] if cfg["enabled"] else None
        with self.sessions() as session:
            profile = session.get(EmbeddingProfile, pid) if pid else None
            spec = dict(profile.spec) if profile else None
        expected = "ready" if spec else "unloaded"
        generation = cfg["lifecycle_id"]
        if observed.get("generation") != generation:
            submit = getattr(self.runtime, "submit", self.runtime.control)
            observed = submit("load" if spec else "unload", spec, runtime_config(cfg), generation, f"ai:{generation}")
        # Never activate a result from a superseded load, even if the fingerprint matches.
        if observed.get("generation") == generation and observed["state"] == expected and spec:
            with self.sessions.begin() as session:
                row = control(session, lock=True)
                current = settings(session)
                profile = session.get(EmbeddingProfile, pid)
                if (current["enabled"] and current["managed_lifecycle"] and current["lifecycle_id"] == generation
                        and current["loaded_profile_id"] == pid and observed.get("fingerprint") == profile.fingerprint
                        and profile.index_status == "ready"
                        and (current["search_profile_id"] != pid or current["building_profile_id"] or current["maintenance"])):
                    row.desired = {**current, "search_profile_id": pid, "building_profile_id": None, "maintenance": False}
                    row.generation += 1
        return observed

    def tick(self):
        with self.sessions.begin() as session:
            if not self.lead(session):
                return
            self.reconcile(session)
            cfg = settings(session)
            task = jobs.claim(session,self.owner,kinds=("operation",))
        if task:
            try:
                self.operate(task)
            except AIError as error:
                with self.sessions.begin() as session:
                    jobs.finish(session,task,"cancelled" if error.code in ("superseded", "stale_generation") else "failed",{"reason":error.code})
            return
        # Admin operations were handled first. Smart work has its own local CPU/memory
        # admission and never depends on runtime availability. One bounded batch per tick.
        try:
            self.smart.tick()
        except Exception as error:
            log.error("Smart Tag batch failed: %s", type(error).__name__)
        try:
            if cfg["managed_lifecycle"]:
                observed = self.managed_runtime(cfg)
            else:
                observed = self.runtime.status()
                expected_pid = cfg["loaded_profile_id"]
                with self.sessions() as session:
                    profile = session.get(EmbeddingProfile,expected_pid) if expected_pid else None
                    spec = dict(profile.spec) if profile else None
                expected_state = "ready" if cfg["runtime"]=="loaded" else "unloaded"
                if observed["state"]!=expected_state or (spec and (observed["fingerprint"]!=spec["fingerprint"] or observed.get("config",runtime_config(cfg))!=runtime_config(cfg))):
                    # Durable desired state recovers after a container restart or a superseded operation.
                    with Heartbeat(self) as heartbeat:
                        generation=self.generation()
                        observed = self.runtime.control("load" if spec else "unload",spec,runtime_config(cfg),
                            generation,"reconcile:"+str(generation))
                        if heartbeat.lost.is_set():return
            decision = self.policy.decide(observed["resources"],cfg["device"],cfg)
            self.publish(observed, list(decision.reasons) if not decision.allowed else None)
        except AIError:
            self.publish({"state":"offline"},"runtime_offline")
            return
        if not cfg["enabled"] or cfg["scheduler"]!="running" or observed["state"]!="ready" or not decision.allowed:
            return
        with self.sessions.begin() as session:
            task = jobs.claim(session,self.owner,profile_id=cfg["loaded_profile_id"],last_user=self.last_user)
        if task:
            try:
                self.embed(task)
            except AIError as error:
                with self.sessions.begin() as session:
                    if error.code in ("unsupported_image","invalid_image","image_too_large","image_too_many_pixels","invalid_video","video_stream_missing","video_frame_missing","video_frame_too_large","video_too_large"):
                        state = session.scalar(select(State).where(State.photo_id==task["photo_id"],State.profile_id==task["profile_id"],State.content_version==task["version"]))
                        if jobs.current_job(session,task) and state:
                            session.execute(delete(VideoFrameEmbedding).where(VideoFrameEmbedding.state_id==state.state_id))
                            state.status,state.embedding,state.error_code,state.frame_count = "unsupported",None,error.code,0
                        jobs.finish(session,task,result={"reason":error.code})
                    elif error.code in ("stale_content","photo_not_found"):
                        jobs.finish(session,task,"cancelled",{"reason":error.code})
                    else:
                        jobs.fail(session,task,error.code,transient=error.status in (429,503,504) and error.code!="video_decode_timeout")

    def generation(self):
        with self.sessions() as session:
            row = session.get(AIControlState,"global")
            return row.generation if row else 0

    def run(self):
        while True:
            try:
                self.tick()
            except Exception as error:
                log.error("Scheduler cycle failed: %s",type(error).__name__)
            time.sleep(2)


if __name__=="__main__":
    logging.basicConfig(level=logging.INFO)
    PhotoEmbeddingScheduler().run()
