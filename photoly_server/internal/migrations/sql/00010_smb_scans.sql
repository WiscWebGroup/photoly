-- +goose Up
ALTER TABLE storage_source DROP CONSTRAINT storage_source_kind_check;
ALTER TABLE storage_source ADD CONSTRAINT storage_source_kind_check CHECK(kind IN ('local','s3','webdav','smb'));
ALTER TABLE storage_source ADD COLUMN scan_phase text NOT NULL DEFAULT 'idle';
ALTER TABLE storage_source ADD COLUMN scan_discovered bigint NOT NULL DEFAULT 0;
ALTER TABLE storage_source ADD COLUMN scan_waiting bigint NOT NULL DEFAULT 0;
ALTER TABLE storage_source ADD COLUMN scan_unchanged bigint NOT NULL DEFAULT 0;
ALTER TABLE storage_source ADD COLUMN full_scanned_at timestamptz;
CREATE TABLE storage_scan_job (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 source_id bigint NOT NULL REFERENCES storage_source(id) ON DELETE CASCADE,
 namespace_id bigint REFERENCES namespace(ns_id) ON DELETE CASCADE,
 force boolean NOT NULL DEFAULT false,
 state text NOT NULL DEFAULT 'queued' CHECK(state IN ('queued','running','complete','error')),
 created_at timestamptz NOT NULL DEFAULT now(),
 finished_at timestamptz,
 error text
);
CREATE INDEX storage_scan_job_pending ON storage_scan_job(source_id,id) WHERE state IN ('queued','running');
CREATE INDEX storage_scan_job_source_history ON storage_scan_job(source_id,id DESC);
-- +goose Down
DROP TABLE storage_scan_job;
ALTER TABLE storage_source DROP COLUMN scan_phase,DROP COLUMN scan_discovered,DROP COLUMN scan_waiting,DROP COLUMN scan_unchanged,DROP COLUMN full_scanned_at;
-- SMB data must be explicitly detached before a downgrade.
ALTER TABLE storage_source DROP CONSTRAINT storage_source_kind_check;
ALTER TABLE storage_source ADD CONSTRAINT storage_source_kind_check CHECK(kind IN ('local','s3','webdav'));
