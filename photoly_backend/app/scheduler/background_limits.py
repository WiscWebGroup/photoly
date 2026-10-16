"""Durable rolling-window admission; caller holds the global control row lock."""
from datetime import datetime, timedelta
from app.ai.state import control, utcnow
from app.entities.ai import AIControlState

NAME = "background_throttle"


def window(data, config, now):
    starts = sorted(datetime.fromisoformat(value) for value in data.get("starts", []))
    starts = [value for value in starts if value > now-timedelta(seconds=60)]
    last = datetime.fromisoformat(data["last_started_at"]) if data.get("last_started_at") else None
    waits = []
    if last:
        waits.append((last+timedelta(seconds=config["background_interval_seconds"]), "interval_wait"))
    maximum = config["background_max_per_minute"]
    if maximum and len(starts) >= maximum:
        waits.append((starts[-maximum]+timedelta(seconds=60), "rate_limited"))
    future = [(deadline, reason) for deadline, reason in waits if deadline > now]
    deadline, reason = max(future) if future else (None, None)
    return starts, {"lastStartedAt": last.isoformat() if last else None,
                   "startsLastMinute": len(starts), "nextAllowedAt": deadline.isoformat() if deadline else None,
                   "reason": reason, "maxConcurrency": 1, "batchSize": 1}


def status(session, config):
    row = session.get(AIControlState, NAME)
    return window(row.observed if row else {}, config, utcnow())[1]


def admit(session, config):
    row = control(session, NAME)
    now = utcnow()
    starts, current = window(row.observed, config, now)
    if current["reason"]:
        return False
    # Max configurable rate is 600; retain enough history even while the cap is disabled.
    row.observed = {"starts": [value.isoformat() for value in (starts+[now])[-600:]],
                    "last_started_at": now.isoformat()}
    return True
