from sqlalchemy import select
from app.entities import Photo, User
from .types import AIError
from .preprocessing import read_bounded, sha256


def snapshot_photo(session, storage, photo_id, user_id=None, version=None):
    photo = session.get(Photo, photo_id)
    if not photo or (user_id is not None and photo.user_id != user_id):
        raise AIError("photo_not_found", 404)
    owner_id = photo.user_id
    owner = session.scalar(select(User).where(User.user_id == owner_id).with_for_update().execution_options(populate_existing=True))
    session.refresh(photo)
    if not owner or not owner.ai_search_enabled or (version is not None and photo.content_version != version):
        session.rollback()
        raise AIError("stale_content", 409)
    expected = photo.content_sha256
    snapshot = {"user_id": owner_id, "version": photo.content_version}
    try:
        stream = storage.media(owner.uuid, photo).open("rb")
    except OSError as error:
        session.rollback()
        raise AIError("image_missing", 404) from error
    # Writers replace immutable inodes while holding the same user lock.
    # The open descriptor remains stable across a subsequent rename/delete.
    session.commit()
    with stream:
        data = read_bounded(stream)
    digest = sha256(data)
    if expected is not None and digest != expected:
        raise AIError("content_hash_mismatch", 409)
    return {**snapshot, "data": data, "hash": digest}
