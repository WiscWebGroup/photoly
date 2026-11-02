---
title: Real folders
sidebar_position: 1
---

The local library is a real directory tree on disk. `/root/Trips/photo.jpg` maps to `<UPLOAD_FOLDER>/<user_uuid>/Trips/photo.jpg`.

## Names and moves

- Photoly keeps names as they are.
- If an upload, rename or move meets an existing name, Photoly adds a number. Example: `photo (1).jpg`.
- A name collision never overwrites a file.
- A directory move moves the whole directory. This includes files that Photoly does not list.

## IDs and hidden directories

Photoly keeps stable database IDs for folders and photos. Tags, albums, sharing and metadata use them. Photoly finds local paths from the storage source and the folder tree.

| Directory | Content |
| --- | --- |
| `.assets/<user_uuid>/` | Thumbnails, previews and avatars |
| `.remote/<user_uuid>/<source_id>/` | Disposable S3 and WebDAV cache |

Neither directory shows in the library.

:::tip
Back up the database, the user directories and `.assets` together.
:::

## Automatic synchronization

A scan runs at server start. It then runs every `STORAGE_SCAN_INTERVAL`. Default: `60s`.

- A new or changed file must keep the same size and modification time in two scans. Then Photoly processes it.
- A large library can take longer than the interval to scan.
- Photoly uses no filesystem watcher and no S3 bucket scanner.
- Photoly indexes empty directories.
- Unsupported non-media files stay on disk. Whole-directory operations include them.
- Photoly does not follow or import symbolic links.

### Keeping IDs

- If the inode matches clearly, Photoly reuses the ID.
- If filesystem identity is not available, one clear content match between a missing photo and a new file keeps the ID.
- Photoly does not merge duplicate content.
- If a file at the same path has new content, the photo ID stays. The content version goes up. Photoly makes the metadata and previews again.
- An unclear move can become a delete and an import.

### Deleting

Two complete, successful scans must confirm a deletion. Only then does Photoly remove records, relationships and derivatives.

These never count as a confirmed deletion:

- A missing, unreadable or replaced source root
- A failed scan

Photoly retries files that are still being written.

### Status

Scan status and errors show in personal **Storage** settings. Open library views check content revisions every 15 seconds when idle.

## Quota

The local quota counts:

- Ordinary files in the user directory
- Media derivatives
- Avatars

It does not count temporary uploads, archive scratch space or the S3 cache.

If an account is over quota, Photoly still indexes files placed from outside. It then blocks actions that raise local usage. Read, download, move and delete stay available.

## Folder downloads

A folder ZIP keeps real names, hierarchy, empty directories and non-media files.

- Local archives exclude application assets and symbolic links.
- Remote archives include original media and Live Photo original attachments.
- A `/root` archive includes all enabled sources under their mount labels.
- If an indexed original is missing or unreadable, Photoly stops before it sends a response.
- Photoly builds archives in temporary disk space. It removes them after serving or cancel.
- The browser download manager handles the response. The ZIP does not go into a JavaScript Blob.

Deleting a folder also deletes non-media files. The confirmation dialog says so.

## API changes

IDs and UUID-based share URLs stay the API identifiers.

- Folder responses add `sourceId`, `sourceType`, `isMount`, `canMove`, `canRename` and `canDelete`.
- Photo responses add `sourceId`.
- Folder create, rename and move return `{nsId, nsName}`. Clients can show collision suffixes.
- `POST /photo/update` and `/photo/changeNamespace` return the updated photo with its final `photoName`.
- JSON responses keep `{msgCode,t}`. Check `msgCode` for business errors.

Source endpoints:

- `GET /user/storage/sources`: own source revisions and scan status.
- `GET /user/storage/s3`: own public settings, fields and a credential-present flag.
- `POST /user/storage/s3/test`: test the supplied settings. Saved credentials merge in.
- `PUT /user/storage/s3`: save settings or toggle `enabled`.

Photoly removed the global `/admin/storage` endpoints and `/user/updateStorageVisibility`. Dates, search, galleries and tags include all enabled sources.
