"""Opt-in acceptance against disposable Compose only. No production URL option."""
import argparse,json,time
from pathlib import Path
from uuid import uuid4
import requests

BASE="http://backend:8084"
EMAIL="real-ai-acceptance@test.local"
PASSWORD="disposable-ai-test"

def call(method,path,headers=None,**kwargs):
    response=requests.request(method,BASE+path,headers=headers,timeout=65,**kwargs)
    assert response.status_code in (200,202),(path,response.status_code,response.text)
    data=response.json()
    assert data.get("msgCode") in (200,202),(path,data)
    return data

def login(email,password):
    return {"HRD-Token":call("POST","/user/signIn",json={"email":email,"password":password})["token"]}

def wait(function,seconds=180):
    deadline=time.monotonic()+seconds
    while time.monotonic()<deadline:
        value=function()
        if value:return value
        time.sleep(1)
    raise AssertionError("AI acceptance state timeout")

def main():
    parser=argparse.ArgumentParser();parser.add_argument("--verify",action="store_true");args=parser.parse_args()
    admin=login("admin@test.local","test-password-only")
    def get(path):return call("GET",path,admin)["t"]
    def operation(command,pid=None):
        data={"command":command,"generation":get("/admin/ai/settings")["generation"]}
        if pid:data["profileId"]=pid
        accepted=call("POST","/admin/ai/operations",{**admin,"Idempotency-Key":uuid4().hex},json=data)["t"]
        def done():
            job=get("/admin/ai/operations/"+str(accepted["operationId"]))
            assert job["status"] not in ("failed","cancelled"),job
            return job["status"]=="succeeded"
        wait(done)
    if args.verify:
        user=login(EMAIL,PASSWORD)
        wait(lambda:call("GET","/ai/capabilities",user)["t"]["ready"])
        result=call("POST","/ai/search/text",user,json={"text":"a cat"})["t"]
        assert len(result["items"])==4 and result["coverage"]["ready"]==4
        assert len(get("/admin/ai/profiles")["profiles"])==2
        print(json.dumps({"restart_persistence":True,"photos":4,"profiles":2}))
        return
    call("POST","/user/signUp",json={"email":EMAIL,"password":PASSWORD,"userName":"AI acceptance"})
    user=login(EMAIL,PASSWORD)
    root=call("GET","/namespace/getRoot",user)["t"]
    call("PATCH",f"/admin/users/{root['userId']}/ai",admin,json={"enabled":True})
    cfg=get("/admin/ai/settings")
    call("PATCH","/admin/ai/settings",admin,json={"generation":cfg["generation"],"settings":{"device":"cuda","precision":"bfloat16","enabled":True}})
    samples=Path("/checks/fixtures/ai")
    manifest=json.loads((samples/"manifest.json").read_text())
    for sample in manifest["samples"]:
        call("POST","/photo/insert",user,files={"file":(sample["file"],(samples/sample["file"]).read_bytes())},data={"photo":json.dumps({"nsId":root["nsId"]})})
    reports=[]
    for provider in ("siglip2","qwen3_vl"):
        if provider=="qwen3_vl":operation("maintenance.enter")
        pid=call("POST","/admin/ai/profiles",admin,json={"provider":provider})["t"]["profileId"]
        wait(lambda:any(p["profileId"]==pid and p["indexStatus"]=="ready" for p in get("/admin/ai/profiles")["profiles"]))
        operation("model.load",pid)
        operation("scheduler.start")
        def covered():
            jobs=get("/admin/ai/jobs?kind=embedding&status=succeeded&limit=100")["items"]
            return sum(j["profileId"]==pid for j in jobs)>=4
        wait(covered)
        operation("activate",pid)
        start=time.perf_counter()
        text=call("POST","/ai/search/text",user,json={"text":"һֻè"})["t"]
        assert len(text["items"])==4 and text["coverage"]["ready"]==4
        image=call("POST","/ai/search/image",user,files={"file":("cat.png",(samples/"chelsea.png").read_bytes())})["t"]
        assert len(image["items"])==4
        photo=image["items"][0]["photo"]
        similar=call("POST",f"/ai/photos/{photo['photoId']}/similar",user,json={})["t"]
        assert len(similar["items"])==3
        reports.append({"provider":provider,"profileId":pid,"coverage":text["coverage"],"three_queries_seconds":time.perf_counter()-start})
        print(json.dumps(reports[-1]),flush=True)
    Path("/results/real-service.json").write_text(json.dumps(reports,indent=2)+"\n")

if __name__=="__main__":main()
