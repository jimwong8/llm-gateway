CREATE TABLE IF NOT EXISTS channels (
    id VARCHAR(128) PRIMARY KEY,
    name VARCHAR(256) NOT NULL,
    provider VARCHAR(64) NOT NULL,
    base_url TEXT NOT NULL DEFAULT '',
    api_key TEXT NOT NULL DEFAULT '',
    priority VARCHAR(16) NOT NULL DEFAULT 'medium',
    weight INTEGER NOT NULL DEFAULT 1,
    models TEXT[] DEFAULT '{}',
    tags TEXT[] DEFAULT '{}',
    notes TEXT DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    latency_ms INTEGER DEFAULT 0,
    total_requests BIGINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_channels_status ON channels(status);
CREATE INDEX IF NOT EXISTS idx_channels_provider ON channels(provider);
