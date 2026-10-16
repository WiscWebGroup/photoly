"""Only this allowlisted worker receives DDL credentials; no user-supplied SQL."""
import logging
import time
import threading
from uuid import uuid4
from sqlalchemy import create_engine, select, text, delete
from sqlalchemy.orm import sessionmaker
from app.config import database_url
from app.ai.state import control, settings, utcnow
from app.entities.ai import EmbeddingProfile, AIJob
from . import job_repository as jobs


class Maintenance:
    def __init__(self, sessions=None, engine=None):
        self.engine = engine or create_engine(database_url(migration=True),pool_pre_ping=True)
        self.sessions = sessions or sessionmaker(create_engine(database_url(),pool_pre_ping=True),expire_on_commit=False)
        self.owner = str(uuid4())

    def tick(self):
        with self.sessions.begin() as session:
            row = control(session,"maintenance")
            row.heartbeat_at,row.observed = utcnow(),{"state":"running"}
            task = jobs.claim(session,self.owner,kinds=("index","purge"))
            if not task:
                return
            profile = session.get(EmbeddingProfile,task["profile_id"])
            if not profile:
                jobs.finish(session,task,"cancelled")
                return
            pid,dimension = profile.profile_id,profile.dimension
            if dimension not in (768,2048) or type(pid) is not int or pid<=0:
                raise ValueError("Invalid registered dimension")
        name = f"ix_embedding_profile_{pid}_hnsw"
        frame_name = f"ix_video_frame_profile_{pid}_hnsw"
        stopped=threading.Event()
        def heartbeat():
            while not stopped.wait(20):
                try:
                    with self.sessions.begin() as session:
                        if not jobs.renew(session,task):return
                        row=control(session,"maintenance")
                        row.heartbeat_at=utcnow()
                except Exception:return
        thread=threading.Thread(target=heartbeat,daemon=True)
        thread.start()
        try:
            with self.engine.connect().execution_options(isolation_level="AUTOCOMMIT") as connection:
                # Serialize schema maintenance across replicas and retries.
                if not connection.scalar(text("SELECT pg_try_advisory_lock(74832002)")):
                    with self.sessions.begin() as session:
                        jobs.fail(session,task,"maintenance_busy",transient=True)
                    return
                try:
                    connection.exec_driver_sql("SET statement_timeout='90s'")
                    if task["kind"]=="index":
                        with self.sessions() as session:
                            cfg, used = settings(session),jobs.usage(session)
                            if max(used["physical_bytes"],used["estimated_bytes"]+used["reserved_bytes"])>=cfg["quota_bytes"]:
                                raise RuntimeError("quota_exceeded")
                        for index_name, table, predicate in (
                            (name, "photo_embedding_state", f"profile_id={pid} AND status='ready'"),
                            (frame_name, "video_frame_embedding", f"profile_id={pid}"),
                        ):
                            valid = connection.scalar(text("SELECT indisvalid FROM pg_index JOIN pg_class ON indexrelid=oid WHERE relname=:name"),{"name":index_name})
                            if valid is False:
                                connection.exec_driver_sql(f'DROP INDEX CONCURRENTLY IF EXISTS "{index_name}"')
                            connection.exec_driver_sql(f'CREATE INDEX CONCURRENTLY IF NOT EXISTS "{index_name}" ON {table} USING hnsw ((embedding::halfvec({dimension})) halfvec_cosine_ops) WHERE {predicate}')
                        with self.sessions.begin() as session:
                            p=session.get(EmbeddingProfile,pid)
                            if p:p.index_status="ready"
                            jobs.finish(session,task)
                    else:
                        with self.sessions.begin() as session:
                            control(session,lock=True)
                            cfg=settings(session)
                            active=session.scalar(select(AIJob.job_id).where(AIJob.kind=="search",AIJob.profile_id==pid,AIJob.status=="running",AIJob.lease_expires_at>utcnow()).limit(1))
                            if pid in (cfg["search_profile_id"],cfg["building_profile_id"],cfg["loaded_profile_id"]) or active:
                                jobs.finish(session,task,"failed",{"reason":"profile_in_use"})
                                self.finish_operation(session,task,"failed",{"reason":"profile_in_use"})
                                return
                            # Keep global lock through deletion; activation and new searches acquire it too.
                            connection.exec_driver_sql(f'DROP INDEX CONCURRENTLY IF EXISTS "{name}"')
                            connection.exec_driver_sql(f'DROP INDEX CONCURRENTLY IF EXISTS "{frame_name}"')
                            session.execute(delete(EmbeddingProfile).where(EmbeddingProfile.profile_id==pid))
                            jobs.finish(session,task)
                            self.finish_operation(session,task,"succeeded",{})
                finally:
                    connection.execute(text("SELECT pg_advisory_unlock(74832002)"))
        except Exception as error:
            with self.sessions.begin() as session:
                jobs.fail(session,task,"quota_exceeded" if str(error)=="quota_exceeded" else "index_maintenance_failed",transient=str(error)=="quota_exceeded")

        finally:
            stopped.set()
            thread.join(timeout=2)

    def finish_operation(self,session,task,status,result):
        job=session.get(AIJob,task["payload"].get("operation_id"))
        if job and job.status=="running":
            job.status,job.result,job.finished_at=status,result,utcnow()

    def run(self):
        while True:
            try:self.tick()
            except Exception as error:logging.error("Maintenance cycle failed: %s",type(error).__name__)
            time.sleep(3)


if __name__=="__main__":
    Maintenance().run()
