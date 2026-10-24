# Photoly single-binary POC (Go + WebGPU)

A proof of concept for the plan in [`docs/single-binary-backend.md`](../../docs/single-binary-backend.md):

- **One static Go binary** serves the API, the web UI, the browser AI runtime and the model
  files, and migrates its own database. Its only dependency is PostgreSQL with pgvector.
- **SigLIP2 runs in the browser** over WebGPU when there is a real GPU, otherwise on the CPU
  with multi-threaded WebAssembly. Software-emulated GPUs (SwiftShader, llvmpipe and so on)
  are skipped because they are much slower than the CPU path. The browser
  embeds each photo before uploading it and embeds search text. The server validates,
  stores and searches the 768-number vectors.
- **Or on the server, per model, at the user's choice.** The page has a switch for photos
  and one for search text: "this browser" or "the server". On the server, the same ONNX
  files run on ONNX Runtime (CPU) through cgo, with Go ports of the image preparation and
  the tokenizer. Both routes produce the same vectors (cosine ≥ 0.9998), so they share
  one profile and one index, and the switch can change at any time. Models run in a
  separate worker process with a memory check first; see [Server memory](#server-memory).

It is not the Photoly API. There is no sign-in (every request acts as user 1) and only
images. See [What's missing](#whats-missing).

## Try it with Docker

From this directory:

```sh
docker compose up -d --build
```

Open <http://localhost:8084>. Then:

1. Choose where each model runs: **this browser** (default) or **the server**. The choice is
   remembered in this browser.
2. Click **Load image model** and **Load text model**. The first time, the server downloads
   the model from Hugging Face (about 1.5 GB for fp32) and verifies it. In the browser, it
   is then cached there and later visits load it from the cache; on the server, it loads
   into memory (it also loads by itself on first use).
3. Choose some photos under **Upload**. Each is embedded where the switch says and stored
   with its vector.
4. Search by description ("a dog on a beach"), or click a photo to find similar ones.

The status line at the top shows where each model runs. In the browser, photos use WebGPU
when the browser finds a real GPU, otherwise the CPU (WebAssembly); search text always uses
the CPU (see [Device choice](#device-choice)). After each search, the status shows tokenizer,
text model and search times; server-side times come from the `Server-Timing` header. To compare devices on your machine, add `?textDevice=webgpu`,
`?imageDevice=wasm`, or `?device=` (both) with `webgpu` or `wasm` to the URL.

| Setting (in `.env` or the shell) | Default | Meaning |
| --- | --- | --- |
| `POC_PORT` | `8084` | Host port, bound to 127.0.0.1 only because the POC has no sign-in |
| `PHOTOLY_MODEL_DTYPE` | `fp32` | `fp16` halves the download; needs a GPU with `shader-f16`, and turns server inference off |
| `POC_DB_PASSWORD` | `photoly-poc` | Password of the POC's own Postgres container |

- **What runs:** two containers. One is the POC image (76.8 MB on arm64: the binary, the
  ONNX Runtime 1.29.0 CPU library, and distroless `cc` for glibc and libstdc++, running as
  a non-root user with no shell). The other is the same pinned
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
pgvector 0.7+ (`halfvec`). For server inference: a C toolchain (cgo) and the
[ONNX Runtime 1.29.0](https://github.com/microsoft/onnxruntime/releases/tag/v1.29.0)
shared library for your platform, the version `onnxruntime_go` v1.37.0 is written for.

```sh
./scripts/vendor.sh                     # fetch transformers.js + ONNX Runtime Web into web/vendor (gzipped)
go build -trimpath -ldflags="-s -w" -o photoly .           # with server inference (cgo)
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o photoly .   # or: static, browser inference only

export DATABASE_URL=postgres://user:pass@localhost:5432/photoly_poc
./photoly                               # = photoly serve: migrate, then listen on :8084
```

Open <http://localhost:8084>. WebGPU needs a secure context: `localhost` or HTTPS.

| Variable | Default | Meaning |
| --- | --- | --- |
| `DATABASE_URL` | required | PostgreSQL connection string |
| `PHOTOLY_ADDR` | `:8084` | Listen address |
| `PHOTOLY_DATA` | `./data` | Photos and the model cache |
| `PHOTOLY_MODEL_DTYPE` | `fp32` | `fp32` (matches the server reference) or `fp16` (half the download, needs a GPU with `shader-f16`; browser only) |
| `ONNXRUNTIME_LIB` | `libonnxruntime.so` (`.dylib` on macOS) | Path of the ONNX Runtime library. If it can't be loaded, the server logs why and the page offers the browser only |

Other commands: `photoly migrate` applies migrations and exits; `photoly fetch-model`
downloads and verifies the model ahead of time (for offline installs); `photoly health` checks a running server (used by the container health check). `photoly ai-worker` is the inference process `serve` starts for itself.

**Retesting:** **Delete all photos** (under Upload) removes every photo and vector
(`DELETE /api/photos`), so the same files can be uploaded again with another setting.

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

switched to the server:
POST /api/photos {file, embed=server} ─────────▶ preprocess.go ─▶ ONNX Runtime ──▶ photo, halfvec(768)
POST /api/search {query} ──────────────────────▶ tokenizer.go ─▶ ONNX Runtime ─▶ cosine search
POST /api/ai/{image|text}/load[?force=1] ──────▶ load + warm up now instead of on first use
POST /api/ai/{image|text}/unload ──────────────▶ free it (stop the worker if it holds nothing)
                                                (all of these run in the ai-worker process)
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
  bad vector only affects its uploader. `photo_embedding.source` records `client` or
  `server`, so client vectors can later be re-checked server-side.
- **Image preparation** (`web/preprocess.js`) copies Photoly's server path exactly: decode
  without colour management, EXIF orientation, white behind transparency, RGB, then a port
  of Pillow's antialiased bilinear resize (22-bit fixed point). On the test fixtures the
  224 × 224 output is **bit-identical to Pillow's**.
- **Browser runtime:** transformers.js 4.3.1 with ONNX Runtime Web
  (`ort-wasm-simd-threaded.asyncify`), embedded in the binary as 6.8 MB of gzip and sent
  compressed. No CDN requests. Pages are cross-origin isolated (COOP/COEP), so WebAssembly
  can use threads.
- **Server inference** (`worker.go`, `embed.go`, `preprocess.go`, `tokenizer.go`): `photoly
  serve` starts `photoly ai-worker`, which loads ONNX Runtime through `onnxruntime_go`;
  each model loads (and warms up) when a user switches to it or on first use, from the
  same verified files the browser gets. The server prepares images itself and sends
  pixels to the worker over `net/rpc` on a pipe. `preprocess.go` is a line-by-line port of
  `preprocess.js` (bit-identical to Pillow on the PNG fixtures; Go's JPEG decoder differs
  slightly from libjpeg). `tokenizer.go` is the Gemma BPE tokenizer read from
  `tokenizer.json` (byte fallback, added tokens), matching Hugging Face `tokenizers` on all
  test strings. Runs are one at a time per model; ONNX Runtime uses every core for each.
  ONNX Runtime's telemetry is turned off (`ORT_DISABLE_TELEMETRY=1`). The server path reads
  JPEG, PNG, GIF and WebP up to 50 megapixels; HEIC and others stay browser-only. fp32 only.
- <a id="server-memory"></a>**Server memory.** A model that doesn't fit used to get the
  whole server killed by the kernel, and every later server-mode request killed it again.
  Now:
  1. **Check first.** Before loading, the server compares the model's peak need
     (`memory.go`: image ~900 MB, text fp32 ~2.25 GB, text int8 ~900 MB; peaks are far
     above steady use) plus 150 MB headroom with free memory: the container's cgroup limit and
     the system's `MemAvailable`, whichever is lower (Linux only; elsewhere it loads).
  2. **Smaller text model.** If fp32 text doesn't fit but int8 does, the int8 text model
     loads without asking (~35% less memory; query vectors agree with fp32 at cosine
     0.935–0.96, 8/8 fixture queries still correct). Only queries use it, so the index and
     profile don't change. fp16 and the 4-bit files use *more* memory than fp32 on the CPU,
     because ONNX Runtime expands them when it loads them.
  3. **Ask.** If nothing fits, the server answers 409 `low-memory` with the numbers, and
     the page asks the user. Cancel keeps that model in the browser; OK retries with
     `force=1`.
  4. **Contain.** Models run in the `ai-worker` child process, which marks itself first
     in line for the out-of-memory killer. If it is killed, the request gets 503
     `worker-died`, the page moves that model back to the browser, and the next
     server-mode request starts a fresh worker. The API, uploads and browser-mode search
     keep working.

  5. **Give it back on switch.** Moving a model back to the browser unloads it on the
     server (`/unload`, then `malloc_trim` so glibc returns the pages); once the worker
     holds no model, it is stopped, so all of its memory goes back to the system. Moving a
     model to the server disposes the browser copy after the server has it. Disposing
     frees GPU memory, but WebAssembly memory never shrinks (3.6 GB stayed after disposing
     both models), so when no model is left in the browser the page reloads itself and
     the switch setting is kept; otherwise it offers a "Reload to free browser memory"
     button. Tested in Chrome: browser page memory 3,584 MB → 4 MB after both models moved
     to the server; text unloaded on the server (worker 1,171 → 441 MB in the worker test);
     the worker stopped once both moved back. This assumes one user per server, as in this
     POC; with several, unload when the last client leaves or after an idle timeout.

  Tested with the container capped at 1 GB: the image model loads, the text model
  triggers the warning, accepting it gets the worker killed with the server still up
  (no container restart), and the next upload works. At 2 GB, the image model plus int8
  text load by themselves and the e2e test passes 8/8.
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

| | Default (software GPU present, so WebAssembly) | `--force-webgpu` (both models) |
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
| Server memory, server inference on | 846 MB RSS with both models loaded (Docker, arm64) |
| Server memory | 22.5 MB peak RSS over a full test run, including streaming 1.5 GB of model files (same in Docker; `docker stats` also counts ~1.5 GB of reclaimable file cache) |
| First download to the browser (fp32) | 1.53 GB: vision 372 MB, text 1.13 GB, tokenizer 34 MB |
| Browser storage afterwards | 1,563 MB in the Cache API |
| Browser vs CPU reference (`parity.py`) | cosine 1.000000 (6 d.p.) for all 4 images and all 8 queries, WebGPU and WebAssembly |
| WebGPU vs WebAssembly text vectors | cosine ≥ 0.9999999999990 |
| Same, before the Pillow-matched resize (canvas scaling) | 0.990–0.997 for images |

**Server inference** (Docker on an Apple-silicon Mac, 8 cores, arm64 Linux VM):

| | Server | This browser (headless Chromium, WebAssembly) |
| --- | --- | --- |
| e2e test, fixture queries correct | pass, 8/8 | pass, 8/8 |
| Model downloads to the browser | 0 | 1.53 GB |
| First model load | 1.7 s image, 5.2 s text (from the server's disk) | 24 s image, 34 s text |
| Per photo, including upload | 303 ms | 531 ms (embedding only) |
| Text model per query | 122–311 ms | 142–194 ms |
| Vectors vs Python reference (Pillow + ONNX Runtime CPU) | images 0.99977 (JPEG) to 1.000000 (PNG); text 1.000000 | see `parity.py` above |

**Known issue, intermittent:** in 1 of 6 full test runs, the 1.13 GB text model was fetched
from the server again after a browser restart instead of coming from the Cache API. In
another early run the restart step stalled. Neither reproduced in 3 dedicated restart
cycles. transformers.js treats any Cache API error as a miss and silently downloads again,
so results stay correct but a ~1 GB download repeats. A sturdier option is a custom cache
on the Origin Private File System, written to a temporary name and renamed only once
complete (`env.useCustomCache`).

### Browser text model: small by default

In the browser, search text uses the **int8** text model unless the user picks "full"
(**Browser search model**, remembered; `?textModel=full|small` overrides). Measured in
Chrome on the M1 Pro:

| Browser text model | Download | Page memory | Query (median) | Fixture queries | Cosine vs fp32 | Smallest top-1 lead |
| --- | --- | --- | --- | --- | --- | --- |
| full (fp32) | 1,164 MB | 3,181 MB | 308 ms | 8/8 | 1.000 | 0.058 |
| small (int8, default) | 318 MB | 900 MB | 289 ms | 8/8 | ≥ 0.938 | 0.031 |

With both models in the browser, storage drops from 1,563 MB to 717 MB, and a reopened
page is ready in 3.0 s instead of ~6 s. The cost is precision: query vectors drift from
fp32 (cosine 0.94–0.98), and the right photo's lead over the runner-up halved on the
fixtures. With 4 test photos that is all the evidence there is; check ranking on a real
library before relying on it, and pick "full" if results look off. Only queries use it:
query vectors are never stored, so the switch is instant and needs no re-indexing, and the
profile fingerprint doesn't change. The server's text model is chosen separately (fp32,
or int8 when memory is short; see [Server memory](#server-memory)). fp16 isn't offered:
on the CPU it uses more memory than fp32. int8 runs on WebAssembly only, so a text model
forced onto WebGPU uses full.

### Device choice

`web/app.js` picks a device for each model once per page load:

| | Photos (vision model) | Search text (text model) |
| --- | --- | --- |
| Default | WebGPU on a real GPU, else CPU | CPU |
| URL override | `?imageDevice=webgpu\|wasm` | `?textDevice=webgpu\|wasm` |

`?device=` sets both. Forcing `webgpu` also allows a software GPU, for testing.

WebGPU falls back to the CPU when there is no WebGPU or no adapter, when the adapter is
software (`isFallbackAdapter`, or SwiftShader, llvmpipe, lavapipe, softpipe, Microsoft
Basic Render or "software" in the adapter info), or for the fp16 model on a GPU without
`shader-f16`.

**Why search text stays on the CPU:** one query is 64 tokens, roughly a third of an image's
work, and it can't be batched. On the GPU, fixed costs per step (475 GPU kernels per run,
plus copying data in and reading the result back) take a large share of such a small job.
WebGPU text embedding was reported slower than WebAssembly on a real GPU. ONNX Runtime's
logs rule out silent CPU fallback: 475 of the text model's 539 nodes run on WebGPU, the 64
CPU nodes are shape calculations, and the only copy to the GPU is the input. Keeping text on
the CPU also keeps the 1.1 GB text model (mostly its 256k-word vocabulary table) out of GPU
memory.

**Warm-up:** right after loading, each model runs once on dummy input. The first run
compiles GPU shaders and initialises kernels: 18.9 s cold vs 14.3 s warm for text on
SwiftShader, and 2.0 s vs ~0.25 s on WebAssembly here. Without warm-up, that cost landed on
the first real search or photo and made it look slow.

WebAssembly uses one thread per core (`navigator.hardwareConcurrency`). This needs
cross-origin isolation, which the server's COOP/COEP headers provide.

## Testing

```sh
go test ./...                                   # vectors, fingerprint, model allowlist, tamper check, gzip serving,
                                                #   image preparation, EXIF orientation
PHOTOLY_TEST_MODELS=DIR go test ./...           # also: tokenizer, server vectors (fp32 and int8 text) vs testdata/reference.json,
                                                #   and the AI worker: RPC, killed mid-request, int8 fallback, low-memory refusal
                                                #   (DIR holds <revision>/tokenizer.json, onnx/…; needs cgo + ONNX Runtime, ~2.5 GB free)
node e2e/e2e.mjs [--wasm | --force-webgpu | --server] [--text-model small|full] [--dump-text t.json]
                                                # full browser run; needs a running server and an empty DB
                                                # (use --text-model full with --dump-text for parity.py)
node e2e/bench.mjs --url URL --container NAME --chrome PATH   # browser vs server: wait time, memory, CPU (deletes all photos)
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

`--server` switches both models to the server and runs the same uploads and queries. It
also requires zero model downloads to the browser, 422 for an over-long query and an
undecodable image, and the switch to survive a browser restart.

`testdata/reference.json` comes from `scripts/make_testdata.py` (Python: `tokenizers`,
Pillow, ONNX Runtime CPU). Regenerate it if the model or the edge-case strings change.

`parity.py` recomputes vectors on ONNX Runtime CPU with Pillow, following Photoly's server
path, and prints the cosine agreement with the browser's vectors.

## What's missing

- Automatic server-side indexing: today the server embeds only when a client asks. ZIP
  imports, API uploads, photos stored without a vector, and re-checking a sample of client
  vectors could now use the same `Embedder`, from a background job.
- A default per device: the switch is manual. The page could pick the server when it finds
  no real GPU (the server's native CPU is faster than WebAssembly) or when the browser
  can't hold the 1.5 GB model, such as phones.
- Server inference on a GPU (CUDA, or CoreML on a Mac). Unloading idle models on a timer
  (today only on switch); switching text back from int8 to fp32 when memory frees up
  (today: move text to the browser and back).
- Memory checks on macOS and Windows (skipped there today).
- Sign-in, the real Photoly API and data model, videos, OCR, EXIF and everything else in
  the backend port.
- A ranking check of the int8 text model on a real library (hundreds of photos, many
  queries: top-10 overlap with fp32), before it stays the browser default.
