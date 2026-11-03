-- +goose Up
ALTER TABLE gallery ADD COLUMN description text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE gallery DROP COLUMN description;
