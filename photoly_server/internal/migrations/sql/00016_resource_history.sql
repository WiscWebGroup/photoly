-- +goose Up
CREATE TABLE resource_history (
  boot_id text NOT NULL,
  bucket timestamptz NOT NULL,
  payload jsonb NOT NULL,
  PRIMARY KEY (boot_id, bucket)
);
CREATE INDEX resource_history_time ON resource_history(bucket);

-- +goose Down
DROP TABLE resource_history;
