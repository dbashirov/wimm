-- +goose Up
ALTER TABLE wallets ADD COLUMN sort_order INT NOT NULL DEFAULT 0;

-- Инициализируем порядок по имени для существующих кошельков
UPDATE wallets w
SET sort_order = sub.rn
FROM (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY name) - 1 AS rn
    FROM wallets
) sub
WHERE w.id = sub.id;

-- +goose Down
ALTER TABLE wallets DROP COLUMN sort_order;
