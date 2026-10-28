-- +goose Up
-- Each photo remembers which storage backend holds its file, so switching backends hides
-- media on the others (and switching back shows it again). `backend_id` identifies the
-- active backend (driver + endpoint/bucket), `photo.storage_id` is stamped from it on insert.
ALTER TABLE storage_config ADD COLUMN backend_id text NOT NULL DEFAULT 'local';
ALTER TABLE photo ADD COLUMN storage_id text NOT NULL DEFAULT 'local';
-- Per-user choice to also list media that lives on the other backends.
ALTER TABLE "user" ADD COLUMN show_other_storage boolean NOT NULL DEFAULT false;

-- +goose StatementBegin
CREATE FUNCTION storage_active() RETURNS text LANGUAGE sql STABLE AS $$
    SELECT coalesce((SELECT backend_id FROM storage_config WHERE id = 1), 'local')
$$;

CREATE FUNCTION photo_visible(sid text, uid integer) RETURNS boolean LANGUAGE sql STABLE AS $$
    SELECT sid = storage_active() OR coalesce((SELECT show_other_storage FROM "user" WHERE user_id = uid), false)
$$;

CREATE FUNCTION photo_set_storage() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.storage_id := storage_active();
    RETURN NEW;
END $$;
-- +goose StatementEnd

CREATE TRIGGER photo_storage BEFORE INSERT ON photo FOR EACH ROW EXECUTE FUNCTION photo_set_storage();

-- +goose Down
DROP TRIGGER photo_storage ON photo;
DROP FUNCTION photo_set_storage();
DROP FUNCTION photo_visible(text, integer);
DROP FUNCTION storage_active();
ALTER TABLE "user" DROP COLUMN show_other_storage;
ALTER TABLE photo DROP COLUMN storage_id;
ALTER TABLE storage_config DROP COLUMN backend_id;
