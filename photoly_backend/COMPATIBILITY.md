# API compatibility with Spring e6619ab

The migration targets all **90 annotated business endpoints**: user 11, admin 19, photo 27, namespace 9, gallery 6, tag 5, cred 11, photo-extra 2. The four empty relationship/setting Controllers exposed no methods and introduce no extra routes. `/healthz` is a new operational route, and `POST /photo/replace` is a deliberate extension (in-place image replacement for the editor's overwrite option). Framework-generated Spring/Swagger pages are not business APIs and are not reproduced.

`tests/contracts/spring_routes.json` preserves the original Java signatures; `spring_entities.json` preserves entity response fields. Both were extracted from Java before removing it. `tests/contract_cases.py` independently enumerates requests and expected response families; tests require its inventory to equal the frozen Spring inventory and the Flask route map. These are source-derived compatibility tests, not a claim that the Java server was run for differential testing.

`spring_field_types.json` and `spring_schema.json` additionally freeze the Java field types and SQL table definitions directly from `e6619ab`. Tests compare all eleven tables' columns, SQL types, nullability and primary keys, including DOUBLE precision for GPS and aperture values.

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
