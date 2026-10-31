-- name: GetPhoto :one
SELECT * FROM photo WHERE photo_id = @photo_id::int AND photo_visible(storage_id,user_id);

-- name: GetPhotoByUUID :one
SELECT * FROM photo WHERE photo_uuid = @photo_uuid::text;

-- name: GetUserPhoto :one
SELECT * FROM photo WHERE photo_id = @photo_id::int AND user_id = @user_id::int;

-- name: ListPhotosByNamespace :many
SELECT * FROM photo
WHERE user_id = @user_id::int AND ns_id IS NOT DISTINCT FROM sqlc.narg(ns_id)::int AND photo_visible(storage_id, user_id)
ORDER BY photo_id;

-- name: ListPhotosByGallery :many
SELECT p.* FROM photo p JOIN gallery_photo g ON g.photo_id = p.photo_id
WHERE g.ga_id = @ga_id::int AND p.user_id = @user_id::int AND photo_visible(p.storage_id, p.user_id)
ORDER BY p.photo_id;

-- Photos carrying every one of tag_ids.
-- name: ListPhotosByTags :many
SELECT * FROM photo
WHERE user_id = @user_id::int AND photo_visible(storage_id, user_id) AND photo_id IN (
	SELECT tp.photo_id FROM tag_photo tp
	WHERE tp.user_id = @user_id::int AND tp.tag_id = ANY(@tag_ids::int[])
	GROUP BY tp.photo_id HAVING count(DISTINCT tp.tag_id) = @tag_count::bigint)
ORDER BY photo_id;

-- name: SearchPhotosByName :many
SELECT * FROM photo WHERE user_id = @user_id::int AND photo_visible(storage_id, user_id) AND photo_name ILIKE @pattern::text ORDER BY photo_id;

-- name: SearchPhotosByNamespace :many
SELECT * FROM photo
WHERE user_id = @user_id::int AND photo_visible(storage_id, user_id)
  AND ns_id IN (SELECT ns_id FROM namespace WHERE user_id = @user_id::int AND ns_name ILIKE @pattern::text)
ORDER BY photo_id;

-- name: SearchPhotosByGallery :many
SELECT * FROM photo
WHERE user_id = @user_id::int AND photo_visible(storage_id, user_id)
  AND photo_id IN (SELECT gp.photo_id FROM gallery_photo gp JOIN gallery g ON g.ga_id = gp.ga_id
                   WHERE g.user_id = @user_id::int AND g.ga_name ILIKE @pattern::text)
ORDER BY photo_id;

-- name: SearchPhotosByTag :many
SELECT * FROM photo
WHERE user_id = @user_id::int AND photo_visible(storage_id, user_id)
  AND photo_id IN (SELECT tp.photo_id FROM tag_photo tp JOIN tag t ON t.tag_id = tp.tag_id
                   WHERE t.user_id = @user_id::int AND t.tag_name ILIKE @pattern::text)
ORDER BY photo_id;

-- name: ListDateSummary :many
-- day is '' for photos without any date.
SELECT coalesce(to_char(date(coalesce(e.extra_photo_taken_date, p.upload_date)), 'YYYY-MM-DD'), '')::text AS day, count(p.photo_id) AS n
FROM photo p LEFT JOIN photo_extra e ON e.f_photo_id = p.photo_id
WHERE p.user_id = @user_id::int AND photo_visible(p.storage_id, p.user_id) AND lower(p.format) = ANY(@formats::text[])
GROUP BY day ORDER BY day;

-- name: ListMonthPreviewLatest :many
SELECT p.photo_id FROM photo p LEFT JOIN photo_extra e ON e.f_photo_id = p.photo_id
WHERE p.user_id = @user_id::int AND photo_visible(p.storage_id, p.user_id) AND lower(p.format) = ANY(@formats::text[])
  AND coalesce(e.extra_photo_taken_date, p.upload_date) >= @since::timestamp
  AND (sqlc.narg(until)::timestamp IS NULL OR coalesce(e.extra_photo_taken_date, p.upload_date) < sqlc.narg(until)::timestamp)
ORDER BY coalesce(e.extra_photo_taken_date, p.upload_date) DESC, p.photo_id DESC LIMIT 10;

-- name: ListMonthPreviewOldest :many
SELECT p.photo_id FROM photo p LEFT JOIN photo_extra e ON e.f_photo_id = p.photo_id
WHERE p.user_id = @user_id::int AND photo_visible(p.storage_id, p.user_id) AND lower(p.format) = ANY(@formats::text[])
  AND coalesce(e.extra_photo_taken_date, p.upload_date) >= @since::timestamp
  AND (sqlc.narg(until)::timestamp IS NULL OR coalesce(e.extra_photo_taken_date, p.upload_date) < sqlc.narg(until)::timestamp)
ORDER BY coalesce(e.extra_photo_taken_date, p.upload_date) ASC, p.photo_id ASC LIMIT 10;

-- name: ListFolderCovers :many
WITH RECURSIVE dirs AS (
 SELECT n.ns_id AS key,n.ns_id,0 AS depth FROM namespace n JOIN storage_source s ON s.id=n.source_id
 WHERE n.user_id = @user_id::int AND n.ns_id=ANY(@keys::int[]) AND s.enabled
 UNION ALL
 SELECT d.key,n.ns_id,d.depth+1 FROM dirs d JOIN namespace n ON n.ns_parent_id=d.ns_id
 JOIN storage_source s ON s.id=n.source_id WHERE n.user_id = @user_id::int AND s.enabled
), candidates AS (
 SELECT d.key,p.photo_id,d.depth FROM dirs d JOIN photo p ON p.ns_id=d.ns_id
 WHERE p.user_id = @user_id::int AND photo_visible(p.storage_id,p.user_id)
 AND lower(p.format)=ANY(@formats::text[])
)
SELECT n.ns_id::int AS key,
 ((SELECT count(*) FROM namespace c JOIN storage_source s ON s.id=c.source_id WHERE c.ns_parent_id=n.ns_id AND c.user_id = @user_id::int AND s.enabled)
 +(SELECT count(*) FROM photo p WHERE p.ns_id=n.ns_id AND p.user_id = @user_id::int AND photo_visible(p.storage_id,p.user_id)))::bigint AS count,
 ARRAY(SELECT c.photo_id FROM candidates c WHERE c.key=n.ns_id
 AND (c.depth=0 OR NOT EXISTS(SELECT 1 FROM candidates direct WHERE direct.key=n.ns_id AND direct.depth=0))
 ORDER BY c.depth,c.photo_id DESC LIMIT 3)::int[] AS cover_ids
FROM namespace n JOIN storage_source s ON s.id=n.source_id
WHERE n.user_id = @user_id::int AND n.ns_id=ANY(@keys::int[]) AND s.enabled ORDER BY n.ns_id;

-- name: ListGalleryCovers :many
SELECT l.ga_id::int AS key, p.photo_id, p.format FROM photo p JOIN gallery_photo l ON l.photo_id = p.photo_id
WHERE p.user_id = @user_id::int AND photo_visible(p.storage_id, p.user_id) AND l.ga_id = ANY(@keys::int[]) ORDER BY p.photo_id DESC;

-- name: ListTagCovers :many
SELECT l.tag_id::int AS key, p.photo_id, p.format FROM photo p JOIN tag_photo l ON l.photo_id = p.photo_id
WHERE p.user_id = @user_id::int AND photo_visible(p.storage_id, p.user_id) AND l.tag_id = ANY(@keys::int[]) ORDER BY p.photo_id DESC;

-- name: UpsertManualSmartMember :exec
INSERT INTO smart_member (item_id, photo_id, content_version, manual)
SELECT @item_id::int, photo_id, content_version, @manual::int FROM photo WHERE photo_id = @photo_id::int
ON CONFLICT (item_id, photo_id) DO UPDATE SET manual = EXCLUDED.manual;

-- name: InsertTagPhoto :execrows
INSERT INTO tag_photo (photo_id, tag_id, user_id)
SELECT @photo_id::int, @tag_id::int, @user_id::int
WHERE NOT EXISTS (SELECT 1 FROM tag_photo WHERE photo_id = @photo_id::int AND tag_id = @tag_id::int);

-- name: DeleteTagPhoto :execrows
DELETE FROM tag_photo WHERE user_id = @user_id::int AND photo_id = @photo_id::int AND tag_id = @tag_id::int;

-- name: GalleryPhotoExists :one
SELECT EXISTS (SELECT 1 FROM gallery_photo WHERE photo_id = @photo_id::int AND ga_id = @ga_id::int)::boolean;

-- name: InsertGalleryPhoto :exec
INSERT INTO gallery_photo (photo_id, ga_id) VALUES (@photo_id::int, @ga_id::int);

-- name: DeleteGalleryPhoto :exec
DELETE FROM gallery_photo WHERE photo_id = @photo_id::int AND ga_id = @ga_id::int;

-- name: ListPhotoTags :many
SELECT t.tag_id, t.tag_name FROM tag t JOIN tag_photo tp ON tp.tag_id = t.tag_id
WHERE tp.photo_id = @photo_id::int AND t.user_id = @user_id::int ORDER BY t.tag_id;

-- name: ListPhotoGalleries :many
SELECT g.ga_id, g.ga_name FROM gallery g JOIN gallery_photo gp ON gp.ga_id = g.ga_id
WHERE gp.photo_id = @photo_id::int AND g.user_id = @user_id::int ORDER BY g.ga_id;

-- name: RenamePhoto :exec
UPDATE photo SET photo_name = @name::text WHERE photo_id = @photo_id::int;

-- name: SetPhotoVisibility :exec
UPDATE photo SET visibility = @visibility::int WHERE photo_id = @photo_id::int;

-- name: MovePhoto :exec
UPDATE photo SET ns_id = @ns_id::int WHERE photo_id = @photo_id::int;

-- name: GetPhotoHasPreview :one
SELECT has_preview FROM photo_media WHERE photo_id = @photo_id::int;

-- name: InsertPhoto :one
INSERT INTO photo (photo_name, upload_date, format, ns_id, visibility, token, photo_uuid, user_id, content_version, content_sha256)
VALUES (@photo_name::text, @upload_date::timestamp, @format::text, @ns_id::int, @visibility::int, @token::text, @photo_uuid::text, @user_id::int, 1, @content_sha256::text)
RETURNING photo_id;

-- name: InsertPhotoMedia :exec
INSERT INTO photo_media (photo_id, animated, has_preview, live) VALUES (@photo_id::int, @animated::boolean, @has_preview::boolean, false);

-- name: UpsertPhotoMediaAnimated :exec
INSERT INTO photo_media (photo_id, animated) VALUES (@photo_id::int, @animated::boolean)
ON CONFLICT (photo_id) DO UPDATE SET animated = EXCLUDED.animated, has_preview = false;

-- name: InsertPhotoExtra :exec
INSERT INTO photo_extra (f_photo_id, extra_photo_gps_lat, extra_photo_gps_lon, extra_photo_camera, extra_photo_lens,
	extra_photo_aperture, extra_photo_focal, extra_photo_iso, extra_photo_shutter_sec, extra_photo_taken_date)
VALUES (@f_photo_id::int, @gps_lat, @gps_lon, @camera, @lens, @aperture, @focal, @iso, @shutter_sec, @taken_date);

-- name: ReplacePhotoContent :exec
UPDATE photo SET format = @format::text, content_sha256 = @content_sha256::text, content_version = content_version + 1
WHERE photo_id = @photo_id::int;

-- name: GetPhotoOCR :one
SELECT content_version, revision, text, engine_version, updated_at FROM photo_ocr WHERE photo_id = @photo_id::int;

-- name: UpsertPhotoOCR :exec
INSERT INTO photo_ocr (photo_id, content_version, text, languages, engine_version, revision, updated_at)
VALUES (@photo_id::int, @content_version::int, @text::text, @languages::text, @engine_version, @revision::int, @updated_at::timestamptz)
ON CONFLICT (photo_id) DO UPDATE SET content_version = EXCLUDED.content_version, text = EXCLUDED.text,
	languages = EXCLUDED.languages, engine_version = EXCLUDED.engine_version, revision = EXCLUDED.revision, updated_at = EXCLUDED.updated_at;

-- name: SetLockTimeout :exec
SELECT set_config('lock_timeout', @timeout::text, true);

-- name: TryAdvisoryLock :one
SELECT pg_try_advisory_lock(@key::bigint)::boolean;

-- name: AdvisoryUnlock :one
SELECT pg_advisory_unlock(@key::bigint)::boolean;

-- name: GetUserStorageForUpdate :one
SELECT u.uuid, u.used_bytes, u.storage_override, u.max_storage_bytes AS override_bytes, g.max_storage_bytes AS group_bytes
FROM "user" u LEFT JOIN user_group g ON g.id = u.group_id
WHERE u.user_id = @user_id::int FOR NO KEY UPDATE OF u;

-- name: SetUserUsedBytes :exec
UPDATE "user" SET used_bytes = @used_bytes::bigint WHERE user_id = @user_id::int;

-- name: GetPhotoMedia :one
SELECT animated, live, live_status, live_error FROM photo_media WHERE photo_id = @photo_id::int;

-- name: GetPhotoLive :one
SELECT live FROM photo_media WHERE photo_id = @photo_id::int;

-- name: GetPhotoLiveStatus :one
SELECT live, live_status FROM photo_media WHERE photo_id = @photo_id::int;

-- name: MarkLivePreviewReady :exec
UPDATE photo_media SET live_status = 'ready', live_error = NULL WHERE photo_id = @photo_id::int;

-- name: MarkPhotoLive :exec
UPDATE photo_media SET live = true, live_status = @live_status::text, live_error = sqlc.narg(live_error)::text WHERE photo_id = @photo_id::int;

-- name: PhotoNameTaken :one
SELECT EXISTS (SELECT 1 FROM photo WHERE user_id = @user_id::int AND ns_id = @ns_id::int AND photo_name = @photo_name::text AND format = @format::text)::boolean;

-- name: CopyPhotoExtra :exec
UPDATE photo_extra n SET extra_photo_gps_lat = o.extra_photo_gps_lat, extra_photo_gps_lon = o.extra_photo_gps_lon,
	extra_photo_camera = o.extra_photo_camera, extra_photo_lens = o.extra_photo_lens, extra_photo_aperture = o.extra_photo_aperture,
	extra_photo_focal = o.extra_photo_focal, extra_photo_iso = o.extra_photo_iso, extra_photo_shutter_sec = o.extra_photo_shutter_sec,
	extra_photo_taken_date = o.extra_photo_taken_date, extra_photo_note = o.extra_photo_note
FROM photo_extra o WHERE n.f_photo_id = @to_photo_id::int AND o.f_photo_id = @from_photo_id::int;
