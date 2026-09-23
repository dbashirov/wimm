-- +goose Up
-- Суммы — целые копейки (model.Money). Колонка переименована, чтобы код,
-- не знающий о смене единиц, падал на запросе, а не показывал суммы ×100.
-- Номер 008, а не 002: на части окружений до сквоша миграций записана версия 7.
ALTER TABLE operations ALTER COLUMN amount TYPE BIGINT USING (amount * 100)::BIGINT;
ALTER TABLE operations RENAME COLUMN amount TO amount_minor;
-- после смены типа PostgreSQL сохранил бы CHECK в виде amount_minor::numeric > 0
ALTER TABLE operations DROP CONSTRAINT IF EXISTS operations_amount_check;
ALTER TABLE operations ADD CONSTRAINT operations_amount_check CHECK (amount_minor > 0);

