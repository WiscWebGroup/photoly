#!/bin/sh
# Never points at a developer's database: this harness owns its isolated container/network.
set -eu
repo=$(CDPATH= cd -- "$(dirname "$0")/../../.." && pwd)
name="photoly-backup-test-$$"
cleanup() {
  docker rm -f "$name-db" >/dev/null 2>&1 || true
  docker network rm "$name" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM
docker build -t photoly-backup-tests -f "$repo/photoly_server/tests/backup/Dockerfile" "$repo/photoly_server/tests/backup"
docker network create "$name" >/dev/null
docker run -d --name "$name-db" --network "$name" -e POSTGRES_PASSWORD=backup-test-only pgvector/pgvector:0.8.7-pg17-bookworm >/dev/null
for attempt in $(seq 1 60); do
  if docker exec "$name-db" pg_isready -U postgres >/dev/null 2>&1; then break; fi
  sleep 1
done
docker run --rm --network "$name" --tmpfs /capacity:rw,size=2m -e BACKUP_TEST_SMALL_VOLUME=/capacity \
  -e "BACKUP_TEST_DATABASE_URL=postgres://postgres:backup-test-only@$name-db:5432/postgres" \
  -e "BACKUP_SCALE_FILES=${BACKUP_SCALE_FILES:-2000}" \
  -e "BACKUP_SCALE_BYTES=${BACKUP_SCALE_BYTES:-8192}" \
  -v "$repo/photoly_server:/src" \
  -v photoly-backup-go-cache:/go \
  -v photoly-backup-build-cache:/root/.cache/go-build \
  photoly-backup-tests
