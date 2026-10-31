-- +goose Up
ALTER TABLE storage_source ADD COLUMN managed_by text NOT NULL DEFAULT 'user' CHECK (managed_by IN ('user','admin','system'));
ALTER TABLE storage_source ADD COLUMN display_name text NOT NULL DEFAULT '';
ALTER TABLE storage_source ADD COLUMN scan_interval integer NOT NULL DEFAULT 60 CHECK (scan_interval >= 60);
ALTER TABLE storage_source ADD COLUMN config_version bigint NOT NULL DEFAULT 1;
ALTER TABLE storage_source ADD COLUMN scan_requested boolean NOT NULL DEFAULT true;
ALTER TABLE storage_source ADD COLUMN scan_force boolean NOT NULL DEFAULT false;
ALTER TABLE storage_source ADD COLUMN scan_started_at timestamptz;
ALTER TABLE storage_source ADD COLUMN scan_finished_at timestamptz;
ALTER TABLE storage_source ADD COLUMN scan_completed bigint NOT NULL DEFAULT 0;
ALTER TABLE storage_source ADD COLUMN scan_total bigint NOT NULL DEFAULT 0;
ALTER TABLE storage_source ADD COLUMN scan_retries bigint NOT NULL DEFAULT 0;
UPDATE storage_source SET managed_by='system',display_name='Photoly' WHERE is_default;
UPDATE storage_source s SET display_name=n.ns_name FROM namespace n WHERE n.source_id=s.id AND n.is_mount;
ALTER TABLE media_asset ADD COLUMN namespace_id integer REFERENCES namespace(ns_id);
ALTER TABLE media_asset ADD COLUMN version text NOT NULL DEFAULT '';
ALTER TABLE media_asset ADD COLUMN content_version bigint NOT NULL DEFAULT 1;
ALTER TABLE media_asset ADD COLUMN fs_identity text NOT NULL DEFAULT '';
ALTER TABLE media_asset ADD COLUMN fs_size bigint NOT NULL DEFAULT 0;
ALTER TABLE media_asset ADD COLUMN fs_mtime bigint NOT NULL DEFAULT 0;
ALTER TABLE media_asset ADD COLUMN content_sha256 text NOT NULL DEFAULT '';
ALTER TABLE media_asset ADD COLUMN missing_scans integer NOT NULL DEFAULT 0;
ALTER TABLE photo ADD COLUMN remote_version text NOT NULL DEFAULT '';
ALTER TABLE storage_observation ADD COLUMN version text NOT NULL DEFAULT '';
CREATE TABLE storage_scan_failure (
 source_id bigint NOT NULL REFERENCES storage_source ON DELETE CASCADE,
 path text NOT NULL,
 error text NOT NULL,
 attempted_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(source_id,path)
);
CREATE INDEX storage_source_scan_due ON storage_source(scan_finished_at) WHERE enabled AND kind <> 's3';
-- +goose StatementBegin
CREATE FUNCTION asset_namespace_source() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.namespace_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM namespace n JOIN photo p ON p.photo_id=NEW.photo_id WHERE n.ns_id=NEW.namespace_id AND n.user_id=p.user_id AND n.source_id=p.source_id) THEN
  RAISE EXCEPTION 'asset namespace belongs to another storage source' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER asset_namespace_source BEFORE INSERT OR UPDATE OF namespace_id,photo_id ON media_asset FOR EACH ROW EXECUTE FUNCTION asset_namespace_source();
-- +goose StatementEnd
-- +goose Down
DROP TRIGGER asset_namespace_source ON media_asset;
DROP FUNCTION asset_namespace_source();
DROP TABLE storage_scan_failure;
DROP INDEX storage_source_scan_due;
ALTER TABLE storage_observation DROP COLUMN version;
ALTER TABLE photo DROP COLUMN remote_version;
ALTER TABLE media_asset DROP COLUMN namespace_id, DROP COLUMN version, DROP COLUMN content_version, DROP COLUMN fs_identity, DROP COLUMN fs_size, DROP COLUMN fs_mtime, DROP COLUMN content_sha256, DROP COLUMN missing_scans;
ALTER TABLE storage_source DROP COLUMN managed_by, DROP COLUMN display_name, DROP COLUMN scan_interval, DROP COLUMN config_version, DROP COLUMN scan_requested, DROP COLUMN scan_force, DROP COLUMN scan_started_at, DROP COLUMN scan_finished_at, DROP COLUMN scan_completed, DROP COLUMN scan_total, DROP COLUMN scan_retries;
