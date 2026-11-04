---
title: Duplicate Detection
---

Open **Organize → Duplicate Detection** to review identical photos across your own folders and enabled storage sources. On phones, use the overlapping-photo icon in the top bar.

Photoly compares the SHA-256 of indexed original files. Renamed copies match; resized, recompressed or visually similar pictures do not. Standalone videos are excluded. Live Photos match only when both their still image and original video match. Files with missing hashes or known missing originals are excluded; refresh the relevant storage folder to finish indexing external changes. Opening this page does not start an AI job or rescan every source.

Each group shows filenames, source names, folder links, file sizes and previews. Group lists default to 10 groups per page, with 20 and 50 available in the pagination controls at the top and bottom. Photos within each group use 10 per page. Keep/Kept updates in place without resetting the current page, scroll position or other selections. Nothing is selected for deletion by default.

- **Keep** marks a photo to retain during duplicate cleanup. Click **Kept** to unmark it. Keep does not prevent normal deletion from the library.
- **Select other copies** selects the group's unprotected copies after at least one photo is marked Keep. Groups above 1,000 photos use page selection; each cleanup request accepts at most 1,000 selected copies.
- **Ignore group** moves the group to **Ignored**. **Restore group** brings it back. These decisions persist across devices.
- **Delete selected** lists the selected files and asks for confirmation. Deletion is permanent and uses the same storage/file handling as normal library deletion; there is no trash or undo.

Keep is bound to a photo's content identity and version. Replacing the content invalidates it. Ignoring a group applies to its current membership and content versions; adding a copy or changing a member brings the group back for review. Folder renames alone do not reset decisions.

Before deleting, the server checks ownership, group version and Keep markers under storage locks, reads fresh original bytes (including remote originals), and verifies a readable independent copy will remain. A stale result, unavailable survivor or changed original blocks the operation. Each group commits separately; a batch reports successful deletions and failed groups. Refresh and review failed groups before retrying.

## API and upgrade

Migration `00019_duplicates` adds the Keep/Ignore tables, indexed candidate lookup and grouping views without rewriting photo files. It runs automatically on server startup. Existing libraries need no AI reindex or additional container.

All endpoints require the owner's session and return private, non-cacheable responses in the usual `msgCode`/`t` envelope:

- `GET /photo/duplicates`: `ignored`, `offset`, `limit`; returns groups and total count.
- `GET /photo/duplicates/members`: `key`, `version`, `offset`, `limit`; returns member details and count.
- `POST /photo/duplicates/keep`: `key`, `version`, `photoId`, `value`.
- `POST /photo/duplicates/ignore`: `key`, `version`, `value`.
- `POST /photo/duplicates/delete`: `key`, `version`, `photoIds`.

Page sizes are 1–100. A `msgCode` of 409 means the group changed; reload it. A 400 indicates invalid input or a blocked cleanup, with the reason in `t`. See Swagger for the full request and response schemas.
