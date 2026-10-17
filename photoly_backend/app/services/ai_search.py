import time
from datetime import timedelta
from uuid import uuid4
from sqlalchemy import select
from sqlalchemy.exc import DBAPIError
from app.entities import User, Photo, Namespace, Gallery
from app.entities.ai import AIJob, EmbeddingProfile, PhotoEmbeddingState as State, VideoFrameEmbedding as Frame
from app.ai.state import settings, control, utcnow
from app.ai.runtime_client import RuntimeClient
from app.ai.snapshot import snapshot_photo
from app.ai.types import AIError
from app.ai.preprocessing import EXTENSIONS, MAX_BYTES, MAX_PIXELS
from app.ai.video import VIDEO_EXTENSIONS, MAX_FRAMES, EXTRACT_SECONDS, SAMPLING_VERSION, max_video_bytes
from app.ai.vector_store_pgvector import PgVectorStore
from app.services.ai_admin import coverage
from app.serialization import serialize


class AISearch:
    def __init__(self, session, infra, runtime=None):
        self.session,self.infra=session,infra
        self.runtime=runtime or RuntimeClient()

    def capabilities(self,uid):
        cfg=settings(self.session)
        user=self.session.get(User,uid)
        profile=self.session.get(EmbeddingProfile,cfg["search_profile_id"]) if cfg["search_profile_id"] else None
        eligible=bool(user and user.ai_search_enabled)
        reason = "disabled" if (not cfg["enabled"] or not cfg["search_enabled"]) else "not_enabled_for_user" if not eligible else "maintenance" if cfg["maintenance"] else "no_active_profile" if not profile else None
        ready=False
        if reason is None:
            self.session.commit()
            try:
                runtime=self.runtime.status()
                ready=runtime["state"]=="ready" and runtime["fingerprint"]==profile.fingerprint and profile.index_status=="ready"
                if not ready:reason="model_not_ready"
            except AIError:reason="runtime_offline"
        return {"enabled":eligible,"globalEnabled":cfg["enabled"] and cfg["search_enabled"],"ready":ready,"reason":reason,
                "profileId":profile.profile_id if profile else None,"profile":({"provider":profile.provider,"dimension":profile.dimension,"modelId":profile.spec["model_id"]} if profile else None),"coverage":coverage(self.session,cfg["search_profile_id"],uid),
                "modalities":["text","image","similar"],"limits":{"textCharacters":512,"imageBytes":MAX_BYTES,"imagePixels":MAX_PIXELS,"topK":100,"videoBytes":max_video_bytes(),"videoFrames":MAX_FRAMES,"videoExtractionSeconds":EXTRACT_SECONDS},
                "similarityThreshold":cfg["similarity_threshold"],"formats":sorted(EXTENSIONS | VIDEO_EXTENSIONS),"videoFormats":sorted(VIDEO_EXTENSIONS),"videoSamplingVersion":SAMPLING_VERSION}

    def page(self, uid, data, photo_id, session_token):
        from app.ai.similar_cursor import SimilarCursor
        cursor_store = SimilarCursor(self.infra.sessions.client, uid, session_token, photo_id)
        context, after = None, None
        if "cursor" in data:
            if set(data) != {"cursor"}: raise AIError("invalid_pagination")
            context, after = cursor_store.load(data["cursor"])
            size = context["size"]
        else:
            if set(data)-{"pageSize"}: raise AIError("invalid_pagination")
            size = data.get("pageSize",24)
            if type(size) is not int or not 1 <= size <= 60: raise AIError("invalid_page_size")
        return self.search(uid, {"topK":size}, photo_id=photo_id, pagination=(cursor_store, context, after))

    def search(self,uid,data,*,image=None,photo_id=None,pagination=None):
        started=time.monotonic()
        image_query = photo_id is not None or image is not None
        k=data.get("topK",20)
        if type(k) is not int or not 1<=k<=100:
            raise AIError("invalid_top_k")
        for name,model,pk in (("namespaceId",Namespace,"ns_id"),("galleryId",Gallery,"ga_id")):
            identifier=data.get(name)
            if identifier is not None:
                if type(identifier) is not int or identifier<=0:
                    raise AIError("invalid_filter")
                obj=self.session.get(model,identifier)
                if not obj or obj.user_id!=uid:
                    raise AIError("filter_not_found",404)
        text_query=data.get("text")
        if image is None and photo_id is None and (not isinstance(text_query,str) or not text_query.strip() or len(text_query)>512):
            raise AIError("invalid_text")
        token=str(uuid4())
        # Redis stores only an expiring concurrency guard, not task truth.
        key=f"photoly:ai-query:{uid}"
        if not self.infra.sessions.client.set(key,token,nx=True,ex=65):
            raise AIError("user_query_busy",429)
        request_job=None
        completed=False
        try:
            user=self.session.scalar(select(User).where(User.user_id==uid).with_for_update().execution_options(populate_existing=True))
            control(self.session,lock=True)
            cfg=settings(self.session)
            if pagination and pagination[1] is not None and pagination[1].get("threshold") != cfg["similarity_threshold"]:
                raise AIError("search_settings_changed",409)
            if not user or not user.ai_search_enabled or (not cfg["enabled"] or not cfg["search_enabled"]):
                raise AIError("ai_not_enabled",403)
            if cfg["maintenance"]:
                raise AIError("maintenance",503)
            profile=self.session.get(EmbeddingProfile,cfg["search_profile_id"]) if cfg["search_profile_id"] else None
            if not profile or profile.index_status!="ready":
                raise AIError("profile_not_ready",503)
            spec,pid=dict(profile.spec),profile.profile_id
            request_job=AIJob(kind="search",profile_id=pid,dedup_key="search:"+token,status="running",
                              lease_token=token,lease_expires_at=utcnow()+timedelta(seconds=60))
            self.session.add(request_job)
            self.session.commit()
            request_id=request_job.job_id
            observed=self.runtime.status()
            if observed["state"]!="ready" or observed["fingerprint"]!=spec["fingerprint"]:
                raise AIError("model_not_ready",503)
            vector=None
            source_version=None
            snapshot=None
            cursor_store, context, after = pagination if pagination else (None, None, None)
            if photo_id is not None:
                photo=self.session.get(Photo,photo_id)
                if not photo or photo.user_id!=uid:
                    raise AIError("photo_not_found",404)
                source_version=photo.content_version
                if context is not None:
                    if context["profile"] != pid: raise AIError("profile_changed",409)
                    if context["version"] != source_version: raise AIError("stale_content",409)
                    vector=context["vector"]
                if vector is None and spec["query_instruction"]==spec["document_instruction"]:
                    state=self.session.scalar(select(State).where(State.photo_id==photo_id,State.profile_id==pid,
                            State.content_version==photo.content_version,State.status=="ready"))
                    if state and state.embedding is not None:
                        vector=state.embedding.to_numpy()
                    elif state and state.sampling_version == SAMPLING_VERSION:
                        vector=[v.to_numpy() for v in self.session.scalars(select(Frame.embedding).where(
                            Frame.state_id==state.state_id).order_by(Frame.frame_index)).all()] or None
                if vector is None:
                    snapshot=snapshot_photo(self.session,self.infra.storage,photo_id,uid,deadline=started+60)
                    image=snapshot.get("data")
            self.session.commit()
            if vector is None:
                vectors=[]
                frames=snapshot["frames"] if snapshot and "frames" in snapshot else [{"data":image}]
                for frame in frames:
                    remaining=60-(time.monotonic()-started)
                    if remaining<=0:raise AIError("query_timeout",504)
                    output=self.runtime.encode(spec,text=text_query,image=frame["data"],role="query",timeout=remaining)
                    if output["fingerprint"]!=spec["fingerprint"]:
                        raise AIError("profile_mismatch",409)
                    vectors.append(output["vector"])
                vector=vectors if snapshot and "frames" in snapshot else vectors[0]
            self.session.expire_all()
            current=settings(self.session)
            user=self.session.get(User,uid)
            if not user.ai_search_enabled or not current["enabled"]:
                raise AIError("ai_not_enabled",403)
            if current["maintenance"] or current["search_profile_id"]!=pid:
                raise AIError("profile_changed",409)
            if photo_id is not None:
                photo=self.session.get(Photo,photo_id)
                if not photo or photo.content_version!=source_version:
                    raise AIError("stale_content",409)
            threshold=current["similarity_threshold"] if image_query else None
            if context is not None and context.get("threshold") != threshold:
                raise AIError("search_settings_changed",409)
            remaining=60-(time.monotonic()-started)
            if remaining<=0:raise AIError("query_timeout",504)
            if pagination:
                hits, boundary=PgVectorStore().search_page(self.session,uid,profile,vector,limit=k,exclude=photo_id,
                    after=after,timeout_ms=int(remaining*1000),min_score=threshold)
                next_cursor=None
                if boundary is not None:
                    if context is None:
                        context=cursor_store.create(pid=pid,version=source_version,vector=vector,page_size=k,threshold=threshold)
                    next_cursor=cursor_store.next(context,boundary)
                completed=True
                return {"items":serialize(hits),"profileId":pid,"coverage":coverage(self.session,pid,uid),
                        "requestId":token,"similarityThreshold":threshold,"nextCursor":next_cursor,"hasMore":next_cursor is not None}
            hits=PgVectorStore().search(self.session,uid,profile,vector,limit=k,namespace_id=data.get("namespaceId"),
                gallery_id=data.get("galleryId"),exclude=photo_id,timeout_ms=int(remaining*1000),min_score=threshold)
            completed=True
            return {"items":serialize(hits),"profileId":pid,"coverage":coverage(self.session,pid,uid),"requestId":token,"similarityThreshold":threshold}
        except DBAPIError as error:
            if getattr(error.orig,"sqlstate",None)=="57014":
                raise AIError("query_timeout",504) from error
            raise
        finally:
            self.session.rollback()
            if request_job is not None:
                job=self.session.get(AIJob,request_job.job_id)
                if job:
                    job.status,job.finished_at=("succeeded" if completed else "failed"),utcnow()
                    self.session.commit()
            self.infra.sessions.client.eval("if redis.call('get',KEYS[1]) == ARGV[1] then return redis.call('del',KEYS[1]) else return 0 end",1,key,token)
