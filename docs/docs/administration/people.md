---
title: People operations and evaluation
sidebar_position: 9
---

## Enable and deploy

Apply Go migration `00013_people` through the existing migration command, rebuild the server and frontend, and use PostgreSQL with pgvector 0.8 or newer (iterative HNSW scans). Migration adds companion tables and a content-replacement trigger; it does not rewrite photos or search vectors. Both People feature switches default off. Enable global AI → administrator People → account AI permission → personal People. People uses the existing account `ai_search_enabled` permission but does not require a search profile, search model or completed search index.

Admin AI offers **Where People runs** (Server / Browser / Let users choose), independent of search indexing. The default is Let users choose; personal default is Browser. The AI worker panel exposes independent People load/unload controls and the shared worker's resident memory. CPU memory admission checks a 300 MB model estimate plus existing safety headroom. Actual process memory depends on image dimensions and other loaded models.

Models are downloaded lazily to `$PHOTOLY_DATA/models/people/yunet2023-sface2021-rgb-v1/`. An offline deployment can place the exact checksum-verified files there beforehand. The same-origin `/models/people/…` route serves weights; reverse-proxy deployments must forward `/models/` to the backend, as with existing browser AI. Models are cached on demand, excluded from the PWA startup precache, and checked against their SHA-256 in both server and browser. Without Cache Storage the browser can still run, but downloads may recur. HTTPS or localhost is needed for WebGPU; WASM remains the fallback. Browser execution still obtains canonical decoded pixels from the backend and stores detections and embeddings there; it is not a local-only privacy mode.

The pinned revision, sizes, checksums and upstream licenses are in `photoly_server/internal/ai/peopledata/`. YuNet is MIT; SFace's weight license is Apache-2.0. Retain these license files when redistributing weights. This records upstream grants, not a claim that every deployment's data processing requirements have been assessed.

## Input and profile contract

Fingerprint: `yunet2023-sface2021-rgb-v1`; grouping configuration: `conservative-v1`. Detection records use independent `yunet-2023mar-8f2383e4-preprocess-v1`, so a compatible future embedding model can reuse detection geometry.

1. Read the owned, visible photo at its current `content_version`. Decode and orient once on the server, composite transparency over white, resize the long edge to at most 1600. JPEG EXIF orientation is applied; the existing decoder treats PNG/WebP EXIF orientation as upright. HEIF support depends on installed media tools.
2. Supply identical canonical input to both runtimes: little-endian uint32 width, uint32 height, then RGB8 pixels. This avoids separate thumbnail/original decoding paths.
3. YuNet receives float32 NCHW 640×640 BGR, values 0–255, bilinear resize, top-left placement and zero padding. Decode strides 8/16/32, score ≥0.8, NMS IoU 0.3, minimum face size 12 canonical pixels, maximum 100 faces per photo.
4. Fit a similarity transform from the five landmarks to the OpenCV SFace 112×112 template. Supply RGB float32 0–255; the ONNX graph contains its input normalization. L2-normalize the 128-component output.
5. Only score ≥0.9 and boxes at least 2.5% of each canonical dimension can expand identities. Candidate retrieval is scoped by user and embedding fingerprint. Find the nearest identity and then the nearest *different* identity, so many photos of one person do not hide the runner-up. Match at cosine ≥0.60 with margin ≥0.08; otherwise create an unnamed singleton. Existing identities are never automatically merged.

Detection, embedding and grouping have separate functions and persisted versions. New embedding profiles require their own dimension/index and fingerprint; never compare incompatible vectors. A second-model migration UI is intentionally deferred. Existing ready detections are immutable to automatic retries; names, manual assignment, dismissal, merges and exclusions remain authoritative. A future migration must preserve these records explicitly.

## Jobs and recovery

The persistent `ai_job` queue carries `kind=people`, photo content version, model fingerprint and reset generation. Discovery reads at most 100 new eligible photos per pass; per-user writes serialize through the preference row. Server scheduling rotates through 25 eligible users every two seconds. Browser tabs and devices claim distinct jobs through leases. Server background admission follows the existing interval/rate throttle; both locations honor maintenance, scheduler pause, CPU/RAM/disk admission and the shared AI database storage budget. A claim reserves 200 KiB for up to 100 faces; successful results replace that reservation with a 2 KiB-per-face estimate. Actual People table and index sizes are included in physical usage. Server inference is serialized with other worker calls and has a six-minute deadline including initial model load.

Leases last 120 seconds and renew every 30 seconds. Failure backs off 30 seconds; three failed or expired attempts become failed until explicit retry. Every commit rechecks all four gates, ownership, storage visibility, content version, fingerprint, generation, placement and lease token. Shape, coordinates, finite values and normalized vector magnitude are validated. Empty face arrays are successful. Repeating an accepted token does not overwrite edits.

Opt-out and placement changes invalidate active personal leases. Disabling global/admin/account gates prevents new claims and rejects late results. Pausing retains browsing. Clear increments the generation, removes tasks and identities, and disables the opt-in in one transaction. Photo replacement invalidates old faces immediately; deletion cascades through vectors. Original-image decoding is limited to two concurrent requests across jobs and avatars. Thumbnail crops are authenticated and generated on demand with `private, no-store`, so there is no persistent crop cache to invalidate. All People APIs require `HRD-Token`; public photo tokens cannot authorize them.

## Reproduce verification

Use a disposable database, never a production library, for database tests. Supply `TEST_DATABASE_URL` to the Go test process. The fixture deletes its generated rows; the scale test also exercises large cascade cleanup.

```sh
cd photoly_server
go test ./...
TEST_DATABASE_URL=postgres://... go test ./internal/ai ./internal/server -run TestPeople -count=1
PHOTOLY_TEST_FACES=1 PHOTOLY_FACE_REPORT=/tmp/people-models go test ./internal/ai -run 'TestPeople(RealInference|Evaluation)' -count=1 -v
PHOTOLY_PEOPLE_SCALE=1 TEST_DATABASE_URL=postgres://... go test ./internal/ai -run TestPeopleScale -count=1 -timeout=20m -v
cd ../photoly_frontend_vue
pnpm typecheck
pnpm exec vitest run
pnpm exec playwright test e2e/people.spec.ts
PHOTOLY_FACE_MODELS=/tmp/people-models PHOTOLY_REQUIRE_WEBGPU=1 pnpm exec playwright test e2e/people-model.spec.ts --project=chromium
pnpm build
```

The report directory must also contain the two pinned model files for the browser test. `PHOTOLY_REQUIRE_WEBGPU=1` uses headed Chromium and requires a WebGPU execution provider; it does not quietly count WASM as a GPU pass. Browser comparison checks face counts, normalized boxes and landmarks within 0.002, and cosine agreement above 0.995 with Go CPU. This tests execution and preprocessing parity, not recognition accuracy.

## Recorded local results (2026-11-02)

Environment: Docker Linux/Go 1.26.10 CPU ONNX Runtime, PostgreSQL 17/pgvector 0.8.7; host Chromium for WASM and WebGPU. These are development measurements, not production sizing promises. Browser fresh-worker wall timings, including model fetch/cache and initialization, are recorded in `people-browser-evaluation.json`, including simulated WebGPU initialization failure and WASM recovery. Browser peak GPU/driver memory was not measured; no claim of hardware-independent latency is made.

Real inference covered no-human-face (cat), two individual portraits, synthetic two-person group, small image, brightness change, 10-degree tilt, JPEG, EXIF-rotated JPEG, animated GIF first frame and TIFF. See the checked-in `people-evaluation.json` for per-image CPU timings and peak process RSS. The minimum same-image-derivative cosine was 0.898077; the one different-identity comparison was 0.149942. Profile threshold 0.60 and margin 0.08 clear this fixed smoke set; they remain **provisional**. This tiny set cannot calibrate an album-wide false-merge rate, lookalikes, age changes, true profile faces or demographic differences. A larger licensed/consented multi-photo benchmark is required before claiming reliable commercial accuracy.

The synthetic database run seeded 100,000 photos and 200,000 faces. Observed person page: 297 ms; photo page: 9 ms; nearest-identity query: 1.9 ms; discovery plus claim: 612 ms. Go query allocations: 0.24 MiB, live heap: 0.97 MiB. These measurements exclude model inference and do not represent processing 100,000 real photos. HNSW is approximate; under high tenant selectivity it can miss candidates and cause extra singleton identities. Pagination and retrieval materialize bounded results.

Automated functional coverage includes all 16 gate combinations, revoked accounts, session/share-token isolation, invalid vectors and boxes, concurrent claims/reset, expiry and retries, permission restoration, shared storage-budget admission, placement changes, singleton promotion, ambiguity, low-quality anchors, merge/split/rename, hidden restoration, photo replacement/deletion, desktop/mobile entry points, Gallery navigation, viewer popup and clear confirmation. Manual names persist through repeated successful submission. Real child-process inference, deadline termination, automatic worker restart and model unload were exercised separately (158 MB observed worker RSS for the portrait fixture). Broader photographic accuracy and production-scale throughput remain separate evaluation work.
