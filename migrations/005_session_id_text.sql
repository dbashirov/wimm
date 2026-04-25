-- +goose Up
-- Переход с UUID на TEXT для хранения криптографически случайных
-- 32-байтных идентификаторов сессии в base64 (43 символа без padding).
ALTER TABLE sessions ALTER COLUMN id DROP DEFAULT;
ALTER TABLE sessions ALTER COLUMN id TYPE TEXT;

-- +goose Down
ALTER TABLE sessions ALTER COLUMN id TYPE UUID USING id::UUID;
ALTER TABLE sessions ALTER COLUMN id SET DEFAULT gen_random_uuid();
