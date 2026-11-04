---
title: Media tools operations
sidebar_position: 7
---

For the user guide, see [Media tools](../features/media-tools.md).

## Settings

| Variable | Default |
| --- | --- |
| `ONLINE_DOWNLOAD_MAX_BYTES` | 52,428,800 (50 MiB source) |
| `ONLINE_DOWNLOAD_CONNECT_TIMEOUT` | 5 seconds |
| `ONLINE_DOWNLOAD_TIMEOUT` | 60 seconds, including DNS, redirects and body |
| `MEDIA_PROCESS_TIMEOUT` | 60 seconds per conversion or encode |

## Tasks

Media tasks use the upload worker inside the Go backend process. No separate worker service is needed.

- Conversion and network work run outside user write locks.
- Commit checks ownership, destination and quota again.
- Source conversions bind the photo content version. They fail if the photo is deleted or replaced.
- Client UUIDs bind idempotent task submissions to their arguments.
- Activity, cancel, retry and completion notices use the existing upload interface.
- Cancel prevents the final install. A bounded encoder or network call that is already running can finish first.

## Upgrade

Follow [Upgrading](../install/upgrading.md), backing up PostgreSQL and media together before rebuilding the backend and frontend. The backend applies embedded migrations at startup, including media resources, upload task metadata and later feature migrations.

The image includes FFmpeg, ExifTool and libvips/HEIF tools. There is no Pillow plugin or separate inference image required for media conversion. Never downgrade while newer media resources exist; restore a matching database and media backup instead.

## API

OpenAPI documents these endpoints:

- `/photo/convert`
- `/photo/onlineDownload`
- `/photo/mediaInfo`
- `/photo/mediaCapabilities`
- `/photo/retryLivePreview`
- `/photo/renderLive/{token}`
- `/photo/downloadLive/{token}`

Task APIs return upload batch IDs. Check the `msgCode` of the legacy envelope and the errors on Upload activity items.

## Maintenance: legacy thumbnail orientation

New uploads and image replacements normalize EXIF orientation before resizing. Older libraries can have thumbnails made without this step.

The old Flask `repair-thumbnail-orientation` command is not shipped in the Go binary. Do not run it against a current deployment. Keep a matching backup and verify affected originals before planning a repair; the current CLI does not offer a bulk thumbnail-orientation repair command.

## References

- [Apple Live Photo movie metadata](https://developer.apple.com/documentation/avfoundation/avcapturephotosettings/livephotomoviemetadata)
- [MDN clipboardData](https://developer.mozilla.org/en-US/docs/Web/API/ClipboardEvent/clipboardData)
- [OWASP SSRF prevention](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html)
