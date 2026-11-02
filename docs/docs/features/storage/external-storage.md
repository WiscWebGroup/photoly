---
title: Local mounts, WebDAV and SMB
sidebar_position: 3
---

Photoly can index storage that lives outside its own library.

| Type | Badge | Who sets it up |
| --- | --- | --- |
| Local mount | Amber disk | Administrator, in **User Management → Storage → Local mounts** |
| WebDAV | Purple network | User, in **Settings → Storage → WebDAV connections** |
| SMB | Teal server | User, in **Settings → Storage** |
| S3 | Blue cloud | User. See [Personal S3](personal-s3.md) |

Each binding shows in `/root`. You can edit entry names in settings. Name collisions get a number. You cannot rename, move or delete entries with ordinary folder actions.

To deploy, see [Storage deployment](../../administration/storage-deployment.md).

:::info
Migration 9 upgrades an existing real-folder library in place. It does not move or delete media. It does not reset the database. Back up the database and upload directory first. For libraries from before real folders, see [Upgrading](../../install/upgrading.md).
:::

## Connections and permissions

### WebDAV

A Nextcloud URL looks like `https://cloud.example/remote.php/dav/files/USERNAME/Photos/`. Use the username and an app password with Basic authentication.

The connection test uses a temporary probe. It checks:

- Directory listing
- Directory creation
- File write and read
- Conditional writes
- Rename
- Delete

Photoly removes the probe afterward.

Photoly encrypts the username and password with `STORAGE_SECRET_KEY`. Settings APIs never return them. Blank fields keep the saved values. Keep the key stable and back it up safely.

### SMB

Add several SMB connections next to the WebDAV button.

1. Paste a UNC path. Example: `\\nas.example\Photos\Family`.
2. Or enter the server, the share and an optional subdirectory.
3. Enter credentials. NTLMv2 works, with an optional domain.

Photoly encrypts usernames, passwords and domains. Blank fields keep saved values.

Supported: SMB2 and SMB3 over TCP 445. You need no privileged container and no CIFS mount.

Not supported: SMB1, guest access, Kerberos, DFS referrals, symlinks and reparse points.

For SMB3, Photoly follows the encryption that the server requires. Use a trusted network for SMB2.

### Read and write rules

- Bindings are read and write.
- Upload, replace, folder create, rename, move, delete, share and download use real file names and directories.
- Photoly rejects moves across sources. This includes two bindings of one physical directory.
- Photoly never scans an S3 bucket for existing objects.

### Repeated and nested bindings

- You can repeat and nest bindings.
- Each binding has its own index, IDs, tags, albums and shares.
- File changes are shared on disk. Other bindings see them at their next scan.
- Administrators see overlapping local bindings. Normal users cannot browse the allowed roots.
- Deleting an ordinary folder deletes **all** its contents. This includes hidden non-media files and contents that other bindings use.

### Disable, detach and delete

- **Disable** hides the entry and keeps its index. **Enable** requests a scan.
- **Detach** a binding, or delete an account: external local and WebDAV originals and non-media files stay. Photoly removes only its index, associations, derivatives and cache.
- You cannot change the location of a binding. Detach it and add a new one.
- Settings have no recursive remote wipe.

For SMB, transactions use the reserved `.photoly-system` directory inside the binding. Photoly never shows it in lists or ZIPs. If staging is interrupted or compensation fails, recovery files can stay there. Do not remove retained backups until you investigate.

## Synchronization

### Schedule

- Photoly scans a source at startup, when you bind or enable it, and then every 60 seconds by default.
- Each external source can use a longer interval. The minimum is 60 seconds.
- The default Photoly source uses `STORAGE_SCAN_INTERVAL`. The minimum is 60 seconds.
- Three sources can scan at the same time. One source never overlaps its own scan.
- The next interval starts when the last scan ends.
- Manual scan requests merge.
- **Verify all content** requests hashing of media that looks unchanged.

### What Photoly indexes

- Photoly indexes empty directories and supported media directly.
- A new or changed file must look stable in two observations. Photoly checks it again after processing.
- Photoly records a failed media decode separately for retry.

### Keeping IDs on moves

- Local inode identity keeps IDs. WebDAV `DAV:resource-id` does too, when present.
- ETags describe versions, not identity.
- Without reliable identity, only one clear missing-and-new content-hash match keeps an ID.
- Duplicate content stays separate.
- If a file at the same path has new content, the content version goes up. Photoly refreshes derivatives and invalidates AI and OCR results.
- Live Photo pairing uses the embedded `ContentIdentifier`. A shared base name alone is not enough.

### Deleting

Two complete, successful listings must confirm that a path is gone. Then Photoly removes the record.

These cannot confirm a deletion:

- Authentication errors
- Unreadable directories
- Partial WebDAV listings
- Missing or replaced mount roots
- Failed scans

Listing and media processing run outside the user's database write lock. Photoly drops stale source snapshots before it reconciles.

### Recycle folders

Scanners skip directories named `@Recycle`, `#recycle` and `$RECYCLE.BIN`. The match ignores case. It includes nested folders. Other hidden folders are not skipped.

After a successful full scan, Photoly removes recycle content that it indexed before. It also removes local derivatives. It does not delete external originals. A failed listing never does this.

## Quota

- Originals in local mounts, WebDAV and SMB do not count toward the Photoly quota.
- Persistent local thumbnails and previews count.
- Download caches and temporary processing files do not count.
- If an account is over quota, Photoly still indexes external content. Quota still blocks writes that raise local persistent usage.
- The default local source counts all its contents, including non-media files.

## ZIP downloads

A ZIP lists the real source. It keeps original names, empty directories and non-media files. `/root` combines the enabled sources. It excludes internal workspaces and symlinks.

Photoly completes the archive in temporary storage before the browser receives it. An error fails the download. Photoly never drops files without saying so. Leave enough temporary disk space for the archive and any WebDAV snapshots.

## Storage activity

The server icon to the right of Upload opens **Storage activity**. It works on desktop and mobile.

It shows all your sources. You can:

- Refresh the current folder and its descendants.
- Scan a whole source.
- Force content verification.

S3 shows its enabled state. It has no scan action. S3 is closed by default until you configure it. If you disable a configured connection, Photoly saves this at once and keeps the location and credentials. If you enable it again, Photoly checks the credentials.

Folder cards count direct subfolders and direct media. If a folder has no direct image, its cover uses a descendant image. Map opens the same full photo viewer as Photos.

### Progress

Settings and folder pages show these states:

- Discovery
- Media processing
- Commit
- Waiting for stability
- Error

Listing shows no percentage until it ends. Processing counts only media that needs work. Unchanged, unstable and failed items show separately.

Progress polls every 2 seconds while active. Otherwise it polls every 15 seconds. Selecting, dragging or editing delays the list refresh. Progress stays visible.

### Scoped scans

**Refresh this folder** scans that folder and its descendants. The scan buttons in settings scan the whole source.

- Duplicate queued requests merge. A parent request covers queued children. Separate areas stay separate.
- A scoped scan never confirms a deletion outside its scope.
- If an ambiguous move crosses scopes, Photoly runs a full rescan to keep identity.
- New or replaced files still need two stable observations.
- A path needs two complete, successful observations of its absence before Photoly deletes it from the index.
- Photoly keeps scan job history for seven days.

Photoly never copies SMB or WebDAV originals into the user's UUID directory.

## File issues

- Invalid or oversized files stop automatic retries while they stay unchanged.
- A file change, a processor upgrade or forced verification retries them.
- Transient failures retry as normal.
- `/user/storage/sources/{sourceId}/failures` is a paginated, authenticated endpoint. It returns relative paths and clean reasons.
- `scanAutomaticRetries` and `scanBlocked` separate transient failures from files that need attention. `scanRetries` is their total.

Migration 11 adds versioned indexing failures. It keeps existing sources and media. Migration 10 extends the schema without a reset. It keeps local, WebDAV and S3 records. Migration 8 still blocks the old flat layout. New deployments need a persistent `STORAGE_SECRET_KEY`.

## Large still images

Photoly uses an isolated libvips preview process for images above 50 million pixels. This is the full-decoder limit. It works for JPEG, PNG, TIFF, WebP and HEIC/HEIF.

- The preview path accepts at most 250 million pixels.
- It keeps the original and the metadata.
- It makes a 400 px thumbnail and a preview with a 2560 px maximum side.
- It rejects animated and multipage large inputs.

### Resource limits

Docker images include `libvips-tools` and `util-linux`. On Linux, `prlimit` sets:

- 2 GiB address-space cap
- 120 CPU seconds
- 512 MiB output file limit

Also:

- One large preview runs at a time per backend.
- The wall deadline is 120 seconds.
- libvips uses two threads.
- The operation cache threshold is 64 MiB. This is not a process memory cap.

If the processor is missing or a limit is exceeded, Photoly shows an actionable file issue.

### Related behavior

- AI embeddings and OCR use generated previews when available.
- Original downloads keep full resolution.
- Full-resolution editing and conversion keep their existing pixel guard.

:::warning
Never remove the pixel guard to enable large previews.
:::
