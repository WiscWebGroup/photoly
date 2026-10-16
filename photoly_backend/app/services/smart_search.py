"""Bounded lexical candidates for the client's equal-weight rank fusion."""
from sqlalchemy import case, func, select
from app.entities import Photo, Tag, TagPhoto, Gallery, GalleryPhoto
from app.services.base import BusinessError


def search_smart(session, user_id, query):
    if not isinstance(query, str) or len(query) > 512:
        raise BusinessError(-1)
    query = query.strip().lower()
    if not query:
        return []
    words = list(dict.fromkeys(query.split()))

    def match(column):
        name = func.lower(func.coalesce(column, ""))
        fraction = sum(case((name.contains(word, autoescape=True), 1), else_=0)
                       for word in words) / len(words)
        return case((name == query, 3), (name.contains(query, autoescape=True), 2), else_=fraction)

    # Aggregate before joining: multiple associations must not inflate a photo's score.
    tags = (select(TagPhoto.photo_id, func.max(match(Tag.tag_name)).label("score"))
            .join(Tag, Tag.tag_id == TagPhoto.tag_id)
            .where(Tag.user_id == user_id, TagPhoto.user_id == user_id)
            .group_by(TagPhoto.photo_id).subquery())
    galleries = (select(GalleryPhoto.photo_id, func.max(match(Gallery.ga_name)).label("score"))
                 .join(Gallery, Gallery.ga_id == GalleryPhoto.ga_id)
                 .where(Gallery.user_id == user_id).group_by(GalleryPhoto.photo_id).subquery())
    score = (0.4 * match(Photo.photo_name) + 0.3 * func.coalesce(tags.c.score, 0)
             + 0.3 * func.coalesce(galleries.c.score, 0))
    statement = (select(Photo).outerjoin(tags, tags.c.photo_id == Photo.photo_id)
                 .outerjoin(galleries, galleries.c.photo_id == Photo.photo_id)
                 .where(Photo.user_id == user_id, score > 0)
                 .order_by(score.desc(), Photo.photo_id).limit(100))
    return session.scalars(statement).all()
