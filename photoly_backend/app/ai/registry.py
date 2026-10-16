import hashlib
import json
from .types import AIError

MODELS = {
    "siglip2": {"model_id": "google/siglip2-base-patch16-224", "revision": "75de2d55ec2d0b4efc50b3e9ad70dba96a7b2fa2", "dimension": 768,
                "query_instruction": "", "document_instruction": "", "max_tokens": 64, "max_pixels": 50176},
    "qwen3_vl": {"model_id": "Qwen/Qwen3-VL-Embedding-2B", "revision": "9f2f7e710d6d81056aa5c0a4f04764fec6bb7bda", "dimension": 2048,
                 "query_instruction": "Retrieve images relevant to the user's query.", "document_instruction": "Represent the user's input.",
                 "max_tokens": 8192, "max_pixels": 1843200},
}


def profile_spec(provider):
    if not isinstance(provider,str) or provider not in MODELS:
        raise AIError("unsupported_provider")
    spec = {"provider": provider, **MODELS[provider], "preprocessing": "exif-rgb-white-first-frame-v1", "metric": "cosine", "normalization": "l2-fp32"}
    spec["fingerprint"] = hashlib.sha256(json.dumps(spec, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
    return spec


def validate_spec(spec):
    if not isinstance(spec,dict):raise AIError("invalid_profile")
    expected = profile_spec(spec.get("provider"))
    if spec != expected:
        raise AIError("profile_mismatch", 409)
    return spec


def adapter(provider):
    if provider == "siglip2":
        from .text_image_embedding_siglip2_base import Siglip2Embedding
        return Siglip2Embedding()
    if provider == "qwen3_vl":
        from .text_image_embedding_qwen import QwenEmbedding
        return QwenEmbedding()
    raise AIError("unsupported_provider")
