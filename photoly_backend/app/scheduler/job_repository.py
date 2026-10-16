from datetime import timedelta
from uuid import uuid4
from sqlalchemy import select, update, func, text
from sqlalchemy.dialects.postgresql import insert
from app.entities import Photo, User
from app.entities.ai import AIJob, EmbeddingProfile, PhotoEmbeddingState, AIControlState
from app.ai.state import control, settings, utcnow
from app.ai.types import normalized
from .background_limits import admit

LEASE_SECONDS = 120


def vector_charge(dimension):
    # Two copies of half-vector payload plus conservative row/HNSW overhead.
    return 2 * (2 * dimension + 8) + 1024


def usage(session):
    used = session.scalar(select(func.coalesce(func.sum(2*(2*EmbeddingProfile.dimension+8)+1024), 0))
        .select_from(PhotoEmbeddingState).join(EmbeddingProfile).where(PhotoEmbeddingState.status == "ready"))
    reserved = session.scalar(select(func.coalesce(func.sum(AIJob.reserved_bytes), 0)).where(AIJob.status == "running"))
    actual = session.scalar(text("SELECT pg_total_relation_size('photo_embedding_state') + pg_total_relation_size('embedding_profile')"))
    return {"estimated_bytes": int(used), "reserved_bytes": int(reserved), "physical_bytes": int(actual)}


def recover(session):
    session.execute(update(AIJob).where(AIJob.kind == "search", AIJob.status == "running", AIJob.lease_expires_at < utcnow())
        .values(status="failed", finished_at=utcnow(), result={"reason":"query_timeout"}))
    session.execute(update(AIJob).where(AIJob.kind != "search", AIJob.status == "running", AIJob.lease_expires_at < utcnow())
        .values(status="queued", reserved_bytes=0, lease_token=None, lease_owner=None, result={"reason": "lease_expired"}))


def claim(session, owner, kinds=("embedding",), profile_id=None, last_user=None):
    config_row = control(session, lock=True)
    recover(session)
    query = select(AIJob).where(AIJob.kind.in_(kinds), AIJob.status == "queued", AIJob.next_run_at <= utcnow())
    if profile_id is not None:
        query = query.where(AIJob.profile_id == profile_id)
    if kinds == ("embedding",):
        query = query.join(Photo).join(User).where(User.ai_search_enabled.is_(True), Photo.content_version == AIJob.content_version)
        if last_user is not None:
            query = query.order_by((Photo.user_id == last_user).asc())
    job = session.scalar(query.order_by(AIJob.priority.desc(), AIJob.job_id).with_for_update(of=AIJob, skip_locked=True).limit(1))
    if job is None:
        return None
    reserve = 0
    if job.kind == "embedding":
        config = {**settings(session), **config_row.desired}
        if not config["enabled"] or config["scheduler"] != "running":
            return None
        profile = session.get(EmbeddingProfile, job.profile_id)
        reserve = vector_charge(profile.dimension)
        current = usage(session)
        if max(current["estimated_bytes"] + current["reserved_bytes"] + reserve, current["physical_bytes"]) > config["quota_bytes"]:
            job.result = {"reason": "quota_exceeded"}
            job.next_run_at = utcnow() + timedelta(seconds=10)
            return None
        if not admit(session, config):
            return None
    job.status, job.lease_owner, job.lease_token = "running", owner, str(uuid4())
    job.lease_expires_at = utcnow() + timedelta(seconds=LEASE_SECONDS)
    job.heartbeat_at, job.reserved_bytes = utcnow(), reserve
    session.flush()
    return {"id": job.job_id, "token": job.lease_token, "photo_id": job.photo_id, "profile_id": job.profile_id,
            "version": job.content_version, "kind": job.kind, "payload": dict(job.payload)}


def renew(session, task):
    result = session.execute(update(AIJob).where(AIJob.job_id == task["id"], AIJob.status == "running",
        AIJob.lease_token == task["token"], AIJob.lease_expires_at > utcnow())
        .values(lease_expires_at=utcnow()+timedelta(seconds=LEASE_SECONDS), heartbeat_at=utcnow()))
    return result.rowcount == 1


def current_job(session, task):
    return session.scalar(select(AIJob).where(AIJob.job_id == task["id"], AIJob.status == "running",
        AIJob.lease_token == task["token"], AIJob.lease_expires_at > utcnow()).with_for_update())


def finish(session, task, status="succeeded", result=None):
    job = current_job(session, task)
    if not job:
        return False
    job.status, job.result, job.reserved_bytes = status, result or {}, 0
    job.finished_at = utcnow()
    return True


def fail(session, task, code, transient=False):
    job = current_job(session, task)
    if not job:
        return
    job.attempts += 0 if transient else 1
    job.status = "failed" if job.attempts >= 5 else "queued"
    job.result, job.reserved_bytes = {"reason": code}, 0
    job.next_run_at = utcnow() + timedelta(seconds=min(900, 10 * 2**job.attempts))
    if job.status == "failed":
        job.finished_at = utcnow()
        state = session.scalar(select(PhotoEmbeddingState).where(PhotoEmbeddingState.photo_id == job.photo_id,
            PhotoEmbeddingState.profile_id == job.profile_id, PhotoEmbeddingState.content_version == job.content_version))
        if state:
            state.status, state.embedding, state.error_code = "failed", None, code


def commit_embedding(session, task, snapshot, vector, duration_ms):
    # Global lock order: user -> quota/control -> job. Claims never acquire user locks.
    user = session.scalar(select(User).where(User.user_id == snapshot["user_id"]).with_for_update())
    control(session, lock=True)
    job = current_job(session, task)
    photo = session.get(Photo, task["photo_id"])
    cfg = settings(session)
    if not job:
        return False
    if (not user or not user.ai_search_enabled or not cfg["enabled"] or not photo or
        photo.content_version != task["version"] or photo.content_sha256 not in (None, snapshot["hash"]) or
        task["profile_id"] not in (cfg["search_profile_id"], cfg["building_profile_id"])):
        return finish(session, task, "cancelled", {"reason": "stale_content_or_eligibility"})
    profile = session.get(EmbeddingProfile, task["profile_id"])
    value = normalized([vector], profile.dimension)[0]
    photo.content_sha256 = snapshot["hash"]
    session.execute(insert(PhotoEmbeddingState).values(photo_id=photo.photo_id, profile_id=profile.profile_id,
        content_version=photo.content_version, content_sha256=snapshot["hash"], status="ready", embedding=value,
        computed_at=utcnow(), duration_ms=duration_ms).on_conflict_do_update(constraint="uq_embedding_profile_photo",
        set_={"content_version": photo.content_version, "content_sha256": snapshot["hash"], "status": "ready", "embedding": value,
              "computed_at": utcnow(), "duration_ms": duration_ms, "error_code": None}))
    return finish(session, task)
