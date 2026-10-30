# Uploads, groups and storage limits

Photoly accepts files, directory selection/drag-and-drop, and ZIP/ZIP64 imports. The selected outer folder and every media-containing subdirectory are preserved (for example, `旅行/北京/photo.jpg` becomes `destination/旅行/北京/photo.jpg`); unsupported non-media entries are reported as skipped. Empty directories are not created. Nested archives are not recursively expanded. A colliding top-level folder gets a numeric suffix instead of merging with existing content.

The browser keeps an application-wide queue with two concurrent raw-file requests. There is no batch file-count or total-byte cap. `MAX_CONTENT_LENGTH` still bounds each file/ZIP request (600 MiB by default); manifest pages contain up to 500 entries and can be repeated without a batch limit. MIME-less camera files are recognized by extension. Closing the picker, closing the activity panel, or navigating within Photoly does not interrupt sending. Closing/reloading the browser loses unsent local file handles: reopen Uploads and reselect matching paths and sizes. There is no byte-range resume. Files already received continue on the server.

The Upload button beside the desktop search control (and in mobile/admin headers) opens private activity and history. Counts distinguish pending transmission, received work, processing, completion, skips and failures. Details page through results and link to their library folders. Retry retains successful imports; Cancel remaining does not delete imported photos. Sign-out/account change aborts local sending and clears account-specific UI state.

**Clear activity** removes one finished batch from the list; **Clear finished** clears finished, failed, skipped and cancelled batches across all pages. Active uploads remain visible: use **Cancel remaining** first if you want to stop one. Clearing activity does not delete imported photos. Idempotency records and temporary sources follow normal retention, so repeat submissions cannot recreate copies merely because history was cleared.

**Dismiss failure alerts** clears the red failure indicator while leaving failed items available for inspection and retry. Acknowledgements are stored per account on the server and survive reloads and other devices. New failures raise the indicator again. Browser transmission errors can also be dismissed without cancelling the upload.

## Deployment and synchronization

The Go server runs the upload worker and local directory scanner in the same process.
See [real folders and personal S3](real-folders.md) before deploying this version:
old flat-layout libraries cannot be upgraded in place. `photoly migrate` applies
the schema for a fresh library; `photoly serve` also migrates on startup.

`MAX_CONTENT_LENGTH` bounds each request (600 MiB by default),
`UPLOAD_ZIP_MAX_BYTES` bounds expanded ZIP content (50 GB), and
`UPLOAD_DISK_RESERVE_BYTES` reserves free disk space (512 MiB). Temporary sources
are retained according to `UPLOAD_TEMP_HOURS` (24), activity according to
`UPLOAD_HISTORY_DAYS` (30). The server checks ZIP paths, entry types, size and quota.

Sources live in `.uploads`; transaction backups live in `.staging`. Normal failures
compensate file mutations. PostgreSQL and filesystem/remote writes are not one
distributed transaction: after a host crash, inspect backups and reconcile the
library before removing orphaned files. Do not delete active upload source files.

## Groups and quotas

User Management has Users and Groups tabs. Each account belongs to one group. The group's Max Storage is **per member**, not a shared pool. The protected default group starts at **50 GB (50,000,000,000 bytes)** and receives new accounts. Administrators can rename it or change its quota. Deleting another group requires a replacement group for all its members.

A personal override can specify a finite limit or unlimited storage; turning the override off restores inheritance. Limits include all local user files, local derivatives and avatars. Personal S3 objects, remote cache, temporary sources, PostgreSQL and AI indexes are excluded. Administrators obey their own effective limits. Changing group membership/limits is serialized with file mutations; concurrent uploads cannot overspend a user's limit. Reducing a quota retains existing files; an over-quota account can read/download/delete and replace content when this does not increase usage.

Legacy photo and credential upload routes remain compatible and enforce the same quotas. New upload endpoints use the existing `{msgCode,t}` envelope: inspect `msgCode` even for HTTP 200. `/uploads` creates/lists batches, `/{batch_id}/items` registers/lists entries, `/{batch_id}/items/{item_id}/content` receives raw bytes, and `finish`, `cancel`, `retry` operate on a batch. Batch/item UUIDs make retries idempotent. All upload queries/actions require the owning login session. Administrative group and personal quota operations are documented in `/docs/`; `/user/getStorage` now reports the effective account limit (`null` means unlimited), its source and over-quota state.

Upload batch and item IDs use UUID v4. Browsers without `crypto.randomUUID()` (including ordinary LAN HTTP origins) use cryptographically secure `crypto.getRandomValues()` instead. No HTTPS exception or browser security override is needed for generating upload IDs.
