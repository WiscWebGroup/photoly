from concurrent.futures import ThreadPoolExecutor
from datetime import timedelta
import io
import os
from pathlib import Path
from uuid import uuid4
import numpy as np
import pytest
from PIL import Image
from sqlalchemy import select, update, func, text, create_engine
from sqlalchemy.orm import sessionmaker
from sqlalchemy.exc import IntegrityError
from app.extensions import db
from app.entities import User, Photo
from app.entities.ai import AIJob, EmbeddingProfile, PhotoEmbeddingState as State
from app.ai.state import control, settings, utcnow
from app.ai.registry import profile_spec
from app.ai.types import AIError
from app.ai.preprocessing import decode_image, read_bounded, MAX_BYTES
from app.ai.snapshot import snapshot_photo
from app.ai.hooks import enqueue_photo
from app.ai.vector_store_pgvector import PgVectorStore
from app.services.ai_admin import AIAdmin, coverage
from app.scheduler import job_repository as jobs
from app.scheduler.photo_embedding_scheduler import PhotoEmbeddingScheduler
from app.scheduler.maintenance import Maintenance
from app.scheduler.resource_policy import ResourcePolicy
from tests.openapi_assertions import assert_documented_response


class FakeRuntime:
    def __init__(self,spec):
        self.spec,self.state,self.generation=spec,"ready",0
        self.after_encode=None
    def status(self):
        return {"state":self.state,"fingerprint":self.spec["fingerprint"] if self.spec else None,"generation":self.generation,
            "resources":{"sampled_at":utcnow().isoformat(),"cpu_percent":1,"memory_total":16*1024**3,"memory_available":8*1024**3,"disk_free":20*1024**3,"gpu_percent":0,"vram_available":8*1024**3}}
    def control(self,command,profile,config,generation,operation_id):
        self.state="unloaded" if command=="unload" else "ready"
        self.spec,self.generation=profile,generation
        return self.status()
    def encode(self,spec,**kwargs):
        if self.after_encode:self.after_encode()
        vector=np.zeros(spec["dimension"],dtype=np.float32);vector[0]=1
        return {"vector":vector.tolist(),"dimension":len(vector),"fingerprint":spec["fingerprint"]}


@pytest.fixture
def ai(world):
    with world.app.app_context():
        admin=AIAdmin(db.session)
        pid=admin.create_profile("siglip2")["profileId"]
        profile=db.session.get(EmbeddingProfile,pid);profile.index_status="ready"
        row=control(db.session)
        row.desired={**settings(db.session),"enabled":True,"scheduler":"running","runtime":"loaded","loaded_profile_id":pid,"search_profile_id":pid,"background_interval_seconds":0}
        db.session.get(User,2).ai_search_enabled=True
        db.session.commit()
        runtime=FakeRuntime(dict(profile.spec))
        sessions=sessionmaker(db.engine,expire_on_commit=False)
    world.app.config["AI_RUNTIME_CLIENT"]=runtime
    world.ai_pid,world.runtime,world.sessions=pid,runtime,sessions
    return world


def ready(session,pid,photo_id=1,vector=None):
    if vector is None:
        vector=np.zeros(768,dtype=np.float32);vector[0]=1
    photo=session.get(Photo,photo_id)
    session.add(State(profile_id=pid,photo_id=photo_id,content_version=photo.content_version,status="ready",embedding=vector))


def test_defaults_and_legacy_json(world):
    response=world.client.get("/ai/capabilities",headers=world.headers)
    assert response.status_code==200 and not response.json["t"]["globalEnabled"]
    assert_documented_response("GET","/ai/capabilities",response)
    assert "aiSearchEnabled" not in world.client.get("/user/getInfo",headers=world.headers).json["t"]
    photos=world.client.get("/photo/getByNamespace?nsId=3",headers=world.headers).json["t"]
    assert "contentVersion" not in photos[0] and "contentSha256" not in photos[0]


@pytest.mark.parametrize("method,path,payload",[
    ("get","/admin/ai/status",None),("get","/admin/ai/settings",None),("get","/admin/ai/profiles",None),
    ("get","/admin/ai/jobs",None),("get","/admin/ai/users?ids=1,2",None),
    ("patch","/admin/users/2/ai",{"enabled":True}),
    ("post","/admin/ai/profiles",{"provider":"siglip2"}),
])
def test_admin_auth_and_contracts(world,method,path,payload):
    call=getattr(world.client,method)
    assert call(path,json=payload).status_code==401
    assert call(path,json=payload,headers=world.headers).status_code==403
    response=call(path,json=payload,headers=world.admin)
    assert response.status_code==200
    assert_documented_response(method.upper(),path.split('?')[0].replace('/users/2/','/users/{userId}/'),response)


def test_settings_generation_new_user_default_and_validation(world):
    client=world.client
    changed=client.patch("/admin/ai/settings",headers=world.admin,json={"generation":0,"settings":{"new_user_default":True}})
    assert changed.json["t"]["generation"]==1
    assert client.patch("/admin/ai/settings",headers=world.admin,json={"generation":0,"settings":{"enabled":True}}).status_code==409
    for value in (1,"true",None):
        assert client.patch("/admin/users/2/ai",headers=world.admin,json={"enabled":value}).status_code==400
    response=client.post("/user/signUp",json={"email":"aienabled@test.local","password":"secret"})
    assert response.json["msgCode"]==200
    with world.app.app_context():
        assert db.session.scalar(select(User).where(User.email=="aienabled@test.local")).ai_search_enabled
        assert not db.session.get(User,2).ai_search_enabled


def test_profile_is_pinned_immutable_and_vector_checked(ai):
    with ai.app.app_context():
        profile=db.session.get(EmbeddingProfile,ai.ai_pid)
        assert profile.spec==profile_spec("siglip2")
        profile.dimension=2048
        with pytest.raises(IntegrityError):db.session.commit()
        db.session.rollback()
        for vector in ([1,2],np.zeros(768)):
            ready(db.session,ai.ai_pid,vector=vector)
            with pytest.raises(IntegrityError):db.session.commit()
            db.session.rollback()
        ready(db.session,ai.ai_pid)
        db.session.commit()


def test_job_dedup_concurrent_claim_and_expired_fence(ai):
    with ai.sessions.begin() as s:
        photo=s.get(Photo,1)
        enqueue_photo(s,photo,ai.ai_pid);enqueue_photo(s,photo,ai.ai_pid)
    def claim(owner):
        with ai.sessions.begin() as s:return jobs.claim(s,owner,profile_id=ai.ai_pid)
    with ThreadPoolExecutor(2) as pool:tasks=list(pool.map(claim,["a","b"]))
    assert sum(t is not None for t in tasks)==1
    old=next(t for t in tasks if t)
    with ai.sessions.begin() as s:
        job=s.get(AIJob,old["id"]);job.lease_expires_at=utcnow()-timedelta(seconds=1)
    new=claim("c")
    assert old["id"]==new["id"] and old["token"]!=new["token"]
    with ai.sessions.begin() as s:
        assert not jobs.finish(s,old)
        assert jobs.finish(s,new)
        assert jobs.usage(s)["reserved_bytes"]==0


def test_quota_wait_does_not_consume_attempt_and_disable_cancels(ai):
    with ai.sessions.begin() as s:
        row=control(s);row.desired={**settings(s),"quota_bytes":1}
        enqueue_photo(s,s.get(Photo,1),ai.ai_pid)
    with ai.sessions.begin() as s:
        assert jobs.claim(s,"a",profile_id=ai.ai_pid) is None
        job=s.scalar(select(AIJob).where(AIJob.kind=="embedding"))
        assert job.attempts==0 and job.result["reason"]=="quota_exceeded"
    assert ai.client.patch("/admin/users/2/ai",headers=ai.admin,json={"enabled":False}).status_code==200
    with ai.sessions() as s:
        assert s.scalar(select(AIJob.status).where(AIJob.kind=="embedding"))=="cancelled"


def test_scheduler_embedding_and_unsupported_reconciliation(ai):
    scheduler=PhotoEmbeddingScheduler(ai.sessions,ai.runtime,ai.app.extensions["photoly"].storage)
    scheduler.tick()
    with ai.sessions() as s:
        state=s.scalar(select(State).where(State.photo_id==1))
        assert state.status=="ready" and state.content_sha256
        video=s.scalar(select(State).where(State.photo_id==2))
        assert video.status=="pending"
        assert jobs.usage(s)["reserved_bytes"]==0
    scheduler.tick()
    with ai.sessions() as s:assert s.scalar(select(func.count()).select_from(AIJob).where(AIJob.kind=="embedding"))==2


def test_stable_snapshot_and_replaced_result_rejected(ai,monkeypatch):
    with ai.sessions.begin() as s:enqueue_photo(s,s.get(Photo,1),ai.ai_pid)
    with ai.sessions.begin() as s:task=jobs.claim(s,"owner",profile_id=ai.ai_pid)
    with ai.sessions() as session:
        snapshot=snapshot_photo(session,ai.app.extensions["photoly"].storage,1,2,task["version"])
    stream=io.BytesIO();Image.new("RGB",(50,50),"blue").save(stream,"JPEG");stream.seek(0)
    response=ai.client.post("/photo/replace?photoId=1",headers=ai.headers,data={"file":(stream,"new.jpg")})
    assert response.json["msgCode"]==200
    with ai.sessions.begin() as s:
        assert not jobs.commit_embedding(s,task,snapshot,[1]+[0]*767,10)
        assert s.get(Photo,1).content_version==2
        assert s.scalar(select(State.status).where(State.photo_id==1))=="pending"
    assert snapshot["data"]==ai.jpeg


def test_search_text_image_similar_permissions_and_filtering(ai):
    with ai.sessions.begin() as s:
        ready(s,ai.ai_pid)
        foreign=Photo(user_id=3,ns_id=4,photo_name="private",format="jpg",content_version=1)
        s.add(foreign);s.flush();ready(s,ai.ai_pid,foreign.photo_id)
    text_response=ai.client.post("/ai/search/text",headers=ai.headers,json={"text":"海边"})
    assert text_response.status_code==200
    assert [x["photo"]["photoId"] for x in text_response.json["t"]["items"]]==[1]
    assert_documented_response("POST","/ai/search/text",text_response)
    response=ai.client.post("/ai/search/image",headers=ai.headers,data={"file":(io.BytesIO(ai.jpeg),"query.jpg")})
    assert response.status_code==200 and len(response.json["t"]["items"])==1
    assert ai.client.post("/ai/photos/1/similar",headers=ai.headers,json={}).json["t"]["items"]==[]
    assert ai.client.post("/ai/search/text",headers=ai.headers,json={"text":"x","namespaceId":4}).status_code==404
    assert ai.client.post("/ai/photos/99999/similar",headers=ai.headers,json={}).status_code==404
    assert ai.client.post("/ai/search/text",headers=ai.headers,json={"text":"x","topK":101}).status_code==400
    with ai.sessions() as s:assert s.scalar(select(func.count()).select_from(Photo))==3


def test_disable_during_inference_and_busy(ai):
    def disable():
        with ai.sessions.begin() as s:s.get(User,2).ai_search_enabled=False
    ai.runtime.after_encode=disable
    response=ai.client.post("/ai/search/text",headers=ai.headers,json={"text":"hello"})
    assert response.status_code==403
    client=ai.app.extensions["photoly"].sessions.client
    client.set("photoly:ai-query:2","other",ex=60)
    try:assert ai.client.post("/ai/search/text",headers=ai.headers,json={"text":"hello"}).status_code==429
    finally:client.delete("photoly:ai-query:2")


def test_operation_idempotency_and_scheduler_control(ai):
    payload={"command":"scheduler.pause","generation":0}
    headers={**ai.admin,"Idempotency-Key":"pause-1"}
    first=ai.client.post("/admin/ai/operations",headers=headers,json=payload)
    assert first.status_code==202
    assert_documented_response("POST","/admin/ai/operations",first)
    again=ai.client.post("/admin/ai/operations",headers=headers,json=payload)
    assert again.json==first.json
    assert ai.client.post("/admin/ai/operations",headers=headers,json={**payload,"command":"scheduler.stop"}).status_code==409
    scheduler=PhotoEmbeddingScheduler(ai.sessions,ai.runtime,ai.app.extensions["photoly"].storage)
    scheduler.tick()
    op=ai.client.get('/admin/ai/operations/'+str(first.json['t']['operationId']),headers=ai.admin)
    assert op.json["t"]["status"]=="succeeded"
    with ai.sessions() as s:assert settings(s)["scheduler"]=="paused"
    assert ai.client.post("/admin/ai/operations",headers={**ai.admin,"Idempotency-Key":"purge"},json={"command":"purge","profileId":ai.ai_pid,"generation":1}).status_code==409


def test_activation_requires_coverage_and_maintenance_for_cross_model(ai):
    created=ai.client.post("/admin/ai/profiles",headers=ai.admin,json={"provider":"qwen3_vl"}).json["t"]
    response=ai.client.post("/admin/ai/operations",headers={**ai.admin,"Idempotency-Key":"qwen"},json={"command":"model.load","profileId":created["profileId"],"generation":0})
    assert response.status_code==409 and response.json["error"]=="maintenance_required"
    with ai.sessions.begin() as s:
        admin=AIAdmin(s)
        result=admin.operation(1,{"command":"activate","profileId":ai.ai_pid,"generation":0},"activation")
    scheduler=PhotoEmbeddingScheduler(ai.sessions,ai.runtime,ai.app.extensions["photoly"].storage)
    scheduler.tick()
    with ai.sessions() as s:assert s.get(AIJob,result["operationId"]).status=="failed"


def test_input_and_resource_limits():
    with pytest.raises(AIError):decode_image(b"bad")
    with pytest.raises(AIError):read_bounded(io.BytesIO(b"x"*(MAX_BYTES+1)))
    stream=io.BytesIO();Image.new("RGBA",(8,8),(0,0,0,0)).save(stream,"PNG")
    assert decode_image(stream.getvalue()).getpixel((0,0))==(255,255,255)
    policy=ResourcePolicy()
    snapshot={"sampled_at":utcnow().isoformat(),"cpu_percent":0,"memory_available":8*1024**3,"disk_free":20*1024**3}
    assert policy.decide(snapshot).allowed
    assert not policy.decide(snapshot,"cuda").allowed
    assert not policy.decide({**snapshot,"memory_available":0}).allowed


def test_runtime_process_restart_auth_and_fingerprint(monkeypatch):
    from app.ai.runtime import Runtime,create_runtime_app
    monkeypatch.setenv("AI_TEST_RUNTIME","1")
    monkeypatch.setenv("AI_INFERENCE_TOKEN","infer")
    monkeypatch.setenv("AI_CONTROL_TOKEN","control")
    runtime=Runtime();client=create_runtime_app(runtime).test_client()
    spec=profile_spec("siglip2")
    try:
        assert client.get("/status").status_code==401
        assert client.post("/control",headers={"Authorization":"Bearer infer"},json={}).status_code==401
        data={"command":"load","profile":spec,"config":{"device":"cpu","precision":"float32"},"generation":1,"operation_id":"a"}
        assert client.post("/control",headers={"Authorization":"Bearer control"},json=data).json["state"]=="ready"
        result=client.post("/encode",headers={"Authorization":"Bearer infer"},json={"fingerprint":spec["fingerprint"],"role":"query","text":"hello"})
        assert result.status_code==200 and len(result.json["vector"])==768
        assert client.post("/encode",headers={"Authorization":"Bearer infer"},json={"fingerprint":"wrong","role":"query","text":"hello"}).status_code==503
        assert client.post("/control",headers={"Authorization":"Bearer control"},json={**data,"generation":0,"operation_id":"old"}).status_code==409
        assert client.post("/control",headers={"Authorization":"Bearer control"},json={**data,"command":"unload","generation":2,"operation_id":"b"}).json["state"]=="unloaded"
    finally:runtime.kill()


def test_retries_recover_reservations_and_cap_attempts(ai):
    with ai.sessions.begin() as session:enqueue_photo(session,session.get(Photo,1),ai.ai_pid)
    for attempt in range(5):
        with ai.sessions.begin() as session:
            job=session.scalar(select(AIJob).where(AIJob.kind=="embedding"));job.next_run_at=utcnow()-timedelta(seconds=1)
        with ai.sessions.begin() as session:task=jobs.claim(session,"worker",profile_id=ai.ai_pid)
        with ai.sessions.begin() as session:
            jobs.fail(session,task,"bad_inference")
            assert jobs.usage(session)["reserved_bytes"]==0
    with ai.sessions() as session:
        assert session.get(AIJob,task["id"]).attempts==5
        assert session.get(AIJob,task["id"]).status=="failed"
        assert session.scalar(select(State.status).where(State.photo_id==1))=="failed"


def test_delete_running_photo_cannot_recreate_vector(ai):
    with ai.sessions.begin() as session:enqueue_photo(session,session.get(Photo,1),ai.ai_pid)
    with ai.sessions.begin() as session:task=jobs.claim(session,"worker",profile_id=ai.ai_pid)
    with ai.sessions() as session:snapshot=snapshot_photo(session,ai.app.extensions["photoly"].storage,1,2)
    assert ai.client.post("/photo/delete?photoId=1",headers=ai.headers).json["msgCode"]==200
    with ai.sessions.begin() as session:
        assert not jobs.commit_embedding(session,task,snapshot,[1]+[0]*767,1)
        assert not session.scalar(select(State.state_id).where(State.photo_id==1))
        assert jobs.usage(session)["reserved_bytes"]==0


def test_snapshot_open_descriptor_survives_atomic_replace(ai,monkeypatch):
    storage=ai.app.extensions["photoly"].storage
    with ai.sessions() as session:
        original_commit=session.commit
        def replace_after_unlock():
            original_commit()
            with ai.sessions.begin() as writer:
                user=writer.scalar(select(User).where(User.user_id==2).with_for_update())
                photo=writer.get(Photo,1)
                path=storage.media(user.uuid,photo)
                alternative=path.with_suffix(".new")
                alternative.write_bytes(b"different bytes")
                os.replace(alternative,path)
                photo.content_version+=1
        monkeypatch.setattr(session,"commit",replace_after_unlock)
        snapshot=snapshot_photo(session,storage,1,2)
        assert snapshot["data"]==ai.jpeg and snapshot["version"]==1


def test_independent_maintenance_hnsw_recall_and_generic_plan(ai,migration_engine):
    from sqlalchemy import insert
    import time
    rng=np.random.default_rng(184)
    vectors=rng.normal(size=(6200,768)).astype(np.float32)
    vectors/=np.linalg.norm(vectors,axis=1,keepdims=True)
    identifiers=[]
    with ai.sessions.begin() as session:
        for start in range(0,len(vectors),200):
            values=[{"photo_name":"ann", "format":"jpg", "user_id":2 if i<6000 else 3,"ns_id":2 if i<6000 else 4,"content_version":1} for i in range(start,min(start+200,len(vectors)))]
            ids=list(session.scalars(insert(Photo).returning(Photo.photo_id),values))
            identifiers.extend(ids)
            session.execute(insert(State),[{"photo_id":pid,"profile_id":ai.ai_pid,"content_version":1,"status":"ready","embedding":vectors[start+j]} for j,pid in enumerate(ids)])
    name=f"ix_embedding_profile_{ai.ai_pid}_hnsw"
    try:
        Maintenance(ai.sessions,migration_engine).tick()
        with migration_engine.connect().execution_options(isolation_level="AUTOCOMMIT") as conn:
            assert conn.scalar(text("SELECT indisvalid FROM pg_index JOIN pg_class ON indexrelid=oid WHERE relname=:name"),{"name":name})
            conn.exec_driver_sql("ANALYZE photo_embedding_state")
            conn.exec_driver_sql("ANALYZE photo")
        recalls=[];durations=[]
        with ai.sessions() as session:
            profile=session.get(EmbeddingProfile,ai.ai_pid)
            for index in (0,11,93,511,999,1501,2222,3199,4011,5900):
                query=vectors[index]
                expected={identifiers[i] for i in np.argsort(-(vectors[:6000]@query))[:20]}
                start=time.perf_counter()
                actual=PgVectorStore().search(session,2,profile,query,limit=20)
                durations.append(time.perf_counter()-start)
                ids={item["photo"].photo_id for item in actual}
                assert all(item["photo"].user_id==2 for item in actual)
                recalls.append(len(expected & ids)/20)
        assert np.mean(recalls)>=0.95
        print(f"ANN Recall@20={np.mean(recalls):.4f}, p95={np.percentile(durations,95):.4f}s")
        with migration_engine.begin() as conn:
            conn.exec_driver_sql("SET LOCAL enable_seqscan=off")
            conn.exec_driver_sql("SET LOCAL plan_cache_mode=force_generic_plan")
            conn.exec_driver_sql(f"PREPARE ai_plan(halfvec, integer) AS SELECT e.photo_id FROM photo_embedding_state e JOIN photo p ON p.photo_id=e.photo_id WHERE e.profile_id={ai.ai_pid} AND e.status='ready' AND e.content_version=p.content_version AND p.user_id=$2 ORDER BY e.embedding::halfvec(768) <=> $1 LIMIT 20")
            literal="["+",".join(str(float(x)) for x in vectors[0])+"]"
            plan=conn.exec_driver_sql("EXPLAIN EXECUTE ai_plan('"+literal+"'::halfvec,2)").all()
            assert name in str(plan)
            conn.exec_driver_sql("DEALLOCATE ai_plan")
    finally:
        with migration_engine.connect().execution_options(isolation_level="AUTOCOMMIT") as conn:
            conn.exec_driver_sql(f'DROP INDEX CONCURRENTLY IF EXISTS "{name}"')


def test_runtime_oom_timeout_and_recovery(monkeypatch):
    from app.ai.runtime import Runtime
    monkeypatch.setenv("AI_TEST_RUNTIME","1")
    runtime=Runtime();spec=profile_spec("siglip2")
    config={"device":"cpu","precision":"float32"}
    try:
        runtime.change({"command":"load","profile":spec,"config":config,"generation":1,"operation_id":"1"})
        with pytest.raises(AIError,match="out_of_memory"):
            runtime.encode({"fingerprint":spec["fingerprint"],"role":"query","text":"__test_oom__"})
        with pytest.raises(AIError):
            runtime.encode({"fingerprint":spec["fingerprint"],"role":"query","text":"__test_timeout__","timeout":.1})
        runtime.change({"command":"restart","profile":spec,"config":config,"generation":2,"operation_id":"2"})
        assert runtime.encode({"fingerprint":spec["fingerprint"],"role":"query","text":"recovered"})["dimension"]==768
    finally:runtime.kill()


def test_crashed_search_expires_without_requeue(ai):
    with ai.sessions.begin() as s:
        job=AIJob(kind="search",profile_id=ai.ai_pid,dedup_key="expired-search",status="running",lease_expires_at=utcnow()-timedelta(seconds=1))
        s.add(job);s.flush();jid=job.job_id
        jobs.recover(s)
    with ai.sessions() as s:assert s.get(AIJob,jid).status=="failed"


def test_restart_cannot_bypass_maintenance(ai):
    with ai.sessions.begin() as s:
        pid=AIAdmin(s).create_profile("qwen3_vl")["profileId"]
        with pytest.raises(AIError,match="maintenance_required"):
            AIAdmin(s).operation(1,{"command":"model.restart","profileId":pid,"generation":0},"restart-other")


def test_search_failure_recorded_and_vector_reuse_requires_runtime(ai):
    ai.runtime.after_encode=lambda: (_ for _ in ()).throw(AIError("runtime_offline",503))
    assert ai.client.post("/ai/search/text",headers=ai.headers,json={"text":"test"}).status_code==503
    with ai.sessions() as s:assert s.scalar(select(AIJob.status).where(AIJob.kind=="search"))=="failed"


def test_invalid_admin_input_is_client_error(ai):
    assert ai.client.post("/admin/ai/profiles",headers=ai.admin,json={"provider":[]}).status_code==400
    assert ai.client.post("/admin/ai/operations",headers={**ai.admin,"Idempotency-Key":"invalid"},json={"command":[],"generation":0}).status_code==400


def test_reconciliation_rotates_users_before_large_library_finishes(ai):
    with ai.sessions.begin() as s:
        s.get(User,3).ai_search_enabled=True
        s.add(Photo(photo_name="small",format="jpg",user_id=3,ns_id=4))
        s.add_all([Photo(photo_name="large",format="jpg",user_id=2,ns_id=2) for _ in range(100)])
    scheduler=PhotoEmbeddingScheduler(ai.sessions,ai.runtime,ai.app.extensions["photoly"].storage)
    with ai.sessions.begin() as s:scheduler.reconcile(s)
    with ai.sessions() as s:
        counts=dict(s.execute(select(Photo.user_id,func.count()).join(AIJob,AIJob.photo_id==Photo.photo_id).where(AIJob.kind=="embedding").group_by(Photo.user_id)).all())
        assert counts[2]<=25 and counts[3]>=1


def test_purge_retired_profile_retains_operation_audit(ai,migration_engine):
    with ai.sessions.begin() as s:pid=AIAdmin(s).create_profile("qwen3_vl")["profileId"]
    maintenance=Maintenance(ai.sessions,migration_engine)
    maintenance.tick();maintenance.tick()
    with ai.sessions.begin() as s:
        op=AIAdmin(s).operation(1,{"command":"purge","profileId":pid,"generation":0},"retired")
    scheduler=PhotoEmbeddingScheduler(ai.sessions,ai.runtime,ai.app.extensions["photoly"].storage)
    try:
        scheduler.tick();maintenance.tick()
        with ai.sessions() as s:
            assert s.get(EmbeddingProfile,pid) is None
            assert s.get(AIJob,op["operationId"]).status=="succeeded"
    finally:
        with migration_engine.connect().execution_options(isolation_level="AUTOCOMMIT") as conn:
            for identifier in (ai.ai_pid,pid):conn.exec_driver_sql(f'DROP INDEX CONCURRENTLY IF EXISTS "ix_embedding_profile_{identifier}_hnsw"')


def test_mpo_jpeg_uses_primary_frame_and_preserves_orientation():
    # Generate real MPF/MPO bytes, as emitted by cameras with a .JPG suffix.
    first = Image.new("RGB", (12, 8), (240, 20, 10))
    second = Image.new("RGB", (12, 8), (10, 20, 240))
    exif = Image.Exif(); exif[274] = 6
    stream = io.BytesIO()
    first.save(stream, format="MPO", save_all=True, append_images=[second], exif=exif)
    data = stream.getvalue()
    with Image.open(io.BytesIO(data)) as source:
        assert source.format == "MPO" and source.n_frames == 2
    decoded = decode_image(data)
    assert decoded.mode == "RGB" and decoded.size == (8, 12)
    red, _, blue = decoded.getpixel((4, 6))
    assert red > 200 and blue < 50  # Primary image, never the second frame.


def test_mpo_still_obeys_pixel_limit(monkeypatch):
    from app.ai import preprocessing
    stream = io.BytesIO()
    Image.new("RGB", (12, 8)).save(stream, format="MPO", save_all=True,
                                  append_images=[Image.new("RGB", (12, 8))])
    monkeypatch.setattr(preprocessing, "MAX_PIXELS", 95)
    with pytest.raises(AIError, match="image_too_many_pixels"):
        decode_image(stream.getvalue())
