CREATE TABLE IF NOT EXISTS config_snapshots (
    id BIGSERIAL PRIMARY KEY,
    version VARCHAR(64) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    config_snapshot JSONB NOT NULL DEFAULT '{}',
    notes TEXT NOT NULL DEFAULT '',
    created_by VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    rolled_back_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_config_snapshots_version ON config_snapshots (version);
CREATE INDEX IF NOT EXISTS idx_config_snapshots_status ON config_snapshots (status);
CREATE INDEX IF NOT EXISTS idx_config_snapshots_created_at ON config_snapshots (created_at);
