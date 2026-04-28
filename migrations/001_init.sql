-- +goose Up

CREATE TYPE operation_type AS ENUM ('income', 'expense');

CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS sessions_expires_at_idx ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS wallets (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    currency    CHAR(3) NOT NULL DEFAULT 'RUB',
    description TEXT NOT NULL DEFAULT '',
    hidden      BOOLEAN NOT NULL DEFAULT false,
    is_default  BOOLEAN NOT NULL DEFAULT false,
    sort_order  INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS wallets_user_id_idx ON wallets(user_id);
CREATE UNIQUE INDEX wallets_one_default_per_user
    ON wallets(user_id) WHERE is_default = true;

CREATE TABLE IF NOT EXISTS categories (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    type        operation_type NOT NULL,
    description TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS categories_user_id_idx ON categories(user_id);

CREATE TABLE IF NOT EXISTS operations (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    wallet_id          UUID NOT NULL REFERENCES wallets(id),
    category_id        UUID REFERENCES categories(id),
    transfer_id        UUID,
    transfer_wallet_id UUID REFERENCES wallets(id),
    type               operation_type NOT NULL,
    amount             NUMERIC(15,2) NOT NULL CHECK (amount > 0),
    operated_at        TIMESTAMPTZ,
    description        TEXT NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS operations_user_id_operated_at_idx
    ON operations(user_id, operated_at DESC);
CREATE INDEX IF NOT EXISTS operations_transfer_id_idx
    ON operations(transfer_id) WHERE transfer_id IS NOT NULL;
