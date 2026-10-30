-- Owner lookups behind db.Owner; a NULL owner reads as 0, which is never a real user_id.

-- name: GetPhotoOwner :one
SELECT coalesce(user_id, 0)::int AS user_id FROM photo WHERE photo_id = @id::int AND photo_visible(storage_id,user_id);

-- name: GetNamespaceOwner :one
SELECT coalesce(user_id, 0)::int AS user_id FROM namespace WHERE ns_id = @id::int AND EXISTS(SELECT 1 FROM storage_source s WHERE s.id=namespace.source_id AND s.enabled);

-- name: GetGalleryOwner :one
SELECT coalesce(user_id, 0)::int AS user_id FROM gallery WHERE ga_id = @id::int;

-- name: GetTagOwner :one
SELECT coalesce(user_id, 0)::int AS user_id FROM tag WHERE tag_id = @id::int;

-- name: GetCredOwner :one
SELECT coalesce(user_id, 0)::int AS user_id FROM cred WHERE cred_id = @id::int;
