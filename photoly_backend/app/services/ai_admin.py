from datetime import timedelta
import hashlib
import json
from sqlalchemy import select, update, func
from sqlalchemy.dialects.postgresql import insert
from app.entities import User, Photo
from app.entities.ai import AIControlState, AIJob, EmbeddingProfile, PhotoEmbeddingState as State
from app.ai.state import settings, control, utcnow, validate_settings
from app.ai.registry import MODELS, profile_spec
from app.ai.types import AIError
from app.ai.devices import DEVICE_PRECISIONS, device_status
from app.ai.limits import validate_limit_pairs, LIMIT_DEFAULTS, LIMIT_BOUNDS
from app.scheduler.background_limits import status as background_limit_status
from app.scheduler.job_repository import usage

COMMANDS = {"model.load", "model.unload", "model.restart", "scheduler.start", "scheduler.pause", "scheduler.resume", "scheduler.stop",
            "maintenance.enter", "maintenance.exit", "backfill", "retry_failed", "rebuild", "activate", "purge"}


def coverage(session, profile_id, user_id=None):
    rows = select(Photo.photo_id, State.status).outerjoin(State, (State.photo_id == Photo.photo_id) &
        (State.profile_id == profile_id) & (State.content_version == Photo.content_version)).join(User).where(User.ai_search_enabled.is_(True))
    if user_id is not None:
        rows = rows.where(Photo.user_id == user_id)
    sub = rows.subquery()
    counts = dict(session.execute(select(sub.c.status, func.count()).group_by(sub.c.status)).all())
    total = sum(counts.values())
    unsupported = counts.get("unsupported",0)
    supported = total-unsupported
    return {"eligible":total, "supported":supported, "ready":counts.get("ready",0), "pending":counts.get("pending",0)+counts.get(None,0),
            "failed":counts.get("failed",0), "unsupported":unsupported, "ratio":counts.get("ready",0)/supported if supported else 1.0}


def profile_dto(profile):
    return {"profileId": profile.profile_id, "provider": profile.provider, "dimension":profile.dimension,
            "fingerprint":profile.fingerprint, "spec":profile.spec, "indexStatus":profile.index_status, "createdAt":profile.created_at.isoformat()}


def job_dto(job):
    return {"operationId":job.job_id, "kind":job.kind, "photoId":job.photo_id, "profileId":job.profile_id,
        "command":job.payload.get("command") if job.kind=="operation" else None,
        "generation":job.payload.get("generation") if job.kind=="operation" else None,
        "stage":job.result.get("step", job.status), "status":job.status, "attempts":job.attempts, "result":job.result, "createdAt":job.created_at.isoformat(),
        "finishedAt":job.finished_at.isoformat() if job.finished_at else None}


class AIAdmin:
    def __init__(self, session):
        self.session = session

    def get_settings(self):
        row = self.session.get(AIControlState, "global")
        return {"generation":row.generation if row else 0, **settings(self.session)}

    def patch_settings(self, generation, changes):
        changes = validate_settings(changes)
        row = control(self.session, lock=True)
        if type(generation) is not int or generation != row.generation:
            raise AIError("generation_conflict",409)
        current = settings(self.session)
        proposed = {**current, **changes}
        validate_limit_pairs(proposed)
        if current["runtime"] == "loaded" and current["cpu_threads"] != proposed["cpu_threads"]:
            raise AIError("unload_before_thread_change",409)
        if DEVICE_PRECISIONS.get(proposed["device"]) != proposed["precision"]:
            raise AIError("unsupported_device_precision")
        if proposed["runtime"] == "loaded" and any(k in changes for k in ("device","precision")):
            raise AIError("unload_before_device_change",409)
        row.desired, row.generation = proposed, row.generation+1
        return {"generation":row.generation, **proposed}

    def capabilities(self):
        row = self.session.get(AIControlState, "runtime")
        fresh = bool(row and row.heartbeat_at and utcnow()-row.heartbeat_at < timedelta(seconds=60))
        observed = row.observed if fresh else {}
        fields = {}
        for key, value in LIMIT_DEFAULTS.items():
            pair = key.replace("pause", "resume") if "pause" in key else key.replace("resume", "pause") if "resume" in key else None
            fields[key] = {"default":value,"minimum":LIMIT_BOUNDS[key][0],"maximum":LIMIT_BOUNDS[key][1],
                "unit":"MiB" if key.endswith("_mib") else "%" if key.endswith("_percent") else "seconds" if key.endswith("_seconds") else "count",
                "apply":"model_reload" if key=="cpu_threads" else "next_cycle", "pairedWith":pair,
                "relation":("less" if "resume" in key else "greater") if key.endswith("_percent") else ("greater" if "resume" in key else "less") if pair else None}
        fields["quota_bytes"] = {"default":10*1024**3,"minimum":1024**2,"maximum":100*1024**4,"unit":"bytes","apply":"next_cycle","pairedWith":None,"relation":None}
        fields["similarity_threshold"] = {"default":None,"minimum":-1,"maximum":1,"unit":"cosine similarity","apply":"next_query","pairedWith":None,"relation":None}
        devices = observed.get("devices", device_status(reason="runtime_offline" if not fresh else "unknown"))
        return {"fields":fields,"devices":devices,"verified":observed.get("verified",[]),"maxConcurrency":1,"batchSize":1}

    def profiles(self):
        cfg = settings(self.session)
        row = self.session.get(AIControlState,"runtime")
        fresh = bool(row and row.heartbeat_at and utcnow()-row.heartbeat_at<timedelta(seconds=60))
        observed = row.observed if fresh else {}
        profiles = []
        for p in self.session.scalars(select(EmbeddingProfile).order_by(EmbeddingProfile.profile_id)):
            counts = coverage(self.session,p.profile_id)
            activation = []
            if p.index_status!="ready":activation.append("index_not_ready")
            if observed.get("state")!="ready" or observed.get("fingerprint")!=p.fingerprint:activation.append("model_not_ready")
            if counts["pending"]:activation.append("pending_images")
            if counts["failed"]:activation.append("failed_images")
            purge = []
            if p.profile_id in (cfg["search_profile_id"],cfg["building_profile_id"],cfg["loaded_profile_id"]):purge.append("profile_in_use")
            if self.session.scalar(select(AIJob.job_id).where(AIJob.profile_id==p.profile_id,AIJob.kind=="search",AIJob.status=="running").limit(1)):purge.append("requests_in_flight")
            profiles.append({**profile_dto(p),"coverage":counts,"activationBlockers":activation,"purgeBlockers":purge})
        return {"registry":[profile_spec(key) for key in MODELS], "profiles":profiles}

    def create_profile(self, provider):
        spec = profile_spec(provider)
        existing = self.session.scalar(select(EmbeddingProfile).where(EmbeddingProfile.fingerprint == spec["fingerprint"]))
        if existing:
            return profile_dto(existing)
        self.session.execute(insert(EmbeddingProfile).values(provider=provider, dimension=spec["dimension"], fingerprint=spec["fingerprint"], spec=spec).on_conflict_do_nothing())
        profile = self.session.scalar(select(EmbeddingProfile).where(EmbeddingProfile.fingerprint == spec["fingerprint"]))
        self.session.execute(insert(AIJob).values(kind="index", profile_id=profile.profile_id, dedup_key=f"index:{profile.profile_id}").on_conflict_do_nothing())
        return profile_dto(profile)

    def set_user(self, user_id, enabled):
        if type(enabled) is not bool:
            raise AIError("invalid_enabled")
        user = self.session.scalar(select(User).where(User.user_id == user_id).with_for_update())
        if not user:
            raise AIError("user_not_found",404)
        user.ai_search_enabled = enabled
        if not enabled:
            self.session.execute(update(AIJob).where(AIJob.photo_id.in_(select(Photo.photo_id).where(Photo.user_id == user_id)),
                AIJob.status.in_(["queued","running"])).values(status="cancelled",reserved_bytes=0,finished_at=utcnow()))
        # Reconciliation pages eligible users, so enabling never enqueues an unbounded library here.
        return {"userId":user_id,"enabled":enabled}

    def user_states(self, ids):
        if len(ids)>100 or any(type(x) is not int or x<=0 for x in ids):
            raise AIError("invalid_user_ids")
        return [{"userId":u.user_id,"enabled":u.ai_search_enabled} for u in self.session.scalars(select(User).where(User.user_id.in_(ids)))]

    def operation(self, user_id, data, key):
        if not isinstance(key,str) or not 1<=len(key)<=80:
            raise AIError("idempotency_key_required")
        command, pid, generation = data.get("command"), data.get("profileId"), data.get("generation")
        if not isinstance(command,str) or command not in COMMANDS or type(generation) is not int or (pid is not None and (type(pid) is not int or pid<=0)):
            raise AIError("invalid_operation")
        request_hash = hashlib.sha256(json.dumps(data,sort_keys=True).encode()).hexdigest()
        row = control(self.session,lock=True)
        dedup = f"operation:{user_id}:{key}"
        old = self.session.scalar(select(AIJob).where(AIJob.kind=="operation", AIJob.dedup_key==dedup))
        if old:
            if old.payload["request_hash"] != request_hash:
                raise AIError("idempotency_conflict",409)
            return job_dto(old)
        if generation != row.generation:
            raise AIError("generation_conflict",409)
        cfg = settings(self.session)
        if pid is not None and not self.session.get(EmbeddingProfile,pid):
            raise AIError("profile_not_found",404)
        if command in ("model.load","backfill","retry_failed","rebuild","activate","purge") and pid is None:
            raise AIError("profile_required")
        if command=="model.restart":
            pid=pid or cfg["loaded_profile_id"]
        if command in ("model.load","model.restart","backfill","retry_failed","rebuild"):
            if cfg["search_profile_id"] not in (None,pid) and not cfg["maintenance"]:
                raise AIError("maintenance_required",409)
        if command=="rebuild" and not cfg["maintenance"]:
            raise AIError("maintenance_required",409)
        if command.startswith("scheduler."):
            cfg["scheduler"] = {"start":"running","resume":"running","pause":"paused","stop":"stopped"}[command.split(".")[1]]
        elif command=="maintenance.enter":
            cfg["maintenance"] = True
        elif command=="maintenance.exit":
            if cfg["building_profile_id"] not in (None,cfg["search_profile_id"]):
                raise AIError("building_profile_not_activated",409)
            cfg["maintenance"] = False
        elif command in ("model.load","model.restart"):
            pid = pid or cfg["loaded_profile_id"]
            if not pid:
                raise AIError("profile_required")
            cfg.update(runtime="loaded",loaded_profile_id=pid)
            if cfg["search_profile_id"] != pid:
                cfg["building_profile_id"] = pid
        elif command=="model.unload":
            cfg.update(runtime="unloaded",loaded_profile_id=None)
        elif command in ("backfill","retry_failed","rebuild"):
            cfg["building_profile_id"] = pid
        if command=="purge" and pid in (cfg["search_profile_id"],cfg["building_profile_id"],cfg["loaded_profile_id"]):
            raise AIError("profile_in_use",409)
        row.generation += 1
        row.desired = cfg
        job = AIJob(kind="operation", profile_id=pid, dedup_key=dedup, payload={"command":command,"generation":row.generation,
                    "actor":user_id,"request_hash":request_hash,"cursor":0})
        self.session.add(job)
        self.session.flush()
        return job_dto(job)

    def status(self):
        from app.services.smart import SmartService
        now = utcnow()
        services = {}
        for name in ("scheduler","runtime","maintenance"):
            row = self.session.get(AIControlState,name)
            fresh = bool(row and row.heartbeat_at and now-row.heartbeat_at<timedelta(seconds=60))
            services[name] = {"online":fresh and row.observed.get("state") not in ("offline","unknown"),"heartbeatAt":row.heartbeat_at.isoformat() if row and row.heartbeat_at else None,
                              "observed":row.observed if fresh else {"state":"offline"}}
        cfg = self.get_settings()
        return {"settings":cfg,"services":services,"quota":usage(self.session),"backgroundLimits":background_limit_status(self.session,cfg),
                "jobs":dict(self.session.execute(select(AIJob.status,func.count()).group_by(AIJob.status)).all()),
                "coverage":coverage(self.session,cfg["search_profile_id"]),
                "buildingCoverage":coverage(self.session,cfg["building_profile_id"]) if cfg["building_profile_id"] else None,
                "smartTag":SmartService(self.session).status()}
