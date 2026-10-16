"""Explicit opt-in real-model evaluation. Never run in default CI."""
import argparse
import hashlib
import json
from pathlib import Path
import resource
import sys
import time
import numpy as np
import torch
from app.ai.registry import adapter, profile_spec
from app.ai.types import RuntimeConfig
from app.ai.preprocessing import decode_image


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument("provider",choices=["siglip2","qwen3_vl"])
    parser.add_argument("device",choices=["cpu","cuda","mps"])
    parser.add_argument("--cpu-threads",type=int,default=4)
    parser.add_argument("--samples",default="/checks/fixtures/ai")
    parser.add_argument("--output",default="/results")
    args=parser.parse_args()
    spec=profile_spec(args.provider)
    config=RuntimeConfig(args.device,"bfloat16" if args.device=="cuda" else "float32",args.cpu_threads)
    samples=Path(args.samples)
    manifest=json.loads((samples/"manifest.json").read_text())
    for sample in manifest["samples"]:
        assert hashlib.sha256((samples/sample["file"]).read_bytes()).hexdigest()==sample["sha256"]
    images=[decode_image((samples/s["file"]).read_bytes()) for s in manifest["samples"]]
    model=adapter(args.provider)
    if args.device=="cuda":torch.cuda.reset_peak_memory_stats()
    start=time.perf_counter();model.load(spec,config);load=time.perf_counter()-start
    assert torch.get_num_threads()==args.cpu_threads
    vectors=[];image_times=[];text_times=[];queries=[];scores=[]
    model.encode_texts(["a photo"],role="query")
    for image in images:
        start=time.perf_counter();batch=model.encode_images([image],role="document");image_times.append(time.perf_counter()-start)
        vectors.append(batch.vectors[0])
    vectors=np.array(vectors)
    assert vectors.shape==(4,spec["dimension"]) and np.isfinite(vectors).all()
    assert np.allclose(np.linalg.norm(vectors,axis=1),1,atol=1e-5)
    for expected,sample in enumerate(manifest["samples"]):
        for language,query in zip(("en","zh"),sample["queries"]):
            start=time.perf_counter();encoded=model.encode_texts([query],role="query").vectors[0];text_times.append(time.perf_counter()-start)
            similarity=vectors@encoded;ranking=np.argsort(-similarity)
            rank=int(np.flatnonzero(ranking==expected)[0])+1
            queries.append({"query":query,"language":language,"expected":sample["file"],"rank":rank,"top1":manifest["samples"][int(ranking[0])]["file"]})
    similar=model.encode_images([images[0]],role="query").vectors[0]
    assert int(np.argmax(vectors@similar))==0
    report={"provider":args.provider,"revision":spec["revision"],"dimension":spec["dimension"],"device":args.device,
        "precision":config.precision,"cpu_threads":torch.get_num_threads(),"torch":torch.__version__,"cuda":torch.version.cuda,"load_seconds":load,
        "image_p50_seconds":float(np.percentile(image_times,50)),"image_p95_seconds":float(np.percentile(image_times,95)),
        "text_p50_seconds":float(np.percentile(text_times,50)),"text_p95_seconds":float(np.percentile(text_times,95)),
        "images_per_second":len(images)/sum(image_times),"peak_rss_bytes":resource.getrusage(resource.RUSAGE_SELF).ru_maxrss*(1 if sys.platform=="darwin" else 1024),
        "peak_vram_bytes":torch.cuda.max_memory_allocated() if args.device=="cuda" else None,
        "recall_at_1":sum(q["rank"]==1 for q in queries)/len(queries),"recall_at_3":sum(q["rank"]<=3 for q in queries)/len(queries),
        "queries":queries,"image_self_match":True,"manifest_sha256":__import__("hashlib").sha256((samples/"manifest.json").read_bytes()).hexdigest()}
    if args.device == "mps":
        report["mps_allocated_bytes"] = torch.mps.current_allocated_memory()
        report["mps_driver_bytes"] = torch.mps.driver_allocated_memory()
    model.close()
    if args.device == "mps":
        report["mps_after_unload_bytes"] = torch.mps.current_allocated_memory()
        assert report["mps_after_unload_bytes"] == 0
    target=Path(args.output);target.mkdir(parents=True,exist_ok=True)
    (target/f"{args.provider}-{args.device}.json").write_text(json.dumps(report,ensure_ascii=False,indent=2)+"\n")
    print(json.dumps({k:v for k,v in report.items() if k!="queries"}),flush=True)


if __name__=="__main__":main()
