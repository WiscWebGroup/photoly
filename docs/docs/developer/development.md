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

The server uses Go 1.24 in `photoly_server`.

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
