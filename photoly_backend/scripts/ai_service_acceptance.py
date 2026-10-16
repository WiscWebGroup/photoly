"""Opt-in acceptance against disposable Compose only. No production URL option."""
import argparse,json,time,subprocess,tempfile,io
from PIL import Image, ImageOps
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
    parser=argparse.ArgumentParser()
    parser.add_argument("--verify",action="store_true")
    parser.add_argument("--device",choices=["cpu","cuda","mps"],default="cuda")
    parser.add_argument("--providers",nargs="+",choices=["siglip2","qwen3_vl"],default=["siglip2","qwen3_vl"])
    parser.add_argument("--video", action="store_true", help="Include two five-scene videos; requires FFmpeg in the test container.")
    args=parser.parse_args()
    count=6 if args.video else 4
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
        assert len(result["items"])==count and result["coverage"]["ready"]==count
        assert len(get("/admin/ai/profiles")["profiles"])==len(args.providers)
        print(json.dumps({"restart_persistence":True,"media":count,"profiles":len(args.providers)}))
        return
    call("POST","/user/signUp",json={"email":EMAIL,"password":PASSWORD,"userName":"AI acceptance"})
    user=login(EMAIL,PASSWORD)
    root=call("GET","/namespace/getRoot",user)["t"]
    call("PATCH",f"/admin/users/{root['userId']}/ai",admin,json={"enabled":True})
    cfg=get("/admin/ai/settings")
    call("PATCH","/admin/ai/settings",admin,json={"generation":cfg["generation"],"settings":{"device":args.device,"precision":"bfloat16" if args.device=="cuda" else "float32","enabled":True}})
    samples=Path("/checks/fixtures/ai")
    manifest=json.loads((samples/"manifest.json").read_text())
    for sample in manifest["samples"]:
        call("POST","/photo/insert",user,files={"file":(sample["file"],(samples/sample["file"]).read_bytes())},data={"photo":json.dumps({"nsId":root["nsId"]})})
    video_ids=[]
    image_query=(samples/"chelsea.png").read_bytes()
    if args.video:
        encoded=io.BytesIO()
        with Image.open(io.BytesIO(image_query)) as cat:
            ImageOps.pad(cat.convert("RGB"),(512,512),color="black").save(encoded,format="PNG")
        image_query=encoded.getvalue()
    if args.video:
        with tempfile.TemporaryDirectory(prefix="video-acceptance-") as folder:
            folder=Path(folder)
            for name, background in (("coffee-cat-video", "coffee.png"), ("rocket-cat-video", "rocket.jpg")):
                for i in range(5):
                    with Image.open(samples/("chelsea.png" if i==2 else background)) as image:
                        ImageOps.pad(image.convert("RGB"),(512,512),color="black").save(folder/f"frame{i:02d}.png")
                path=folder/(name+".mp4")
                subprocess.run(["ffmpeg","-v","error","-nostdin","-y","-framerate","1","-i",str(folder/"frame%02d.png"),
                    "-c:v","libx264","-threads","1","-r","10","-t","5","-pix_fmt","yuv420p",str(path)],check=True,timeout=30)
                call("POST","/photo/insert",user,files={"file":(path.name,path.read_bytes())},data={"photo":json.dumps({"nsId":root["nsId"]})})
            media=call("GET","/photo/getByNamespace",user,params={"nsId":root["nsId"]})["t"]
            video_ids=[p["photoId"] for p in media if p["format"]=="mp4"]
            assert len(video_ids)==2
    reports=[]
    for provider in args.providers:
        if provider=="qwen3_vl":operation("maintenance.enter")
        pid=call("POST","/admin/ai/profiles",admin,json={"provider":provider})["t"]["profileId"]
        wait(lambda:any(p["profileId"]==pid and p["indexStatus"]=="ready" for p in get("/admin/ai/profiles")["profiles"]))
        operation("model.load",pid)
        operation("scheduler.start")
        def covered():
            jobs=get("/admin/ai/jobs?kind=embedding&status=succeeded&limit=100")["items"]
            return sum(j["profileId"]==pid for j in jobs)>=count
        wait(covered,seconds=600 if args.video else 180)
        operation("activate",pid)
        start=time.perf_counter()
        text=call("POST","/ai/search/text",user,json={"text":"a cat"})["t"]
        assert len(text["items"])==count and text["coverage"]["ready"]==count
        image=call("POST","/ai/search/image",user,files={"file":("cat.png",image_query)})["t"]
        assert len(image["items"])==count
        photo=image["items"][0]["photo"]
        similar=call("POST",f"/ai/photos/{photo['photoId']}/similar",user,json={})["t"]
        assert len(similar["items"])==count-1
        smart=call("GET","/photo/searchSmart",user,params={"query":"chelsea"})["t"]
        assert smart and smart[0]["photoName"] == "chelsea", smart
        video_report={}
        if args.video:
            matched=[hit for hit in image["items"] if hit["photo"]["photoId"] in video_ids]
            assert len(matched)==2 and all(hit["matchedTimestampMs"]==2500 for hit in matched),matched
            for vid in video_ids:
                first=call("POST",f"/ai/photos/{vid}/similar/page",user,json={"pageSize":2})["t"]
                hits=list(first["items"]);page=first
                while page["hasMore"]:
                    page=call("POST",f"/ai/photos/{vid}/similar/page",user,json={"cursor":page["nextCursor"]})["t"]
                    hits+=page["items"]
                assert len(hits)==count-1 and len({h["photo"]["photoId"] for h in hits})==count-1
                assert vid not in [h["photo"]["photoId"] for h in hits]
                assert any(h["photo"]["format"]=="mp4" for h in hits)
            video_report={"video_count":2,"frames_per_video":5,"middle_frame_image_match":True,"video_queries_and_paging":True}
        reports.append({"provider":provider,"profileId":pid,"coverage":text["coverage"],**video_report,"queries_seconds":time.perf_counter()-start})
        print(json.dumps(reports[-1]),flush=True)
    Path("/results/real-service.json").write_text(json.dumps(reports,indent=2)+"\n")

if __name__=="__main__":main()
