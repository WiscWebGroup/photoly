"""Separate processes and real HTTP; deterministic adapter, no weights in CI."""
import json
import time
from uuid import uuid4
import pytest
from tests.test_http import http, register

pytestmark=pytest.mark.http


def eventually(function,timeout=45):
    deadline=time.monotonic()+timeout
    while time.monotonic()<deadline:
        result=function()
        if result:return result
        time.sleep(.4)
    pytest.fail("AI service did not reach the expected state")


def test_ai_http_services_and_lifecycle(http,jpeg):
    login=http("POST","/user/signIn",json={"email":"admin@test.local","password":"test-password-only"}).json()
    admin={"HRD-Token":login["token"]}
    def get(path):return http("GET",path,headers=admin).json()["t"]
    def operation(command,pid=None):
        data={"command":command,"generation":get("/admin/ai/settings")["generation"]}
        if pid is not None:data["profileId"]=pid
        headers={**admin,"Idempotency-Key":uuid4().hex}
        accepted=http("POST","/admin/ai/operations",headers=headers,json=data)
        assert accepted.status_code==202,accepted.text
        oid=accepted.json()["t"]["operationId"]
        def done():
            result=get(f"/admin/ai/operations/{oid}")
            assert result["status"] not in ("failed","cancelled"),result
            return result if result["status"]=="succeeded" else None
        eventually(done)
        return oid
    cfg=get("/admin/ai/settings")
    assert cfg["enabled"] is False
    user,_=register(http)
    root=http("GET","/namespace/getRoot",headers=user).json()["t"]
    uid=root["userId"]
    assert http("POST","/ai/search/text",headers=user,json={"text":"a cat"}).status_code==403
    pid=http("POST","/admin/ai/profiles",headers=admin,json={"provider":"siglip2"}).json()["t"]["profileId"]
    eventually(lambda: next(p for p in get("/admin/ai/profiles")["profiles"] if p["profileId"]==pid)["indexStatus"]=="ready")
    cfg=get("/admin/ai/settings")
    changed=http("PATCH","/admin/ai/settings",headers=admin,json={"generation":cfg["generation"],"settings":{"cpu_threads":2,"background_interval_seconds":8,"background_max_per_minute":10}})
    assert changed.status_code==200
    operation("model.load",pid)
    eventually(lambda: get("/admin/ai/status")["services"]["runtime"]["observed"].get("config",{}).get("cpu_threads")==2)
    operation("activate",pid)
    cfg=get("/admin/ai/settings")
    assert http("PATCH","/admin/ai/settings",headers=admin,json={"generation":cfg["generation"],"settings":{"enabled":True}}).status_code==200
    assert http("PATCH",f"/admin/users/{uid}/ai",headers=admin,json={"enabled":True}).status_code==200
    operation("scheduler.start")
    for name in ("one.jpg","two.jpg"):
        assert http("POST","/photo/insert",headers=user,files={"file":(name,jpeg)},data={"photo":json.dumps({"nsId":root["nsId"]})}).json()["t"]==1
    eventually(lambda: http("GET","/ai/capabilities",headers=user).json()["t"]["coverage"]["ready"]>=1)
    assert http("GET","/ai/capabilities",headers=user).json()["t"]["coverage"]["ready"]==1
    assert get("/admin/ai/status")["backgroundLimits"]["reason"]=="interval_wait"
    cfg=get("/admin/ai/settings")
    assert http("PATCH","/admin/ai/settings",headers=admin,json={"generation":cfg["generation"],"settings":{"cpu_threads":3}}).status_code==409
    assert http("PATCH","/admin/ai/settings",headers=admin,json={"generation":cfg["generation"],"settings":{"background_interval_seconds":0}}).status_code==200
    eventually(lambda: http("GET","/ai/capabilities",headers=user).json()["t"]["coverage"]["ready"]==2)
    text=http("POST","/ai/search/text",headers=user,json={"text":"a cat"})
    assert text.status_code==200,text.text
    assert len(text.json()["t"]["items"])==2
    photo=text.json()["t"]["items"][0]["photo"]
    assert "contentVersion" not in photo and "embedding" not in photo
    assert len(http("POST","/ai/search/image",headers=user,files={"file":("query.jpg",jpeg)}).json()["t"]["items"])==2
    assert len(http("POST",f"/ai/photos/{photo['photoId']}/similar",headers=user,json={}).json()["t"]["items"])==1
    assert http("POST","/ai/search/image",headers=user,files={"file":("bad.jpg",b"bad")}).status_code==415
    operation("scheduler.pause")
    eventually(lambda: get("/admin/ai/status")["services"]["scheduler"]["observed"].get("state")=="paused")
    operation("model.restart")
    assert http("GET","/ai/capabilities",headers=user).json()["t"]["ready"]
    assert http("PATCH",f"/admin/users/{uid}/ai",headers=admin,json={"enabled":False}).status_code==200
    assert http("POST","/ai/search/text",headers=user,json={"text":"cat"}).status_code==403
    operation("scheduler.stop")
    operation("model.unload")
    cfg=get("/admin/ai/settings")
    assert http("PATCH","/admin/ai/settings",headers=admin,json={"generation":cfg["generation"],"settings":{"enabled":False}}).status_code==200
    assert http("GET","/healthz").status_code==200
    assert http("GET","/user/deleteUser",headers=user).json()["msgCode"]==200
