#!/bin/sh
set -eu
mkdir -p "$UPLOAD_FOLDER"
if [ "$(id -u)" = "0" ]; then
    chown photoly:photoly "$UPLOAD_FOLDER"
    exec gosu photoly "$0" "$@"
fi
flask --app app:create_app wait-ready
alembic upgrade head
flask --app app:create_app seed-db
flask --app app:create_app reconcile-storage
