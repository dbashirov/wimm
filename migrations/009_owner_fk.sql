-- +goose Up
-- Изоляция пользователей на уровне БД: операция ссылается только на кошельки
-- и категории своего владельца, а её тип совпадает с типом категории. Раньше
-- это держали только проверки в сервисе, и гонку смены типа категории с
-- созданием операции они не закрывали.
--
-- Перед накатыванием все три запроса должны вернуть 0, иначе миграция упадёт:
--   SELECT count(*) FROM operations o JOIN wallets w ON w.id = o.wallet_id
--    WHERE w.user_id <> o.user_id;
--   SELECT count(*) FROM operations o JOIN wallets w ON w.id = o.transfer_wallet_id
--    WHERE w.user_id <> o.user_id;
--   SELECT count(*) FROM operations o JOIN categories c ON c.id = o.category_id
--    WHERE c.user_id <> o.user_id OR c.type <> o.type;
--
-- MATCH SIMPLE (по умолчанию): у переводов category_id NULL, у обычных
-- операций NULL transfer_wallet_id, такие ссылки не проверяются.
--
-- UNIQUE берёт ACCESS EXCLUSIVE на wallets и categories, FK проверяет все
-- operations под блокировкой записи, и всё держится до конца транзакции:
-- накатывать в тихое время. lock_timeout - чтобы не встать в очередь за
-- долгим запросом и не заблокировать приложение; при отказе просто повторить.

SET LOCAL lock_timeout = '5s';

ALTER TABLE wallets ADD CONSTRAINT wallets_id_user_id_key UNIQUE (id, user_id);
ALTER TABLE categories ADD CONSTRAINT categories_id_user_id_type_key UNIQUE (id, user_id, type);

ALTER TABLE operations
    ADD CONSTRAINT operations_wallet_owner_fkey
        FOREIGN KEY (wallet_id, user_id) REFERENCES wallets (id, user_id),
    ADD CONSTRAINT operations_transfer_wallet_owner_fkey
        FOREIGN KEY (transfer_wallet_id, user_id) REFERENCES wallets (id, user_id),
    ADD CONSTRAINT operations_category_owner_fkey
        FOREIGN KEY (category_id, user_id, type) REFERENCES categories (id, user_id, type);
