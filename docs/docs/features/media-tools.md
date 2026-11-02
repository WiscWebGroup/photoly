---
title: Media tools
sidebar_position: 4
---

## Format Convert

**Format Convert** is in the image context menu. You can open it from Library, Dates, albums and search.

1. Right-click an image.
2. Choose **Format Convert**.
3. Choose JPEG, PNG or GIF.

Photoly saves a private copy in the folder where the source file really is. The folder is the one at submit time. The copy is named `name (converted).format`. If the name exists, Photoly adds a number. The source does not change.

### What the copy keeps

- Compatible EXIF and capture metadata stay.
- Tags and album memberships do not copy.
- JPEG uses quality 90. Transparency becomes white.
- PNG keeps alpha.
- An animated image converted to JPEG or PNG uses its first frame.
- GIF conversion keeps frame order, duration and loop data.
- Live-to-GIF uses the companion movie. It has no sound.

### Supported sources

JPEG, PNG, GIF, WebP, BMP, TIFF, ICO, HEIC and HEIF.

- Photoly keeps HEIC and HEIF originals. The browser gets a JPEG cover and a static thumbnail.
- Original downloads do not change.
- Previews and thumbnails use their real MIME types.
- GIF originals animate only in the viewer.

### Limits

- Images: 50 million pixels.
- Animations: 1,000 frames and 200 million total pixels.
- Process time has a limit.

## Clipboard upload

1. Open the upload file picker.
2. Paste an image with Cmd+V or Ctrl+V.

Each paste adds files to the preview list. Upload is a separate step. Text paste is not affected.

If you close the picker or sign out, Photoly clears the previews and releases the object URLs. Outside the picker, Photoly does not listen to the clipboard.

## Apple Live Photos

Upload the original JPEG, HEIC or HEIF file and the MOV file together. Use one file selection, one folder or one ZIP.

### Pairing

The worker reads Apple's `ContentIdentifier`. It pairs one still and one MOV when all of these match:

- The identifier
- The batch
- The relative directory

File names need not match. If an identifier is missing or ambiguous, the files stay separate. Photoly never merges existing library entries later. Android Motion Photos are not supported.

### Upload behavior

- Photoly seals the upload manifest before sending.
- Candidates show `waiting` until their partner arrives and Photoly inspects it.
- If a partner never arrives, the upload fails. Photoly imports the received file alone.
- Cancel and retry work in Upload activity.

### Result

A pair makes one Photo record with media resources. The still cover appears in Dates, maps and image search. The movie is not a separate result.

The server keeps the original still and MOV. It also makes an H.264/AAC MP4.

### Viewer

- A **LIVE** button plays and stops the movie.
- Sound is off by default. You can turn it on.
- You can download the original pair as a ZIP.
- Closing the viewer or switching items stops playback.
- If encoding fails, the cover still works. **Retry playback preview** queues a new encode.

### Other behavior

- Public shares show the still cover.
- Rename, move and delete act on the whole record.
- Resource paths come from the record UUID.
- Quota and folder totals include originals, covers and playback copies.
- Folder ZIP export includes both originals. It counts the photo once.

## Online Download

Online Download saves an image from a URL. An administrator turns it on in Settings. It is off by default.

Users read `/photo/mediaCapabilities` to see if it is on. The download icon is left of Upload. It opens a form with a URL, a format and a folder. The folder is the Library folder you are viewing. If you are not in one, it is the Library root.

### Security rules

Photoly accepts only public HTTP and HTTPS image URLs.

- Each DNS result must be globally routable.
- Photoly connects to a validated IP. It verifies TLS and uses the original host name for SNI.
- It checks every redirect again. The maximum is three.
- It rejects cookies, credentials and environment proxies.
- It rejects private, link-local and loopback addresses.
- It rejects non-HTTP schemes.

Photoly streams bytes to temporary storage and decodes them as an image. It checks the switch when you submit, when work starts and when it saves the result.

If a job fails or you cancel it, Photoly removes the temporary files. If the folder is gone or the quota is full, the job fails. No partial photo remains.

For limits and settings, see [Media tools operations](../administration/media-tools.md).
