"""One control process, one replaceable inference child, no database or media mount."""
from dataclasses import asdict
import base64
from collections import deque
import hmac
import multiprocessing as mp
import os
import json
import subprocess
import threading
import time
from flask import Flask, request, jsonify
from werkzeug.serving import make_server
from .types import AIError, RuntimeConfig
from .devices import device_status
from .registry import adapter, validate_spec
from .preprocessing import decode_image, MAX_BYTES
from app.util.sys_usage_util import system_usage


def model_error(error, default):
    if isinstance(error, AIError):
        return {"error": error.code, "status": error.status}
    message = str(error).lower()
    if "out of memory" in message:
        return {"error": "out_of_memory", "status": 503}
    if isinstance(error, NotImplementedError) or ("mps" in message and any(
            marker in message for marker in ("not implemented", "not supported", "does not support"))):
        return {"error": "unsupported_operator", "status": 503}
    return {"error": default, "status": 503 if default == "model_load_failed" else 502}


def model_child(connection, spec, config):
    model = None
    try:
        if os.getenv("AI_TEST_RUNTIME") == "1":
            from tests.ai_fake import FakeEmbedding
            model = FakeEmbedding()
        else:
            model = adapter(spec["provider"])
        model.load(spec, RuntimeConfig(**config))
        connection.send({"stage": "warming"})
        from PIL import Image
        # Readiness is proven by both modalities, not merely a successful allocation.
        model.encode_texts(["a photo"], role="query")
        model.encode_images([Image.new("RGB", (224,224), "gray")], role="document")
        connection.send({"ready": True})
        while True:
            data = connection.recv()
            try:
                if "image" in data:
                    image = decode_image(base64.b64decode(data["image"], validate=True))
                    batch = model.encode_images([image], role=data["role"])
                else:
                    batch = model.encode_texts([data["text"]], role=data["role"])
                connection.send({"vector": batch.vectors[0].tolist(), "dimension": batch.dimension, "fingerprint": batch.fingerprint})
            except AIError as error:
                connection.send({"error": error.code, "status": error.status})
            except Exception as error:
                connection.send(model_error(error, "inference_failed"))
    except Exception as error:
        try:
            connection.send(model_error(error, "model_load_failed"))
        except OSError:
            pass
    finally:
        if model:
            model.close()
        connection.close()


class Runtime:
    def __init__(self):
        self.condition = threading.Condition()
        self.control_lock, self.io_lock = threading.Lock(), threading.Lock()
        self.online, self.background = deque(), deque()
        self.process = self.pipe = None
        self.state, self.spec = "unloaded", None
        self.config = asdict(RuntimeConfig())
        self.generation, self.last_operation, self.online_streak = -1, None, 0
        self.verified = []
        self.devices = device_status(reason="probing")
        threading.Thread(target=self.probe_devices, daemon=True).start()
        threading.Thread(target=self.dispatch, daemon=True).start()

    def probe_devices(self):
        try:
            if os.getenv("AI_TEST_RUNTIME") == "1":
                self.devices = device_status(False, "device_unavailable")
                self.devices[0].update(available=True, reason=None)
            else:
                import sys
                result = subprocess.run([sys.executable, "-c",
                    "import json; from app.ai.devices import probe_hardware; print(json.dumps(probe_hardware()))"],
                    capture_output=True, text=True, timeout=30, check=True)
                self.devices = json.loads(result.stdout.strip())
        except Exception:
            self.devices = device_status(reason="probe_failed")

    def kill(self):
        process, self.process = self.process, None
        if process and process.pid is not None:
            if process.is_alive():
                process.terminate()
            process.join(timeout=5)
            if process.is_alive():
                process.kill()
                process.join(timeout=5)
        if self.pipe:
            self.pipe.close()
            self.pipe = None

    def rpc(self, data, timeout):
        if not self.pipe or not self.process or not self.process.is_alive():
            raise AIError("runtime_offline", 503)
        try:
            self.pipe.send(data)
            if not self.pipe.poll(timeout):
                self.state = "failed"
                self.kill()
                raise AIError("inference_timeout", 504)
            result = self.pipe.recv()
        except (EOFError, OSError) as error:
            self.state = "failed"
            raise AIError("runtime_offline", 503) from error
        if "error" in result:
            raise AIError(result["error"], result.get("status", 502))
        return result

    def change(self, data):
        if not isinstance(data,dict):raise AIError("invalid_control")
        if data.get("command") not in ("load", "unload", "restart") or type(data.get("generation")) is not int:
            raise AIError("invalid_control")
        with self.control_lock:
            if data["generation"] < self.generation:
                raise AIError("stale_generation", 409)
            if data.get("operation_id") == self.last_operation and self.state in ("ready", "unloaded"):
                return self.status()
            spec = data.get("profile")
            if data["command"] != "unload":
                validate_spec(spec)
                try:
                    config = asdict(RuntimeConfig(**data["config"]))
                except (TypeError, KeyError):
                    raise AIError("invalid_runtime_config")
            self.generation = data["generation"]
            self.state = "draining"
            with self.condition:
                for queue in (self.online, self.background):
                    while queue:
                        item = queue.popleft()
                        item["error"] = AIError("model_restarting", 503)
                        item["event"].set()
            acquired = self.io_lock.acquire(timeout=30)
            if not acquired:
                self.kill()
                self.io_lock.acquire()
            try:
                self.kill()
                self.spec = None
                if data["command"] == "unload":
                    self.state = "unloaded"
                else:
                    self.state, self.config = "loading", config
                    context = mp.get_context("spawn")
                    parent, child = context.Pipe()
                    self.pipe = parent
                    self.process = context.Process(target=model_child, args=(child, spec, self.config), daemon=True)
                    try:
                        self.process.start()
                    finally:
                        child.close()
                    deadline = time.monotonic()+840
                    while True:
                        if not parent.poll(max(0, deadline-time.monotonic())):
                            raise AIError("model_load_timeout", 504)
                        result = parent.recv()
                        if result.get("stage") == "warming":
                            self.state = "warming"
                            continue
                        break
                    if not result.get("ready"):
                        raise AIError(result.get("error", "model_load_failed"), result.get("status", 503))
                    self.spec, self.state = spec, "ready"
                    combination = {"fingerprint": spec["fingerprint"], **self.config}
                    if combination not in self.verified:
                        self.verified.append(combination)
                self.last_operation = data.get("operation_id")
            except Exception:
                self.state = "failed"
                self.kill()
                raise
            finally:
                self.io_lock.release()
            return self.status()

    def status(self):
        if self.state == "ready" and (not self.process or not self.process.is_alive()):
            self.state = "failed"
        return {"state": self.state, "fingerprint": self.spec["fingerprint"] if self.spec else None,
                "generation": self.generation, "config": self.config, "verified": list(self.verified),
                "devices": list(self.devices), "queued": len(self.online)+len(self.background), "resources": system_usage(self.config["device"])}

    def encode(self, data):
        if not isinstance(data,dict):raise AIError("invalid_input")
        if data.get("role") not in ("query", "document"):
            raise AIError("invalid_role")
        if ("image" in data) == ("text" in data):
            raise AIError("invalid_input")
        if "text" in data and (not isinstance(data["text"], str) or not data["text"].strip() or len(data["text"]) > 512):
            raise AIError("invalid_text")
        if "image" in data and (not isinstance(data["image"], str) or len(data["image"]) > 4*((MAX_BYTES+2)//3)):
            raise AIError("image_too_large", 413)
        timeout = data.get("timeout", 60)
        if not isinstance(timeout, (int,float)) or not 0 < timeout <= 60:
            raise AIError("invalid_timeout")
        item = {"data": data, "deadline": time.monotonic()+timeout, "event": threading.Event()}
        with self.condition:
            if self.state != "ready" or not self.spec or data.get("fingerprint") != self.spec["fingerprint"]:
                raise AIError("model_not_ready", 503)
            queue = self.background if data.get("background") else self.online
            if len(queue) >= (1 if data.get("background") else 8):
                raise AIError("runtime_busy", 429)
            queue.append(item)
            self.condition.notify()
        if not item["event"].wait(timeout):
            raise AIError("query_timeout", 504)
        if "error" in item:
            raise item["error"]
        return item["result"]

    def dispatch(self):
        while True:
            with self.condition:
                self.condition.wait_for(lambda: self.online or self.background)
                if self.background and (self.online_streak >= 8 or not self.online):
                    item = self.background.popleft()
                    self.online_streak = 0
                else:
                    item = self.online.popleft()
                    self.online_streak += 1
            try:
                with self.io_lock:
                    remaining = item["deadline"]-time.monotonic()
                    if remaining <= 0:
                        raise AIError("query_timeout", 504)
                    if self.state != "ready" or self.spec["fingerprint"] != item["data"]["fingerprint"]:
                        raise AIError("model_not_ready", 503)
                    item["result"] = self.rpc(item["data"], remaining)
            except AIError as error:
                item["error"] = error
            except Exception:
                item["error"] = AIError("runtime_error", 503)
            finally:
                item["event"].set()


def create_runtime_app(runtime=None):
    app = Flask("photoly-ai-runtime")
    app.config["MAX_CONTENT_LENGTH"] = 29*1024*1024
    runtime = runtime or Runtime()
    @app.before_request
    def authenticate():
        expected = os.getenv("AI_CONTROL_TOKEN" if request.path == "/control" else "AI_INFERENCE_TOKEN", "")
        if not expected or not hmac.compare_digest(request.headers.get("Authorization", ""), "Bearer "+expected):
            raise AIError("unauthorized", 401)
    @app.errorhandler(AIError)
    def ai_error(error):
        return jsonify(error=error.code), error.status
    @app.get("/status")
    def status():
        return jsonify(runtime.status())
    @app.post("/control")
    def control():
        return jsonify(runtime.change(request.get_json()))
    @app.post("/encode")
    def encode():
        return jsonify(runtime.encode(request.get_json()))
    return app


def serve():
    import signal
    runtime = Runtime()
    server = make_server(os.getenv("AI_RUNTIME_HOST", "0.0.0.0"), int(os.getenv("AI_RUNTIME_PORT", "8090")),
                         create_runtime_app(runtime), threaded=True)
    def stop(signum, frame):
        raise SystemExit(0)
    signal.signal(signal.SIGTERM, stop)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()
        runtime.kill()


if __name__ == "__main__":
    serve()
