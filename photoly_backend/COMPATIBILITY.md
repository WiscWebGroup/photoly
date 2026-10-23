# API compatibility with Spring e6619ab

The migration targets all **90 annotated business endpoints**: user 11, admin 19, photo 27, namespace 9, gallery 6, tag 5, cred 11, photo-extra 2. The four empty relationship/setting Controllers exposed no methods and introduce no extra routes. `/healthz` is a new operational route, and `POST /photo/replace` is a deliberate extension (in-place image replacement for the editor's overwrite option). Framework-generated Spring/Swagger pages are not business APIs and are not reproduced.

`tests/contracts/spring_routes.json` preserves the original Java signatures; `spring_entities.json` preserves entity response fields. Both were extracted from Java before removing it. `tests/contract_cases.py` independently enumerates requests and expected response families; tests require its inventory to equal the frozen Spring inventory and the Flask route map. These are source-derived compatibility tests, not a claim that the Java server was run for differential testing.

`spring_field_types.json` and `spring_schema.json` additionally freeze the Java field types and SQL table definitions directly from `e6619ab`. These MySQL snapshots remain historical evidence. postgresql_schema.json records target types; tests compare ORM metadata and the actual Alembic-created PostgreSQL tables.

## Preserved contract

- Methods and URLs, including trailing slashes used by the Vue frontend. POST requests do not redirect.
- Query/form scalar arguments, JSON entity bodies, multipart `file`/`photo` and repeated `files`/`photosStr`; comma-separated and repeated `tagIds`.
- `HRD-Token` login headers, login tokens embedded in media URLs, per-photo share tokens, and C/R/D API credentials.
- Entity camelCase versus Mapper result snake_case. Null fields remain present. `User.password` is always null in responses.
- `{t,msgCode}`, `{t,token,msgCode}`, and `{t,pageNum,msgCode}`; `pageNum` is total pages. Defined business failures generally remain HTTP 200 with the original JSON code.
- Cred media endpoints retain direct UUID text, integer, array, entity, or empty-body responses instead of acquiring an envelope.
- Entity dates are ISO LocalDateTime strings. Mapper timestamp fields use `yyyy-MM-dd`, matching the original `spring.jackson.date-format` for JDBC timestamp values.
- `visibility`: 0 private, 1 per-photo token required, 2 public. Image and QR routes retain the legacy `image/jpeg` declaration, including PNG QR bytes. Video routes retain `application/octet-stream`.
- Single-file downloads use the stored UUID filename; folder/gallery downloads use `download.zip`. The `Content-Disposition` filename is emitted using standard HTTP encoding.
- `/admin/stop` preserves `{t:1,msgCode:591023}` on success and stops the Gunicorn master and all workers. In unsupported development-server mode it returns `t:-1`; no guessed PID is signalled.

## Intentional fixes and bounded differences

1. **Fresh deployment only.** Passwords now use scrypt; Java AES passwords and unprefixed Redis login sessions are not migrated. Initialization no longer contains the old embedded administrator ciphertext. Java remains available in Git.
2. **Ownership and authorization.** Previously unchecked album associations, tags, folder lookups/parents, map filters and admin settings reject cross-user or non-admin access. New authorization failures use the route's existing response family; file access failures return HTTP 400/403/404 instead of null-pointer errors or an ambiguous empty success. Password hashes are never returned, including admin user lists.
3. **Input constraints.** Folder/media names cannot be path components (`.`, `..`, slashes, NUL, CR or LF). Root folders cannot be moved or renamed. A folder cannot move into itself or a descendant. All entries of batch uploads are checked before writing. Invalid visibility is rejected on updates and normalized to private on upload; unsupported visibility 3 is no longer stored. Settings accept sensible values (0/1 switches, 0–365 token days).
4. **Uploads and metadata.** Single API upload inserts the photo before its EXIF record. Missing/unsupported EXIF does not reject valid non-image media. Supported but malformed raster images are rejected. ExifTool replaces metadata-extractor: public keys and value types remain compatible, but descriptive strings may differ between parsers. Coordinates correctly preserve south/west signs; camera and lens fields tolerate partial metadata. Thumbnail geometry retains the existing 400-pixel-width rule; encoded bytes can differ because Pillow replaces Java ImageIO.
5. **Consistency and deletion.** Normal transactional failures undo filesystem changes; batch uploads are atomic instead of partially succeeding. Deletion handles missing originals and removes dependent records, thumbnails, avatars and user API credentials/sessions. Namespace/account count conventions remain counts of removed media files plus removed folders/catalog/account rows; photo deletion still returns 1.
6. **Downloads.** Videos are streamed, with correct HTTP 206/416 Range handling; full responses do not emit the old invalid full-file Content-Range. ZIP exports use separate temporary storage, preserve duplicate filenames with suffixes, and safely export the root as `root/`. No request deletes a named directory containing original files.
7. **Initialization and schema.** Tables/columns and API IDs retain their roles; fresh-schema foreign keys, lookup indexes and uniqueness constraints additionally protect email, UUID/token, relationship and EXIF integrity. The unused `namespace_photo` table remains mapped. UUID uniqueness is enforced even when the legacy SSafeUUID switch is off.
8. **Infrastructure failures.** Invalid HTTP payloads, upload limits and unavailable dependencies use explicit 4xx/503 responses rather than leaking stack traces. CORS remains compatible with header-based tokens and no longer combines wildcard origins with credentialed cookies.

The Vue source was not changed. Existing unrelated frontend defects (for example the password-edit form passing its email field as `newPass`) are not silently compensated for by changing API parameter semantics.

## PostgreSQL fresh deployment (phase 1)

- PostgreSQL 17 / pgvector 0.8.7 replaces MySQL; no MySQL data import is provided. Eleven business tables retain names and fields, plus an Alembic revision table.
- Keys use generated-by-default identity. Nullable/date/response semantics are preserved. A unique lower(email) index enforces case-insensitive login identity while preserving spelling. ILIKE does not emulate MySQL accent-insensitive collation.
- GET /admin/pingDatabase and GET /admin/databaseInfo are explicit additions. GET /user/getStorage, GET /photo/getCovers, GET /photo-extra/getMapPlaces and GET /admin/getUsers are read-only additions for the web app (storage meter, card covers, map place names, and the user table's media counts, role filter and total); the legacy page routes keep their exact response shape. Place names come from bundled GeoNames data (CC BY 4.0, `app/places/`). /admin/pingMySQL is a SELECT 1 compatibility alias. /admin/getAddress retains its mysql field for the PostgreSQL address.
- Only the migration service upgrades schema; init-db is now a seed-only alias requiring a migrated database.
- Frontend code is unchanged; its old database label will be corrected in phase 3.

## Smart Search and avatar normalization

`GET /photo/searchSmart?query=...` adds bounded, owner-scoped text candidates for Smart Search without changing `/photo/searchPhoto`. Clients combine these with `/ai/search/text` using equal-weight reciprocal rank fusion. Avatar uploads keep their existing multipart contract but are now decoded, orientation-corrected, center-cropped and stored as 512 × 512 JPEG; invalid images leave the previous avatar intact.

## Video semantic indexing

Revision `0003_video_vectors` adds `video_frame_embedding` and sampling/count columns to `photo_embedding_state`. Existing image vectors, model profiles and fingerprints are retained. A video is ready only when all sampled frame vectors are committed in one transaction. Enabled users' existing videos are reconciled once per sampling version; decode failures remain visible instead of continuously requeuing.

Text, image, similar-media and recommendation-page endpoints now return both photos and videos. Each result retains `photo` and `score`; video results additionally include `matchedTimestampMs` (the best matching sample time). Scores are the maximum cosine similarity across all query/document frame pairs, with media IDs breaking ties. `/ai/capabilities` reports `videoFormats`, `videoSamplingVersion` and video limits. Image-upload search still accepts images only; a library video's existing ID can be used by the similar-media endpoints.

## On-demand OCR

Revision `0004_photo_ocr` adds owner-only editable text in `photo_ocr`. New endpoints are `GET /photo/ocr`, `POST /photo/ocr/recognize`, and `PUT /photo/ocr`, using the existing HRD-Token and result envelope. Reads do not trigger OCR. Mutations require the current image content version and OCR revision, and reject conflicts instead of overwriting newer text. Content overwrite invalidates text through a database trigger; deletion cascades. Existing Photo fields, search ranking, embeddings and shared-media routes are unchanged. Runtime dependencies, limits and measured accuracy/memory are documented in [OCR operations](../docs/ocr.md).

## Smart organization

Migration `0005_smart_organization` adds companion preferences, metadata, smart identities, membership overrides and lineage tables. Ordinary catalog fields remain unchanged unless `includeSmart=true` is requested on catalog lists/pages or photo associations. Existing edits/deletes/member changes also record smart overrides. Six owner-authenticated Smart Tag operations are documented under `/ai/smart/*`; admin AI settings/status add `smart_tag_enabled` and `smartTag`. Both Smart Tag switches default off. See [Smart Tag](../docs/smart-tag.md).

## Bulk uploads and quotas

Migration 0006 adds user groups, persistent storage counters and private upload history. Legacy entity response fields remain frozen. `/user/getStorage.capacityBytes` now means effective account quota (nullable for unlimited), and its response adds group/source/over-quota information. `/admin/getUsers` adds corresponding fields and an optional groupId filter. All legacy write routes enforce quota in their file transaction; new routes are listed in OpenAPI.

Media tools add the owner-only capabilities/info, conversion, online download and Live playback/download APIs documented in OpenAPI. Legacy Photo JSON fields remain unchanged. Preview MIME now matches response bytes (including `video/mp4` for MP4 playback); raw downloads retain attachment behavior. Settings includes `OnlineDownload` (default `0`). Upload items can be `waiting` while a sealed Apple Live Photo manifest is inspected. Migration `0007_media_tools` adds private resource/task metadata without changing existing Photo projections.
