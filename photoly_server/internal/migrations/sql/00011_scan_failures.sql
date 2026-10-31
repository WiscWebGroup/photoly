-- +goose Up
ALTER TABLE storage_scan_failure ADD COLUMN file_version text NOT NULL DEFAULT '';
ALTER TABLE storage_scan_failure ADD COLUMN retryable boolean NOT NULL DEFAULT true;
ALTER TABLE storage_scan_failure ADD COLUMN processor_version text NOT NULL DEFAULT '';
-- +goose Down
ALTER TABLE storage_scan_failure DROP COLUMN file_version, DROP COLUMN retryable, DROP COLUMN processor_version;
