-- +goose Up
-- This release deliberately does not migrate the old flat media layout.
-- +goose StatementBegin
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM photo) OR EXISTS (SELECT 1 FROM namespace WHERE ns_parent_id <> -1 OR ns_parent_id IS NULL) THEN
    RAISE EXCEPTION 'Real folders require a fresh library. Existing media and folders were not modified; back up and use a new database and upload directory.';
  END IF;
END $$;
-- +goose StatementEnd

CREATE TABLE storage_source (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id integer NOT NULL REFERENCES "user" ON DELETE CASCADE,
  kind text NOT NULL CHECK (kind IN ('local','s3','webdav')),
  is_default boolean NOT NULL DEFAULT false,
  root_path text NOT NULL DEFAULT '',
  config jsonb NOT NULL DEFAULT '{}',
  secret bytea,
  enabled boolean NOT NULL DEFAULT true,
  revision bigint NOT NULL DEFAULT 0,
  root_identity text NOT NULL DEFAULT '',
  scan_status text NOT NULL DEFAULT 'pending',
  scan_error text,
  scanned_at timestamptz,
  UNIQUE(id,user_id)
);
CREATE UNIQUE INDEX storage_default_user ON storage_source(user_id) WHERE is_default;
CREATE UNIQUE INDEX storage_s3_user ON storage_source(user_id) WHERE kind='s3';
INSERT INTO storage_source(user_id,kind,is_default,root_path)
SELECT user_id,'local',true,uuid FROM "user";

ALTER TABLE namespace ADD COLUMN source_id bigint;
ALTER TABLE namespace ADD COLUMN source_type text NOT NULL DEFAULT 'local';
ALTER TABLE namespace ADD COLUMN is_mount boolean NOT NULL DEFAULT false;
ALTER TABLE namespace ADD COLUMN fs_identity text NOT NULL DEFAULT '';
ALTER TABLE namespace ADD COLUMN missing_scans integer NOT NULL DEFAULT 0;
UPDATE namespace n SET source_id=s.id FROM storage_source s WHERE s.user_id=n.user_id AND s.is_default;
ALTER TABLE namespace ALTER COLUMN source_id SET NOT NULL;
ALTER TABLE namespace ADD CONSTRAINT namespace_source_owner FOREIGN KEY(source_id,user_id) REFERENCES storage_source(id,user_id);
ALTER TABLE namespace ADD CONSTRAINT namespace_id_owner_source UNIQUE(ns_id,user_id,source_id);
CREATE UNIQUE INDEX namespace_user_root ON namespace(user_id) WHERE ns_parent_id=-1;
ALTER TABLE namespace ADD CONSTRAINT namespace_sibling_name UNIQUE(user_id,ns_parent_id,ns_name) DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE photo ADD COLUMN source_id bigint NOT NULL;
ALTER TABLE photo ADD COLUMN file_name text NOT NULL;
ALTER TABLE photo ADD COLUMN fs_identity text NOT NULL DEFAULT '';
ALTER TABLE photo ADD COLUMN fs_size bigint NOT NULL DEFAULT 0;
ALTER TABLE photo ADD COLUMN fs_mtime bigint NOT NULL DEFAULT 0;
ALTER TABLE photo ADD COLUMN missing_scans integer NOT NULL DEFAULT 0;
ALTER TABLE photo ADD CONSTRAINT photo_source_owner FOREIGN KEY(source_id,user_id) REFERENCES storage_source(id,user_id);
ALTER TABLE photo ADD CONSTRAINT photo_namespace_owner FOREIGN KEY(ns_id,user_id,source_id) REFERENCES namespace(ns_id,user_id,source_id);
ALTER TABLE photo ADD CONSTRAINT photo_file_name UNIQUE(ns_id,file_name) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE media_asset (
  photo_id integer NOT NULL REFERENCES photo ON DELETE CASCADE,
  role text NOT NULL CHECK(role IN ('original','thumbnail','preview','live_original','live_preview')),
  filename text NOT NULL,
  display_name text NOT NULL DEFAULT '',
  PRIMARY KEY(photo_id,role)
);
CREATE TABLE storage_observation (
  source_id bigint NOT NULL REFERENCES storage_source ON DELETE CASCADE,
  path text NOT NULL,
  identity text NOT NULL,
  size bigint NOT NULL,
  mtime bigint NOT NULL,
  PRIMARY KEY(source_id,path)
);

DROP TRIGGER IF EXISTS photo_storage ON photo;
DROP FUNCTION IF EXISTS photo_set_storage();
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION photo_visible(sid text, uid integer) RETURNS boolean LANGUAGE sql STABLE AS $$
  SELECT EXISTS(SELECT 1 FROM storage_source WHERE id::text=sid AND user_id=uid AND enabled)
$$;
CREATE FUNCTION namespace_source() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE p namespace; s storage_source;
BEGIN
  IF NEW.ns_parent_id=-1 THEN
    INSERT INTO storage_source(user_id,kind,is_default,root_path)
      SELECT NEW.user_id,'local',true,uuid FROM "user" WHERE user_id=NEW.user_id
      ON CONFLICT(user_id) WHERE is_default DO NOTHING;
    SELECT * INTO s FROM storage_source WHERE user_id=NEW.user_id AND is_default;
    NEW.source_id=s.id;
  ELSE
    SELECT * INTO p FROM namespace WHERE ns_id=NEW.ns_parent_id AND user_id=NEW.user_id;
    IF NOT FOUND THEN RAISE EXCEPTION 'invalid folder parent' USING ERRCODE='23514'; END IF;
    IF NEW.source_id IS NULL THEN NEW.source_id=p.source_id; END IF;
    IF NOT NEW.is_mount AND NEW.source_id<>p.source_id THEN
      RAISE EXCEPTION 'cross source move is not supported' USING ERRCODE='23514';
    END IF;
    SELECT * INTO s FROM storage_source WHERE id=NEW.source_id AND user_id=NEW.user_id;
  END IF;
  IF s.id IS NULL THEN RAISE EXCEPTION 'invalid source' USING ERRCODE='23514'; END IF;
  NEW.source_type=s.kind;
  RETURN NEW;
END $$;
CREATE TRIGGER namespace_source BEFORE INSERT OR UPDATE OF ns_parent_id,source_id,user_id ON namespace FOR EACH ROW EXECUTE FUNCTION namespace_source();
CREATE FUNCTION photo_source() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE sid bigint;
BEGIN
  SELECT source_id INTO sid FROM namespace WHERE ns_id=NEW.ns_id AND user_id=NEW.user_id;
  IF sid IS NULL THEN RAISE EXCEPTION 'invalid photo folder' USING ERRCODE='23514'; END IF;
  IF TG_OP='UPDATE' AND sid<>OLD.source_id THEN RAISE EXCEPTION 'cross source move is not supported' USING ERRCODE='23514'; END IF;
  NEW.source_id=sid; NEW.storage_id=sid::text;
  IF NEW.file_name IS NULL THEN NEW.file_name=NEW.photo_name||'.'||NEW.format; END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER photo_source BEFORE INSERT OR UPDATE OF ns_id,user_id,source_id ON photo FOR EACH ROW EXECUTE FUNCTION photo_source();
CREATE FUNCTION source_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP='DELETE' THEN UPDATE storage_source SET revision=revision+1 WHERE id=OLD.source_id; RETURN OLD;
  ELSE UPDATE storage_source SET revision=revision+1 WHERE id=NEW.source_id; RETURN NEW; END IF;
END $$;
CREATE TRIGGER namespace_revision AFTER INSERT OR UPDATE OR DELETE ON namespace FOR EACH ROW EXECUTE FUNCTION source_revision();
CREATE TRIGGER photo_revision AFTER INSERT OR UPDATE OR DELETE ON photo FOR EACH ROW EXECUTE FUNCTION source_revision();
-- +goose StatementEnd

DROP FUNCTION storage_active();
DROP TABLE storage_driver_config;
DROP TABLE storage_config;
ALTER TABLE "user" DROP COLUMN show_other_storage;

-- +goose Down
-- Reverting the schema cannot safely revert physical directories.
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Restore a matching database and media backup to downgrade real folders'; END $$;
-- +goose StatementEnd
