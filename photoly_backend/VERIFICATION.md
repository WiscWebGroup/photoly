# Flask migration verification

Verified on 2026-10-12 against the migrated working tree. Compatibility baseline: Git commit `e6619ab`.

## Results

| Check | Passed | Failed | Skipped | Deselected | Duration |
|---|---:|---:|---:|---:|---:|
| Local pytest: SQLite, fakeredis, real media libraries | 255 | 0 | 0 | 3 HTTP tests | 13.09 s |
| Container pytest: real MySQL and Redis | 255 | 0 | 0 | 3 HTTP tests | 33.70 s |
| HTTP acceptance: production Gunicorn container | 3 | 0 | 0 | 0 | 0.94 s |

The same 255 tests run with both dependency configurations; these are not 510 distinct tests. Statement coverage is **93%** in each configuration. The three HTTP tests run separately because they require the disposable server and the last test intentionally stops it.

Production and test Docker images built successfully on Docker Desktop / Linux ARM64. The production image runs Python 3.12.15, Flask 3.1.2 and SQLAlchemy 2.0.43. Dependencies were installed from the frozen `uv.lock`. Integration services used MySQL 8.3.0 and Redis 7.2.4.

## Commands

From `photoly_backend`:

```sh
uv sync --python 3.12
uv run pytest -q --cov=app --cov-report=term-missing
```

From the repository root, sequentially:

```sh
docker compose -p photoly-flask-check -f compose.test.yml build tests backend smoke
docker compose -p photoly-flask-check -f compose.test.yml run --rm tests
docker compose -p photoly-flask-check -f compose.test.yml run --rm smoke
docker compose -p photoly-flask-check -f compose.test.yml ps -a
docker compose -p photoly-flask-check -f compose.test.yml down -v --remove-orphans

docker compose --env-file .env.example config --quiet
docker compose -f compose.test.yml config --quiet
git diff --check
```

Both Compose configurations validate, and the Git whitespace check passes. The test stack uses its own network, disposable MySQL storage, Redis databases 14/15, and temporary media directories. It mounts no existing business data and publishes no host ports.

## What was verified

- All 90 Spring business routes have independent success assertions and applicable authentication failures; route inventory matches exactly. HTTP methods, parameter names/encodings, frontend trailing slashes, raw Cred responses, envelope/null/boolean/date/page semantics, and Java entity field names/types are checked.
- All eleven table column names, SQL types, nullability and primary keys match the original SQL snapshot. New integrity constraints are described in `COMPATIBILITY.md`.
- Registration, default folders/galleries, password hashing/change, session expiry/revocation, admin permissions, C/R/D permissions, ownership checks, folder cycles, logical moves, associations, tag intersection and four search modes.
- Uploads, metadata without EXIF, GPS EXIF, thumbnails and fallback, share/private/public access, QR decoding, original downloads, Range 206/416, archive contents, duplicate filenames, file deletion during ZIP creation, path/symlink protection and transactional rollback/cleanup.
- Fresh and repeated initialization, administrator root/gallery creation, deletion cascades and credentials/session cleanup.
- Real HTTP initialization → registration/login → image/video upload → folder/tag classification → share → download/ZIP → deletion, plus concurrent opposite folder moves across Gunicorn workers.
- `/admin/stop` returns `{ "t": 1, "msgCode": 591023 }`; Gunicorn logs show the master receiving SIGTERM, both workers exiting, and the master shutting down. Docker reports the backend **Exited (0)**. Tests using Flask's test client inject a fake lifecycle service and never signal the test runner.

## Limits of this acceptance

Compatibility expectations were derived from the original Controllers, Services, SQL, serialization settings and frontend/Postman references. The old Java server was not run for live differential comparison. Known intentional behavior changes are recorded in `COMPATIBILITY.md`.

The backend images and HTTP server were built and exercised. The root Compose was subsequently built and started locally with all four services running; MySQL, Redis and the backend passed their health checks. The unchanged Vue frontend was successfully built, its `/` and `/login` pages and JS/CSS assets returned HTTP 200, and its API URL and browser CORS preflight were verified. Administrator login, profile, root folder, default gallery and admin permissions also passed HTTP checks. No automated browser interaction suite was run. The small MP4 fixture verifies opaque storage, exact response bytes and Range handling; it does not exercise a browser video codec.

## OpenAPI / Swagger acceptance — 2026-10-12

Based on `main` at `5005c91` (merged migration PR #22), on branch `codex/openapi-swagger-docs`. This change adds public `/docs`, `/docs/`, `/openapi.json` and local documentation assets. Existing business Controllers, Services, Entity mappings, storage, SQL and Vue calls are unchanged.

| Check | Passed | Failed | Skipped | Deselected | Duration |
|---|---:|---:|---:|---:|---:|
| Full local suite, including documentation and original contracts | 262 | 0 | 0 | 4 HTTP | 13.99 s |
| Full container suite with real MySQL / Redis | 262 | 0 | 0 | 4 HTTP | 35.55 s |
| Real HTTP / production Gunicorn acceptance | 4 | 0 | 0 | 0 | 1.85 s |

Coverage is **94%** locally and with real dependencies. The 90 original success cases and 87 applicable authentication-failure cases now also validate their actual responses against the OpenAPI schemas; the independent original assertions remain intact. New checks cover OpenAPI 3.2 validity, exact route inventory, unique operation IDs, all references and examples, request parameters/bodies against independent contract cases, entity fields/types, multipart metadata, CSV/repeated lists, range/download contracts and static asset checksums. Docs and assets are tested with MySQL/Redis calls configured to fail if attempted.

Commands (repository root, except local pytest):

```sh
# In photoly_backend:
uv run pytest -q --cov=app --cov-report=term-missing

# In repository root:
docker compose -p photoly-openapi-check -f compose.test.yml build tests backend smoke
docker compose -p photoly-openapi-check -f compose.test.yml run --rm tests
docker compose -p photoly-openapi-check -f compose.test.yml run --rm smoke
docker compose -p photoly-openapi-check -f compose.test.yml ps -a

docker compose up -d --build backend
docker compose ps
curl -fsS http://localhost:8384/openapi.json
```

The disposable backend exited with code **0** after the HTTP stop test. Swagger UI 5.32.0 assets came from the official npm tarball with verified SHA-512 integrity. JS/CSS, upstream LICENSE/NOTICE and the bundle's third-party license notices are packaged locally. After adding the third-party license file, both images were rebuilt and all seven documentation tests were rerun inside the final test image; the same seven checks also passed locally.

Browser acceptance in Chrome used an isolated test backend temporarily bound to **127.0.0.1:8385** with this uncommitted override:

```yaml
services:
  backend:
    ports:
      - "127.0.0.1:8385:8084"
```

The page displayed OpenAPI 3.2 and all English business groups. Using Swagger's controls, administrator sign-in returned a session token; Authorize sent it as HRD-Token; `/namespace/getRoot` returned the owned root. `/photo/insert` uploaded one generated JPEG and returned `{ "t": 1, "msgCode": 200 }`. `/photo/inserts` uploaded two generated JPEGs through separate file selectors with `photosStr` and returned `{ "t": 2, "msgCode": 200 }`. Chrome's extension lacked file-URL permission, so native system file pickers completed the uploads without changing extension permissions. A page reload cleared authorization. There were no Swagger application errors; an unrelated Grammarly extension logged its own error.

The test stack was removed with `docker compose -p photoly-openapi-check -f compose.test.yml down -v --remove-orphans`. Only disposable test data was removed. The normal local stack remains running with healthy MySQL, Redis and backend; `/docs/` and `/openapi.json` are available at **http://localhost:8384** and Vue remains at **http://localhost:5200**. Existing production MySQL and upload volumes were retained. No production upload, delete or stop action was used for this acceptance.
