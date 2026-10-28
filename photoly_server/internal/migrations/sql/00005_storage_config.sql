-- +goose Up
-- Single-row admin setting: which storage backend holds media. `config` has the
-- non-secret driver fields, `secret` the AES-GCM sealed secret fields.
CREATE TABLE storage_config (
    id smallint PRIMARY KEY CHECK (id = 1),
    driver text NOT NULL DEFAULT 'local',
    config jsonb NOT NULL DEFAULT '{}'::jsonb,
    secret bytea,
    updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO storage_config (id) VALUES (1);

-- +goose Down
DROP TABLE storage_config;
