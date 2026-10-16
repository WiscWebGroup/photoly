from copy import deepcopy
from datetime import datetime, timezone
from sqlalchemy import select
from sqlalchemy.dialects.postgresql import insert
from app.entities.ai import AIControlState
from .types import AIError
from .limits import LIMIT_DEFAULTS, validate_limit_values

DEFAULTS = {"enabled": False, "new_user_default": False, "maintenance": False, "similarity_threshold": None,
            "search_profile_id": None, "building_profile_id": None, "quota_bytes": 10 * 1024**3,
            "scheduler": "stopped", "runtime": "unloaded", "loaded_profile_id": None,
            "device": "cpu", "precision": "float32", **LIMIT_DEFAULTS}


def utcnow():
    return datetime.now(timezone.utc)


def control(session, name="global", lock=False):
    session.execute(insert(AIControlState).values(name=name, desired=deepcopy(DEFAULTS) if name == "global" else {}).on_conflict_do_nothing())
    query = select(AIControlState).where(AIControlState.name == name).execution_options(populate_existing=True)
    if lock:
        query = query.with_for_update()
    return session.scalar(query)


def settings(session):
    row = session.get(AIControlState, "global")
    return {**DEFAULTS, **(row.desired if row else {})}


def validate_settings(changes):
    if not isinstance(changes, dict) or set(changes) - ({"enabled", "new_user_default", "quota_bytes", "device", "precision", "similarity_threshold"} | set(LIMIT_DEFAULTS)):
        raise AIError("invalid_settings")
    for name in ("enabled", "new_user_default"):
        if name in changes and type(changes[name]) is not bool:
            raise AIError("invalid_settings")
    if "quota_bytes" in changes and (type(changes["quota_bytes"]) is not int or not 1024**2 <= changes["quota_bytes"] <= 100*1024**4):
        raise AIError("invalid_quota")
    if "device" in changes and changes["device"] not in ("cpu", "cuda"):
        raise AIError("unsupported_device")
    if "precision" in changes and changes["precision"] not in ("float32", "bfloat16"):
        raise AIError("unsupported_precision")
    threshold = changes.get("similarity_threshold")
    if threshold is not None and (type(threshold) not in (int, float) or not -1 <= threshold <= 1):
        raise AIError("invalid_similarity_threshold")
    validate_limit_values(changes)
    return changes
