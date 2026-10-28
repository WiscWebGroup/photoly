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
WHERE p.user_id = @user_id::int AND p.photo_uuid = @photo_uuid::text;

-- name: ListUserCreds :many
SELECT * FROM cred WHERE user_id = @user_id::int ORDER BY cred_id;
