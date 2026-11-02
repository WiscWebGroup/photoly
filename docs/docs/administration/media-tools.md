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

All media tasks use the existing `upload-worker`. You need no new daemon.

- Conversion and network work run outside user write locks.
- Commit checks ownership, destination and quota again.
- Source conversions bind the photo content version. They fail if the photo is deleted or replaced.
- Client UUIDs bind idempotent task submissions to their arguments.
- Activity, cancel, retry and completion notices use the existing upload interface.
- Cancel prevents the final install. A bounded encoder or network call that is already running can finish first.

## Upgrade

1. Back up PostgreSQL and media together.
2. Stop the application and worker services.
3. Rebuild the images.
4. Run the existing one-shot `migrate` service.
5. Restart the services as the root README describes.

Migration `0007_media_tools` adds:

- `photo_media`
- Upload task source, options and pairing metadata
- The default-off setting

Existing Photo response fields and records do not change.

Notes:

- The image bundles Pillow HEIF support.
- FFmpeg and ExifTool are still required.
- Deploy the backend, worker and frontend together.
- Never downgrade while Live resources exist. Export originals and restore a matching database and media backup instead.

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

To repair them:

1. Back up the database and media.
2. Run an audit:

   ```sh
   docker compose exec backend flask --app app:create_app repair-thumbnail-orientation
   ```

3. Apply the repair:

   ```sh
   docker compose exec backend flask --app app:create_app repair-thumbnail-orientation --apply
   ```

4. Deploy the updated frontend. Its thumbnail URL revision bypasses cached sideways thumbnails.

The repair:

- Handles EXIF orientations 2–8, including mirrored images.
- Keeps originals and capture metadata unchanged.
- Adjusts storage counters in transactions.
- Processes records in bounded pages.
- Is safe to run again.

## References

- [Pillow HEIF plugin](https://pillow-heif.readthedocs.io/en/stable/pillow-plugin.html)
- [Apple Live Photo movie metadata](https://developer.apple.com/documentation/avfoundation/avcapturephotosettings/livephotomoviemetadata)
- [MDN clipboardData](https://developer.mozilla.org/en-US/docs/Web/API/ClipboardEvent/clipboardData)
- [OWASP SSRF prevention](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html)
