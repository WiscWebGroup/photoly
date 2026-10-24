-- +goose Up
-- Login sessions (replaces Redis). UNLOGGED: lost on crash, users sign in again.
CREATE UNLOGGED TABLE photoly_session (
    token text PRIMARY KEY,
    user_id integer NOT NULL REFERENCES "user" (user_id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL
);
CREATE INDEX ix_photoly_session_user_id ON photoly_session (user_id);

-- +goose Down
DROP TABLE photoly_session;
