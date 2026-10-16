"""Supported inference pairs, shared by configuration and capability reporting."""
DEVICE_PRECISIONS = {"cpu": "float32", "cuda": "bfloat16", "mps": "float32"}


def device_status(available=None, reason="unknown"):
    return [{"device": device, "precision": precision, "available": available, "reason": reason}
            for device, precision in DEVICE_PRECISIONS.items()]


def probe_hardware():
    # Called in a disposable subprocess; the HTTP controller never allocates GPU memory.
    import torch
    available = {"cpu": True,
                 "cuda": torch.cuda.is_available() and torch.cuda.is_bf16_supported(),
                 "mps": torch.backends.mps.is_built() and torch.backends.mps.is_available()}
    return [{"device": device, "precision": precision, "available": bool(available[device]),
             "reason": None if available[device] else "device_unavailable"}
            for device, precision in DEVICE_PRECISIONS.items()]
