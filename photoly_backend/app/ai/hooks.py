from sqlalchemy import select, delete
from sqlalchemy.dialects.postgresql import insert
from app.entities import User
from app.entities.ai import AIJob, EmbeddingProfile, PhotoEmbeddingState, VideoFrameEmbedding
from .state import settings
from .preprocessing import EXTENSIONS
from .video import is_video, SAMPLING_VERSION


def enqueue_photo(session, photo, profile_id):
    if not profile_id:
        return
    video = is_video(photo.format)
    supported = video or (photo.format or "").lower() in EXTENSIONS
    session.execute(delete(VideoFrameEmbedding).where(VideoFrameEmbedding.state_id.in_(select(PhotoEmbeddingState.state_id).where(PhotoEmbeddingState.photo_id==photo.photo_id, PhotoEmbeddingState.profile_id==profile_id))))
    session.execute(insert(PhotoEmbeddingState).values(photo_id=photo.photo_id, profile_id=profile_id,
        content_version=photo.content_version, sampling_version=SAMPLING_VERSION if video else None, frame_count=0, status="pending" if supported else "unsupported").on_conflict_do_update(
        constraint="uq_embedding_profile_photo", set_={"content_version": photo.content_version, "embedding": None, "frame_count": 0, "sampling_version": SAMPLING_VERSION if video else None,
        "status": "pending" if supported else "unsupported", "computed_at": None, "error_code": None}))
    if supported:
        session.execute(insert(AIJob).values(kind="embedding", photo_id=photo.photo_id, profile_id=profile_id,
            content_version=photo.content_version, dedup_key=f"embed:{profile_id}:{photo.photo_id}:{photo.content_version}").on_conflict_do_nothing())


def photo_changed(session, photo):
    from sqlalchemy import update
    session.execute(update(AIJob).where(AIJob.photo_id == photo.photo_id, AIJob.status.in_(["queued", "running"])).values(status="cancelled", reserved_bytes=0))
    session.execute(delete(VideoFrameEmbedding).where(VideoFrameEmbedding.state_id.in_(select(PhotoEmbeddingState.state_id).where(PhotoEmbeddingState.photo_id==photo.photo_id))))
    session.execute(update(PhotoEmbeddingState).where(PhotoEmbeddingState.photo_id == photo.photo_id).values(status="pending", embedding=None, frame_count=0, sampling_version=None))
    config = settings(session)
    user = session.get(User, photo.user_id)
    if config["enabled"] and user.ai_search_enabled:
        for profile_id in {config["search_profile_id"], config["building_profile_id"]} - {None}:
            enqueue_photo(session, photo, profile_id)
