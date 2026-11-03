#!/bin/sh
# Disposable AIO browser test. Never attaches the developer's media or database volumes.
set -eu
repo=$(CDPATH= cd -- "$(dirname "$0")/../../.." && pwd)
name=photoly-maintenance-e2e
if docker inspect "$name" >/dev/null 2>&1; then
  echo "The dedicated test container $name already exists; stop/remove it before running this harness." >&2
  exit 1
fi
docker build --target aio --build-context "dbinit=$repo/database/postgres-init" \
  -t photoly-backup-browser "$repo/photoly_server"
docker run -d --rm --name "$name" -p 127.0.0.1:18384:8084 \
  --mount type=volume,destination=/photoly \
  --mount type=volume,destination=/backups \
  --mount type=volume,destination=/var/lib/postgresql/data \
  -e POSTGRES_PASSWORD=maintenance-root-test-only \
  -e POSTGRES_APP_PASSWORD=maintenance-app-test-only \
  -e POSTGRES_MIGRATION_PASSWORD=maintenance-migration-test-only \
  -e ADMIN_EMAIL=admin@test.local -e ADMIN_PASSWORD=test-password-only \
  -e STORAGE_SECRET_KEY=maintenance-e2e-key photoly-backup-browser >/dev/null
trap 'docker stop "$name" >/dev/null 2>&1 || true' EXIT INT TERM
ready=false
for attempt in $(seq 1 90); do
  if docker exec "$name" photoly health >/dev/null 2>&1; then ready=true; break; fi
  sleep 1
done
if [ "$ready" != true ]; then echo "AIO test instance did not become healthy" >&2; exit 1; fi
cd "$repo/photoly_frontend_vue"
pnpm exec playwright test -c playwright.backup.config.ts
