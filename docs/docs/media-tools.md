# Media tools

## Usage and formats

**Format Convert** is available from image context menus in Library, Dates, albums and search. Choose JPEG, PNG or GIF. A private copy is saved in the source's actual folder captured when submitting, named `name (converted).format`, with numbered collision suffixes. The source remains unchanged. Compatible EXIF/capture metadata is retained; tags and album memberships are not copied. JPEG uses quality 90 and white transparency, PNG retains alpha. Animated images converted to JPEG/PNG use their first frame; GIF conversion retains frame order, duration and loop metadata. Live-to-GIF uses the companion movie without sound.

JPEG, PNG, GIF, WebP, BMP, TIFF, ICO, HEIC and HEIF are supported conversion sources. HEIC/HEIF originals are retained; the browser receives a JPEG cover and static thumbnail. Originals download unchanged. Image previews and thumbnails advertise their actual MIME types. GIF originals animate only when opened in the viewer. The conversion decoder limits images to 50 million pixels and animations to 1,000 frames / 200 million cumulative pixels; process time is bounded.

**Clipboard upload:** open the upload file picker and paste an image with Cmd+V or Ctrl+V. Each paste adds files to the preview list; Upload remains a separate action. Text paste is untouched. Closing the picker or signing out clears previews and releases object URLs. No clipboard listener is installed outside the picker.

## Apple Live Photos

Upload original JPEG/HEIC/HEIF and MOV components together using one file selection, folder or ZIP. The worker reads Apple's ContentIdentifier and pairs only one still and one MOV with the same identifier, batch and relative directory. Names need not match. Missing/ambiguous identifiers remain independent files. Existing library entries are never retroactively merged; Android Motion Photos are not supported.

The upload manifest is sealed before transmission; candidates show `waiting` until their siblings have arrived and been inspected. An expired missing transmission fails and the received component is imported independently. Cancel/retry applies through Upload activity. A pair creates one Photo record plus media resources. Its still cover participates in Dates, maps and image search; its movie is not a separate result.

The server preserves original still/MOV and generates an H.264/AAC MP4. The viewer has a LIVE play/stop button, defaults to muted, and offers sound and original-pair ZIP download. Closing or switching stops playback. If encoding fails, the cover remains usable and **Retry playback preview** queues regeneration. Public shares show the still cover. Rename/move/delete operate on the complete record; resource paths derive from its UUID. Quota and folder byte totals include originals, covers and playback copies. Folder ZIP export includes both originals and counts the photo once.

## Online Download

Administrators enable **Online Download** in Settings; it defaults off. Ordinary users read `/photo/mediaCapabilities`. The icon left of Upload opens a URL/format/folder form. It defaults to the currently browsed Library folder, otherwise Library root.

Only public HTTP/HTTPS image URLs are accepted. Each DNS result must be globally routable; connections use a validated IP with TLS verification/SNI for the original hostname. Every redirect is checked again (maximum three). Cookies, credentials, environment proxies, private/link-local/loopback addresses and non-HTTP schemes are rejected. Bytes are streamed to temporary storage and decoded as an image. The switch is checked when submitting, starting work and committing the result. Temporary files are cleaned after failure/cancellation; a removed folder or exhausted quota fails the job without a partial photo.

| Server configuration | Default |
| --- | --- |
| `ONLINE_DOWNLOAD_MAX_BYTES` | 52,428,800 (50 MiB source) |
| `ONLINE_DOWNLOAD_CONNECT_TIMEOUT` | 5 seconds |
| `ONLINE_DOWNLOAD_TIMEOUT` | 60 seconds including DNS/redirects/body |
| `MEDIA_PROCESS_TIMEOUT` | 60 seconds per conversion/encoding |

## Tasks and upgrade

All media tasks use the existing `upload-worker`. No new daemon is required. Conversion and network work occur outside user write locks; commit rechecks ownership, destination and quota. Source conversions bind the photo content version and fail if deleted/replaced. Client UUIDs bind idempotent task submissions to their arguments. Activity, cancellation, retry and completion notifications use the existing upload interface. Cancellation prevents final installation; an in-flight bounded encoder/network operation may finish before the worker observes cancellation.

Back up PostgreSQL and media together. Stop application/worker services, rebuild images, run the existing one-shot `migrate` service, then restart services as described in the root README. Migration `0007_media_tools` adds `photo_media`, upload task source/options/pairing metadata, and the default-off setting; existing Photo response fields and records are unchanged. Pillow HEIF support is bundled in the image; FFmpeg and ExifTool continue to be required. Deploy backend, worker and frontend together. Never downgrade while keeping Live resources: export originals and restore a matching database/media backup instead.

OpenAPI documents `/photo/convert`, `/photo/onlineDownload`, `/photo/mediaInfo`, `/photo/mediaCapabilities`, `/photo/retryLivePreview`, `/photo/renderLive/{token}` and `/photo/downloadLive/{token}`. Task APIs return upload batch IDs; inspect the legacy envelope's `msgCode` and Upload activity item errors.

Implementation references: [Pillow HEIF plugin](https://pillow-heif.readthedocs.io/en/stable/pillow-plugin.html), [Apple Live Photo movie metadata](https://developer.apple.com/documentation/avfoundation/avcapturephotosettings/livephotomoviemetadata), [MDN clipboardData](https://developer.mozilla.org/en-US/docs/Web/API/ClipboardEvent/clipboardData), and [OWASP SSRF prevention](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html).


## Legacy thumbnail orientation

New uploads and image replacements normalize EXIF orientation before resizing. Older libraries may
contain thumbnails generated without this normalization. Audit and repair those thumbnails with:

```sh
docker compose exec backend flask --app app:create_app repair-thumbnail-orientation
docker compose exec backend flask --app app:create_app repair-thumbnail-orientation --apply
```

The repair handles EXIF orientations 2–8, including mirrored images, keeps originals and capture
metadata unchanged, and adjusts storage counters transactionally. It processes records in bounded
pages and can be rerun safely. Back up the database and media before maintenance. Deploy the updated
frontend as well: its thumbnail URL revision bypasses previously cached sideways thumbnails.
