"""Helper for onnx_vs_pytorch.sh. `prepare ROOT W` writes W/request.json; `compare ROOT W` runs the
PyTorch SigLIP2 profile (same calls as photoly_backend/app/ai/text_image_embedding_siglip2_base.py,
fp32 CPU) on the same inputs, reads W/onnx.json from the Go worker and writes W/results.json + prints it."""
import json, sys
from pathlib import Path

QUERIES = [
    "a portrait of an astronaut", "宇航员肖像", "a cat", "a dog playing outside", "sunset over the ocean",
    "mountain landscape with snow", "city street at night", "people at a wedding", "a plate of food",
    "a cup of coffee", "a red car", "a rocket launch", "flowers in a garden", "a beach with palm trees",
    "a group of friends smiling", "a building with large windows", "a child playing", "screenshot of a document",
    "blue sky with clouds", "夜晚的城市", "一只猫", "a bicycle", "text on a sign", "a bird on a branch",
]


def images(root):
    fx = sorted((root / "photoly_backend/tests/fixtures/ai").glob("*.[pj][np]g"))
    from PIL import Image
    up = sorted(p for p in (root / "upload_data").rglob("*.jpg") if "thumbnail" not in p.name)
    up = [p for p in up if p.stat().st_size <= 20 * 1024 * 1024 and Image.open(p).width * Image.open(p).height <= 40_000_000]  # app limit (MAX_PIXELS)
    return [str(p) for p in fx + up]


def unit(v):
    import numpy as np
    v = np.asarray(v, dtype=np.float64)
    return v / np.linalg.norm(v)


def top(sim, k=10):
    import numpy as np
    return set(np.argsort(-sim)[:k].tolist())


def main():
    mode, root, w = sys.argv[1], Path(sys.argv[2]), Path(sys.argv[3])
    if mode == "prepare":
        (w / "request.json").write_text(json.dumps({"images": [p.replace(str(root), "/repo") for p in images(root)], "queries": QUERIES}))
        return
    import numpy as np, torch
    from PIL import Image
    from transformers import AutoModel, AutoProcessor
    import types
    for name, path in (("app", root / "photoly_backend/app"), ("app.ai", root / "photoly_backend/app/ai")):
        sys.modules[name] = types.ModuleType(name); sys.modules[name].__path__ = [str(path)]  # skip app/__init__ (Flask)
    sys.modules.setdefault("pillow_heif", types.SimpleNamespace(register_heif_opener=lambda **k: None))  # no HEIC in this set
    from app.ai.registry import profile_spec
    from app.ai.preprocessing import decode_image
    onnx = json.loads((w / "onnx.json").read_text())
    spec = profile_spec("siglip2")
    mid, rev = spec["model_id"], spec["revision"]
    proc = AutoProcessor.from_pretrained(mid, revision=rev, use_fast=False, local_files_only=True)
    model = AutoModel.from_pretrained(mid, revision=rev, torch_dtype=torch.float32, attn_implementation="sdpa", local_files_only=True).eval()
    imgs, ti, tq = {}, {}, {}
    with torch.inference_mode():
        for p in onnx["images"]:
            real = p.replace("/repo", str(root), 1)
            img = decode_image(Path(real).read_bytes())  # the app's own decode (EXIF, white bg, RGB)
            ti[p] = unit(model.get_image_features(**proc(images=[img], return_tensors="pt")).numpy()[0])
        for q in onnx["queries"]:
            inp = proc(text=[q.lower()], padding="max_length", max_length=64, truncation=False, return_tensors="pt")
            tq[q] = unit(model.get_text_features(**inp).numpy()[0])
    names = list(onnx["images"])
    ic = np.array([float(unit(onnx["images"][n]) @ ti[n]) for n in names])
    qc = {q: float(unit(onnx["queries"][q]) @ tq[q]) for q in onnx["queries"]}
    P = np.stack([ti[n] for n in names])                      # PyTorch image vectors (existing library)
    O = np.stack([unit(onnx["images"][n]) for n in names])    # ONNX image vectors
    ov_full, ov_mixed = [], []
    for q in onnx["queries"]:
        ref = top(P @ tq[q])
        ov_full.append(len(ref & top(O @ unit(onnx["queries"][q]))) / 10)        # all-ONNX (re-indexed)
        ov_mixed.append(len(ref & top(P @ unit(onnx["queries"][q]))) / 10)       # ONNX query vs old torch image vectors
    worst = [(names[i].replace("/repo/", ""), float(ic[i])) for i in np.argsort(ic)[:5]]
    res = {
        "n_images": len(names), "n_queries": len(QUERIES), "go_fingerprint": onnx["fingerprint"], "python_fingerprint": spec["fingerprint"],
        "python_spec": spec,
        "image": {"min": float(ic.min()), "mean": float(ic.mean()), "p1": float(np.percentile(ic, 1)), "max": float(ic.max()), "worst5": worst},
        "text": {"min": min(qc.values()), "mean": sum(qc.values()) / len(qc), "per_query": qc},
        "top10_overlap_all_onnx": {"mean": float(np.mean(ov_full)), "min": float(min(ov_full))},
        "top10_overlap_onnx_query_vs_torch_images": {"mean": float(np.mean(ov_mixed)), "min": float(min(ov_mixed))},
        "versions": {"torch": torch.__version__},
    }
    (w / "results.json").write_text(json.dumps(res, indent=1, ensure_ascii=False))
    print(json.dumps({k: v for k, v in res.items() if k not in ("python_spec",)}, indent=1, ensure_ascii=False))


main()
