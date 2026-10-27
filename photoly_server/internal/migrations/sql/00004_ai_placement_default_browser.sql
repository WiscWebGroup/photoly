-- +goose Up
-- Users with no saved choice run AI in the browser (rows already saved keep their choice).
ALTER TABLE user_ai_preference ALTER COLUMN image SET DEFAULT 'browser', ALTER COLUMN text SET DEFAULT 'browser';

-- +goose Down
ALTER TABLE user_ai_preference ALTER COLUMN image SET DEFAULT 'server', ALTER COLUMN text SET DEFAULT 'server';
