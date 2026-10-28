-- name: CreateSession :exec
INSERT INTO photoly_session (token, user_id, expires_at)
VALUES (@token, @user_id::int, now() + make_interval(days => @days::int));

-- name: LookupSession :one
SELECT u.user_id, u.uuid, u.email, u.role
FROM photoly_session s
JOIN "user" u ON u.user_id = s.user_id
WHERE s.token = @token AND s.expires_at > now();

-- name: DeleteSession :exec
DELETE FROM photoly_session WHERE token = @token;

-- name: DeleteUserSessions :exec
DELETE FROM photoly_session WHERE user_id = @user_id::int;

-- name: DeleteExpiredSessions :exec
DELETE FROM photoly_session WHERE expires_at <= now();
