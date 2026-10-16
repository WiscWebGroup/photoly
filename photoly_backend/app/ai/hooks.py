from sqlalchemy import select
from sqlalchemy.dialects.postgresql import insert
from app.entities import User
from app.entities.ai import AIJob, EmbeddingProfile, PhotoEmbeddingState
from .state import settings
from .preprocessing import EXTENSIONS


def enqueue_photo(session, photo, profile_id):
    if not profile_id:
        return
    supported = (photo.format or "").lower() in EXTENSIONS
    session.execute(insert(PhotoEmbeddingState).values(photo_id=photo.photo_id, profile_id=profile_id,
        content_version=photo.content_version, status="pending" if supported else "unsupported").on_conflict_do_update(
        constraint="uq_embedding_profile_photo", set_={"content_version": photo.content_version, "embedding": None,
        "status": "pending" if supported else "unsupported", "computed_at": None, "error_code": None}))
    if supported:
        session.execute(insert(AIJob).values(kind="embedding", photo_id=photo.photo_id, profile_id=profile_id,
            content_version=photo.content_version, dedup_key=f"embed:{profile_id}:{photo.photo_id}:{photo.content_version}").on_conflict_do_nothing())


def photo_changed(session, photo):
    from sqlalchemy import update
    session.execute(update(AIJob).where(AIJob.photo_id == photo.photo_id, AIJob.status.in_(["queued", "running"])).values(status="cancelled", reserved_bytes=0))
    session.execute(update(PhotoEmbeddingState).where(PhotoEmbeddingState.photo_id == photo.photo_id).values(status="pending", embedding=None))
    config = settings(session)
    user = session.get(User, photo.user_id)
    if config["enabled"] and user.ai_search_enabled:
        for profile_id in {config["search_profile_id"], config["building_profile_id"]} - {None}:
            enqueue_photo(session, photo, profile_id)
