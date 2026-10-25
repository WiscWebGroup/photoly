# Plan: bring the single-binary POC into Photoly (Go rewrite)

Status: plan. Builds on [single-binary-backend.md](single-binary-backend.md) (the dependency
choices, Qwen removal and trust rules are settled there) and on the tested POC in
[`poc/single-binary/`](../poc/single-binary/README.md).

## Goal

Replace the Flask backend, its worker containers, Redis and the PyTorch `ai-runtime` with one
Go binary (`photoly`) plus PostgreSQL. SigLIP2 runs on ONNX Runtime in a `photoly ai-worker`
child process, **or in the user's browser**: the admin sets where each model may run, and
users choose when the admin allows it.

Deployment goes from 6–9 containers to 2. The API contract (`msgCode` envelope, routes,
OpenAPI) stays exactly the same, so the Vue app keeps working throughout.

## What the POC already proves (reuse as-is)

| POC file | Becomes |
| --- | --- |
| `worker.go`, `memory.go` | AI worker process: RPC over pipes, `oom_score_adj`, memory check, int8 fallback, low-memory 409, unload on switch, worker restart |
| `embed.go`, `trim_linux.go` | ONNX Runtime sessions, `malloc_trim` after unload |
| `preprocess.go`, `tokenizer.go` | Image preprocessing and Gemma tokenizer, tested for parity with Python |
| `model.go` | Model download + sha256 allowlist, served to browsers under `/models/<rev>/` |
| `vectors.go`, `store.go` | Vector validation (768 finite, unit length) and pgvector storage |
| `web/app.js` (switch, int8, cache labels) | Ported into Vue composables (below) |
| `Dockerfile` | Distroless image with the ORT shared library, `ORT_DISABLE_TELEMETRY` |

## Phases

### 0. Ground rules (before any port)

- Branch is current with `main` (media tools, Dates, upload activity: migrations 0007–0008).
- **Acceptance suite** = the HTTP tests that call a running server (`tests/test_http.py`,
  `test_upload_http.py`, `test_ai_http.py`, `test_media_tools.py` HTTP parts, the smoke route
  inventory) plus `tests/contracts/`. Add a `PHOTOLY_BASE_URL` switch so the same suite runs
  against Flask or Go. A route moves to Go only when its tests pass there.
- Freeze `openapi.json` from the current definitions; the Go binary serves the file.

### 1. Skeleton — `photoly_server/` (Go module)

- Move the POC code in; subcommands `serve`, `migrate`, `ai-worker`, `verify`.
- Same env vars as `.env.example` (`DATABASE_URL`, `UPLOAD_FOLDER`, `ADMIN_EMAIL`, …).
- **Strangler proxy:** `serve` handles ported routes and reverse-proxies the rest to Flask
  (`httputil.ReverseProxy`). nginx's `/api/` now points at Go.
- Sessions in a Postgres `UNLOGGED` table; Redis lock → advisory lock. Users sign in once.

### 2. Database

- Keep the schema. Port Alembic 0001–0008 to embedded SQL (goose). Baseline: if
  `alembic_version` is `0008…`, mark goose as up to date; refuse to start on anything older.
- No schema changes until cutover, so Flask can still run against the same DB (rollback).

### 3. Port routes, in this order (each group: port → suite green → flip)

1. user / cred / common (scrypt hashes verified in the first POC)
2. namespace, tag, gallery, photo_extra
3. photo (50 routes: Dates timeline, media tools, remote image import, live import)
4. uploads + the upload worker as a goroutine (ZIP expansion, `install_one`, activity)
5. admin (28 routes, shutdown, storage, user groups)
6. ai (35 routes) — after phase 4

EXIF: pure-Go reader for photos, `ffprobe` for video dates; compare field-by-field against
exiftool on the fixture set before flipping photo routes. ffmpeg and tesseract stay CLIs.

### 4. AI on the server

- `ai-worker` replaces `ai-runtime`: keep the online (8) / background (1) queues, the
  status/control semantics and the `limits.py` resource thresholds; drop the HTTP tokens
  (it is a child process on pipes).
- Embedding scheduler, Smart Tag and maintenance (HNSW builds, purge) become goroutines over
  the same `ai_job` table — leases and dedup keys unchanged.
- **Qwen removed** first in the Python app (see the investigation doc), and startup refuses
  while a Qwen profile is active.
- **New profile fingerprint** (`runtime: onnx`, `preprocessing: exif-white-rgb-224-bilinear-v1`).
  The existing build → activate flow re-indexes in the background; search keeps using the old
  PyTorch profile until the new one is ready. Measured (docs/onnx-vs-pytorch-report.md): min image cosine 0.99888 < 0.999, so
  **decision: re-index in the background; no "keep existing vectors" option.**
- Devices: CPU everywhere; CUDA provider in the CUDA image; CoreML on macOS (replaces MPS).

### 5. Browser inference (the POC switch, made multi-user)

**Settings** (in `ai_control_state.desired`, validated in `state.py`'s Go port):

| Key | Values | Default |
| --- | --- | --- |
| `placement.image` | `server` · `browser` · `user` | `server` |
| `placement.text` | `server` · `browser` · `user` | `user` |
| `browser_text_model` | `small` (int8) · `full` | `small` |

Per-user preference (`user_ai_preference`: `image`, `text` ∈ `server`/`browser`) only applies
when the admin's value is `user`.

**API additions**

- `GET /ai/capabilities` gains `placement` (effective, per model), `modelBaseUrl`,
  fingerprint and file hashes.
- Upload: `PUT /uploads/{batch}/items/{id}/embedding` `{fingerprint, vector}` after content.
  Server validates, stores with `source = 'client'`, marks the state `done`; anything missing
  or rejected falls back to the server queue (if server placement is allowed) or stays
  `pending` with reason "waiting for a browser".
- Search: `POST /ai/search/vector` `{fingerprint, vector, topK}`; same lock, threshold and
  `user_id` filter as text search.
- Server-side: `POST /admin/ai/models/{kind}/load?force=1` / `unload`, returning the
  POC's `low-memory` 409 and `worker-died` 503 bodies.
- Spot check: when idle, re-embed a random 1% of `client` vectors on the server; drop a
  user's client vectors if cosine < 0.98.

**Frontend**

- `src/lib/browserAi.ts`: transformers.js + ORT Web loader (from `web/app.js`), model files
  from `/models/<rev>/`, Cache API, "Downloading" vs "Loading from this device" progress,
  dispose + reload to free WebAssembly memory.
- Hooks: `stores/uploads.ts` `send()` (after content PUT), `useAiSearch.ts` request function,
  `useSmartSearch.ts`. Image search by file and "similar" stay server-side in v1.
- **Cross-origin isolation** for WASM threads: COOP + `COEP: credentialless` (not
  `require-corp`, which would block Leaflet's external map tiles). If unsupported, run 1
  thread.
- Not in the PWA precache; download only on opt-in.

### 6. Cutover

- Compose: `photoly` + `db_postgres` (or, with `compose.aio.yml`, both in one image: `target: aio`) (+ optional embedded Vue build replacing `web`).
  Remove `backend`, `upload-worker`, `migrate`, `ai-*`, `db_redis`.
- Same volumes and `UPLOAD_FOLDER/<user_uuid>/<photo_uuid>.<ext>` layout — no file moves.
- Then drop Flask, `inference/`, PyTorch images, and narrow the dimension CHECK to 768.

## UI changes (designs on the redesign canvas)

| Artboard | What changes |
| --- | --- |
| Admin · AI · Where AI runs | "Inference" card replaces the device select: per model (photo indexing, search) Server / Browser / Let users choose; server memory meter showing which text model fits; worker status with Unload; SigLIP2 only |
| Admin · AI · Not enough memory | The confirm dialog when loading a model on the server may not fit (POC's 409 flow) |
| Account · AI on this device | User choice where allowed, browser search model size, saved-on-device status, keep / remove from device |
| Search · Loading model in browser | First search: download vs load-from-device progress |
| Library · Uploading · In-browser indexing | Upload panel row "Indexing in this browser · 3 of 12", with fallback note |

## Verification

- Acceptance suite green against Go for every route group before its flip; full suite before
  cutover; frontend Vitest + Playwright e2e against the Go stack.
- AI: `go test` parity tests (preprocess, tokenizer, fp32/int8 vs `testdata/reference.json`),
  worker kill/restart and low-memory tests (from the POC), ONNX-vs-PyTorch cosine report on a
  real library, recall on `tests/fixtures/ai`.
- Browser: POC e2e flows ported to Playwright (switch, reopen with zero model downloads,
  int8 search recall), plus a check that map tiles load with COEP on.
- Ops: memory per process at idle and under upload, container count, upgrade from a real
  Alembic `0008` database and rollback to Flask on the same DB.

## Risks

- Size: ~187 route handlers and ~2.8k lines of services. Expect 15–25k lines of Go.
- EXIF parity without exiftool (HEIC, odd makers) — keep an exiftool fallback flag if needed.
- One full re-index on the ONNX profile; big libraries on CPU take hours (runs in background).
- Browser vectors are only as trustworthy as the client; the spot check bounds the damage to
  the uploader's own results.
