"""Media-level cosine retrieval: a picture is one frame, a video is up to five."""
import time
import numpy as np
from sqlalchemy import select, func, text, cast, literal_column, union_all, or_, and_
from pgvector.sqlalchemy import HALFVEC
from app.entities import Photo, GalleryPhoto
from app.entities.ai import PhotoEmbeddingState as State, VideoFrameEmbedding as Frame
from .vector_store_interface import VectorStoreInterface
from .types import normalized, AIError
from .video import MAX_FRAMES, SAMPLING_VERSION


class PgVectorStore(VectorStoreInterface):
    def _vectors(self, profile, vector):
        if profile.dimension not in (768, 2048) or type(profile.profile_id) is not int or profile.profile_id <= 0:
            raise ValueError("Invalid server profile")
        data = np.asarray(vector, dtype=np.float32)
        if data.ndim == 1:
            data = data[None, :]
        if data.ndim != 2 or not 1 <= len(data) <= MAX_FRAMES:
            raise AIError("invalid_embedding", 502)
        return normalized(data, profile.dimension)

    def _filters(self, uid, profile, namespace_id=None, gallery_id=None, exclude=None):
        # Trusted literals let PostgreSQL prove partial HNSW predicates with generic plans.
        filters = [Photo.user_id == uid, State.profile_id == literal_column(str(profile.profile_id)),
                   State.status == literal_column("'ready'"), State.content_version == Photo.content_version,
                   or_(State.sampling_version.is_(None), State.sampling_version == SAMPLING_VERSION)]
        if namespace_id is not None:
            filters.append(Photo.ns_id == namespace_id)
        if gallery_id is not None:
            filters.append(select(GalleryPhoto.gp_id).where(GalleryPhoto.photo_id == Photo.photo_id, GalleryPhoto.ga_id == gallery_id).exists())
        if exclude is not None:
            filters.append(Photo.photo_id != exclude)
        return filters

    def _branch(self, profile, vectors, filters, video):
        column = Frame.embedding if video else State.embedding
        distances = [cast(column, HALFVEC(profile.dimension)).cosine_distance(v) for v in vectors]
        distance = distances[0] if len(distances) == 1 else func.least(*distances)
        stamp = Frame.timestamp_ms if video else literal_column("NULL::bigint")
        query = select(Photo.photo_id.label("photo_id"), distance.label("distance"), stamp.label("timestamp_ms"))\
            .select_from(Photo).join(State, State.photo_id == Photo.photo_id)
        if video:
            query = query.join(Frame, Frame.state_id == State.state_id).where(
                Frame.profile_id == literal_column(str(profile.profile_id)), State.sampling_version == SAMPLING_VERSION)
        else:
            query = query.where(State.embedding.is_not(None), State.sampling_version.is_(None))
        return query.where(*filters), distance

    def _exact(self, profile, vectors, filters, min_score=None, after=None):
        frames = union_all(*(self._branch(profile, vectors, filters, video)[0] for video in (False, True))).subquery()
        ranked = select(frames, func.row_number().over(partition_by=frames.c.photo_id,
            order_by=(frames.c.distance, frames.c.timestamp_ms)).label("position")).subquery()
        query = select(Photo, ranked.c.distance, ranked.c.timestamp_ms).join(ranked, ranked.c.photo_id == Photo.photo_id)\
            .where(ranked.c.position == 1)
        if min_score is not None:
            query = query.where(1.0 - ranked.c.distance >= min_score)
        if after is not None:
            query = query.where(or_(ranked.c.distance > after[0],
                and_(ranked.c.distance == after[0], Photo.photo_id > after[1])))
        # The window aggregates all frames before threshold, seek and limit are applied.
        return query.order_by(ranked.c.distance + 0.0, Photo.photo_id)

    @staticmethod
    def _hits(rows):
        return [{"photo": photo, "score": float(1-distance),
                 **({"matchedTimestampMs": int(stamp)} if stamp is not None else {})}
                for photo, distance, stamp in rows]

    def search(self, session, user_id, profile, vector, *, limit=20, namespace_id=None, gallery_id=None, exclude=None, timeout_ms=5000, min_score=None):
        vectors = self._vectors(profile, vector)
        filters = self._filters(user_id, profile, namespace_id, gallery_id, exclude)
        deadline = time.monotonic() + timeout_ms/1000
        def remaining():
            left = int((deadline-time.monotonic())*1000)
            if left <= 0:
                raise AIError("query_timeout", 504)
            session.execute(text("SELECT set_config('statement_timeout', :timeout, true)"), {"timeout":str(left)})
        remaining()
        count = session.scalar(select(func.count()).select_from(Photo).join(State, State.photo_id == Photo.photo_id).where(*filters))
        if count > 5000:
            session.execute(text("SET LOCAL hnsw.iterative_scan = strict_order"))
            session.execute(text("SET LOCAL hnsw.ef_search = 1000"))
            session.execute(text("SET LOCAL hnsw.max_scan_tuples = 20000"))
            candidates = set()
            for vector in vectors:
                for video in (False, True):
                    remaining()
                    query, distance = self._branch(profile, [vector], filters, video)
                    candidates.update(row.photo_id for row in session.execute(query.order_by(distance).limit(limit*MAX_FRAMES)))
            remaining()
            rows = session.execute(self._exact(profile, vectors, [*filters, Photo.photo_id.in_(candidates)], min_score).limit(limit)).all()
            if len(rows) >= min(count, limit):
                return self._hits(rows)
        remaining()
        rows = session.execute(self._exact(profile, vectors, filters, min_score).limit(limit)).all()
        return self._hits(rows)

    def search_page(self, session, user_id, profile, vector, *, limit, exclude, after=None, timeout_ms=5000, min_score=None):
        vectors = self._vectors(profile, vector)
        filters = self._filters(user_id, profile, exclude=exclude)
        session.execute(text("SELECT set_config('statement_timeout', :timeout, true)"), {"timeout":str(max(1,int(timeout_ms)))})
        rows = session.execute(self._exact(profile, vectors, filters, min_score, after).limit(limit+1)).all()
        more = len(rows) > limit
        rows = rows[:limit]
        boundary = [float(rows[-1][1]), rows[-1][0].photo_id] if more else None
        return self._hits(rows), boundary
