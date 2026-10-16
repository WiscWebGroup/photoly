from datetime import timedelta
from app.extensions import db
from app.ai.state import control, utcnow
from app.entities.ai import AIJob
from app.services.ai_admin import AIAdmin
from tests.test_ai import ai, ready
from tests.openapi_assertions import assert_documented_response


def test_admin_capabilities_auth_limits_and_offline(world):
    path = "/admin/ai/capabilities"
    assert world.client.get(path).status_code == 401
    assert world.client.get(path, headers=world.headers).status_code == 403
    response = world.client.get(path, headers=world.admin)
    assert_documented_response("GET", path, response)
    data = response.json["t"]
    assert data["maxConcurrency"] == data["batchSize"] == 1
    assert data["fields"]["cpu_threads"]["apply"] == "model_reload"
    assert data["fields"]["cpu_resume_percent"]["relation"] == "less"
    assert data["fields"]["vram_resume_free_mib"]["relation"] == "greater"
    assert all(d["available"] is None for d in data["devices"])


def test_capabilities_freshness_and_profile_readiness(ai):
    with ai.sessions.begin() as session:
        row = control(session, "runtime")
        row.heartbeat_at = utcnow()
        row.observed = {**ai.runtime.status(), "devices":[{"device":"cuda","precision":"bfloat16","available":True,"reason":None}], "verified":[]}
    data = ai.client.get("/admin/ai/capabilities", headers=ai.admin).json["t"]
    assert data["devices"][0]["available"] is True
    assert data["verified"] == []  # Available hardware is not a real-model verification.
    result = ai.client.get("/admin/ai/profiles", headers=ai.admin)
    assert_documented_response("GET", "/admin/ai/profiles", result)
    profile = result.json["t"]["profiles"][0]
    assert "pending_images" in profile["activationBlockers"]
    assert "model_not_ready" not in profile["activationBlockers"]
    assert "profile_in_use" in profile["purgeBlockers"]
    with ai.sessions.begin() as session:
        control(session,"runtime").heartbeat_at = utcnow()-timedelta(seconds=61)
    assert ai.client.get("/admin/ai/capabilities", headers=ai.admin).json["t"]["devices"][0]["available"] is None
    assert "model_not_ready" in ai.client.get("/admin/ai/profiles", headers=ai.admin).json["t"]["profiles"][0]["activationBlockers"]


def test_profile_summary_is_safe_and_user_scoped(ai):
    response = ai.client.get("/ai/capabilities", headers=ai.headers)
    assert_documented_response("GET", "/ai/capabilities", response)
    assert response.json["t"]["profile"] == {"provider":"siglip2","dimension":768,"modelId":"google/siglip2-base-patch16-224"}
    assert "embedding" not in response.json["t"]


def test_recent_operations_cursor_and_safe_metadata(ai):
    with ai.sessions.begin() as session:
        for i in range(4):
            session.add(AIJob(kind="operation", status="succeeded", dedup_key=f"metadata:{i}", payload={"command":"scheduler.pause","generation":i,"secret":"must-not-leak"}, result={"step":"completed"}))
    path = "/admin/ai/jobs"
    first = ai.client.get(path+"?order=desc&kind=operation&limit=2", headers=ai.admin)
    assert_documented_response("GET", path, first)
    data = first.json["t"]
    assert data["nextAfter"] is None
    ids = [j["operationId"] for j in data["items"]]
    assert ids == sorted(ids, reverse=True)
    assert data["items"][0]["command"] == "scheduler.pause"
    assert data["items"][0]["stage"] == "completed"
    assert "must-not-leak" not in first.text
    second = ai.client.get(path+f"?order=desc&kind=operation&limit=2&before={data['nextBefore']}", headers=ai.admin).json["t"]
    assert all(j["operationId"] < min(ids) for j in second["items"])
    for query in ("order=bad", "before=bad", "order=desc&after=1", "before=1", "order=desc&before=-1"):
        assert ai.client.get(path+"?"+query, headers=ai.admin).status_code == 400


def test_cors_supports_browser_admin_mutations(world):
    response = world.client.options("/admin/ai/operations", headers={"Origin":"http://localhost:5173","Access-Control-Request-Method":"POST","Access-Control-Request-Headers":"HRD-Token,Idempotency-Key,Content-Type"})
    assert response.status_code == 200
    assert "Idempotency-Key" in response.headers["Access-Control-Allow-Headers"]
    assert "PATCH" in response.headers["Access-Control-Allow-Methods"]
