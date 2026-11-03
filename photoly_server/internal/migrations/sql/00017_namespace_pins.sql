-- +goose Up
CREATE TABLE namespace_pin (
  user_id integer NOT NULL REFERENCES "user" ON DELETE CASCADE,
  ns_id integer NOT NULL REFERENCES namespace ON DELETE CASCADE,
  position bigint GENERATED ALWAYS AS IDENTITY,
  is_default boolean NOT NULL DEFAULT false,
  PRIMARY KEY (user_id, ns_id)
);
CREATE UNIQUE INDEX namespace_pin_default ON namespace_pin(user_id) WHERE is_default;
CREATE INDEX namespace_pin_order ON namespace_pin(user_id, position);

-- +goose Down
DROP TABLE namespace_pin;
