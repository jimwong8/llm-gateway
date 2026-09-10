CREATE TABLE IF NOT EXISTS wallets (
    id BIGSERIAL PRIMARY KEY,
    user_id TEXT UNIQUE NOT NULL,
    balance DOUBLE PRECISION NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'USD',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wallets_user_id ON wallets (user_id);

CREATE TABLE IF NOT EXISTS pricing (
    id BIGSERIAL PRIMARY KEY,
    provider TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    input_price_per_1k DOUBLE PRECISION NOT NULL DEFAULT 0,
    output_price_per_1k DOUBLE PRECISION NOT NULL DEFAULT 0,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_pricing_provider_model ON pricing (provider, model);
CREATE INDEX IF NOT EXISTS idx_pricing_provider_default ON pricing (provider) WHERE is_default = TRUE;

CREATE TABLE IF NOT EXISTS ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id TEXT NOT NULL,
    type TEXT NOT NULL,
    amount DOUBLE PRECISION NOT NULL,
    balance_after DOUBLE PRECISION NOT NULL,
    reference_id TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ledger_user_id ON ledger (user_id);
CREATE INDEX IF NOT EXISTS idx_ledger_user_created ON ledger (user_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ledger_reference_id ON ledger (reference_id) WHERE reference_id <> '';
