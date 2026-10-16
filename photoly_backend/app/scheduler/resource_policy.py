from dataclasses import dataclass
from datetime import datetime, timezone
from app.ai.limits import LIMIT_DEFAULTS


@dataclass(frozen=True)
class AdmissionDecision:
    allowed: bool
    batch_size: int
    retry_after: int
    reasons: tuple


class ResourcePolicy:
    def __init__(self):
        self.blocked = False
        self.cpu_average = None

    def decide(self, snapshot, device="cpu", limits=None):
        limits = {**LIMIT_DEFAULTS, **(limits or {})}
        mode = "resume" if self.blocked else "pause"
        reasons = []
        try:
            age = (datetime.now(timezone.utc)-datetime.fromisoformat(snapshot["sampled_at"])).total_seconds()
            if age > 15:
                reasons.append("stale_metrics")
        except (KeyError, ValueError):
            reasons.append("missing_metrics")
        cpu = snapshot.get("cpu_percent")
        self.cpu_average = cpu if self.cpu_average is None else (0.7*self.cpu_average + 0.3*cpu) if cpu is not None else None
        if self.cpu_average is None or self.cpu_average > limits[f"cpu_{mode}_percent"]:
            reasons.append("cpu_busy_or_unknown")
        if (snapshot.get("memory_available") or 0) < limits[f"memory_{mode}_free_mib"]*1024**2:
            reasons.append("memory_pressure")
        if (snapshot.get("disk_free") or 0) < limits[f"disk_{mode}_free_mib"]*1024**2:
            reasons.append("disk_pressure")
        if device == "cuda":
            if (snapshot.get("vram_available") or 0) < limits[f"vram_{mode}_free_mib"]*1024**2:
                reasons.append("vram_pressure_or_unknown")
            gpu = snapshot.get("gpu_percent")
            if gpu is None or gpu > limits[f"gpu_{mode}_percent"]:
                reasons.append("gpu_busy_or_unknown")
        self.blocked = bool(reasons)
        return AdmissionDecision(not reasons, 1, 10 if reasons else 0, tuple(reasons))
