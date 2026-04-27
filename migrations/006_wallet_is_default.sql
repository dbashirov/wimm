-- +goose Up
ALTER TABLE wallets ADD COLUMN is_default BOOLEAN NOT NULL DEFAULT false;

CREATE UNIQUE INDEX wallets_one_default_per_user
    ON wallets (user_id)
    WHERE is_default = true;

-- +goose Down
DROP INDEX IF EXISTS wallets_one_default_per_user;
ALTER TABLE wallets DROP COLUMN is_default;
