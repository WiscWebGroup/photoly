-- name: GetRootNamespace :one
SELECT * FROM namespace WHERE user_id = @user_id::int AND ns_parent_id = -1 LIMIT 1;

-- name: ListChildNamespaces :many
SELECT * FROM namespace
WHERE user_id = @user_id::int AND ns_parent_id IS NOT DISTINCT FROM sqlc.narg(parent_id)::int
AND EXISTS(SELECT 1 FROM storage_source s WHERE s.id=namespace.source_id AND s.enabled)
ORDER BY ns_id;

-- name: ListNamespacePhotos :many
SELECT * FROM photo WHERE user_id = @user_id::int AND ns_id = @ns_id::int ORDER BY photo_id;

-- name: GetNamespace :one
SELECT * FROM namespace WHERE ns_id = @ns_id::int;

-- name: GetNamespaceName :one
SELECT ns_name FROM namespace WHERE ns_id = @ns_id::int;

-- name: GetNamespaceSummary :one
WITH RECURSIVE d(ns_id) AS (
    SELECT n.ns_id FROM namespace n WHERE n.ns_id = @ns_id::int AND n.user_id = @user_id::int
    UNION
    SELECT n.ns_id FROM namespace n JOIN d ON n.ns_parent_id = d.ns_id WHERE n.user_id = @user_id::int AND EXISTS(SELECT 1 FROM storage_source s WHERE s.id=n.source_id AND s.enabled)
)
SELECT (SELECT ns_name FROM namespace WHERE namespace.ns_id = @ns_id::int) AS ns_name,
       (SELECT uuid FROM "user" WHERE user_id = @user_id::int)::text AS uuid,
       (SELECT count(*) FROM d)::bigint AS folders;

-- name: ListNamespaceTreePhotos :many
WITH RECURSIVE d(ns_id) AS (
    SELECT n.ns_id FROM namespace n WHERE n.ns_id = @ns_id::int AND n.user_id = @user_id::int
    UNION
    SELECT n.ns_id FROM namespace n JOIN d ON n.ns_parent_id = d.ns_id WHERE n.user_id = @user_id::int AND EXISTS(SELECT 1 FROM storage_source s WHERE s.id=n.source_id AND s.enabled)
)
SELECT p.photo_uuid::text AS photo_uuid, p.format::text AS format,
       (m.photo_id IS NOT NULL)::bool AS has_media, (e.f_photo_id IS NOT NULL)::bool AS has_extra,
       e.extra_photo_camera, e.extra_photo_lens, e.extra_photo_gps_lat, e.extra_photo_gps_lon
FROM photo p
LEFT JOIN photo_extra e ON e.f_photo_id = p.photo_id
LEFT JOIN photo_media m ON m.photo_id = p.photo_id
WHERE p.user_id = @user_id::int AND p.ns_id IN (SELECT ns_id FROM d) AND photo_visible(p.storage_id, p.user_id)
ORDER BY p.photo_id;

-- name: InsertNamespace :exec
INSERT INTO namespace (ns_name, ns_parent_id, user_id) VALUES (@ns_name::text, @parent_id::int, @user_id::int);

-- name: RenameNamespace :exec
UPDATE namespace SET ns_name = @ns_name::text WHERE ns_id = @ns_id::int;

-- name: MoveNamespace :exec
UPDATE namespace SET ns_parent_id = @parent_id::int WHERE ns_id = @ns_id::int;

-- name: ListChildNamespaceNames :many
SELECT ns_id, ns_name FROM namespace WHERE user_id = @user_id::int AND ns_parent_id = @parent_id::int ORDER BY ns_id;

-- name: ListPhotoIDsInNamespace :many
SELECT photo_id FROM photo WHERE user_id = @user_id::int AND ns_id = @ns_id::int AND photo_visible(storage_id, user_id) ORDER BY photo_id;

-- name: DeleteNamespace :exec
DELETE FROM namespace WHERE ns_id = @ns_id::int;

-- name: ListNamespaceDownloadPhotos :many
SELECT p.photo_id, p.photo_uuid::text AS photo_uuid, p.photo_name, p.format::text AS format, coalesce(m.live, false)::bool AS live
FROM photo p
LEFT JOIN photo_media m ON m.photo_id = p.photo_id
WHERE p.user_id = @user_id::int AND p.ns_id = @ns_id::int AND photo_visible(p.storage_id, p.user_id)
ORDER BY p.photo_id;
