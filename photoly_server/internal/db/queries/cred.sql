-- name: GetCredByToken :one
SELECT c.user_id::int AS user_id, coalesce(c."authorization", '')::text AS authorization
FROM cred c
JOIN "user" u ON u.user_id = c.user_id
WHERE c.token = @token::text;

-- name: InsertCred :exec
INSERT INTO cred (user_id, token, "authorization") VALUES (@user_id::int, @token::text, @auth::text);

-- name: UpdateCredAuthorization :exec
UPDATE cred SET "authorization" = @auth::text WHERE cred_id = @cred_id::int;

-- name: DeleteCred :exec
DELETE FROM cred WHERE cred_id = @cred_id::int;

-- name: GetPhotoForCred :one
SELECT p.photo_id, u.uuid AS owner_uuid, p.format
FROM photo p
JOIN "user" u ON u.user_id = p.user_id
WHERE p.user_id = @user_id::int AND p.photo_uuid = @photo_uuid::text
AND EXISTS(SELECT 1 FROM storage_source s WHERE s.id=p.source_id AND s.enabled);

-- name: ListUserCreds :many
SELECT * FROM cred WHERE user_id = @user_id::int ORDER BY cred_id;

-- name: ListCredentialMedia :many
SELECT p.photo_id,p.photo_uuid::text AS uuid,p.photo_name::text AS name,p.format::text AS format,p.ns_id,p.content_version,
 coalesce(m.animated,false)::boolean AS animated,coalesce(m.live,false)::boolean AS live,
 ARRAY(SELECT category FROM effective_media_category c WHERE c.photo_id=p.photo_id ORDER BY category)::text[] AS categories
FROM photo p JOIN storage_source s ON s.id=p.source_id LEFT JOIN photo_media m ON m.photo_id=p.photo_id
WHERE p.user_id= @user_id::int AND s.enabled
 AND (@ns_id::int=0 OR p.ns_id= @ns_id::int)
 AND (@uuid::text='' OR p.photo_uuid= @uuid::text)
 AND (@after_id::int=0 OR p.photo_id< @after_id::int)
 AND (@category::text='' OR EXISTS(SELECT 1 FROM effective_media_category c WHERE c.photo_id=p.photo_id AND c.category= @category::text))
ORDER BY p.photo_id DESC LIMIT @page_limit::int;
