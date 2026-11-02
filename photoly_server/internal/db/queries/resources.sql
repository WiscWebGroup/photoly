-- name: SaveResourceHistory :exec
INSERT INTO resource_history(boot_id,bucket,payload) VALUES(@boot_id,@bucket,@payload)
ON CONFLICT(boot_id,bucket) DO UPDATE SET payload=excluded.payload;

-- name: ListResourceHistory :many
SELECT payload FROM resource_history WHERE bucket >= @since::timestamptz ORDER BY bucket,boot_id;

-- name: PruneResourceHistory :exec
DELETE FROM resource_history WHERE bucket < @before::timestamptz;
