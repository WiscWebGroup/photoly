# Photoly Flask backend

Python 3.12 / Flask 3.1 / SQLAlchemy 2. The 90 legacy business routes are preserved. AI features are outside this version.

## Structure

```text
app/
  __init__.py       create_app(config): configuration, extensions, blueprints, errors
  controllers/     HTTP parsing, legacy response families, file responses
  services/        Business rules, ownership, transactions; no Flask request dependency
  entities/        Explicit ORM mappings of the eleven existing tables
  docs/            Public Swagger UI, local vendor assets and eight API definition modules
  infrastructure/  Redis sessions, filesystem compensation, EXIF, QR, process lifecycle
  serialization.py Entity camelCase and legacy result envelopes
  cli.py           Idempotent initialization and dependency readiness
tests/
  contracts/       Frozen Spring route signatures, entity fields and Java types
  contract_cases.py Independent request/response expectations for all 90 routes
  test_contract.py Contract and unauthenticated-response checks
  test_business.py Business, permissions, media, rollback and metadata tests
  test_http.py     Disposable Gunicorn HTTP and shutdown acceptance tests
  test_openapi.py  OpenAPI 3.2 validation, coverage, examples and offline documentation
```

`create_app()` has no database, Redis or filesystem initialization side effects. `Infrastructure` is injected into services; tests replace the Redis client and lifecycle implementation. ORM sessions are request scoped and cleaned up by Flask-SQLAlchemy. Mutations commit in the Service layer. MySQL uses READ COMMITTED plus per-user row locks to serialize concurrent tree mutations after authentication.

## Local setup

Install `uv`, Python 3.12 (uv can download it), ExifTool, MySQL and Redis. Configure:

```sh
export DATABASE_URL='mysql+pymysql://photoly:password@localhost:3306/photoly?charset=utf8mb4'
export REDIS_URL='redis://localhost:6379/14'
export UPLOAD_FOLDER="$PWD/uploads"
export PUBLIC_BASE_URL='http://localhost:8084'
export ADMIN_EMAIL='admin@example.com'
export ADMIN_PASSWORD='your-initial-admin-password'
uv sync --python 3.12
uv run flask --app app:create_app wait-ready
uv run flask --app app:create_app init-db
uv run flask --app app:create_app run --port 8084
```

The Flask development server is for development only. Docker starts Gunicorn (2 workers, 4 threads each by default). Runtime configuration is read from environment variables; the standalone Flask process does not implicitly load the repository `.env`. Compose does.

`init-db` creates missing tables and defaults without dropping data, creates the configured administrator if absent, and ensures its root directory and default gallery exist. Running it again does not change existing passwords or settings. New user passwords use Werkzeug scrypt. The database SQL file is a fresh-schema reference, not an upgrade script; the CLI is the authoritative initializer.

## Interactive API documentation

Open `/docs/` (or `/docs`) on the backend, for example <http://localhost:8384/docs/> in Docker or <http://localhost:8084/docs/> with the development server. `/openapi.json` serves OpenAPI **3.2.0** for all **90 business operations plus `/healthz`**. Reading the documentation requires no login, database connection or Redis connection. Actual API calls retain their existing authentication and business behavior.

1. Expand **User → POST /user/signIn**, select **Try it out**, replace the fictitious example with your account credentials, and execute.
2. Copy the returned `token` into **Authorize → SessionToken**, without a `Bearer` prefix. The UI sends `HRD-Token` to header-authenticated operations and keeps it in memory only.
3. Execute **Namespace → GET /namespace/getRoot** to obtain your folder's `nsId`.
4. For **Photo → POST /photo/insert**, select a local file and set `photo` to a JSON string such as `{"nsId":123,"visibility":0}` using your actual folder ID. For `/photo/inserts`, use **Add string item** to add file selectors, choose one file per item, and set `photosStr` to an array with one metadata object per file, in matching order.

Requests go to the current backend origin. Login tokens in paths/queries, photo share tokens, and C/R/D API credentials must be entered in their separate parameter fields; they are not interchangeable with Authorize. The docs preserve legacy HTTP 200 business errors, response envelopes, raw Cred responses, nullable fields, offset-free timestamps, total-page semantics, binary downloads and video ranges. Read `msgCode` as well as HTTP status. Destructive actions, including deleting accounts/files and stopping the entire backend, are labeled explicitly.

Swagger UI **5.32.0** JS/CSS, Apache license and NOTICE are bundled under `app/docs/static/vendor`; there are no runtime CDN or online validator requests. `SOURCE.json` records the official npm tarball, its verified SHA-512 integrity and per-file SHA-256 checksums. The version's [official compatibility table](https://github.com/swagger-api/swagger-ui/blob/v5.32.0/README.md) includes OpenAPI 3.2.

Definitions in `app/docs/definitions/` are documentation only; shared schemas live in `app/docs/schemas.py`. They do not inspect test fixtures or add request validation/serialization. When changing an API, update its module and run the suite: the independent Spring inventory, real success/authentication responses, schema examples and `openapi-spec-validator==0.9.0` all check for drift. Development validators are excluded from the production image.

To update an existing local Docker installation while preserving database and upload volumes:

```sh
docker compose up -d --build backend web
```

## Configuration

| Variable | Default / meaning |
|---|---|
| `DATABASE_URL` | SQLAlchemy MySQL/PyMySQL URL; deployment supplies credentials |
| `REDIS_URL` | Redis connection URL, including database number |
| `UPLOAD_FOLDER` | `/photoly`; writable by the service user |
| `PUBLIC_BASE_URL` | `http://localhost:8384`; externally reachable URL for sharing |
| `TIMEZONE` | `America/New_York`; dates remain offset-free ISO strings on entity APIs |
| `ADMIN_EMAIL` | `admin@a.com` |
| `ADMIN_PASSWORD` | Required for initialization; never logged or stored in plaintext |
| `MAX_CONTENT_LENGTH` | `629145600` bytes (600 MiB per HTTP request) |
| `EXIFTOOL_PATH` | `exiftool` |
| `CORS_ORIGINS` | `*`, or a comma-separated allowlist |
| `GUNICORN_WORKERS` / `GUNICORN_THREADS` | `2` / `4` |
| `GUNICORN_TIMEOUT` | `120` seconds |
| `GUNICORN_PID_FILE` | Set by Docker to the master PID file used by `/admin/stop` |

Redis contains expiring login sessions. API credentials and photo sharing tokens stay in MySQL. Losing Redis sessions requires users to sign in again and does not invalidate media shares or API credentials.

## Storage and transactions

Originals: `<UPLOAD_FOLDER>/<userUUID>/<photoUUID>.<format>`; thumbnails add `_thumbnail`; avatars use `<userUUID>.jpg`. Folder moves only update database links. ZIP downloads stream from independent temporary archives, preserving logical folder structure; duplicate names receive deterministic suffixes instead of overwriting files.

Uploads and deletions enlist filesystem changes in the database transaction: new files are removed on rollback, and deletions are quarantined and restored if commit fails. User-level database locks coordinate concurrent requests. This compensates ordinary failures, but is not a distributed transaction across a database and filesystem: an abrupt host/process crash may leave files in `.staging`. Do not purge that directory blindly after a crash; reconcile it with the database first. Back up both MySQL and the media volume.

## Validation

```sh
uv run pytest -q --cov=app --cov-report=term-missing
```

Local tests use isolated SQLite (with foreign keys enabled), fakeredis and temporary media directories. The suite uses real Pillow/ExifTool/QR decoding, not mocked media processing. `test_http.py` is excluded by default.

Use the repository's `compose.test.yml` for the same suite against MySQL 8.3 / Redis 7.2 and for HTTP tests against the production Gunicorn image. Integration fixtures refuse to reset databases without a `_test` suffix or Redis databases other than DB 15. HTTP tests only target `http://backend:8084` inside the disposable Compose network, and their final test stops that backend.

The root Jenkinsfile builds images, runs integration and HTTP tests, cleans up the test stack, and supports opt-in deployment using a supplied production environment file.

See [the recorded migration acceptance results](VERIFICATION.md) and [the compatibility differences](COMPATIBILITY.md).
