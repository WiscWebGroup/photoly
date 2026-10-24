"""Write testdata/reference.json: what Photoly's Python path produces for the AI fixtures.

The Go tests compare against it: token ids (Hugging Face `tokenizers`), the 224 x 224 RGB
bytes after Pillow's resize (as SHA-256), and SigLIP2 vectors from ONNX Runtime CPU.

usage:
  uv run --no-project --with onnxruntime --with tokenizers --with pillow --with numpy \
      python -I scripts/make_testdata.py --model-dir DIR/<revision> --fixtures DIR
"""
import argparse
import hashlib
import json
from pathlib import Path

import numpy as np
import onnxruntime as ort
from PIL import Image, ImageOps
from tokenizers import Tokenizer

# Edge cases for the Go tokenizer, on top of the fixture queries.
EXTRA_TEXT = [
    "hello  world", " leading and trailing ", "tab\there", "emoji 🐶 dog", "café crème",
    "<eos> and <pad> typed", "\n\n\n", "a" * 40, "ＦＵＬＬ width", "mixed 中文 and english",
]


def resized(path):
    with Image.open(path) as source:
        image = ImageOps.exif_transpose(source).convert("RGBA")
    background = Image.new("RGBA", image.size, "white")
    background.alpha_composite(image)
    return background.convert("RGB").resize((224, 224), Image.Resampling.BILINEAR)


def unit(v):
    v = np.asarray(v, dtype=np.float64)
    return [round(float(x), 6) for x in v / np.linalg.norm(v)]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--model-dir", required=True, type=Path)
    parser.add_argument("--fixtures", required=True, type=Path)
    parser.add_argument("--out", type=Path, default=Path(__file__).parent.parent / "testdata/reference.json")
    args = parser.parse_args()

    manifest = json.loads((args.fixtures / "manifest.json").read_text())
    tokenizer = Tokenizer.from_file(str(args.model_dir / "tokenizer.json"))
    tokenizer.no_padding()  # tokenizer.json pads to 64; keep the raw ids here and pad below
    vision = ort.InferenceSession(str(args.model_dir / "onnx/vision_model.onnx"), providers=["CPUExecutionProvider"])
    text = ort.InferenceSession(str(args.model_dir / "onnx/text_model.onnx"), providers=["CPUExecutionProvider"])

    images, queries = {}, {}
    for sample in manifest["samples"]:
        rgb = resized(args.fixtures / sample["file"])
        pixels = ((np.asarray(rgb, dtype=np.float32) / 255.0 - 0.5) / 0.5).transpose(2, 0, 1)[None]
        images[sample["file"]] = {
            "rgbSha256": hashlib.sha256(rgb.tobytes()).hexdigest(),
            "vector": unit(vision.run(["pooler_output"], {"pixel_values": pixels})[0][0]),
        }
        for query in sample["queries"]:
            ids = tokenizer.encode(query.lower()).ids
            padded = np.asarray([ids + [0] * (64 - len(ids))], dtype=np.int64)
            queries[query] = {"expected": sample["file"], "vector": unit(text.run(["pooler_output"], {"input_ids": padded})[0][0])}

    tokens = {t: tokenizer.encode(t).ids for t in EXTRA_TEXT + [q.lower() for q in queries]}
    args.out.parent.mkdir(exist_ok=True)
    args.out.write_text(json.dumps({"images": images, "queries": queries, "tokens": tokens}, ensure_ascii=False))
    print(f"wrote {args.out}")


if __name__ == "__main__":
    main()
