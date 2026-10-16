from datetime import datetime, timezone
from pathlib import Path
import os
import shutil
import threading
import time

_cpu_lock=threading.Lock()
_cpu_previous=None


def cpu_sample(psutil):
    """Shared deltas work across short-lived HTTP threads; psutil.cpu_percent does not."""
    global _cpu_previous
    now=time.monotonic()
    values=psutil.cpu_times()
    total=sum(values)-getattr(values,"guest",0)-getattr(values,"guest_nice",0)
    idle=values.idle+getattr(values,"iowait",0)
    usage=capacity=None
    try:
        stat=dict(line.split() for line in Path("/sys/fs/cgroup/cpu.stat").read_text().splitlines())
        usage=int(stat["usage_usec"])/1_000_000
        quota,period=Path("/sys/fs/cgroup/cpu.max").read_text().split()
        capacity=len(os.sched_getaffinity(0))
        if quota!="max":capacity=min(capacity,int(quota)/int(period))
    except (OSError,ValueError,KeyError,AttributeError):pass
    with _cpu_lock:
        previous=_cpu_previous
        _cpu_previous=(now,total,idle,usage)
    host=container=None
    if previous:
        elapsed=now-previous[0];delta=total-previous[1]
        if delta>0:host=max(0,min(100,100*(1-(idle-previous[2])/delta)))
        if usage is not None and previous[3] is not None and capacity and elapsed>0:
            container=max(0,min(100,100*(usage-previous[3])/elapsed/capacity))
    return host,container,capacity


def system_usage(device="cpu", path="/tmp"):
    import psutil
    memory = psutil.virtual_memory()
    available, total = memory.available, memory.total
    missing = []
    try:
        limit = Path("/sys/fs/cgroup/memory.max").read_text().strip()
        if limit != "max":
            limit = int(limit)
            used = int(Path("/sys/fs/cgroup/memory.current").read_text())
            total, available = min(total, limit), min(available, max(0, limit-used))
    except (OSError, ValueError):
        missing.append("cgroup_memory")
    host_cpu,container_cpu,cpu_capacity=cpu_sample(psutil)
    cpu=max([x for x in (host_cpu,container_cpu) if x is not None],default=None)
    if cpu is None:missing.append("cpu_first_sample")
    snapshot = {"sampled_at": datetime.now(timezone.utc).isoformat(), "cpu_percent":cpu, "host_cpu_percent":host_cpu,"container_cpu_percent":container_cpu,"cpu_capacity":cpu_capacity,
        "memory_available": available, "memory_total": total, "disk_free": shutil.disk_usage(path).free,
        "gpu_percent": None, "vram_available": None, "vram_total": None, "missing": missing}
    if device == "cuda":
        try:
            import pynvml
            pynvml.nvmlInit()
            handle = pynvml.nvmlDeviceGetHandleByIndex(0)
            mem = pynvml.nvmlDeviceGetMemoryInfo(handle)
            snapshot.update(vram_available=mem.free, vram_total=mem.total)
            try:
                snapshot["gpu_percent"] = pynvml.nvmlDeviceGetUtilizationRates(handle).gpu
            except pynvml.NVMLError:
                missing.append("gpu_utilization")
        except Exception:
            missing.append("gpu_memory")
    scores = [max(0, 1-(snapshot["cpu_percent"] if snapshot["cpu_percent"] is not None else 100)/100), min(1, available/max(total,1))]
    if snapshot["vram_available"] is not None:
        scores.append(snapshot["vram_available"]/snapshot["vram_total"])
    snapshot["free_score"] = round(100*min(scores), 2)
    return snapshot
