---
title: Configuration
sidebar_position: 2
---
Set Photoly options with environment variables. Put them in `.env`. The file `.env.example` in the repository is the full template.

## Core

| Variable | Purpose |
| --- | --- |
| `POSTGRES_ADMIN_PASSWORD`, `POSTGRES_MIGRATION_PASSWORD`, `POSTGRES_APP_PASSWORD` | Three separate database passwords. |
| `ADMIN_EMAIL`, `ADMIN_PASSWORD` | Administrator account seeded on first start. |
| `PUBLIC_BASE_URL` | Externally reachable API URL used for share links. |
| `TIMEZONE` | Fallback time zone. Used when a capture time has no offset or location. |
| `MAX_CONTENT_LENGTH` | Maximum size of one upload request. Default: 600 MiB. |

## Uploads and media

See [Upload operations](../administration/upload-operations.md) and [Media tools operations](../administration/media-tools.md).

| Variable | Default |
| --- | --- |
| `UPLOAD_ZIP_MAX_BYTES` | 50 GB expanded ZIP content |
| `UPLOAD_DISK_RESERVE_BYTES` | 512 MiB free disk reserve |
| `UPLOAD_TEMP_HOURS` | 24 |
| `UPLOAD_HISTORY_DAYS` | 30 |
| `ONLINE_DOWNLOAD_MAX_BYTES` | 52,428,800 |
| `ONLINE_DOWNLOAD_CONNECT_TIMEOUT` / `ONLINE_DOWNLOAD_TIMEOUT` | 5 / 60 seconds |
| `MEDIA_PROCESS_TIMEOUT` | 60 seconds |

## Storage

See [Storage deployment](../administration/storage-deployment.md).

| Variable | Purpose |
| --- | --- |
| `STORAGE_SECRET_KEY` | Encrypts saved S3, WebDAV and SMB credentials. If you lose or change it, saved credentials become unreadable. |
| `STORAGE_CACHE_TTL` | Time before unused files from remote storage leave the local cache. Default: `24h`. `0` keeps them. |
| `STORAGE_SCAN_INTERVAL` | Local scan interval. Default: `60s`. Minimum: 60 seconds. |
| `STORAGE_LOCAL_ROOTS` | JSON list of allowed external local roots. Empty means none. |
| `STORAGE_WEBDAV_ALLOWED_HOSTS` | Exact origins of private or Docker WebDAV servers. |
| `STORAGE_WEBDAV_CA_FILE` | PEM CA bundle for private HTTPS. |
| `STORAGE_SMB_ALLOWED_HOSTS` | Comma-separated SMB servers that may use private networks (TCP 445). |
