-- +goose Up
-- Last saved settings per driver, so switching to another backend and back keeps them.
-- Same shape as storage_config: plain fields in `config`, sealed secret fields in `secret`.
CREATE TABLE storage_driver_config (
    driver text PRIMARY KEY,
    config jsonb NOT NULL DEFAULT '{}'::jsonb,
    secret bytea
);
INSERT INTO storage_driver_config (driver, config, secret)
SELECT driver, config, secret FROM storage_config WHERE driver <> 'local';

-- +goose Down
DROP TABLE storage_driver_config;
