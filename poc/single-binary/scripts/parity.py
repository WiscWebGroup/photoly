"""Compare browser-computed vectors with a CPU reference.

The reference follows Photoly's server path: Pillow decode, EXIF transpose, white behind
transparency, RGB (app/ai/preprocessing.py), then SigLIP's processor steps (bilinear resize
to 224 x 224, rescale, normalize to [-1, 1]) and the same ONNX files on ONNX Runtime CPU.

usage:
  python -I scripts/parity.py --model-dir DATA/models/<revision> --fixtures DIR \
      --database-url postgres://... [--browser-text e2e/text-vectors.json]

Needs: onnxruntime, pillow, numpy, tokenizers, and psql on PATH.
"""
import argparse
import json
import subprocess
from pathlib import Path

import numpy as np
import onnxruntime as ort
from PIL import Image, ImageOps
from tokenizers import Tokenizer


def unit(v):
    v = np.asarray(v, dtype=np.float32)
    return v / np.linalg.norm(v)


def prepare(path):
    with Image.open(path) as source:
        image = ImageOps.exif_transpose(source).convert("RGBA")
    background = Image.new("RGBA", image.size, "white")
    background.alpha_composite(image)
    rgb = background.convert("RGB").resize((224, 224), Image.Resampling.BILINEAR)
    pixels = (np.asarray(rgb, dtype=np.float32) / 255.0 - 0.5) / 0.5
    return pixels.transpose(2, 0, 1)[None]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--model-dir", required=True, type=Path)
    parser.add_argument("--fixtures", required=True, type=Path)
    parser.add_argument("--database-url", required=True)
    parser.add_argument("--dtype", default="fp32", choices=["fp32", "fp16"])
    parser.add_argument("--browser-text", type=Path)
    args = parser.parse_args()
    suffix = "" if args.dtype == "fp32" else "_fp16"

    manifest = json.loads((args.fixtures / "manifest.json").read_text())
    vision = ort.InferenceSession(str(args.model_dir / f"onnx/vision_model{suffix}.onnx"), providers=["CPUExecutionProvider"])
    text = ort.InferenceSession(str(args.model_dir / f"onnx/text_model{suffix}.onnx"), providers=["CPUExecutionProvider"])
    tokenizer = Tokenizer.from_file(str(args.model_dir / "tokenizer.json"))
    tokenizer.enable_padding(length=64, pad_id=0)

    rows = subprocess.run(["psql", args.database_url, "-At", "-F", "\t", "-c",
                           "SELECT p.name, e.embedding::text FROM photo p JOIN photo_embedding e ON e.photo_id = p.id"],
                          check=True, capture_output=True, text=True).stdout.splitlines()
    browser = {name: unit(json.loads(vector)) for name, vector in (row.split("\t") for row in rows)}

    print("image vectors: browser vs CPU reference (cosine)")
    reference = {}
    for sample in manifest["samples"]:
        out = vision.run(["pooler_output"], {"pixel_values": prepare(args.fixtures / sample["file"]).astype(np.float16 if suffix else np.float32)})[0]
        reference[sample["file"]] = unit(out[0])
        if sample["file"] in browser:
            print(f"  {sample['file']:15} {float(browser[sample['file']] @ reference[sample['file']]):.6f}")

    browser_text = json.loads(args.browser_text.read_text()) if args.browser_text else {}
    print("text queries: reference top hit" + (" | browser vs reference (cosine)" if browser_text else ""))
    names = list(reference)
    matrix = np.stack([reference[n] for n in names])
    correct = total = 0
    for sample in manifest["samples"]:
        for query in sample["queries"]:
            ids = np.asarray([tokenizer.encode(query.lower()).ids], dtype=np.int64)
            vector = unit(text.run(["pooler_output"], {"input_ids": ids})[0][0])
            top = names[int(np.argmax(matrix @ vector))]
            correct += top == sample["file"]
            total += 1
            line = f"  {query:32} -> {top:15}"
            if query in browser_text:
                line += f" | {float(unit(browser_text[query]) @ vector):.6f}"
            print(line)
    print(f"reference recall@1: {correct}/{total}")


if __name__ == "__main__":
    main()
