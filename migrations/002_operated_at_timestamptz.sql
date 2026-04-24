-- +goose Up
ALTER TABLE operations
  ALTER COLUMN operated_at TYPE TIMESTAMPTZ USING operated_at::timestamptz;

-- +goose Down
ALTER TABLE operations
  ALTER COLUMN operated_at TYPE DATE USING operated_at::date;
