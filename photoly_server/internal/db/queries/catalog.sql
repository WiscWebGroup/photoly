-- name: ListTags :many
SELECT * FROM tag WHERE user_id = @user_id::int ORDER BY tag_id LIMIT @row_limit::bigint OFFSET @row_offset::bigint;

-- name: ListGalleries :many
SELECT * FROM gallery WHERE user_id = @user_id::int ORDER BY ga_id LIMIT @row_limit::bigint OFFSET @row_offset::bigint;

-- name: CountTags :one
SELECT count(*) FROM tag WHERE user_id = @user_id::int;

-- name: CountGalleries :one
SELECT count(*) FROM gallery WHERE user_id = @user_id::int;

-- name: InsertTag :exec
INSERT INTO tag (user_id, tag_name) VALUES (@user_id::int, @tag_name::text);

-- name: InsertGallery :exec
INSERT INTO gallery (user_id, ga_name, create_date, cover_id, cover_color)
VALUES (@user_id::int, @ga_name::text, @create_date::timestamp, @cover_id, @cover_color);

-- name: UpdateTag :exec
UPDATE tag SET tag_name = @tag_name::text WHERE tag_id = @tag_id::int;

-- name: UpdateGallery :exec
UPDATE gallery SET ga_name = @ga_name::text, cover_color = @cover_color, cover_id = @cover_id WHERE ga_id = @ga_id::int;

-- name: DeleteTag :exec
DELETE FROM tag WHERE tag_id = @tag_id::int;

-- name: DeleteGallery :exec
DELETE FROM gallery WHERE ga_id = @ga_id::int;

-- name: GetTagName :one
SELECT tag_name::text FROM tag WHERE tag_id = @tag_id::int;

-- name: GetGallery :one
SELECT * FROM gallery WHERE ga_id = @ga_id::int;

-- The smart_item bound to a tag or a gallery: pass the one id that applies, NULL for the other.

-- name: GetSmartItemFor :one
SELECT item_id, user_id FROM smart_item
WHERE tag_id = sqlc.narg(tag_id)::int OR ga_id = sqlc.narg(ga_id)::int;

-- name: ListSmartItemsFor :many
SELECT tag_id, ga_id, item_id, source, name, custom_name, suppressed, basis FROM smart_item
WHERE tag_id = ANY(@tag_ids::int[]) OR ga_id = ANY(@ga_ids::int[]);

-- name: RenameSmartItem :exec
UPDATE smart_item SET custom_name = @custom_name::text WHERE item_id = @item_id::int;

-- name: RenameSmartItemWithCover :exec
UPDATE smart_item SET custom_name = @custom_name::text, cover = @cover::jsonb WHERE item_id = @item_id::int;

-- name: SuppressSmartItem :exec
UPDATE smart_item SET suppressed = true, ga_id = NULL, tag_id = NULL WHERE item_id = @item_id::int;

-- name: BumpSmartRevision :exec
UPDATE smart_preference SET revision = revision + 1 WHERE user_id = @user_id::int;

-- name: GetGalleryDownloadOwner :one
SELECT g.ga_name::text AS ga_name, u.uuid::text AS uuid
FROM gallery g, "user" u
WHERE g.ga_id = @ga_id::int AND u.user_id = @user_id::int;

-- name: ListGalleryDownloadPhotos :many
SELECT p.photo_id, p.photo_uuid::text AS photo_uuid, p.photo_name::text AS photo_name, p.format::text AS format
FROM photo p
JOIN gallery_photo gp ON gp.photo_id = p.photo_id
WHERE gp.ga_id = @ga_id::int AND p.user_id = @user_id::int
ORDER BY p.photo_id;

-- A gallery id of NULL lists every geotagged photo of the user.

-- name: ListMapPhotos :many
SELECT p.photo_id, p.photo_name, p.format, p.ns_id, p.token, p.photo_uuid, p.upload_date,
       e.extra_photo_gps_lat, e.extra_photo_gps_lon, e.extra_photo_taken_date
FROM photo p
JOIN photo_extra e ON e.f_photo_id = p.photo_id
WHERE p.user_id = @user_id::int
  AND (sqlc.narg(ga_id)::int IS NULL
       OR EXISTS (SELECT 1 FROM gallery_photo gp WHERE gp.photo_id = p.photo_id AND gp.ga_id = sqlc.narg(ga_id)::int))
ORDER BY p.photo_id;

-- name: ListMapPlaces :many
SELECT p.photo_id, e.extra_photo_gps_lat::float8 AS lat, e.extra_photo_gps_lon::float8 AS lon
FROM photo p
JOIN photo_extra e ON e.f_photo_id = p.photo_id
WHERE p.user_id = @user_id::int AND e.extra_photo_gps_lat IS NOT NULL AND e.extra_photo_gps_lon IS NOT NULL
ORDER BY p.photo_id;
