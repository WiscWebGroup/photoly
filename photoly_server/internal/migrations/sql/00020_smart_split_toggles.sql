-- +goose Up
-- enabled stays the master flag (devices OR trips) so existing scheduler queries still work.
ALTER TABLE smart_preference
 ADD COLUMN devices_enabled boolean NOT NULL DEFAULT false,
 ADD COLUMN trips_enabled boolean NOT NULL DEFAULT false,
 ADD COLUMN devices_dirty boolean NOT NULL DEFAULT false;
UPDATE smart_preference SET devices_enabled=enabled, trips_enabled=enabled;
-- +goose Down
ALTER TABLE smart_preference DROP COLUMN devices_enabled, DROP COLUMN trips_enabled, DROP COLUMN devices_dirty;
