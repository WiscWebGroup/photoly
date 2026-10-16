import time
from sqlalchemy import select, func, text, cast, literal_column
from pgvector.sqlalchemy import HALFVEC
from app.entities import Photo, GalleryPhoto
from app.entities.ai import PhotoEmbeddingState as State
from .vector_store_interface import VectorStoreInterface
from .types import normalized, AIError


class PgVectorStore(VectorStoreInterface):
    def search(self, session, user_id, profile, vector, *, limit=20, namespace_id=None, gallery_id=None, exclude=None, timeout_ms=5000, min_score=None):
        dimension, profile_id = profile.dimension, profile.profile_id
        if dimension not in (768,2048) or type(profile_id) is not int or profile_id <= 0:
            raise ValueError("Invalid server profile")
        query_vector = normalized([vector], dimension)[0]
        # Trusted integer literals are necessary for PostgreSQL to prove a partial-index predicate
        # even when a prepared statement uses a generic plan. All user inputs remain bound.
        distance = cast(State.embedding, HALFVEC(dimension)).cosine_distance(query_vector)
        filters = [Photo.user_id == user_id, State.profile_id == literal_column(str(profile_id)),
                   State.status == literal_column("'ready'"), State.content_version == Photo.content_version]
        if min_score is not None:
            filters.append(1.0 - distance >= min_score)
        if namespace_id is not None:
            filters.append(Photo.ns_id == namespace_id)
        if gallery_id is not None:
            filters.append(select(GalleryPhoto.gp_id).where(GalleryPhoto.photo_id == Photo.photo_id, GalleryPhoto.ga_id == gallery_id).exists())
        if exclude is not None:
            filters.append(Photo.photo_id != exclude)
        deadline=time.monotonic()+timeout_ms/1000
        def remaining():
            left=int((deadline-time.monotonic())*1000)
            if left<=0:raise AIError("query_timeout",504)
            session.execute(text("SELECT set_config('statement_timeout', :timeout, true)"), {"timeout":str(left)})
        session.execute(text("SELECT set_config('statement_timeout', :timeout, true)"), {"timeout": str(max(1, int(timeout_ms)))})
        query = select(Photo, distance.label("distance")).join(State, State.photo_id == Photo.photo_id).where(*filters)
        count = session.scalar(select(func.count()).select_from(Photo).join(State, State.photo_id == Photo.photo_id).where(*filters))
        remaining()
        if count <= 5000:
            # Adding zero intentionally prevents ANN ordering for the exact path.
            rows = session.execute(query.order_by(distance+0.0, Photo.photo_id).limit(limit)).all()
        else:
            session.execute(text("SET LOCAL hnsw.iterative_scan = strict_order"))
            session.execute(text("SET LOCAL hnsw.ef_search = 1000"))
            session.execute(text("SET LOCAL hnsw.max_scan_tuples = 20000"))
            rows = session.execute(query.order_by(distance).limit(limit)).all()
            if len(rows) < min(count, limit):
                remaining()
                rows = session.execute(query.order_by(distance+0.0, Photo.photo_id).limit(limit)).all()
        return [{"photo": photo, "score": float(1-distance)} for photo, distance in rows]


    def search_page(self, session, user_id, profile, vector, *, limit, exclude, after=None, timeout_ms=5000, min_score=None):
        """Exact seek ordering, including ties; ANN is deliberately not used for cursors."""
        dimension, pid = profile.dimension, profile.profile_id
        if dimension not in (768, 2048) or type(pid) is not int or pid <= 0:
            raise ValueError("Invalid server profile")
        distance = cast(State.embedding, HALFVEC(dimension)).cosine_distance(normalized([vector], dimension)[0]) + 0.0
        filters = [Photo.user_id == user_id, Photo.photo_id != exclude,
                   State.profile_id == pid, State.status == "ready", State.content_version == Photo.content_version]
        if min_score is not None:
            filters.append(1.0 - distance >= min_score)
        if after is not None:
            from sqlalchemy import or_, and_
            filters.append(or_(distance > after[0], and_(distance == after[0], Photo.photo_id > after[1])))
        session.execute(text("SELECT set_config('statement_timeout', :timeout, true)"), {"timeout":str(max(1,int(timeout_ms)))})
        rows = session.execute(select(Photo, distance.label("distance")).join(State, State.photo_id == Photo.photo_id)
                               .where(*filters).order_by(distance, Photo.photo_id).limit(limit+1)).all()
        more = len(rows) > limit
        rows = rows[:limit]
        boundary = [float(rows[-1][1]), rows[-1][0].photo_id] if more else None
        return [{"photo":photo,"score":float(1-dist)} for photo,dist in rows], boundary
