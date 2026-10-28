-- Admin policy lock: shared for ordinary edits, exclusive for quota and group edits.

-- name: LockPolicyShared :exec
SELECT pg_advisory_xact_lock_shared(74831003);

-- name: LockPolicyExclusive :exec
SELECT pg_advisory_xact_lock(74831003);

-- name: LockUser :one
SELECT user_id FROM "user" WHERE user_id = @user_id::int FOR UPDATE;

-- name: Ping :one
SELECT 1::bigint;

-- name: ServerVersion :one
SELECT current_setting('server_version')::text;

-- name: ListSettings :many
SELECT setting_name, setting_value FROM setting ORDER BY setting_id;

-- name: UpdateSetting :execrows
UPDATE setting SET setting_value = @value::text WHERE setting_name = @name::text;

-- name: CountUsers :one
SELECT count(*) FROM "user"
WHERE (sqlc.narg(search)::text IS NULL OR user_name ILIKE sqlc.narg(search) OR email ILIKE sqlc.narg(search))
  AND (sqlc.narg(role)::text IS NULL OR role = sqlc.narg(role))
  AND (sqlc.narg(group_id)::int IS NULL OR group_id = sqlc.narg(group_id));

-- name: ListUsers :many
SELECT user_id, user_name, email, create_date, role, uuid, group_id, storage_override, max_storage_bytes, used_bytes
FROM "user"
WHERE (sqlc.narg(search)::text IS NULL OR user_name ILIKE sqlc.narg(search) OR email ILIKE sqlc.narg(search))
  AND (sqlc.narg(role)::text IS NULL OR role = sqlc.narg(role))
  AND (sqlc.narg(group_id)::int IS NULL OR group_id = sqlc.narg(group_id))
ORDER BY user_id OFFSET @row_offset::bigint LIMIT @row_limit::bigint;

-- name: GetAdminUser :one
SELECT user_id, user_name, email, create_date, role, uuid, group_id, storage_override, max_storage_bytes, used_bytes
FROM "user" WHERE user_id = @user_id::int;

-- name: CountUserPhotos :one
SELECT count(*) FROM photo WHERE user_id = @user_id::int;

-- name: EmailExists :one
SELECT EXISTS (SELECT 1 FROM "user" WHERE email = @email::text);

-- name: GetUserIDByEmail :one
SELECT user_id FROM "user" WHERE email = @email::text;

-- name: UpdateUserRole :exec
UPDATE "user" SET role = @value WHERE user_id = @user_id::int;

-- name: UpdateUserStorage :exec
UPDATE "user"
SET group_id = @group_id::int, storage_override = @storage_override::bool, max_storage_bytes = sqlc.narg(max_storage_bytes)::bigint
WHERE user_id = @user_id::int;

-- name: ListUserGroups :many
SELECT g.id, g.name, g.max_storage_bytes, g.is_default,
       (SELECT count(*) FROM "user" WHERE group_id = g.id) AS member_count
FROM user_group g ORDER BY g.id;

-- name: GetUserGroup :one
SELECT * FROM user_group WHERE id = @id::int;

-- name: LockUserGroup :one
SELECT id FROM user_group WHERE id = @id::int FOR UPDATE;

-- name: InsertUserGroup :one
INSERT INTO user_group (name, max_storage_bytes) VALUES (@name::text, sqlc.narg(max_storage_bytes)::bigint) RETURNING id;

-- name: UpdateUserGroup :exec
UPDATE user_group SET name = @name::text, max_storage_bytes = sqlc.narg(max_storage_bytes)::bigint WHERE id = @id::int;

-- name: ReassignUserGroup :exec
UPDATE "user" SET group_id = @to_group::int WHERE group_id = @from_group::int;

-- name: DeleteUserGroup :exec
DELETE FROM user_group WHERE id = @id::int;
