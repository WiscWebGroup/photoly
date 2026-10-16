"""Device behavior is deterministic in CI; real MPS checks are explicitly opt-in."""
from types import SimpleNamespace
import sys
import pytest
from app.ai.devices import DEVICE_PRECISIONS, probe_hardware
from app.ai.types import AIError, RuntimeConfig
from app.ai.text_image_embedding_siglip2_base import Siglip2Embedding
from app.ai.registry import profile_spec
from app.ai.runtime import Runtime, model_error, model_child
from app.scheduler.resource_policy import ResourcePolicy
from app.ai.state import utcnow


def fake_torch(monkeypatch, mps=True, cuda=False):
    calls = []
    torch = SimpleNamespace(
        float32="fp32", bfloat16="bf16", set_num_threads=lambda n: calls.append(("threads", n)),
        backends=SimpleNamespace(mps=SimpleNamespace(is_built=lambda: mps, is_available=lambda: mps)),
        mps=SimpleNamespace(empty_cache=lambda: calls.append("mps_clean")),
        cuda=SimpleNamespace(is_available=lambda: cuda, is_bf16_supported=lambda: cuda,
                             empty_cache=lambda: calls.append("cuda_clean")))
    monkeypatch.setitem(sys.modules, "torch", torch)
    return calls


@pytest.mark.parametrize("device,precision", DEVICE_PRECISIONS.items())
def test_device_precision_and_cleanup(monkeypatch, device, precision):
    calls = fake_torch(monkeypatch, cuda=True)
    model = Siglip2Embedding()
    model.prepare(profile_spec("siglip2"), RuntimeConfig(device, precision))
    assert model.dtype == ("bf16" if device == "cuda" else "fp32")
    model.close()
    assert ("mps_clean" in calls) == (device == "mps")
    assert ("cuda_clean" in calls) == (device == "cuda")


def test_mps_probe_and_unavailable(monkeypatch):
    fake_torch(monkeypatch)
    assert {d["device"]: d["available"] for d in probe_hardware()} == {"cpu": True, "cuda": False, "mps": True}
    fake_torch(monkeypatch, mps=False)
    assert probe_hardware()[2]["reason"] == "device_unavailable"
    with pytest.raises(AIError, match="device_unavailable"):
        Siglip2Embedding().prepare(profile_spec("siglip2"), RuntimeConfig("mps", "float32"))
    with pytest.raises(AIError, match="unsupported_device_precision"):
        RuntimeConfig("mps", "bfloat16")


def test_probe_failure_and_test_runtime(monkeypatch):
    runtime = Runtime.__new__(Runtime)
    monkeypatch.delenv("AI_TEST_RUNTIME", raising=False)
    def fail(*args, **kwargs):
        raise TimeoutError()
    monkeypatch.setattr("app.ai.runtime.subprocess.run", fail)
    runtime.probe_devices()
    assert len(runtime.devices) == 3
    assert all(d["available"] is None and d["reason"] == "probe_failed" for d in runtime.devices)
    monkeypatch.setenv("AI_TEST_RUNTIME", "1")
    runtime.probe_devices()
    assert [d["available"] for d in runtime.devices] == [True, False, False]


@pytest.mark.parametrize("error,code", [
    (RuntimeError("MPS backend out of memory"), "out_of_memory"),
    (AIError("device_unavailable", 503), "device_unavailable"),
    (NotImplementedError("aten::example on MPS"), "unsupported_operator"),
    (RuntimeError("MPS does not support this dtype"), "unsupported_operator"),
    (RuntimeError("other failure"), "model_load_failed"),
])
def test_load_errors_retain_reason_and_release(monkeypatch, error, code):
    monkeypatch.delenv("AI_TEST_RUNTIME", raising=False)
    cleaned = []
    def fail(*args):
        raise error
    model = SimpleNamespace(load=fail, close=lambda: cleaned.append(True))
    monkeypatch.setattr("app.ai.runtime.adapter", lambda provider: model)
    sent = []
    connection = SimpleNamespace(send=sent.append, close=lambda: None)
    model_child(connection, {"provider": "siglip2"}, {"device": "mps", "precision": "float32"})
    assert sent == [{"error": code, "status": 503}]
    assert cleaned == [True]
    assert model_error(error, "inference_failed")["error"] == ("inference_failed" if code == "model_load_failed" else code)


def test_unified_memory_admission_without_gpu_counters():
    snapshot = {"sampled_at": utcnow().isoformat(), "cpu_percent": 10, "memory_available": 4*1024**3,
                "disk_free": 10*1024**3, "gpu_percent": None, "vram_available": None}
    policy = ResourcePolicy()
    assert policy.decide(snapshot, "mps").allowed
    assert not ResourcePolicy().decide(snapshot, "cuda").allowed
    assert policy.decide({**snapshot, "memory_available": 100}, "mps").reasons == ("memory_pressure",)
    assert not policy.decide({**snapshot, "memory_available": 1500*1024**2}, "mps").allowed
    assert policy.decide(snapshot, "mps").allowed


def test_admin_mps_configuration(world):
    from tests.openapi_assertions import assert_documented_response
    client, admin = world.client, world.admin
    response = client.patch("/admin/ai/settings", headers=admin,
                            json={"generation": 0, "settings": {"device": "mps", "precision": "float32"}})
    assert response.status_code == 200
    assert response.json["t"]["device"] == "mps"
    assert_documented_response("PATCH", "/admin/ai/settings", response)
    assert client.patch("/admin/ai/settings", headers=admin,
                        json={"generation": 1, "settings": {"precision": "bfloat16"}}).status_code == 400
    caps = client.get("/admin/ai/capabilities", headers=admin).json["t"]
    assert next(d for d in caps["devices"] if d["device"] == "mps")["reason"] == "runtime_offline"


def test_mps_adapters_send_inputs_to_selected_device(monkeypatch):
    from contextlib import nullcontext
    from app.ai.text_image_embedding_qwen import QwenEmbedding
    calls = []
    fake_torch(monkeypatch)
    torch = sys.modules["torch"]
    torch.inference_mode = nullcontext
    torch.arange = lambda count, device: "rows"

    class Tensor:
        shape = (1, 3)
        device = "mps"
        def flip(self, **kw): return self
        def argmax(self, **kw): return 0
        def __getitem__(self, key): return "features"
    class Inputs(dict):
        input_ids = attention_mask = Tensor()
        def to(self, *args, **kwargs):
            calls.append((args, kwargs))
            return self
    class Processor:
        def tokenizer(self, *args, **kwargs): return {"input_ids": [[1]]}
        def __call__(self, *args, **kwargs): return Inputs()
        def apply_chat_template(self, *args, **kwargs): return ["text"]
    for cls, provider in ((Siglip2Embedding, "siglip2"), (QwenEmbedding, "qwen3_vl")):
        model = cls()
        model.prepare(profile_spec(provider), RuntimeConfig("mps", "float32"))
        model.processor = Processor()
        model.batch = lambda features: features
        if provider == "siglip2":
            model.model = SimpleNamespace(get_text_features=lambda **kw: "features", get_image_features=lambda **kw: "features")
        else:
            model.model = lambda **kw: SimpleNamespace(last_hidden_state=Tensor())
            monkeypatch.setitem(sys.modules, "qwen_vl_utils", SimpleNamespace(process_vision_info=lambda *a, **kw: (None, None)))
        assert model.encode_texts(["cat"], role="query") == "features"
        assert model.encode_images([object()], role="document") == "features"
    assert calls == [(("mps",), {}), ((), {"device": "mps", "dtype": "fp32"}), (("mps",), {}), (("mps",), {})]


def test_runtime_load_preserves_child_error(monkeypatch):
    import threading
    from collections import deque
    runtime = Runtime.__new__(Runtime)
    runtime.control_lock, runtime.io_lock = threading.Lock(), threading.Lock()
    runtime.condition = threading.Condition()
    runtime.online, runtime.background = deque(), deque()
    runtime.process = runtime.pipe = None
    runtime.generation, runtime.last_operation = -1, None
    runtime.verified = []
    killed = []
    runtime.kill = lambda: killed.append(True)
    pipe = SimpleNamespace(close=lambda: None, poll=lambda timeout: True,
                           recv=lambda: {"error": "out_of_memory", "status": 503})
    process = SimpleNamespace(start=lambda: None)
    monkeypatch.setattr("app.ai.runtime.mp.get_context", lambda method: SimpleNamespace(
        Pipe=lambda: (pipe, pipe), Process=lambda **kw: process))
    with pytest.raises(AIError, match="out_of_memory"):
        runtime.change({"command": "load", "generation": 0, "operation_id": "oom-test",
                        "profile": profile_spec("siglip2"), "config": {"device": "mps", "precision": "float32"}})
    assert runtime.state == "failed" and not runtime.verified
    assert len(killed) == 2


def test_native_launcher_reads_only_known_keys_and_does_not_expand(tmp_path, monkeypatch):
    import importlib.util
    from pathlib import Path
    path = Path(__file__).resolve().parents[1] / "scripts/mps_runtime.py"
    spec = importlib.util.spec_from_file_location("mps_launcher", path)
    launcher = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(launcher)
    for key in launcher.KEYS:
        monkeypatch.delenv(key, raising=False)
    monkeypatch.delenv("POSTGRES_APP_PASSWORD", raising=False)
    monkeypatch.setenv("AI_CONTROL_TOKEN", "environment-wins")
    monkeypatch.setenv("AI_TEST_RUNTIME", "1")
    monkeypatch.setenv("PYTORCH_ENABLE_MPS_FALLBACK", "1")
    env = tmp_path / ".env"
    env.write_text("AI_INFERENCE_TOKEN='literal-${HOME}-$(touch SHOULD_NOT_EXIST)'\nAI_CONTROL_TOKEN=ignored\nPOSTGRES_APP_PASSWORD=ignored\nAI_RUNTIME_PORT=18090\n")
    launcher.configure(env)
    import os
    assert os.environ["AI_INFERENCE_TOKEN"] == "literal-${HOME}-$(touch SHOULD_NOT_EXIST)"
    assert os.environ["AI_CONTROL_TOKEN"] == "environment-wins"
    assert "POSTGRES_APP_PASSWORD" not in os.environ
    assert "AI_TEST_RUNTIME" not in os.environ
    assert os.environ["PYTORCH_ENABLE_MPS_FALLBACK"] == "0"
    monkeypatch.setenv("AI_RUNTIME_PORT", "0")
    with pytest.raises(SystemExit, match="AI_RUNTIME_PORT"):
        launcher.configure(env)
    monkeypatch.setenv("AI_CONTROL_TOKEN", "")
    with pytest.raises(SystemExit, match="AI_CONTROL_TOKEN"):
        launcher.configure(env)


@pytest.mark.parametrize("device,precision", [("unknown", None), ([], "float32"), (None, None), ("mps", None)])
def test_invalid_runtime_pair(device, precision):
    with pytest.raises(AIError, match="unsupported_device_precision"):
        RuntimeConfig(device, precision)


def test_unstarted_child_cleanup_does_not_mask_start_error():
    runtime = Runtime.__new__(Runtime)
    closed = []
    runtime.process = SimpleNamespace(pid=None)
    runtime.pipe = SimpleNamespace(close=lambda: closed.append(True))
    runtime.kill()
    assert closed == [True] and runtime.process is None and runtime.pipe is None


def test_server_shutdown_releases_model(monkeypatch):
    import app.ai.runtime as module
    events = []
    runtime = SimpleNamespace(kill=lambda: events.append("kill"))
    def interrupted():
        raise KeyboardInterrupt()
    server = SimpleNamespace(serve_forever=interrupted, server_close=lambda: events.append("close"))
    monkeypatch.setattr(module, "Runtime", lambda: runtime)
    monkeypatch.setattr(module, "make_server", lambda *args, **kwargs: server)
    monkeypatch.setattr("signal.signal", lambda *args: None)
    module.serve()
    assert events == ["close", "kill"]
