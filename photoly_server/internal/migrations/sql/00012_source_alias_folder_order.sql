-- +goose Up
DROP INDEX storage_s3_user;
ALTER TABLE namespace ADD COLUMN display_order bigint NOT NULL DEFAULT 0;
WITH ranked AS (
 SELECT ns_id, row_number() OVER (PARTITION BY user_id,ns_parent_id ORDER BY ns_id) AS position FROM namespace
) UPDATE namespace n SET display_order=r.position FROM ranked r WHERE r.ns_id=n.ns_id;
-- All writers (including scanners) append new or moved-in folders. Renames and
-- explicit order changes leave the position intact. Writers hold the user lock.
-- +goose StatementBegin
CREATE FUNCTION namespace_append_order() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='INSERT' OR NEW.ns_parent_id IS DISTINCT FROM OLD.ns_parent_id THEN
  SELECT coalesce(max(display_order),0)+1 INTO NEW.display_order FROM namespace
   WHERE user_id=NEW.user_id AND ns_parent_id IS NOT DISTINCT FROM NEW.ns_parent_id;
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER namespace_append_order BEFORE INSERT OR UPDATE OF ns_parent_id ON namespace
 FOR EACH ROW EXECUTE FUNCTION namespace_append_order();
CREATE INDEX namespace_display_order ON namespace(user_id,ns_parent_id,display_order,ns_id);
UPDATE storage_source s SET display_name=n.ns_name FROM namespace n WHERE n.source_id=s.id AND n.is_mount AND s.kind='s3';
-- +goose Down
CREATE UNIQUE INDEX storage_s3_user ON storage_source(user_id) WHERE kind='s3';
DROP TRIGGER namespace_append_order ON namespace;
DROP FUNCTION namespace_append_order();
DROP INDEX namespace_display_order;
ALTER TABLE namespace DROP COLUMN display_order;
