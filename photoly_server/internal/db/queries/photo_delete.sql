-- name: GetPhotoFileNames :one
SELECT photo_uuid::text AS photo_uuid, format::text AS format FROM photo WHERE photo_id = @photo_id::int;

-- name: DeletePhotoMedia :exec
DELETE FROM photo_media WHERE photo_id = @photo_id::int;

-- name: DeletePhoto :exec
DELETE FROM photo WHERE photo_id = @photo_id::int;
