---
title: Uploads
sidebar_position: 1
---

Photoly accepts files, folders and ZIP or ZIP64 archives. You can pick them or drag and drop them.

## What Photoly keeps

- The outer folder and every subfolder that contains media. Example: `旅行/北京/photo.jpg` becomes `destination/旅行/北京/photo.jpg`.
- Nothing from empty folders. Photoly does not create them.
- Nothing from nested archives. Photoly does not expand them.

Photoly reports unsupported non-media files as skipped.

If a top-level folder name already exists, Photoly adds a number to the new name. It does not merge folders.

## Upload queue

The browser keeps one queue for the whole app. It sends two raw-file requests at a time.

- There is no limit on file count or total size.
- `MAX_CONTENT_LENGTH` limits each file or ZIP request. Default: 600 MiB.
- A manifest page holds up to 500 entries. You can send more pages.
- Photoly recognizes camera files without a MIME type by their extension.
- Closing the picker or the activity panel does not stop the upload.
- Navigating inside Photoly does not stop the upload.

If you close or reload the browser, it loses the unsent file handles. Open Uploads again. Select the same paths and sizes. There is no byte-range resume. Files that the server already received continue.

## Upload activity

The Upload button opens private activity and history. It is next to the search box on desktop. It is in the header on mobile and admin pages.

Counts show these states:

- Waiting to send
- Received
- Processing
- Done
- Skipped
- Failed

Details pages link to the library folders. **Retry** keeps successful imports. **Cancel remaining** does not delete imported photos. Signing out or changing account stops local sending. It also clears account-specific state.

### Clear activity

- **Clear activity** removes one finished batch from the list.
- **Clear finished** removes all finished, failed, skipped and cancelled batches. It works on all pages.
- Active uploads stay in the list. Use **Cancel remaining** first to stop one.

Clearing activity does not delete imported photos. Repeat submissions cannot create copies because of cleared history. Idempotency records and temporary sources keep their normal retention.

### Dismiss failure alerts

**Dismiss failure alerts** clears the red indicator. Failed items stay available for inspection and retry. The server stores the dismissal per account. It survives reloads and other devices. A new failure shows the indicator again.

You can also dismiss browser transmission errors. The upload continues.

## Upload IDs

Batch and item IDs use UUID v4. Browsers without `crypto.randomUUID()` use `crypto.getRandomValues()`. This includes ordinary LAN HTTP origins. You do not need HTTPS or a browser security override.

## Related

- Limits and operation: [Upload operations](../administration/upload-operations.md)
- Quotas: [Groups and quotas](../administration/groups-and-quotas.md)
