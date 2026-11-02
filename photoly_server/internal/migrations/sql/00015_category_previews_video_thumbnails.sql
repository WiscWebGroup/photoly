-- +goose Up
UPDATE media_classification SET state='pending',attempts=0,next_attempt=now(),lease_token=NULL,error=NULL WHERE rule_version<2;
CREATE TABLE video_thumbnail_job (
 photo_id integer PRIMARY KEY REFERENCES photo(photo_id) ON DELETE CASCADE,
 content_version integer NOT NULL,
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','processing','ready','failed')),
 attempts integer NOT NULL DEFAULT 0,
 next_attempt timestamptz NOT NULL DEFAULT now(),
 lease_token text, error text
);
CREATE INDEX video_thumbnail_work ON video_thumbnail_job(next_attempt,photo_id) WHERE state IN ('pending','processing');
-- +goose StatementBegin
CREATE FUNCTION video_thumbnail_changed() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF lower(NEW.format) IN ('mp4','avi','wmv','mpeg','m4v','mov','flv','asf','f4v','rmvb','vob','rm') THEN
  INSERT INTO video_thumbnail_job(photo_id,content_version) VALUES(NEW.photo_id,NEW.content_version)
  ON CONFLICT(photo_id) DO UPDATE SET content_version=NEW.content_version,state='pending',attempts=0,next_attempt=now(),lease_token=NULL,error=NULL;
 ELSE
  DELETE FROM video_thumbnail_job WHERE photo_id=NEW.photo_id;
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER video_thumbnail_insert AFTER INSERT ON photo FOR EACH ROW EXECUTE FUNCTION video_thumbnail_changed();
CREATE TRIGGER video_thumbnail_replace AFTER UPDATE OF content_version ON photo FOR EACH ROW WHEN (OLD.content_version IS DISTINCT FROM NEW.content_version) EXECUTE FUNCTION video_thumbnail_changed();
INSERT INTO video_thumbnail_job(photo_id,content_version)
 SELECT photo_id,content_version FROM photo WHERE lower(format) IN ('mp4','avi','wmv','mpeg','m4v','mov','flv','asf','f4v','rmvb','vob','rm');
-- +goose Down
DROP TRIGGER video_thumbnail_insert ON photo;
DROP TRIGGER video_thumbnail_replace ON photo;
DROP FUNCTION video_thumbnail_changed();
DROP TABLE video_thumbnail_job;
