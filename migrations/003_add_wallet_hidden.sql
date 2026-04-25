-- +goose Up
ALTER TABLE wallets ADD COLUMN hidden BOOLEAN DEFAULT FALSE NOT NULL;

-- +goose Down
ALTER TABLE wallets DROP COLUMN hidden;
