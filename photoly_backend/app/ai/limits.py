"""Validated admin settings; resource thresholds govern background admission only."""
from .types import AIError

LIMIT_DEFAULTS = {
    "background_interval_seconds": 2,
    "background_max_per_minute": 0,
    "cpu_threads": 4,
    "cpu_pause_percent": 90,
    "cpu_resume_percent": 70,
    "gpu_pause_percent": 95,
    "gpu_resume_percent": 70,
    "memory_pause_free_mib": 1024,
    "memory_resume_free_mib": 2048,
    "vram_pause_free_mib": 1024,
    "vram_resume_free_mib": 2048,
    "disk_pause_free_mib": 2048,
    "disk_resume_free_mib": 3072,
}
LIMIT_BOUNDS = {
    "background_interval_seconds": (0, 3600),
    "background_max_per_minute": (0, 600),
    "cpu_threads": (1, 64),
    **{key: (1, 100) for key in LIMIT_DEFAULTS if key.endswith("_percent")},
    **{key: (1, 16 * 1024 * 1024) for key in LIMIT_DEFAULTS if key.endswith("_mib")},
}


def validate_limit_values(changes):
    for key, (minimum, maximum) in LIMIT_BOUNDS.items():
        if key in changes and (type(changes[key]) is not int or not minimum <= changes[key] <= maximum):
            raise AIError("invalid_resource_limits")


def validate_limit_pairs(config):
    for resource in ("cpu", "gpu"):
        if config[f"{resource}_resume_percent"] >= config[f"{resource}_pause_percent"]:
            raise AIError("invalid_resource_hysteresis")
    for resource in ("memory", "vram", "disk"):
        if config[f"{resource}_resume_free_mib"] <= config[f"{resource}_pause_free_mib"]:
            raise AIError("invalid_resource_hysteresis")


def runtime_config(config):
    return {key: config[key] for key in ("device", "precision", "cpu_threads")}
