import base64
import json
import os
import urllib.request
import urllib.error
from .types import AIError


class RuntimeClient:
    def __init__(self, url=None, inference_token=None, control_token=None):
        self.url = (url or os.getenv("AI_RUNTIME_URL", "http://ai-runtime:8090")).rstrip("/")
        self.inference_token = inference_token or os.getenv("AI_INFERENCE_TOKEN", "")
        self.control_token = control_token or os.getenv("AI_CONTROL_TOKEN", "")

    def call(self, path, data=None, *, control=False, timeout=65):
        token = self.control_token if control else self.inference_token
        if not token:
            raise AIError("runtime_unconfigured", 503)
        request = urllib.request.Request(self.url+path, data=None if data is None else json.dumps(data).encode(),
            headers={"Authorization": "Bearer "+token, "Content-Type": "application/json"})
        try:
            with urllib.request.urlopen(request, timeout=timeout) as response:
                return json.load(response)
        except urllib.error.HTTPError as error:
            try:
                code = json.loads(error.read()).get("error", "runtime_error")
            except (ValueError, AttributeError):
                code = "runtime_error"
            raise AIError(code, error.code) from error
        except (OSError, ValueError) as error:
            raise AIError("runtime_offline", 503) from error

    def status(self):
        return self.call("/status", timeout=3)

    def control(self, command, profile, config, generation, operation_id):
        return self.call("/control", {"command": command, "profile": profile, "config": config,
            "generation": generation, "operation_id": operation_id}, control=True, timeout=900)

    def submit(self, command, profile, config, generation, operation_id):
        return self.call("/control", {"command": command, "profile": profile, "config": config,
            "generation": generation, "operation_id": operation_id, "asynchronous": True}, control=True, timeout=10)

    def encode(self, profile, *, text=None, image=None, role="query", background=False, timeout=60):
        value = {"fingerprint": profile["fingerprint"], "role": role, "background": background, "timeout": min(60,timeout)}
        if image is not None:
            value["image"] = base64.b64encode(image).decode("ascii")
        else:
            value["text"] = text
        return self.call("/encode", value, timeout=timeout+3)
