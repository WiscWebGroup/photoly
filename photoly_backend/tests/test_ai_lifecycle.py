"""End-to-end state transitions for the simplified admin controls."""
import time
from types import SimpleNamespace
import sys
import pytest
from sqlalchemy import select, func
from app.ai.state import DEFAULTS, control, settings, utcnow
from app.ai.registry import profile_spec
from app.ai.types import AIError
from app.entities.ai import AIJob, EmbeddingProfile
from app.services.ai_admin import AIAdmin
from app.scheduler.photo_embedding_scheduler import PhotoEmbeddingScheduler
from app.scheduler.resource_policy import ResourcePolicy
from tests.test_ai import ai, ready
from tests.openapi_assertions import assert_documented_response


def operate(ai, command, **extra):
    generation = ai.client.get('/admin/ai/settings', headers=ai.admin).json['t']['generation']
    response = ai.client.post('/admin/ai/operations', headers={**ai.admin, 'Idempotency-Key': f'{command}:{generation}'},
        json={'command': command, 'generation': generation, **extra})
    assert response.status_code == 202, response.json
    assert_documented_response('POST', '/admin/ai/operations', response)
    return response.json['t']


def scheduler(ai):
    return PhotoEmbeddingScheduler(ai.sessions, ai.runtime, ai.app.extensions['photoly'].storage)


def test_one_click_bootstrap_and_partial_search(ai):
    with ai.sessions.begin() as session:
        control(session).desired = dict(DEFAULTS)
    job = operate(ai, 'ai.enable')
    worker = scheduler(ai)
    worker.tick()  # Accept operation; preparation is reconciled independently.
    worker.tick()
    status = ai.client.get('/admin/ai/status', headers=ai.admin)
    assert_documented_response('GET', '/admin/ai/status', status)
    cfg = status.json['t']['settings']
    assert cfg['enabled'] and cfg['scheduler'] == 'running'
    assert cfg['search_profile_id'] == ai.ai_pid and cfg['building_profile_id'] is None
    with ai.sessions() as session:
        assert session.scalar(select(func.count()).select_from(EmbeddingProfile)) == 1
        assert session.get(AIJob, job['operationId']).status == 'succeeded'
    # Unindexed media do not block activation or empty/partial searches.
    assert ai.client.post('/ai/search/text', headers=ai.headers, json={'text':'cat'}).status_code == 200


def test_pause_preserves_preferences_and_resume_uses_profile(ai):
    operate(ai, 'ai.enable')
    worker = scheduler(ai)
    worker.tick(); worker.tick()
    operate(ai, 'scheduler.pause')
    worker.tick()
    with ai.sessions.begin() as session:
        row = control(session)
        row.desired = {**settings(session), 'search_enabled':False, 'smart_tag_enabled':True}
    operate(ai, 'ai.pause')
    worker.tick(); worker.tick()
    with ai.sessions() as session:
        cfg = settings(session)
        assert not cfg['enabled'] and cfg['runtime'] == 'unloaded'
        assert cfg['scheduler'] == 'paused' and not cfg['search_enabled'] and cfg['smart_tag_enabled']
        assert cfg['search_profile_id'] == ai.ai_pid
    assert ai.runtime.state == 'unloaded'
    operate(ai, 'ai.enable')
    worker.tick(); worker.tick()
    with ai.sessions() as session:
        assert settings(session)['scheduler'] == 'paused'
        assert settings(session)['loaded_profile_id'] == ai.ai_pid
    assert ai.runtime.state == 'ready'


def test_search_toggle_does_not_disable_tag_or_master(ai):
    with ai.sessions.begin() as session:
        ready(session, ai.ai_pid)
        row = control(session)
        row.desired = {**settings(session), 'smart_tag_enabled':True}
    response = ai.client.patch('/admin/ai/settings', headers=ai.admin,
        json={'generation':0, 'settings':{'search_enabled':False}})
    assert response.status_code == 200
    cfg = response.json['t']
    assert cfg['enabled'] and cfg['smart_tag_enabled'] and not cfg['search_enabled']
    assert not ai.client.get('/ai/capabilities', headers=ai.headers).json['t']['globalEnabled']
    assert ai.client.post('/ai/search/text', headers=ai.headers, json={'text':'cat'}).status_code == 403
    assert ai.client.get('/ai/smart/settings', headers=ai.headers).json['t']['available']


def test_settings_change_does_not_supersede_startup_and_duplicate_is_idempotent(ai):
    first = operate(ai, 'ai.enable')
    response = ai.client.post('/admin/ai/operations', headers={**ai.admin, 'Idempotency-Key':'ai.enable:0'},
        json={'command':'ai.enable', 'generation':0})
    assert response.json['t']['operationId'] == first['operationId']
    assert ai.client.patch('/admin/ai/settings', headers=ai.admin,
        json={'generation':1, 'settings':{'ram_max_percent':88}}).status_code == 200
    worker = scheduler(ai)
    worker.tick(); worker.tick()
    with ai.sessions() as session:
        assert session.get(AIJob, first['operationId']).status == 'succeeded'
        assert settings(session)['lifecycle_id'] == 1
        assert settings(session)['ram_max_percent'] == 88
    assert ai.runtime.generation == 1


def test_pause_supersedes_queued_enable_and_stale_admin_rejected(ai):
    first = operate(ai, 'ai.enable')
    operate(ai, 'ai.pause')
    stale = ai.client.post('/admin/ai/operations', headers={**ai.admin, 'Idempotency-Key':'stale'},
        json={'command':'ai.enable','generation':1})
    assert stale.status_code == 409
    with ai.sessions() as session:
        assert session.get(AIJob, first['operationId']).status == 'cancelled'
    worker = scheduler(ai)
    worker.tick(); worker.tick()
    assert ai.runtime.state == 'unloaded'


def test_switch_waits_for_index_and_rejects_old_progress(ai):
    operate(ai, 'ai.enable', provider='qwen3_vl')
    worker = scheduler(ai)
    worker.tick(); worker.tick()
    with ai.sessions() as session:
        cfg = settings(session)
        pid = cfg['loaded_profile_id']
        assert cfg['search_profile_id'] == ai.ai_pid and cfg['maintenance']
    result = ai.client.get('/admin/ai/status', headers=ai.admin).json['t']
    assert result['startup']['stage'] == 'preparing_index'
    with ai.sessions.begin() as session:
        session.get(EmbeddingProfile, pid).index_status = 'ready'
    worker.tick()
    with ai.sessions() as session:
        assert settings(session)['search_profile_id'] == pid
        assert session.get(EmbeddingProfile, ai.ai_pid) is not None
    operate(ai, 'ai.pause')
    stale = ai.client.get('/admin/ai/status', headers=ai.admin).json['t']['startup']
    assert stale['stage'] == 'pausing' and stale['completed'] is None


def test_restart_reconciles_durable_lifecycle(ai):
    operate(ai, 'ai.enable')
    worker = scheduler(ai)
    worker.tick(); worker.tick()
    ai.runtime.state, ai.runtime.generation = 'unloaded', -1
    # Recreated worker uses the existing leader identity to simulate process replacement after lease recovery.
    with ai.sessions.begin() as session:
        row = control(session, 'scheduler'); row.lease_expires_at = utcnow()
    scheduler(ai).tick()
    assert ai.runtime.state == 'ready' and ai.runtime.generation == 1


def test_resource_migration_and_derived_resume(ai):
    with ai.sessions.begin() as session:
        control(session).desired = {'enabled':False, 'cpu_pause_percent':92, 'memory_pause_free_mib':512}
    cfg = ai.client.get('/admin/ai/settings', headers=ai.admin).json['t']
    assert cfg['ram_policy'] == 'free' and cfg['cpu_pause_percent'] == 92 and not cfg['enabled']
    result = ai.client.patch('/admin/ai/settings', headers=ai.admin,
        json={'generation':0, 'settings':{'cpu_pause_percent':85, 'gpu_pause_percent':90, 'ram_max_percent':85}})
    cfg = result.json['t']
    assert cfg['cpu_resume_percent'] == 75 and cfg['gpu_resume_percent'] == 80 and cfg['ram_policy'] == 'percent'
    assert not cfg['enabled']


def test_ram_hysteresis_and_missing_metrics():
    policy = ResourcePolicy()
    base = {'sampled_at':utcnow().isoformat(), 'cpu_percent':0, 'memory_total':1000,
            'memory_available':140, 'disk_free':10*1024**3}
    cfg = {'ram_policy':'percent', 'ram_max_percent':85}
    assert not policy.decide(base, 'mps', cfg).allowed
    assert not policy.decide({**base, 'memory_available':190}, 'mps', cfg).allowed
    assert policy.decide({**base, 'memory_available':200}, 'mps', cfg).allowed
    assert not policy.decide({**base, 'memory_total':None}, 'mps', cfg).allowed


def wait_until(predicate):
    deadline = time.monotonic() + 10
    while time.monotonic() < deadline:
        if predicate(): return
        time.sleep(.05)
    assert predicate()


@pytest.mark.parametrize('phase', ['downloading', 'loading'])
def test_runtime_async_pause_interrupts_long_startup(monkeypatch, phase):
    from app.ai.runtime import Runtime
    monkeypatch.setenv('AI_TEST_RUNTIME', '1')
    monkeypatch.setenv('AI_TEST_DOWNLOAD_SECONDS' if phase == 'downloading' else 'AI_TEST_LOAD_SECONDS', '60')
    runtime = Runtime()
    data = {'command':'load', 'profile':profile_spec('siglip2'), 'config':{'device':'cpu','precision':'float32'},
            'generation':1, 'operation_id':'start'}
    try:
        runtime.submit(data)
        # Loading is announced before the child is assigned; wait for the process
        # this test intends to interrupt, especially on busy CI/build hosts.
        wait_until(lambda: runtime.state == phase and runtime.process is not None and runtime.process.is_alive())
        before = runtime.process
        start = time.monotonic()
        runtime.submit({**data, 'command':'unload', 'generation':2, 'operation_id':'pause'})
        wait_until(lambda: runtime.state == 'unloaded')
        assert time.monotonic() - start < 5
        assert not before.is_alive()
        assert runtime.progress['stage'] == 'paused' and runtime.progress['lifecycleId'] == 2
        with pytest.raises(AIError, match='stale_generation'): runtime.submit(data)
    finally:
        runtime.cancel_load.set()
        with runtime.control_lock: runtime.kill()


def test_download_progress_uses_real_file_completions_and_cache(monkeypatch):
    from app.ai.text_image_embedding_siglip2_base import Siglip2Embedding
    downloaded, progress = [], []
    files = ['config.json', 'model.safetensors', 'tokenizer.json', 'pytorch_model.bin', 'README.md']
    monkeypatch.setitem(sys.modules, 'huggingface_hub', SimpleNamespace(
        HfApi=lambda: SimpleNamespace(list_repo_files=lambda *a, **kw: files),
        hf_hub_download=lambda repo, filename, **kw: downloaded.append(filename),
        snapshot_download=lambda *a, **kw: '/cached/snapshot'))
    model = Siglip2Embedding()
    model.download(profile_spec('siglip2'), lambda *args: progress.append(args))
    assert downloaded == ['config.json', 'model.safetensors', 'tokenizer.json']
    assert progress[-1] == ('downloading', 3, 3) and model.model_path == '/cached/snapshot'


def test_explicit_scheduler_preference_before_first_enable(ai):
    with ai.sessions.begin() as session: control(session).desired = dict(DEFAULTS)
    operate(ai, 'scheduler.pause')
    operate(ai, 'ai.enable')
    with ai.sessions() as session: assert settings(session)['scheduler'] == 'paused'


def test_index_failure_is_visible_and_enable_retries(ai):
    operate(ai, 'ai.enable', provider='qwen3_vl')
    with ai.sessions.begin() as session:
        pid = settings(session)['building_profile_id']
        job = session.scalar(select(AIJob).where(AIJob.kind == 'index', AIJob.profile_id == pid))
        job.status, job.result = 'failed', {'reason':'index_maintenance_failed'}
    startup = ai.client.get('/admin/ai/status', headers=ai.admin).json['t']['startup']
    assert startup['stage'] == 'failed' and startup['reason'] == 'index_maintenance_failed'
    operate(ai, 'ai.enable')
    with ai.sessions() as session:
        job = session.scalar(select(AIJob).where(AIJob.kind == 'index', AIJob.profile_id == pid))
        assert job.status == 'queued' and job.attempts == 0


def test_offline_download_uses_cache_and_missing_cache_reports_failure(monkeypatch):
    from app.ai.text_image_embedding_siglip2_base import Siglip2Embedding
    def offline(*args, **kwargs): raise OSError('offline')
    hub = SimpleNamespace(HfApi=lambda:SimpleNamespace(list_repo_files=offline),
                          hf_hub_download=offline, snapshot_download=lambda *a, **kw:'/cached')
    monkeypatch.setitem(sys.modules, 'huggingface_hub', hub)
    model = Siglip2Embedding()
    model.download(profile_spec('siglip2'), lambda *a:None)
    assert model.model_path == '/cached'
    hub.snapshot_download = offline
    with pytest.raises(AIError, match='model_download_failed'):
        Siglip2Embedding().download(profile_spec('siglip2'), lambda *a:None)


def test_pause_waits_for_observed_unload_and_legacy_disabled_is_paused(ai):
    initial = ai.client.get('/admin/ai/status', headers=ai.admin).json['t']
    assert initial['settings']['enabled']
    operate(ai, 'ai.pause')
    with ai.sessions.begin() as session:
        cfg = settings(session)
        runtime = control(session, 'runtime')
        runtime.heartbeat_at = utcnow()
        runtime.observed = {'generation':cfg['lifecycle_id'], 'state':'draining'}
    assert ai.client.get('/admin/ai/status', headers=ai.admin).json['t']['startup']['stage'] == 'draining'
    with ai.sessions.begin() as session:
        runtime = control(session, 'runtime')
        runtime.observed = {**runtime.observed, 'state':'unloaded'}
    assert ai.client.get('/admin/ai/status', headers=ai.admin).json['t']['startup']['stage'] == 'paused'
    with ai.sessions.begin() as session:
        row = control(session); row.desired = {**settings(session), 'managed_lifecycle':False}
        control(session, 'runtime').observed = {'state':'ready'}
    assert ai.client.get('/admin/ai/status', headers=ai.admin).json['t']['startup']['stage'] == 'paused'


def test_async_pause_also_interrupts_legacy_synchronous_load(monkeypatch):
    from concurrent.futures import ThreadPoolExecutor
    from app.ai.runtime import Runtime
    monkeypatch.setenv('AI_TEST_RUNTIME', '1')
    monkeypatch.setenv('AI_TEST_DOWNLOAD_SECONDS', '60')
    runtime = Runtime()
    data = {'command':'load', 'profile':profile_spec('siglip2'), 'config':{'device':'cpu','precision':'float32'},
            'generation':1, 'operation_id':'legacy'}
    try:
        with ThreadPoolExecutor(1) as pool:
            future = pool.submit(runtime.change, data)
            wait_until(lambda:runtime.state == 'downloading')
            runtime.submit({**data, 'command':'unload', 'generation':2, 'operation_id':'pause'})
            with pytest.raises(AIError, match='superseded'): future.result(timeout=5)
            wait_until(lambda:runtime.state == 'unloaded')
    finally:
        runtime.cancel_load.set()
        with runtime.control_lock: runtime.kill()
