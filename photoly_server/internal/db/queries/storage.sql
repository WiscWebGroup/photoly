-- name: GetStorageConfig :one
SELECT driver, backend_id, config, secret FROM storage_config WHERE id = 1;

-- name: SetStorageConfig :exec
UPDATE storage_config SET driver = @driver::text, backend_id = @backend_id::text, config = @config::jsonb, secret = @secret::bytea, updated_at = now()
WHERE id = 1;

-- name: ListStorageDriverConfigs :many
SELECT driver, config, secret FROM storage_driver_config;

-- name: SetStorageDriverConfig :exec
INSERT INTO storage_driver_config (driver, config, secret) VALUES (@driver::text, @config::jsonb, @secret::bytea)
ON CONFLICT (driver) DO UPDATE SET config = EXCLUDED.config, secret = EXCLUDED.secret;
