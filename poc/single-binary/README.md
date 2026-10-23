# Photoly single-binary POC (Go + WebGPU)

A proof of concept for the plan in [`docs/single-binary-backend.md`](../../docs/single-binary-backend.md):

- **One static Go binary** serves the API, the web UI, the browser AI runtime and the model
  files, and migrates its own database. Its only dependency is PostgreSQL with pgvector.
- **SigLIP2 runs in the browser** over WebGPU when there is a real GPU, otherwise on the CPU
  with multi-threaded WebAssembly. Software-emulated GPUs (SwiftShader, llvmpipe and so on)
  are skipped because they are much slower than the CPU path. The browser
  embeds each photo before uploading it and embeds search text. The server runs no model:
  it validates, stores and searches the 768-number vectors.

It is not the Photoly API. There is no sign-in (every request acts as user 1), only images,
and no server-side fallback indexing yet. See [What's missing](#whats-missing).

## Try it with Docker

From this directory:

```sh
docker compose up -d --build
```

Open <http://localhost:8084>. Then:

1. Click **Load image model** and **Load text model**. The first time, the server downloads
   the model from Hugging Face (about 1.5 GB for fp32) and verifies it, and the browser
   caches it. Later visits load it from the browser.
2. Choose some photos under **Upload**. Each is embedded in your browser, then uploaded with
   its vector.
3. Search by description ("a dog on a beach"), or click a photo to find similar ones.

The status line at the top shows the route: WebGPU when the browser finds a real GPU,
otherwise WebAssembly on the CPU. To compare, add `?device=wasm` or `?device=webgpu` to the
URL.

| Setting (in `.env` or the shell) | Default | Meaning |
| --- | --- | --- |
| `POC_PORT` | `8084` | Host port, bound to 127.0.0.1 only because the POC has no sign-in |
| `PHOTOLY_MODEL_DTYPE` | `fp32` | `fp16` halves the download; needs a GPU with `shader-f16` |
| `POC_DB_PASSWORD` | `photoly-poc` | Password of the POC's own Postgres container |

- **What runs:** two containers. One is the POC image (28.8 MB: the static binary, CA
  certificates and nothing else, running as a non-root user). The other is the same pinned
  `pgvector/pgvector:0.8.7-pg17` image Photoly uses. Photos and the model cache live in the
  `photoly_data` volume, and the database in `db_data`.
- **Another machine:** WebGPU and the browser's model cache need a secure context.
  `localhost` counts; a LAN IP over plain HTTP does not. Put an HTTPS reverse proxy (for
  example Caddy) in front, and change the port binding. Remember that anyone who can reach
  it can upload and delete.
- **Behind a TLS-intercepting proxy:** pass its CA to the build with
  `docker build --secret id=extra_ca,src=proxy-ca.crt .`. At runtime, mount it and set
  `SSL_CERT_FILE` for the model download.
- **Reset:** `docker compose down -v` removes the containers and both volumes. The browser
  keeps its model copy until you click **Remove cached model**.

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
downloads and verifies the model ahead of time (for offline installs); `photoly health` checks a running server (used by the container health check).

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

Measured in a Linux cloud sandbox: 4 cores (AVX2), no GPU. "WebGPU" below is Chromium's
software GPU (SwiftShader), forced with `?device=webgpu`. The page no longer picks it by
itself.

**Model time only, per item (fp32):**

| Route | Image | Text query |
| --- | --- | --- |
| Native ONNX Runtime on the server CPU, 4 threads | 113 ms | 48 ms |
| Native, 1 thread | 327 ms | 123 ms |
| Browser WebAssembly, 4 threads (what the page uses now) | 492 ms | 284 ms |
| Browser WebAssembly, ONNX Runtime Web's default (2 threads here) | 846 ms | 274 ms |
| Browser WebAssembly, 1 thread | 1,662 ms | 493 ms |
| Browser WebGPU on SwiftShader (software) | ~51,600 ms | 12,000–15,000 ms |

- Without a GPU, the browser is CPU-bound like the server, but WebAssembly is about 4–5×
  slower than native code per core. Native ONNX Runtime uses AVX2/FMA kernels; WebAssembly
  is limited to 128-bit SIMD.
- An emulated GPU is ~100× slower than WebAssembly, which is why the page skips it.
- Real-GPU WebGPU timings (Apple, NVIDIA, Intel, phones) still need measuring.

**End-to-end runs:**

| | Default (software GPU present, so WebAssembly) | `--force-webgpu` |
| --- | --- | --- |
| Test | pass | pass |
| Fixture queries, top hit correct | 8/8 (4 English, 4 Chinese) | 8/8 |
| Per photo, including preparation and upload | 0.71 s | 51.6 s |
| Search, including tokenizing and the request | 155–250 ms | 12–15 s |
| First model load (download from the binary over localhost) | 56 s | 70 s |
| After closing and reopening the browser | 0 model requests, ready in 14 s | 0 model requests, ready in 19 s |

| | Result |
| --- | --- |
| Binary | 17.7 MB static ELF (`CGO_ENABLED=0`), including the 6.8 MB gzipped browser runtime |
| Server memory | 22.5 MB peak RSS over a full test run, including streaming 1.5 GB of model files (same in Docker; `docker stats` also counts ~1.5 GB of reclaimable file cache) |
| First download to the browser (fp32) | 1.53 GB: vision 372 MB, text 1.13 GB, tokenizer 34 MB |
| Browser storage afterwards | 1,563 MB in the Cache API |
| Browser vs CPU reference (`parity.py`) | cosine 1.000000 (6 d.p.) for all 4 images and all 8 queries, WebGPU and WebAssembly |
| WebGPU vs WebAssembly text vectors | cosine ≥ 0.9999999999990 |
| Same, before the Pillow-matched resize (canvas scaling) | 0.990–0.997 for images |

The first test run, before the preprocessing change, stalled at the browser-restart step;
later full runs did not reproduce it.

### Device choice

`web/app.js` picks the device once per page load:

1. `?device=wasm` or `?device=webgpu` in the URL forces one, for testing.
2. No WebGPU, or no adapter: WebAssembly.
3. A software adapter (`isFallbackAdapter`, or SwiftShader, llvmpipe, lavapipe, softpipe,
   Microsoft Basic Render, "software" in the adapter info): WebAssembly.
4. The fp16 model on a GPU without `shader-f16`: WebAssembly.
5. Otherwise: WebGPU.

WebAssembly uses one thread per core (`navigator.hardwareConcurrency`). This needs
cross-origin isolation, which the server's COOP/COEP headers provide.

## Testing

```sh
go test ./...                                   # vectors, fingerprint, model allowlist, tamper check, gzip serving
node e2e/e2e.mjs [--wasm | --force-webgpu] [--dump-text t.json]  # full browser run; needs a running server and an empty DB
python -I scripts/parity.py --model-dir data/models/<revision> \
    --fixtures ../../photoly_backend/tests/fixtures/ai --database-url "$DATABASE_URL" [--browser-text t.json]
```

The e2e test uses the AI fixtures from `photoly_backend/tests/fixtures/ai` and their 8
queries (English and Chinese). It requires every query's top hit to be the right photo,
checks that bad vectors are rejected, then restarts the browser and requires the model to
load with zero model downloads. By default it offers Chromium only its software GPU and
requires the page to fall back to WebAssembly. `--wasm` removes WebGPU entirely, and
`--force-webgpu` runs the whole test on the software GPU. The software GPU comes from
`--enable-unsafe-webgpu --enable-features=Vulkan --use-vulkan=swiftshader --use-webgpu-adapter=swiftshader`.

`parity.py` recomputes vectors on ONNX Runtime CPU with Pillow, following Photoly's server
path, and prints the cosine agreement with the browser's vectors.

## What's missing

- Server-side fallback indexing (ZIP imports, API uploads, phones without the model) and
  spot-checking client vectors. That needs ONNX Runtime in Go (cgo) and a Go port of
  `preprocess.js`.
- Sign-in, the real Photoly API and data model, videos, OCR, EXIF and everything else in
  the backend port.
- Without a GPU, the server's native CPU is ~4× faster than the browser here (113 ms vs
  492 ms per photo). A full design could send photos from GPU-less devices to the server for
  indexing, once the server has ONNX Runtime (above).
- The text model is the big download (1.13 GB fp32). Options: fp16 (565 MB), a quantized
  text model, or embedding queries on the server, which is cheap there. Photos are where the
  compute is, and the vision model is the smaller file.
