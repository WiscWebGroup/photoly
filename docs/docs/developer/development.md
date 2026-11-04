---
title: Development and tests
sidebar_position: 1
---

## Setup

### Frontend

1. Start the dev services:

   ```sh
   docker compose -f compose.dev.yml up -d --build
   ```

2. Run `pnpm dev` in `photoly_frontend_vue`.

See the frontend README in the repository.

### Server

Use the Go toolchain declared by `photoly_server/go.mod` (currently Go 1.26.10). Older local Devbox entries do not override the module requirement.

`go test ./...` needs:

- `TEST_DATABASE_URL` for database-backed tests
- The ONNX Runtime library for the AI worker test

### Database access

Photoly uses sqlc.

1. Edit `photoly_server/internal/db/queries/*.sql`.
2. Run `devbox run -- sqlc generate` in `photoly_server`.

CI fails if `internal/db/gen` is stale.

## Server checks

Run these in `photoly_server`:

```sh
sqlc generate
go vet ./...
go test -skip TestWorker ./...
```

| Variable | Use |
| --- | --- |
| `TEST_DATABASE_URL` | A fresh migrated database. Runs source, scan, quota and API integration tests. |
| `PHOTOLY_TEST_DATABASE_URL` | Legacy user and catalog suites. |

:::danger
The legacy suites **reset the public schema** of `PHOTOLY_TEST_DATABASE_URL`. Use a separate disposable database.
:::

`TestWorker` also needs ONNX Runtime.

## Storage tests

S3 contract and lifecycle tests use a disposable bucket on an S3-compatible server:

- `PHOTOLY_TEST_S3_ENDPOINT`
- `PHOTOLY_TEST_S3_BUCKET`
- `PHOTOLY_TEST_S3_ACCESS_KEY`
- `PHOTOLY_TEST_S3_SECRET_KEY`

WebDAV tests run a real Nextcloud write probe:

- `PHOTOLY_TEST_WEBDAV_URL`
- `PHOTOLY_TEST_WEBDAV_USERNAME`
- `PHOTOLY_TEST_WEBDAV_PASSWORD`

Use a dedicated test account and directory. If the server is private, use an allowed origin. The protocol fixture also tests conditional conflicts, unsafe hrefs, partial listings and transaction compensation.

SMB tests:

- Set `SMB_FIXTURE_ROOT` to use a disposable Samba share.
- For a real NAS, set `SMB_TEST_ENV_FILE` to a protected local file with `user=` and `password=`. Then run `TestSMBIntegration` and `TestNASMediaIndex`.
- The tests create unique child directories, leave existing files alone and remove their fixtures.

:::warning
Never commit the credential file.
:::

## Frontend checks

```sh
pnpm typecheck
pnpm exec vitest run
pnpm exec playwright test e2e/storage-sources.spec.ts e2e/folder-tiles.spec.ts
```

## OpenAPI contract checks

The Go integration suite records the actual ServeMux registrations, including loop-generated aliases, and compares normalized method/path pairs in both directions. `/docs`, `/docs/assets/` and `/openapi.json` are the explicit infrastructure exclusions. The maintenance controller predicates are extracted from its dispatcher source and have independent response tests. Neither the route check nor the schema checker regenerates the contract.

From the repository root, install the checker in a virtual environment:

```sh
python3 -m venv /tmp/photoly-openapi-venv
/tmp/photoly-openapi-venv/bin/pip install -r scripts/requirements-openapi.txt
/tmp/photoly-openapi-venv/bin/python scripts/check-openapi.py
```

With `TEST_DATABASE_URL` pointing at a disposable migrated database, run in `photoly_server`:

```sh
OPENAPI_SAMPLE_OUTPUT=/tmp/openapi-samples.json OPENAPI_BACKUP_SAMPLE_OUTPUT=/tmp/openapi-backup-samples.json go test ./internal/server ./internal/backup -run TestOpenAPI -count=1
```

Then run the checker from the repository root with `--samples /tmp/openapi-samples.json --samples /tmp/openapi-backup-samples.json`. It checks references, operation IDs, path parameters, documented examples and actual JSON response samples. Server CI runs both stages and checks that the credential/resource generator reproduces the committed contract. The samples cover all session-authenticated operations without a session, account/admin GETs and targeted success/error flows; they do not prove every combination of request data or model inference.

To check the actual bundled Swagger UI, start an isolated backend, then run in `photoly_frontend_vue`:

```sh
PHOTOLY_SWAGGER_URL=http://127.0.0.1:18484 pnpm exec playwright test e2e/swagger.spec.ts --project=chromium --workers=1
```

The browser test fetches the real UI/spec/assets and intercepts operation execution with synthetic replies. Real backend contracts are checked separately by the Go tests. It verifies authorization, GET/POST/PUT/PATCH/DELETE execution and clearing credentials on reload.

## Reproduce documentation screenshots

Run `pnpm docs:screenshots` in `photoly_frontend_vue` after installing dependencies and Playwright Chromium. This uses the real local Vue interface at a fixed 1440 × 1000 viewport, English locale and UTC time zone. It writes the seven public PNGs in `docs/screenshots`.

The capture suite uses an isolated browser context, fictional account/folder/gallery/backup data, generated landscape illustrations, schematic map tiles and the project's synthetic video fixture. It intercepts backend calls and blocks external requests; it never signs in to a real account. Captions identify synthetic content. Do not replace these fixtures with production data or personal media.

Before committing new captures, check names, paths, coordinates, passwords, tokens and image metadata, then build Docs with `pnpm --dir docs build`. Review the rendered pages on desktop and mobile. The existing welcome image contains bundled demonstration assets; the two point-note images use a synthetic landscape and sample notes.

For the reviewed differences and validation boundaries, see [API and documentation audit](api-documentation-audit.md).
