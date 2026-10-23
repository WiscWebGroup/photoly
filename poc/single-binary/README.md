# Photoly single-binary POC (Go + WebGPU)

A proof of concept for the plan in [`docs/single-binary-backend.md`](../../docs/single-binary-backend.md):

- **One static Go binary** serves the API, the web UI, the browser AI runtime and the model
  files, and migrates its own database. Its only dependency is PostgreSQL with pgvector.
- **SigLIP2 runs in the browser** over WebGPU, with WebAssembly as the fallback. The browser
  embeds each photo before uploading it and embeds search text. The server runs no model:
  it validates, stores and searches the 768-number vectors.

It is not the Photoly API. There is no sign-in (every request acts as user 1), only images,
and no server-side fallback indexing yet. See [What's missing](#whats-missing).

## Build and run

Requirements: Go 1.24+, Node/npm (only to fetch the browser runtime), PostgreSQL with
pgvector 0.7+ (`halfvec`).

```sh
./scripts/vendor.sh                     # fetch transformers.js + ONNX Runtime Web into web/vendor (gzipped)
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o photoly .

export DATABASE_URL=postgres://user:pass@localhost:5432/photoly_poc
./photoly                               # = photoly serve: migrate, then listen on :8084
```

Open <http://localhost:8084>. WebGPU needs a secure context: `localhost` or HTTPS.

| Variable | Default | Meaning |
| --- | --- | --- |
| `DATABASE_URL` | required | PostgreSQL connection string |
| `PHOTOLY_ADDR` | `:8084` | Listen address |
| `PHOTOLY_DATA` | `./data` | Photos and the model cache |
| `PHOTOLY_MODEL_DTYPE` | `fp32` | `fp32` (matches the server reference) or `fp16` (half the download, needs a GPU with `shader-f16`) |

Other commands: `photoly migrate` applies migrations and exits; `photoly fetch-model`
downloads and verifies the model ahead of time (for offline installs).

## How it works

```
browser                                         photoly (one Go binary)            PostgreSQL
───────                                         ───────────────────────            ──────────
GET /  /app.js  /vendor/*.wasm  ───────────────▶ embedded files (gzipped)
GET /models/<rev>/…/vision_model.onnx  ────────▶ download from Hugging Face once,
     (cached in the browser's Cache API)         verify SHA-256, serve from disk
photo ─▶ preprocess.js ─▶ WebGPU ─▶ 768 floats
POST /api/photos {file, embedding, profile} ───▶ check profile + vector shape ───▶ photo, halfvec(768)
query ─▶ tokenizer ─▶ WebGPU ─▶ 768 floats
POST /api/search {embedding, profile} ─────────▶ check, cosine search (own photos) ▶ HNSW index
GET  /api/photos/{id}/similar ─────────────────▶ stored vector, no model needed
```

- **Model:** [`onnx-community/siglip2-base-patch16-224-ONNX`](https://huggingface.co/onnx-community/siglip2-base-patch16-224-ONNX),
  the ONNX export of `google/siglip2-base-patch16-224` at revision `75de2d55`, the same
  revision Photoly's PyTorch runtime pins. Separate vision (372 MB fp32 / 186 MB fp16) and
  text (1.13 GB / 565 MB) files. Every file is pinned by SHA-256 in `model.go`, and a
  download that doesn't match is discarded.
- **Profile fingerprint:** a hash of the model, revision, file hashes, dtype, preprocessing
  version and normalisation. Every vector is sent with it, and a mismatch is rejected, so
  vectors from different setups never mix.
- **Trust:** the server cannot prove a vector came from the model. It checks the profile,
  length 768, finite values and unit length, and searches only the caller's own photos, so a
  bad vector only affects its uploader. `photo_embedding.source` records `client` so
  vectors can later be re-checked server-side.
- **Image preparation** (`web/preprocess.js`) copies Photoly's server path exactly: decode
  without colour management, EXIF orientation, white behind transparency, RGB, then a port
  of Pillow's antialiased bilinear resize (22-bit fixed point). On the test fixtures the
  224 × 224 output is **bit-identical to Pillow's**.
- **Browser runtime:** transformers.js 4.3.1 with ONNX Runtime Web
  (`ort-wasm-simd-threaded.asyncify`), embedded in the binary as 6.8 MB of gzip and sent
  compressed. No CDN requests. Pages are cross-origin isolated (COOP/COEP), so WebAssembly
  can use threads.
- **Keeping the model:** transformers.js stores model files in the Cache API. Model URLs
  contain the revision, so a new model never reuses a stale cache entry. The page offers
  "Keep model on this device" (`navigator.storage.persist()`) and shows storage use.

## Measurements

Measured in a Linux cloud sandbox with no GPU, using Chromium's software WebGPU
(SwiftShader), so WebGPU timings are far slower than on real hardware.

| | WebGPU (SwiftShader, software) | WebAssembly (4 threads) |
| --- | --- | --- |
| End-to-end test | pass | pass |
| Fixture queries, top hit correct | 8/8 (4 English, 4 Chinese) | 8/8 |
| Image embedding, per photo | 51.6 s | 0.86 s |
| Text query embedding | 12–15 s | 0.26–0.39 s |
| First model load (download from the binary over localhost) | 70 s | 50 s |
| After closing and reopening the browser | 0 model requests, ready in 19 s | 0 model requests, ready in 16 s |

| | Result |
| --- | --- |
| Binary | 17.7 MB static ELF (`CGO_ENABLED=0`), including the 6.8 MB gzipped browser runtime |
| Server memory | 22.5 MB peak RSS over a full test run, including streaming 1.5 GB of model files |
| First download to the browser (fp32) | 1.53 GB: vision 372 MB, text 1.13 GB, tokenizer 34 MB |
| Browser storage afterwards | 1,563 MB in the Cache API |
| Browser vs CPU reference (`parity.py`) | cosine 1.000000 (6 d.p.) for all 4 images and all 8 queries, on both paths |
| WebGPU vs WebAssembly text vectors | cosine ≥ 0.9999999999990 |
| Same, before the Pillow-matched resize (canvas scaling) | 0.990–0.997 for images |

The software WebGPU numbers only show that the WebGPU path works; a real GPU would be
far faster. Real-GPU timings (Apple, NVIDIA, Intel, phones) still need measuring. The first
test run, before the preprocessing change, stalled at the browser-restart step; two later
full runs did not reproduce it.

## Testing

```sh
go test ./...                                   # vectors, fingerprint, model allowlist, tamper check, gzip serving
node e2e/e2e.mjs [--wasm] [--dump-text t.json]  # full browser run against a running server with an empty DB
python -I scripts/parity.py --model-dir data/models/<revision> \
    --fixtures ../../photoly_backend/tests/fixtures/ai --database-url "$DATABASE_URL" [--browser-text t.json]
```

The e2e test uses the AI fixtures from `photoly_backend/tests/fixtures/ai` and their 8
queries (English and Chinese). It requires every query's top hit to be the right photo,
checks that bad vectors are rejected, then restarts the browser and requires the model to
load with zero model downloads. Headless Chromium gets software WebGPU with
`--enable-unsafe-webgpu --enable-features=Vulkan --use-vulkan=swiftshader --use-webgpu-adapter=swiftshader`.

`parity.py` recomputes vectors on ONNX Runtime CPU with Pillow, following Photoly's server
path, and prints the cosine agreement with the browser's vectors.

## What's missing

- Server-side fallback indexing (ZIP imports, API uploads, phones without the model) and
  spot-checking client vectors. That needs ONNX Runtime in Go (cgo) and a Go port of
  `preprocess.js`.
- Sign-in, the real Photoly API and data model, videos, OCR, EXIF and everything else in
  the backend port.
- The text model is the big download (1.13 GB fp32). Options: fp16 (565 MB), a quantized
  text model, or embedding queries on the server, which is cheap there. Photos are where the
  compute is, and the vision model is the smaller file.
