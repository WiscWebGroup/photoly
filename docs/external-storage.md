# Local mounts and WebDAV

Migration 9 upgrades an existing real-folder library in place. It does not move or delete media, rewrite migration 8, or reset the database. Back up the database and upload directory before deploying. Libraries predating real folders still need the fresh-library procedure documented in [real-folders.md](real-folders.md).

Administrators can bind multiple local directories to any user from **User Management → Storage → Local mounts**. Users manage multiple WebDAV directories from **Settings → Storage → WebDAV connections**. Each binding appears in `/root`, with an amber disk badge for local mounts, a purple network badge for WebDAV, or the existing blue S3 badge. Entry names are editable in settings; collisions receive numbered suffixes. Entries cannot be renamed, moved or deleted through ordinary folder actions.

## Deployment

Configure allowed roots as JSON. An empty value allows no external local roots:

```dotenv
STORAGE_LOCAL_ROOTS=[{"id":"family","name":"Family NAS","path":"/mnt/family"},{"id":"archive","name":"Archive disk","path":"/mnt/archive"}]
STORAGE_WEBDAV_ALLOWED_HOSTS=http://nextcloud,https://nas.example.internal:8443
STORAGE_WEBDAV_CA_FILE=/etc/photoly/private-ca.pem
STORAGE_SECRET_KEY=replace-with-a-long-random-secret
```

The WebDAV allowlist contains **exact origins**, including the scheme and non-default port; no wildcards. Public HTTP/HTTPS servers require no allowlist entry. Private, loopback and Docker service destinations require an explicit origin. DNS is validated again for each actual connection, and addresses are pinned for that connection. Link-local, cloud metadata and other special destinations remain blocked. Redirects are refused; supply the canonical full directory URL. Credentials are never forwarded to another server. HTTPS verifies certificates; optionally mount a private CA PEM bundle. There is no insecure TLS toggle.

Use the following override with `docker compose -f docker-compose.yml -f compose.mounts.yml up -d`. Replace the example host paths with existing directories and ensure UID 65532 has read, write and directory traversal access:

```yaml
# compose.mounts.yml
services:
  backend:
    environment:
      STORAGE_LOCAL_ROOTS: '[{"id":"family","name":"Family NAS","path":"/mnt/family"},{"id":"archive","name":"Archive disk","path":"/mnt/archive"}]'
      STORAGE_WEBDAV_ALLOWED_HOSTS: 'http://nextcloud'
    volumes:
      - type: bind
        source: /srv/nas/family
        target: /mnt/family
        bind:
          create_host_path: false
      - type: bind
        source: /srv/archive
        target: /mnt/archive
        bind:
          create_host_path: false
      # Optional private CA:
      # - /etc/photoly/private-ca.pem:/etc/photoly/private-ca.pem:ro
```

Configure each separately mounted filesystem as its own allowed root. Photoly never creates or recreates a binding root. The administrator's browser submits only root IDs and relative paths. Traversal, symlinks, and overlapping access to upload, internal assets, PostgreSQL or model directories are rejected. `PHOTOLY_DATA`, `PGDATA`, `POSTGRES_DATA_DIR`, `AI_MODEL_DIR` and `MODEL_DIR` also protect customized internal locations.

Keep `.photoly-system` reserved. Local transaction staging/backups live under this directory in the configured allowed root, on the same filesystem as the originals. WebDAV transaction backups use this directory in the bound remote root. It is excluded from scanning, ordinary operations and ZIP downloads. Do not remove active workspaces. On a failed compensation, backups are retained, a source error is recorded, and a scan is requested. This is compensation for ordinary failures, not a distributed transaction or crash recovery journal.

## Connections and permissions

A Nextcloud URL typically looks like `https://cloud.example/remote.php/dav/files/USERNAME/Photos/`. Use the username and an app password with Basic authentication. The connection test checks directory listing, creation, file writing/reading, conditional writes, rename and deletion using a temporary probe. The probe is removed afterwards. Username and password are encrypted using `STORAGE_SECRET_KEY`; neither is returned by settings APIs. Blank credentials keep their saved values. Keep the encryption key stable across restarts and back it up securely.

Bindings are read/write. Upload, replacement, folder creation, rename, moves, deletion, sharing and downloads use actual filenames and directories. Moves across sources are rejected, including between bindings of the same physical directory. S3 continues to use UUID object keys and is never scanned for existing bucket objects.

Repeated and nested bindings are allowed. Each owns an independent index, IDs, tags, albums and shares. File changes are physically shared and become visible to other bindings on their scans. Administrators see overlapping local bindings; normal users cannot browse the deployment's allowed roots. Deleting an ordinary folder removes **all** its contents, including hidden non-media files and contents used by other bindings.

Disabling hides the entry and keeps its index. Re-enabling requests a scan. **Detaching a binding or deleting an account preserves external local and WebDAV originals and non-media files**; only Photoly's index, associations, derivatives and cache are removed. A location cannot be retargeted: detach and create a new binding. There is no recursive remote wipe action in settings.

## Synchronization and quota

Sources are scanned on startup, binding and re-enabling, then every 60 seconds by default. Each external source can set a longer interval, with a minimum of 60 seconds. Three sources can scan concurrently; one source cannot overlap its own scan. The next interval starts when the previous scan finishes. Manual scan requests coalesce; **Verify all content** requests hashing of unchanged-looking media. The default Photoly source retains the deployment `STORAGE_SCAN_INTERVAL` setting (minimum 60 seconds).

Empty directories and supported media are indexed directly. New or changed files must appear stable in two observations and are checked again after processing. Local inode identity and optional WebDAV `DAV:resource-id` preserve IDs on moves. ETags describe versions, not identity. Without reliable identity, only an unambiguous missing/new content-hash match preserves an ID. Duplicate content remains separate. Same-path replacement increments the content version, refreshes derivatives and invalidates AI/OCR results. Live Photo pairing uses embedded `ContentIdentifier` metadata, never a shared basename alone.

Two complete successful enumerations must confirm disappearance before an indexed record is removed. Authentication errors, unreadable directories, partial WebDAV listings, missing/replaced mount roots and failed scans cannot confirm deletion. A failed media decode is recorded separately for retry. Enumeration and media processing happen outside the user's database write lock; stale source configuration/revision snapshots are discarded before reconciliation.

Originals in external local mounts and WebDAV do not count toward Photoly quota. Persistent local thumbnails/previews do count. Download caches and temporary processing files do not. External content continues to be indexed when the account exceeds its quota; quota still blocks application writes that increase local persistent usage. The normal default local source continues counting all contents, including non-media files.

ZIP downloads enumerate the actual source and include original names, empty directories and non-media files; `/root` combines enabled sources. Internal workspaces and symlinks are excluded. The archive is completed in temporary storage before the browser receives it; errors fail the download rather than silently dropping files. Allow enough temporary disk space for the archive and any WebDAV snapshots.

## API

All APIs use the existing authenticated `{msgCode,t}` envelope:

- Admin: `GET /admin/storage/local-roots`, `GET /admin/storage/local-directories?rootId=ID&relativePath=PATH`, `GET/POST /admin/users/{userId}/storage/local`, `PATCH/DELETE /admin/users/{userId}/storage/local/{sourceId}`.
- User: `GET/POST /user/storage/webdav`, `POST /user/storage/webdav/test?sourceId=ID` (optional ID keeps existing credentials), `PATCH/DELETE /user/storage/webdav/{sourceId}`.
- Status: `GET /user/storage/sources`; manual scan: `POST /user/storage/sources/{sourceId}/scan` with `{ "force": true }` for content verification.

Binding fields are `name`, `enabled`, `scanInterval`; local adds `rootId`, `relativePath`, WebDAV adds `url`, `username`, `password`. Status includes source name/type, management ownership, capabilities, revision, scan progress, last success, errors and retry count. The library continues checking revisions every 15 seconds, delaying refresh during selection, dragging or editing.

For integration tests, use disposable PostgreSQL databases via `TEST_DATABASE_URL` and `PHOTOLY_TEST_DATABASE_URL` (the latter resets its schema). `PHOTOLY_TEST_WEBDAV_URL`, `PHOTOLY_TEST_WEBDAV_USERNAME`, `PHOTOLY_TEST_WEBDAV_PASSWORD` run the real Nextcloud write probe; use a dedicated test account/directory and an allowed origin if private. The protocol fixture also tests conditional conflicts, unsafe hrefs, partial listings and transaction compensation.

## SMB connections and indexing progress

Users can add multiple SMB connections next to the WebDAV button in personal
Storage settings. SMB mounts are teal, with a server icon and SMB label. Paste a
UNC path such as `\\nas.example\Photos\Family`, or enter a server, share and
optional subdirectory separately. Credentials support NTLMv2 with an optional
domain. Usernames, passwords and domains are encrypted; blank fields retain saved
values. Changing the location requires detaching and adding a new connection.
SMB1, guest access, Kerberos, DFS referrals and symlinks/reparse points are not
supported. SMB2/3 runs directly over TCP 445; no privileged container or CIFS mount
is necessary. The pinned Go client includes the small guarded-handle extensions
documented in `photoly_server/third_party/go-smb2/PHOTOLY.md`.

Configure private servers explicitly, for example:

```yaml
services:
  backend:
    environment:
      STORAGE_SMB_ALLOWED_HOSTS: nas.example,10.0.0.10
    extra_hosts:
      - "nas.example:10.0.0.10"
```

Names must resolve inside Docker. Every connection validates DNS answers and dials
a validated address. Metadata, link-local and other special addresses are refused
even if allowlisted. No credentials are sent to another server through referrals.
Use a trusted network for SMB2; SMB3 honors encryption required by the server.

SMB has the same external-source retention and quota rules as WebDAV. Only local
persistent previews/thumbnails count toward Photoly quota. Unbinding and deleting
an account keep shared originals; deleting media or ordinary folders deletes their
actual contents. Transactions use the reserved `.photoly-system` directory within
the binding, including upload staging, and never expose it in listings or ZIPs.
Interrupted staging/failed compensation can leave recovery files there; do not
remove retained backups until the operation has been investigated.

Settings and folder pages display discovery, media processing, commit, stability
waiting and error states. Enumeration is indeterminate until complete; processing
counts only media that needs work, with unchanged, unstable and failed items shown
separately. Progress polls every 2 seconds while active, otherwise every 15 seconds.
Selecting, dragging or editing postpones list refresh without hiding progress.

**Refresh this folder** scans that folder and descendants. Settings scan buttons
scan the complete source. The authenticated scan endpoint accepts `{nsId?, force?}`
and returns `{jobId, state:"queued"}` in the usual result envelope. Duplicate
queued requests coalesce; a parent request covers queued children. Disjoint
requests remain separate. Scoped scans never confirm deletion outside their
scope. Ambiguous moves across scopes cause a full rescan to preserve identity.
Two stable observations are still required for new/replaced files, and two
successful complete observations of a path's absence precede index deletion.
Automatic whole-source scans continue at the configured interval (minimum 60s).
Scan job history is retained for seven days; no SMB or WebDAV bucket-style
background import copies originals into the user's UUID directory.

Migration 10 extends the current schema without resetting data. Existing local,
WebDAV and S3 records remain usable. The older flat media layout remains blocked
by migration 8. New deployments need a persistent `STORAGE_SECRET_KEY`.

Integration tests can use a disposable Samba share through `SMB_FIXTURE_ROOT`.
For an actual NAS, set `SMB_TEST_ENV_FILE` to a protected local file containing
`user=` and `password=` and run `TestSMBIntegration`/`TestNASMediaIndex`. Tests create
unique child directories, keep pre-existing files untouched, and remove their
fixtures. Never commit the credential file. S3 regression uses the existing
`PHOTOLY_TEST_S3_*` settings; Nextcloud uses `PHOTOLY_TEST_WEBDAV_*`.
