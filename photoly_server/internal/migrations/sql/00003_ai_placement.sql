-- +goose Up
-- Phase 5: browser-side embeddings and per-user placement.
-- source: where a vector was computed ('server' or 'client'); spot_check marks client rows
-- the server re-embeds when idle to confirm the browser's numbers.
ALTER TABLE photo_embedding_state
    ADD COLUMN source varchar(16) NOT NULL DEFAULT 'server',
    ADD COLUMN spot_check boolean NOT NULL DEFAULT false,
    ADD COLUMN spot_checked_at timestamptz;

CREATE INDEX ix_embedding_spot_check ON photo_embedding_state (profile_id, state_id)
    WHERE spot_check AND spot_checked_at IS NULL;

-- A user's own choice of where their embeddings run; used where the admin placement is "user".
CREATE TABLE user_ai_preference (
    user_id integer PRIMARY KEY REFERENCES "user"(user_id) ON DELETE CASCADE,
    image varchar(16) NOT NULL DEFAULT 'server',
    text varchar(16) NOT NULL DEFAULT 'server',
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_user_ai_pref_image CHECK (image IN ('server', 'browser')),
    CONSTRAINT ck_user_ai_pref_text CHECK (text IN ('server', 'browser'))
);

-- +goose Down
DROP TABLE user_ai_preference;
DROP INDEX ix_embedding_spot_check;
ALTER TABLE photo_embedding_state DROP COLUMN spot_checked_at, DROP COLUMN spot_check, DROP COLUMN source;
