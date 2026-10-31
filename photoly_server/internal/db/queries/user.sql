-- name: GetSetting :one
SELECT setting_value FROM setting WHERE setting_name = @name::text;

-- name: CountUsersByEmail :one
SELECT count(*) FROM "user" WHERE lower(email) = @email::text;

-- name: NewUserAIDefault :one
SELECT coalesce((desired->>'new_user_default')::boolean, false)::boolean
FROM ai_control_state WHERE name = 'global';

-- name: InsertUser :one
INSERT INTO "user" (user_name, email, password, role, uuid, create_date, group_id, used_bytes, ai_search_enabled)
VALUES (@user_name, @email::text, @password::text, @role::text, @uuid::text, @create_date::timestamp,
        (SELECT id FROM user_group WHERE is_default LIMIT 1), 0, @ai_search_enabled)
RETURNING user_id;

-- name: InsertRootNamespace :exec
INSERT INTO namespace (ns_name, ns_parent_id, user_id) VALUES ('/', -1, @user_id::int);

-- name: InsertFavoriteGallery :exec
INSERT INTO gallery (ga_name, user_id, create_date, cover_id, cover_color)
VALUES ('My Favorite', @user_id::int, @create_date::timestamp, 2, '#FFFFFF');

-- name: GetUserLogin :one
SELECT user_id, password FROM "user" WHERE lower(email) = @email::text;

-- name: GetUserInfo :one
SELECT user_id, user_name, email, create_date, role, uuid FROM "user" WHERE user_id = @user_id::int;

-- name: GetUserStorage :one
SELECT u.used_bytes, u.storage_override, u.max_storage_bytes,
       g.id AS group_id, g.name AS group_name, g.max_storage_bytes AS group_max_storage_bytes
FROM "user" u
JOIN user_group g ON g.id = u.group_id
WHERE u.user_id = @user_id::int;

-- name: GetUserPassword :one
SELECT password FROM "user" WHERE user_id = @user_id::int;

-- name: GetUserUUID :one
SELECT uuid FROM "user" WHERE user_id = @user_id::int;

-- name: UpdateUserName :exec
UPDATE "user" SET user_name = @value WHERE user_id = @user_id::int;

-- name: UpdateUserEmail :exec
UPDATE "user" SET email = @value WHERE user_id = @user_id::int;

-- name: UpdateUserPassword :exec
UPDATE "user" SET password = @value WHERE user_id = @user_id::int;

-- name: ListUserPhotoIDs :many
SELECT photo_id FROM photo WHERE user_id = @user_id::int ORDER BY photo_id;

-- name: DeleteUserNamespaces :execrows
DELETE FROM namespace WHERE user_id = @user_id::int;

-- name: DeleteUserTags :execrows
DELETE FROM tag WHERE user_id = @user_id::int;

-- name: DeleteUserGalleries :execrows
DELETE FROM gallery WHERE user_id = @user_id::int;

-- name: DeleteUserPhotos :execrows
DELETE FROM photo WHERE user_id = @user_id::int;

-- name: DeleteUserCreds :execrows
DELETE FROM cred WHERE user_id = @user_id::int;

-- name: DeleteUser :execrows
DELETE FROM "user" WHERE user_id = @user_id::int;
