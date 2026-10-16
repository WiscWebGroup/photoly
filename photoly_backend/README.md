# Photoly Flask backend

Python 3.12 / Flask 3.1 / SQLAlchemy 2 / PostgreSQL 17 / pgvector 0.8.7. The 90 legacy business routes and image replacement are preserved. Phase 2 adds isolated AI services and search/admin APIs; the UI is scheduled for phase 3.

## Architecture

`create_app()` registers configuration, infrastructure and blueprints without connecting to dependencies or initializing data. Controllers parse HTTP; services enforce ownership and transactions; entities describe eleven business tables. PostgreSQL READ COMMITTED and per-user row locks serialize directory mutations. The storage layer compensates ordinary database failures; it is not an atomic transaction across PostgreSQL and the filesystem. After a process/host crash, reconcile `.staging` with the database instead of blindly deleting it. Back up PostgreSQL and media together.

Redis stores expiring login sessions; API credentials and share tokens live in PostgreSQL. Redis loss signs users out without changing share/API tokens.

## Bootstrap, schema and roles

Root Compose pins the database image by digest and publishes no PostgreSQL port. On an empty volume, `database/postgres-init/01-photoly.sh` installs `vector` and creates:

- `postgres`: bootstrap superuser, only in the database service.
- `photoly_migrator`: owns business tables/indexes and performs Alembic DDL.
- `photoly`: application DML and sequence usage; cannot create schemas/tables/temporary tables, assume the migration role or modify Alembic revision metadata.

The one-shot `migrate` service runs Alembic using the schema owner and then seeds defaults using app credentials. Separate advisory locks serialize migrations and seeds. Web startup only waits for dependencies and checks schema/extension compatibility. It receives neither migration nor initial administrator passwords.

`seed-db` and its compatibility alias `init-db` require an already migrated database. Repeated seed/startup preserves passwords/settings. Bootstrap passwords apply to new volumes; editing `.env` does not rotate roles in an existing database. No MySQL data import tool is provided. The old MySQL SQL file is historical reference only; Alembic is authoritative.

## Running

Prefer the root Compose instructions. For separately provisioned PostgreSQL with the same bootstrap roles/grants, install Python 3.12, uv and ExifTool, then:

```sh
export POSTGRES_HOST=localhost
export POSTGRES_DB=photoly
export POSTGRES_APP_PASSWORD='your-application-password'
export POSTGRES_MIGRATION_PASSWORD='your-migration-password'
export REDIS_URL='redis://localhost:6379/14'
export UPLOAD_FOLDER="$PWD/uploads"
export ADMIN_EMAIL='admin@example.com'
export ADMIN_PASSWORD='your-initial-admin-password'
uv sync --frozen --python 3.12
uv run alembic upgrade head
uv run flask --app app:create_app seed-db
uv run flask --app app:create_app check-schema
uv run flask --app app:create_app run --port 8084
```

Standalone commands read exported environment variables; Compose reads the root `.env`. The Flask server is for development. Docker runs Gunicorn with two workers/four threads by default.

For subsequent application upgrades with existing PostgreSQL/media volumes:

```sh
docker compose build backend migrate web
docker compose run --rm migrate
docker compose up -d backend web
```

A failed migration must be resolved before starting the new Web image; the schema check rejects mismatched revisions. Do not use `down -v` on the application project to upgrade it.

## Configuration

| Variable | Meaning |
| --- | --- |
| DATABASE_URL | Optional postgresql+psycopg application URL override |
| MIGRATION_DATABASE_URL | Optional schema-owner URL override, Alembic only |
| POSTGRES_HOST / POSTGRES_PORT / POSTGRES_DB | localhost / 5432 / photoly outside Compose |
| POSTGRES_APP_PASSWORD | Restricted app role password |
| POSTGRES_MIGRATION_PASSWORD | Schema-owner password, migration service only |
| POSTGRES_ADMIN_PASSWORD | Bootstrap password, database only |
| REDIS_URL | Redis URL and database number |
| UPLOAD_FOLDER | Writable media directory, default /photoly |
| PUBLIC_BASE_URL | External sharing URL, default http://localhost:8384 |
| TIMEZONE | America/New_York; entity dates remain offset-free |
| ADMIN_EMAIL / ADMIN_PASSWORD | Initial admin seed; password required for seed only |
| MAX_CONTENT_LENGTH | 629145600 bytes by default |
| EXIFTOOL_PATH | exiftool |
| CORS_ORIGINS | * or comma-separated allowlist |
| GUNICORN_WORKERS / GUNICORN_THREADS | 2 / 4 |
| GUNICORN_TIMEOUT | 120 seconds |
| GUNICORN_PID_FILE | Docker-managed master PID used by admin stop |

PostgreSQL password values use SQLAlchemy URL objects, so reserved characters need no manual escaping with the separate variables. Explicit URL overrides must be correctly encoded. Redis credentials still enter a URL.

## API documentation and compatibility

`/docs/` provides local Swagger UI, `/openapi.json` serves OpenAPI 3.2 for legacy photo operations, image replacement, PostgreSQL health, AI search and AI administration. Reading docs needs no dependency connection. Actual requests retain authentication and ownership checks.

Sign in with POST /user/signIn, then put its token into SessionToken without a Bearer prefix. Tokens remain in memory, not localStorage. Share/API/path tokens are separate. Upload multipart metadata uses JSON strings. Read msgCode as well as HTTP status; legacy business failures commonly return HTTP 200. Destructive operations are explicitly labeled.

Swagger UI 5.32.0 assets and licenses are bundled. SOURCE.json retains official integrity checksums; Git attributes preserve their LF bytes on Windows. No runtime CDN or validator request is needed.

GET /admin/pingDatabase tests PostgreSQL. GET /admin/databaseInfo reports type, server version, address, pgvector version and schema revision. /admin/pingMySQL is a legacy alias; /admin/getAddress keeps its mysql key containing the PostgreSQL address. The administrator dashboard displays PostgreSQL, pgvector and schema revision.

## Validation

Run root compose.test.yml. There is no SQLite database fallback. Alembic initializes photoly_test; fixture cleanup uses the owner, while business requests use the app role. Explicit fixture IDs are followed by identity sequence resets.

TEST_DATABASE_URL and TEST_MIGRATION_DATABASE_URL must point to the same PostgreSQL database ending in _test; Redis integration uses DB 15. HTTP tests use separate photoly_http_test and Redis DB 14, no production mounts and no host ports. Their final test intentionally stops the disposable server.

Tests retain frozen Spring response contracts and independently inspect PostgreSQL schema, constraints, identity behavior, grants, concurrent email uniqueness, schema readiness, halfvec(2048) cosine error, HNSW use, zero-vector exclusion and rollback. Default CI uses a deterministic AI adapter; real model verification runs separately.

See [compatibility notes](COMPATIBILITY.md) and the test commands above. Historical verification notes remain in VERIFICATION.md.


## AI backend

See [deployment and API operations](../docs/ai-backend-operations.md) for CPU/CUDA images, model lifecycle, user eligibility, search contracts, capacity and recovery. AI starts disabled. The AI Management page controls eligibility, models, scheduling and resource limits; the photo viewer provides paginated similar-image recommendations with an optional administrator-configured similarity threshold.
