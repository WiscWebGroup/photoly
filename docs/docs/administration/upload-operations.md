---
title: Upload operations
sidebar_position: 3
---

The Go server runs the upload worker and the per-source scanner in one process.

Before you deploy, read:

- [Local mounts, WebDAV and SMB](../features/storage/external-storage.md): bindings, allowlists, synchronization and file retention.
- [Real folders](../features/storage/real-folders.md): old flat-layout libraries cannot upgrade in place.

`photoly migrate` applies the schema for a fresh library. `photoly serve` also migrates on start.

## Limits

| Variable | Default | Purpose |
| --- | --- | --- |
| `MAX_CONTENT_LENGTH` | 600 MiB | Size of each request |
| `UPLOAD_ZIP_MAX_BYTES` | 50 GB | Expanded ZIP content |
| `UPLOAD_DISK_RESERVE_BYTES` | 512 MiB | Reserved free disk space |
| `UPLOAD_TEMP_HOURS` | 24 | Retention of temporary sources |
| `UPLOAD_HISTORY_DAYS` | 30 | Retention of activity |

The server checks ZIP paths, entry types, size and quota.

## Directories

- Sources live in `.uploads`.
- Transaction backups live in `.staging`.

Normal failures undo their file changes.

PostgreSQL and filesystem or remote writes are not one distributed transaction. After a host crash:

1. Inspect the backups.
2. Reconcile the library.
3. Only then remove orphaned files.

Do not delete active upload source files.
