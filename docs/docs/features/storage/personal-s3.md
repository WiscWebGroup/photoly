---
title: Personal S3
sidebar_position: 2
---

Each account can add several S3-compatible connections. Add them in personal **Storage** settings.

## Add a connection

Enter:

- Endpoint
- Bucket
- Optional prefix and region
- Credentials

The administrator must set `STORAGE_SECRET_KEY` first. Without it, Photoly cannot save credentials.

Photoly encrypts access and secret keys in the database. The settings API never returns them. If you leave a saved credential field blank, Photoly keeps its value.

:::warning
If you lose or change the encryption key, saved credentials become unreadable.
:::

## How it looks

- Each connection shows as a blue cloud folder next to local content.
- **Alias** sets the root entry name. If a name collides, Photoly adds a number.
- You cannot rename, move or delete the mount with folder actions.
- **Disable** hides the mount. Records and remote files stay. **Enable** restores access.
- An S3 outage does not disable local storage.

## Manage connections

- **Add S3** creates a connection.
- **Edit** opens its form. A successful **Save** closes it.
- An Alias-only edit needs no remote connection.
- Enabling or saving the configuration checks remote access.
- **Remove** works only when the connection has no media and no unfinished uploads. It never deletes remote objects.
- Several connections to one bucket stay separate by source ID.
- Photoly does not scan existing buckets.

## How Photoly stores media

- Photoly manages only media uploaded through Photoly.
- S3 folders stay database folders. Objects use user, source and photo UUID keys.
- Renaming a folder or photo does not copy S3 objects.
- Uploads wait for remote writes.
- After a normal failure, Photoly rolls back the database and undoes completed writes. Batch source files stay available for retry.
- This is not a distributed transaction. It is not a crash recovery system.

## Limits

- You cannot change the endpoint, bucket or prefix while media or pending uploads exist.
- You can rotate credentials.
- S3 media and derivatives live remotely. They do not use the local quota.
- Cached downloads expire after `STORAGE_CACHE_TTL`. Default: `24h`. `0` turns eviction off.
- Cache files in an active write transaction stay protected.
- Not supported: moves across sources and import of existing bucket objects.

## Folder order

Folder handles reorder sibling folders. No files move. To move a folder to another folder, drag the card itself.

- Handles work with mouse, touch, arrow keys, Home and End.
- All three browsing layouts use the same order. The account keeps it.
- New, newly scanned and moved-in folders go to the end.
- Renames keep their position.
- Disabled mounts keep their place.

`POST /namespace/order` accepts `{parentId, nsId, beforeNsId}`. `null` means append. It returns the visible sibling list. A stale sibling reference returns 409. The client then restores and refreshes the list.

## API

- `GET /user/storage/s3` returns `{connections, fields, canSave}`.
- `POST /user/storage/s3` creates a connection.
- `PATCH` and `DELETE /user/storage/s3/{sourceId}` change one owned connection.
- `/user/storage/s3/test` accepts `sourceId`. Use it to keep saved credentials.
- Legacy `PUT` works with zero or one connection. With more, it returns a 409 business error.

## Migration 12

Migration 12 upgrades existing real-folder libraries in place.

1. Back up PostgreSQL and the deployment configuration. Include `STORAGE_SECRET_KEY`.
2. Deploy.

Photoly keeps existing source IDs, namespace IDs, credentials, media records and object keys. It removes the old per-user S3 uniqueness constraint. A downgrade to version 11 fails safely while an account has more than one S3 connection.

For other sources, see [Local mounts, WebDAV and SMB](external-storage.md).
