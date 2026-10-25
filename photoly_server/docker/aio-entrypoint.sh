#!/bin/bash
# PostgreSQL and photoly in one container. The stock postgres entrypoint initialises the data
# directory on first start (running /docker-entrypoint-initdb.d) and then runs the server.
set -eu
docker-entrypoint.sh postgres &
pg=$!
trap 'kill -TERM "$pg" "${app:-}" 2>/dev/null || true' TERM INT

# TCP only answers once the final server is up; the init-time server listens on a socket.
until pg_isready -q -h 127.0.0.1 -U postgres -d "$POSTGRES_DB"; do
  kill -0 "$pg" 2>/dev/null || exit 1
  sleep 1
done
gosu nonroot photoly serve &
app=$!

# Either process ending takes the container down, so Docker restarts both together.
wait -n "$pg" "$app" || true
kill -TERM "$pg" "$app" 2>/dev/null || true
wait "$pg" 2>/dev/null || true
wait "$app" 2>/dev/null
