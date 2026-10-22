# Uploads, groups and storage limits

Photoly accepts files, directory selection/drag-and-drop, and ZIP/ZIP64 imports. The selected outer folder and every media-containing subdirectory are preserved (for example, `旅行/北京/photo.jpg` becomes `destination/旅行/北京/photo.jpg`); unsupported non-media entries are reported as skipped. Empty directories are not created. Nested archives are not recursively expanded. A colliding top-level folder gets a numeric suffix instead of merging with existing content.

The browser keeps an application-wide queue with two concurrent raw-file requests. There is no batch file-count or total-byte cap. `MAX_CONTENT_LENGTH` still bounds each file/ZIP request (600 MiB by default); manifest pages contain up to 500 entries and can be repeated without a batch limit. MIME-less camera files are recognized by extension. Closing the picker, closing the activity panel, or navigating within Photoly does not interrupt sending. Closing/reloading the browser loses unsent local file handles: reopen Uploads and reselect matching paths and sizes. There is no byte-range resume. Files already received continue on the server.

The Upload button beside the desktop search control (and in mobile/admin headers) opens private activity and history. Counts distinguish pending transmission, received work, processing, completion, skips and failures. Details page through results and link to their library folders. Retry retains successful imports; Cancel remaining does not delete imported photos. Sign-out/account change aborts local sending and clears account-specific UI state.

## Deployment and upgrades

Back up PostgreSQL and media together. Stop writers before migrating and reconciling existing usage:

```sh
docker compose stop backend upload-worker ai-scheduler ai-maintenance
docker compose build
docker compose run --rm migrate
docker compose up -d
```

Migration `0006_uploads_storage` creates the default group, assigns existing accounts, and adds durable batch/item tables. `migrate.sh` then initializes usage counters from originals, thumbnails and avatars. The one-time scan can take time on a large existing library. Restarted migrations preserve administrator changes and do not recount initialized accounts. The `upload-worker` service starts by default without any AI profile. Its database credentials have only application DML grants.

| Setting | Default | Meaning |
| --- | --- | --- |
| `MAX_CONTENT_LENGTH` | 629145600 | Maximum bytes in one request; also maximum extracted media entry size |
| `UPLOAD_ZIP_MAX_BYTES` | 50000000000 | Maximum total declared uncompressed bytes in a ZIP; actual entry bytes are checked while streaming |
| `UPLOAD_DISK_RESERVE_BYTES` | 536870912 | Free disk space reserved while receiving/extracting |
| `UPLOAD_TEMP_HOURS` | 24 | Expire inactive incomplete transmissions and retained source files |
| `UPLOAD_HISTORY_DAYS` | 30 | Retain inactive completed upload history |

Use positive retention values and a nonnegative reserve. ZIP processing rejects encrypted entries, absolute/traversing/invalid paths, symbolic links and other special files. Entry sizes, quota and free space remain enforced during extraction. The default 50 GB ZIP expansion budget is an operational protection independent of an account's quota.

The worker uses PostgreSQL session advisory locks to prevent duplicate processing. A worker process exiting releases its claim so another worker can reclaim a `processing` item. Photo creation and item completion share the same transaction; normal failures compensate filesystem writes. As with existing Photoly writes, PostgreSQL and the filesystem are not a distributed atomic transaction: after a host crash, inspect transaction `.staging` directories and reconcile orphaned media before deleting anything. Received sources live in `.uploads`, outside account usage. Inactive temporary files are swept every five minutes; active processing sources are retained. Failures are logged with the item UUID. `.upload-worker.json` records the last worker-cycle heartbeat; it can be older during a long file operation.

For an intentional recount after repairing or manually changing stored files:

```sh
docker compose exec backend flask --app app:create_app reconcile-storage --all-users
```

## Groups and quotas

User Management has Users and Groups tabs. Each account belongs to one group. The group's Max Storage is **per member**, not a shared pool. The protected default group starts at **50 GB (50,000,000,000 bytes)** and receives new accounts. Administrators can rename it or change its quota. Deleting another group requires a replacement group for all its members.

A personal override can specify a finite limit or unlimited storage; turning the override off restores inheritance. Limits include originals, thumbnails and avatars, not temporary upload sources, PostgreSQL or AI indexes. Administrators obey their own effective limits. Changing group membership/limits is serialized with file mutations; concurrent uploads cannot overspend a user's limit. Reducing a quota retains existing files; an over-quota account can read/download/delete and replace content when this does not increase usage.

Legacy photo and credential upload routes remain compatible and enforce the same quotas. New upload endpoints use the existing `{msgCode,t}` envelope: inspect `msgCode` even for HTTP 200. `/uploads` creates/lists batches, `/{batch_id}/items` registers/lists entries, `/{batch_id}/items/{item_id}/content` receives raw bytes, and `finish`, `cancel`, `retry` operate on a batch. Batch/item UUIDs make retries idempotent. All upload queries/actions require the owning login session. Administrative group and personal quota operations are documented in `/docs/`; `/user/getStorage` now reports the effective account limit (`null` means unlimited), its source and over-quota state.

Upload batch and item IDs use UUID v4. Browsers without `crypto.randomUUID()` (including ordinary LAN HTTP origins) use cryptographically secure `crypto.getRandomValues()` instead. No HTTPS exception or browser security override is needed for generating upload IDs.
