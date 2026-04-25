-- +goose Up
CREATE INDEX IF NOT EXISTS wallets_user_id_idx ON wallets(user_id);
CREATE INDEX IF NOT EXISTS categories_user_id_idx ON categories(user_id);

-- +goose Down
DROP INDEX IF EXISTS wallets_user_id_idx;
DROP INDEX IF EXISTS categories_user_id_idx;
