from concurrent.futures import ThreadPoolExecutor
from datetime import timedelta
import pytest
from sqlalchemy import select
from app.ai.state import control, settings, utcnow
from app.ai.types import RuntimeConfig, AIError
from app.entities import Photo
from app.entities.ai import AIJob, AIControlState, PhotoEmbeddingState
from app.ai.hooks import enqueue_photo
from app.scheduler import background_limits, job_repository as jobs
from app.scheduler.photo_embedding_scheduler import PhotoEmbeddingScheduler
from app.scheduler.resource_policy import ResourcePolicy
from tests.test_ai import ai, ready
from tests.openapi_assertions import assert_documented_response


def queue_photos(ai, count=3):
    with ai.sessions.begin() as session:
        photos=[Photo(user_id=2,ns_id=2,photo_name="throttle",format="jpg") for _ in range(count)]
        session.add_all(photos);session.flush()
        for photo in photos:enqueue_photo(session,photo,ai.ai_pid)


def patch(ai, changes, generation=0):
    return ai.client.patch("/admin/ai/settings",headers=ai.admin,json={"generation":generation,"settings":changes})


def claim(ai, owner="worker"):
    with ai.sessions.begin() as session:return jobs.claim(session,owner,profile_id=ai.ai_pid)


def test_old_configuration_gets_compatible_defaults(world):
    response=world.client.get("/admin/ai/settings",headers=world.admin)
    cfg=response.json["t"]
    assert cfg["background_interval_seconds"]==2 and cfg["background_max_per_minute"]==0
    assert cfg["cpu_threads"]==4 and cfg["memory_resume_free_mib"]==2048
    assert_documented_response("GET","/admin/ai/settings",response)
    with world.app.app_context():
        from app.extensions import db
        control(db.session).desired={"enabled":False}
        db.session.commit()
    assert world.client.get("/admin/ai/settings",headers=world.admin).json["t"]["cpu_pause_percent"]==90


def test_settings_patch_persists_and_requires_admin_and_generation(ai):
    changes={"background_interval_seconds":12,"background_max_per_minute":5,
             "cpu_pause_percent":80,"cpu_resume_percent":50,"memory_pause_free_mib":512,"memory_resume_free_mib":1024}
    response=patch(ai,changes)
    assert response.status_code==200
    assert_documented_response("PATCH","/admin/ai/settings",response)
    assert all(response.json["t"][key]==value for key,value in changes.items())
    with ai.sessions() as session:assert settings(session)["background_max_per_minute"]==5
    assert patch(ai,{"background_max_per_minute":6}).status_code==409
    assert ai.client.patch("/admin/ai/settings",headers=ai.headers,json={"generation":1,"settings":changes}).status_code==403
    assert ai.client.patch("/admin/ai/settings",json={"generation":1,"settings":changes}).status_code==401


@pytest.mark.parametrize("changes",[
    {"background_interval_seconds":-1},{"background_interval_seconds":3601},{"background_interval_seconds":1.5},
    {"background_max_per_minute":True},{"background_max_per_minute":601},{"cpu_threads":0},{"cpu_threads":65},
    {"cpu_pause_percent":101},{"gpu_resume_percent":None},{"memory_pause_free_mib":0},
    {"disk_resume_free_mib":16777217},{"cpu_resume_percent":90},{"gpu_pause_percent":70},
    {"memory_resume_free_mib":1024},{"vram_pause_free_mib":2048},{"disk_resume_free_mib":2048},
    {"max_concurrency":8},
])
def test_invalid_limits_do_not_change_generation(ai,changes):
    assert patch(ai,changes).status_code==400
    assert ai.client.get("/admin/ai/settings",headers=ai.admin).json["t"]["generation"]==0


def test_cpu_threads_require_unload_and_reach_runtime(ai):
    assert patch(ai,{"cpu_threads":2}).json["error"]=="unload_before_thread_change"
    with ai.sessions.begin() as session:
        row=control(session);row.desired={**settings(session),"runtime":"unloaded","loaded_profile_id":None}
    assert patch(ai,{"cpu_threads":2}).status_code==200
    captured=[]
    original=ai.runtime.control
    def observe(command,profile,config,generation,operation_id):
        captured.append(config)
        return original(command,profile,config,generation,operation_id)
    ai.runtime.control=observe
    response=ai.client.post("/admin/ai/operations",headers={**ai.admin,"Idempotency-Key":"threads"},
        json={"command":"model.load","profileId":ai.ai_pid,"generation":1})
    assert response.status_code==202
    PhotoEmbeddingScheduler(ai.sessions,ai.runtime,ai.app.extensions["photoly"].storage).tick()
    assert captured[0]["cpu_threads"]==2
    assert RuntimeConfig().cpu_threads==4
    with pytest.raises(AIError):RuntimeConfig(cpu_threads=True)


def test_rolling_limit_concurrent_claims_restart_and_deleted_job(ai,monkeypatch):
    clock=[utcnow()]
    monkeypatch.setattr(background_limits,"utcnow",lambda:clock[0])
    assert patch(ai,{"background_max_per_minute":1}).status_code==200
    queue_photos(ai)
    with ThreadPoolExecutor(2) as pool:tasks=list(pool.map(lambda owner:claim(ai,owner),["a","b"]))
    assert sum(task is not None for task in tasks)==1
    task=next(task for task in tasks if task)
    with ai.sessions.begin() as session:
        session.delete(session.get(Photo,task["photo_id"]))
    # New worker/session and even deletion cannot reset the durable rolling window.
    assert claim(ai,"replacement-worker") is None
    status=ai.client.get("/admin/ai/status",headers=ai.admin)
    assert_documented_response("GET","/admin/ai/status",status)
    assert status.json["t"]["backgroundLimits"]["reason"]=="rate_limited"
    clock[0]+=timedelta(seconds=59)
    assert claim(ai) is None
    clock[0]+=timedelta(seconds=1)
    assert claim(ai) is not None


def test_interval_hot_update_and_transaction_rollback(ai,monkeypatch):
    clock=[utcnow()]
    monkeypatch.setattr(background_limits,"utcnow",lambda:clock[0])
    assert patch(ai,{"background_interval_seconds":20}).status_code==200
    queue_photos(ai)
    with ai.sessions() as session:
        assert jobs.claim(session,"rollback",profile_id=ai.ai_pid)
        session.rollback()
    assert claim(ai)
    clock[0]+=timedelta(seconds=19)
    assert claim(ai) is None
    assert patch(ai,{"background_interval_seconds":5},generation=1).status_code==200
    assert claim(ai)


def test_quota_wait_does_not_consume_rate_slot(ai):
    queue_photos(ai)
    with ai.sessions.begin() as session:
        row=control(session);row.desired={**settings(session),"quota_bytes":1}
    assert claim(ai) is None
    with ai.sessions() as session:
        assert background_limits.status(session,settings(session))["startsLastMinute"]==0
        assert not session.scalar(select(AIJob.reserved_bytes).where(AIJob.kind=="embedding").limit(1))


def test_throttle_does_not_block_online_search(ai):
    with ai.sessions.begin() as session:ready(session,ai.ai_pid)
    assert patch(ai,{"background_max_per_minute":1}).status_code==200
    queue_photos(ai)
    assert claim(ai)
    assert claim(ai) is None
    assert ai.client.post("/ai/search/text",headers=ai.headers,json={"text":"cat"}).status_code==200


def snapshot(**changes):
    return {"sampled_at":utcnow().isoformat(),"cpu_percent":0,"gpu_percent":0,
            "memory_available":8*1024**3,"vram_available":8*1024**3,"disk_free":20*1024**3,**changes}


def test_custom_resource_thresholds_and_hysteresis():
    policy=ResourcePolicy()
    limits={"memory_pause_free_mib":2048,"memory_resume_free_mib":3072}
    assert not policy.decide(snapshot(memory_available=1536*1024**2),limits=limits).allowed
    assert not policy.decide(snapshot(memory_available=2560*1024**2),limits=limits).allowed
    assert policy.decide(snapshot(memory_available=3072*1024**2),limits=limits).allowed
    policy=ResourcePolicy()
    limits={"cpu_pause_percent":50,"cpu_resume_percent":40}
    assert not policy.decide(snapshot(cpu_percent=60),limits=limits).allowed
    for _ in range(10):decision=policy.decide(snapshot(cpu_percent=30),limits=limits)
    assert decision.allowed
    policy=ResourcePolicy();limits={"gpu_pause_percent":70,"gpu_resume_percent":50}
    assert not policy.decide(snapshot(gpu_percent=80),"cuda",limits).allowed
    assert not policy.decide(snapshot(gpu_percent=55),"cuda",limits).allowed
    assert policy.decide(snapshot(gpu_percent=40),"cuda",limits).allowed
    assert not policy.decide(snapshot(vram_available=None),"cuda",limits).allowed


def test_scheduler_applies_resource_settings_without_model_reload(ai):
    scheduler=PhotoEmbeddingScheduler(ai.sessions,ai.runtime,ai.app.extensions["photoly"].storage)
    assert patch(ai,{"memory_pause_free_mib":16384,"memory_resume_free_mib":17408}).status_code==200
    scheduler.tick()
    with ai.sessions() as session:
        assert not session.scalar(select(PhotoEmbeddingState.state_id).where(PhotoEmbeddingState.status=="ready"))
    assert patch(ai,{"memory_pause_free_mib":1024,"memory_resume_free_mib":2048},generation=1).status_code==200
    scheduler.tick()
    with ai.sessions() as session:
        assert session.scalar(select(PhotoEmbeddingState.state_id).where(PhotoEmbeddingState.status=="ready"))
