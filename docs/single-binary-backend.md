# Investigation: the backend as a single binary

Status: investigation only. Nothing in the running app has changed.

## Summary

A single, lightweight backend binary is feasible if the backend is rewritten in **Go**.
A proof of concept built a **15 MB statically linked binary** that served HTTP, ran the
background workers as goroutines, embedded the static assets and city index, and used
**~22 MB of memory**. The current Python API process group alone uses ~200 MB.

Two things cannot reasonably go into the binary: **PostgreSQL** (pgvector is central) and
the optional **AI runtime** (PyTorch). Both already run as separate services, and the
binary would talk to them the same way the Python backend does today.

Packaging the existing Python code as one file (PyInstaller, Nuitka) is not worth doing:
it keeps the same memory use and the same external tools, and only changes how the files
are shipped.

## What runs today

| Piece | How it runs | Fate in a single binary |
| --- | --- | --- |
| `backend` (Gunicorn, 2 workers) | container | `photoly serve` |
| `upload-worker` | container, same image | goroutine inside `serve` |
| `migrate` (Alembic + seed + reconcile) | one-shot container | `photoly migrate`, or run at startup |
| `ai-scheduler` (+ Smart Tag) | container, AI profiles | goroutine, on when AI is enabled |
| `ai-maintenance` | container, AI profiles | goroutine |
| `db_redis` | container | **removed** (see below) |
| `web` (nginx + Vue build) | container | optional: embed `dist/` and serve it |
| `db_postgres` + pgvector | container | **stays** |
| `ai-runtime` (PyTorch) | container / native MPS | **stays**, HTTP API unchanged |

Deployment goes from 6–9 containers to **2** (`photoly` + Postgres), plus the AI runtime
when AI is enabled.

## Measurements (this sandbox, x86-64)

| | Python today | Go proof of concept |
| --- | --- | --- |
| Install size | 166 MB virtualenv + Python 3.12 runtime | 15 MB single file, no runtime |
| API memory at idle | 24 MB master + 2 × 89 MB workers ≈ 200 MB | 22 MB (city index loaded) |
| Worker containers | each loads the same stack again (3 more processes) | goroutines in the same process |

The Go figure covers the proof of concept, not a full port. A full port would probably sit
around 30–60 MB of memory, still well below today's total.

## External dependencies

| Dependency | Used for | Go option |
| --- | --- | --- |
| Redis | session tokens, similar-photo paging cursors, one compare-and-delete lock | Postgres `UNLOGGED` table for sessions and cursors (in memory if only one process runs), Postgres advisory lock for the lock. Proof of concept includes the session table. |
| exiftool (Perl) | ~20 EXIF fields: camera, lens, GPS, exposure, dates | Pure-Go EXIF reader (e.g. `evanoberholster/imagemeta`) for photos. Video `CreationDate` from `ffprobe`, which is already required. |
| ffmpeg / ffprobe | video probing and frame sampling | Keep as an external CLI. A trimmed static build (only the demuxers/decoders in `app/ai/video.py`) can ship next to the binary. |
| tesseract | on-demand OCR | Keep as an external CLI. OCR is optional; without it only the OCR feature is off. |
| Pillow | decode checks, `exif_transpose`, 512×512 avatar crop | Go `image` + `golang.org/x/image` + a resize library. Check format coverage (TIFF, WebP) and decompression-bomb limits. |
| GeoNames SQLite file / TSV | offline reverse geocoding | Embedded with `go:embed`. Loaded into a sorted in-memory slice; no SQLite driver needed. Done in the proof of concept. |
| Swagger UI assets, OpenAPI | `/docs`, `/openapi.json` | Embedded. Generate `openapi.json` once from the current definitions and serve it as a file. |
| Alembic migrations (6) | schema | Port to embedded SQL (`goose` or `golang-migrate`). Existing installs need a baseline mapping from the current Alembic head. |

### Compatibility checks done

- **Passwords:** existing hashes use Werkzeug's `scrypt:32768:8:1$salt$hex` format. The
  proof of concept checks them with `golang.org/x/crypto/scrypt` (right password accepted,
  wrong one rejected), so users keep their passwords.
- **Sessions:** Redis session tokens would not carry over. Users sign in again once after
  the switch.

## Size of the rewrite

- ~370 KB of dense Python application code, **138 route handlers**, background schedulers,
  Smart Tag/trip grouping and upload/ZIP import logic.
- The Go version would probably be 15–25k lines.
- The legacy API contract (`msgCode` semantics, HTTP 200 on business errors, frozen Spring
  route/schema contracts in `tests/contracts/`) has to stay exactly the same.

**Test reuse:** `tests/test_http.py`, `test_upload_http.py` and `test_ai_http.py` call a
running server over HTTP, so they can serve as the acceptance suite for the Go binary
unchanged. The unit tests import Python internals and would need Go equivalents.

## Recommended plan

1. **Go, not Rust.** It has mature Postgres/pgvector libraries (`pgx`, `pgvector-go`),
   built-in `embed`, trivial static builds and goroutines for the workers. Rust would be
   slightly leaner but slower to write.
2. **Port one piece at a time.** The Go binary sits in front of the API, serves the routes
   it has ported, and proxies the rest to the existing Flask app. Move route groups across
   (user/session → namespace/tag/gallery → photo → uploads → admin → AI/Smart), running the
   HTTP test suite against the Go binary at each step.
3. Move the workers into goroutines, replace Redis, then retire Gunicorn and the worker
   containers.
4. Optionally embed the Vue build so the frontend container goes away too.

### Cheaper alternative (keep Python)

If a rewrite is too much, most of the operational savings come from consolidating:
one container running Gunicorn plus the upload/maintenance/scheduler loops as threads,
Redis replaced by Postgres, and migrations run at startup. That takes 6–9 containers
down to 2–3 in a few days of work. Memory per process stays the same as today.

## Proof of concept

A ~200-line `main.go` with subcommands `serve`, `migrate` and `verify`:

- `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"` produced a 15 MB static ELF.
- `serve`: `/healthz` (pings Postgres), embedded Swagger assets (1.5 MB bundle served),
  nearest-city lookup from the embedded index (Madison, WI → `America/Chicago`), sessions
  read from Postgres, upload/maintenance loops as goroutines, graceful shutdown on SIGTERM.
- `verify`: checks Werkzeug scrypt hashes.

It was a feasibility check and is not committed.

## Follow-up: the AI runtime in the same binary

The runtime serves SigLIP2 base (`google/siglip2-base-patch16-224`, 375M parameters,
768-dim) and Qwen3-VL-Embedding-2B (2048-dim) through PyTorch on CPU, CUDA or MPS.

**It is possible, but the result is not a lightweight binary.** The inference code can be
compiled in. The parts that make AI heavy cannot:

- **Model weights:** roughly 0.8–1.5 GB for SigLIP2 and ~4 GB for Qwen at 16-bit. They are
  downloaded on first load, as they are today.
- **GPU libraries:** the CUDA libraries are several GB and cannot be statically linked.
  Go would also need cgo, so each platform gets its own build (Linux CPU, Linux CUDA,
  macOS Metal) instead of one universal file.

### Engine options

| Model | Engine | Notes |
| --- | --- | --- |
| SigLIP2 | ONNX Runtime via cgo (`yalue/onnxruntime_go`) | Standard encoder that exports cleanly to ONNX. CPU, CUDA and CoreML providers. Needs the Gemma tokenizer and SigLIP preprocessing ported to Go. |
| Qwen3-VL-Embedding-2B | llama.cpp / ggml via cgo | Qwen3-VL is supported upstream and community GGUF builds of the embedding model exist. The embedding path (last-token pooling, chat template, image patching) is unverified and would need testing. Quantizing would cut memory a lot, at some cost to accuracy. |

Running two native engines in one binary adds real build and maintenance cost.

### Consequences

- **Vectors change.** A different engine (or quantization) won't produce identical
  embeddings, so this would be a new search profile and the library would be re-indexed.
  The existing profile/fingerprint design already handles that. `scripts/ai_model_smoke.py`
  and `tests/fixtures/ai` can check cosine agreement with the PyTorch reference.
- **Crash isolation.** Today a model that runs out of memory while loading only takes down
  the runtime. In-process, it would take down the API. Fix: ship one binary but run AI as a
  child process (`photoly ai-worker`) started and restarted by `photoly serve`.
- **macOS gets simpler.** Docker on a Mac cannot use the GPU, which is why MPS needs a
  native runtime today. A native binary with Metal could run everything outside Docker
  (Postgres aside).

### Recommendation

1. Port the backend first. Keep the Python AI runtime as is; it is already a separate HTTP
   service.
2. Then, optionally, build SigLIP2 into the binary with ONNX Runtime as a built-in "light AI"
   (CPU by default, child process), so basic smart search works without the PyTorch
   container.
3. Leave Qwen3-VL in the Python runtime unless the llama.cpp embedding path is shown to
   match quality.
