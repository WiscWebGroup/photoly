# Real folders and personal S3 storage

The local library is a real directory tree. `/root/Trips/photo.jpg` maps to
`<UPLOAD_FOLDER>/<user_uuid>/Trips/photo.jpg`. Names are preserved; an upload,
rename or move that encounters an existing name gets a numeric suffix, such as
`photo (1).jpg`. Existing files are never overwritten by a naming collision.
Directory moves move the whole directory, including files Photoly does not list.

Photoly keeps stable database folder and photo IDs for tags, albums, sharing and
metadata. Local paths are resolved from the storage source and folder tree.
Thumbnails, previews and avatars live in `.assets/<user_uuid>/`; expendable S3
and WebDAV cache lives in `.remote/<user_uuid>/<source_id>/`. Neither directory appears in
the library. Back up the database, user directories and `.assets` together.

## Upgrading from the old flat layout

Migration `00008_real_folders` refuses a database containing old-layout media or
non-root virtual folders. It does not move or delete them. There is no
old-layout media migration or schema downgrade. Keep the old deployment and its
matching database/media backup; initialize this version with a **new database
and new upload directory**. Do not run `down -v` to upgrade an existing library.

The normal `photoly serve`/`photoly migrate` startup applies the schema and seeds
the default account. The application process must be able to create, rename and
delete files in the configured upload directory. Docker defaults to uid 65532.

## Automatic local synchronization

A scan runs at server startup, then every `STORAGE_SCAN_INTERVAL` (Go duration,
default `60s`). A file's size and modification time must match two consecutive
observations before new or changed content is processed. Large libraries can take
longer than the interval to scan. No filesystem watcher or S3 bucket scanner is
used. Empty directories are indexed; unsupported non-media files remain on disk
and are included in whole-directory operations.

The scanner reuses IDs for unambiguous inode matches. When filesystem identity is
unavailable, a unique content match between a missing photo and a new file can
retain its ID. Duplicate content is not merged. Content replacement at the same
path retains the photo ID, increments its content version and regenerates its
metadata and previews. An ambiguous move may be treated as a deletion and import.

Two successful complete scans must confirm a deletion before database records,
relationships and derivatives are removed. A missing/unreadable/replaced source
root or a failed scan never counts as a confirmed deletion. Files still being
written are retried. Scan status and errors appear in personal **Storage**
settings; open library views check content revisions every 15 seconds while idle.
Symbolic links are not followed or imported.

Local quota includes ordinary files in the user directory, media derivatives and
avatars. Files placed externally are still indexed if the account is over quota;
Photoly then blocks operations that increase local usage. Read, download, move
and delete remain available. Temporary uploads, archive scratch space and S3
cache are not counted toward the account quota.

## Personal S3

Each account can configure one S3-compatible connection in personal **Storage**
settings: endpoint, bucket, optional prefix/region and credentials. The server
administrator must set `STORAGE_SECRET_KEY` before credentials can be saved.
Access and secret keys are encrypted in the database and are never returned by
the settings API; leaving a saved credential field blank keeps its value.
Losing or changing the encryption key makes saved credentials unreadable.

The connection appears as a blue cloud folder named `S3` alongside local content.
If a local name collides, the mount label receives a numeric suffix. The mount
itself cannot be renamed, moved or deleted through folder actions. Disable it in
settings to hide it while retaining its records and remote files. Re-enable it
to restore access. An outage does not disable local storage.

Only media uploaded through Photoly is managed. S3 folders remain database
folders and objects use user/source/photo UUID keys. Renaming a folder or photo
does not copy S3 objects. Uploads wait for remote writes; a normal failure rolls
back the database and compensates completed writes. Batch source files remain
available for the existing retry workflow. This is not a distributed transaction
or a host-crash recovery system.

The endpoint, bucket or prefix cannot be changed while media or pending uploads
remain. Credential rotation is supported. S3 media and derivatives are stored
remotely and do not consume the local user quota. Cached downloads expire after
`STORAGE_CACHE_TTL` (default `24h`, `0` disables eviction); cache files in an active
write transaction are protected from eviction.

Cross-source moves, importing existing bucket objects and multiple S3 connections remain unsupported. Migration 9 adds [multiple external local mounts and WebDAV sources](external-storage.md) without resetting existing real-folder libraries.

## Downloads and APIs

Folder ZIPs preserve actual names, hierarchy, empty directories and non-media
files. Local archives exclude application assets and symbolic links. Remote
archives include original media and Live Photo original attachments. A `/root`
archive includes all enabled sources beneath their mount labels. Missing indexed
originals or read failures abort archive creation before a response is sent.
Archives are built in temporary disk space and removed after serving or
cancellation. The browser download manager handles the response without keeping
the whole ZIP in a JavaScript Blob. Folder deletion also removes non-media files;
the confirmation dialog explicitly says so.

Existing folder/photo IDs and UUID-based share URLs remain the API identifiers.
Folder responses add `sourceId`, `sourceType`, `isMount`, `canMove`, `canRename`
and `canDelete`; photo responses add `sourceId`. Folder creation, rename and move
return the resulting `{nsId, nsName}` so clients can display collision suffixes.
`POST /photo/update` and `/photo/changeNamespace` return the updated photo with
its final `photoName`.
Normal JSON responses retain `{msgCode,t}`; inspect `msgCode` for business errors.

- `GET /user/storage/sources`: own source revisions and scan status.
- `GET /user/storage/s3`: own public settings, fields and credential-presence flag.
- `POST /user/storage/s3/test`: test supplied settings, merging saved credentials.
- `PUT /user/storage/s3`: save settings or toggle `enabled`.

The global `/admin/storage` endpoints and `/user/updateStorageVisibility` have
been removed. Dates, search, galleries and tags include all enabled sources.

## Verification

Run `sqlc generate`, `go vet ./...` and `go test -skip TestWorker ./...` in
`photoly_server`. Set `TEST_DATABASE_URL` to a fresh migrated test database to run
source, scan, quota and API integration tests. The legacy user/catalog suites use
`PHOTOLY_TEST_DATABASE_URL` and **reset that database's public schema**; provide a
separate disposable database. `TestWorker` additionally needs ONNX Runtime.

S3 contract and lifecycle tests use `PHOTOLY_TEST_S3_ENDPOINT`,
`PHOTOLY_TEST_S3_BUCKET`, `PHOTOLY_TEST_S3_ACCESS_KEY` and
`PHOTOLY_TEST_S3_SECRET_KEY`. Use a disposable bucket on an S3-compatible server.
Frontend checks are `pnpm typecheck`, `pnpm exec vitest run` and
`pnpm exec playwright test e2e/storage-sources.spec.ts e2e/folder-tiles.spec.ts`.
