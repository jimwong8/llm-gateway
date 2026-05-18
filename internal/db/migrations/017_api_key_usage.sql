ALTER TABLE user_api_keys ADD COLUMN IF NOT EXISTS rpm_limit INT NOT NULL DEFAULT 60;

CREATE TABLE IF NOT EXISTS api_key_usage (
    id BIGSERIAL PRIMARY KEY,
    key_id BIGINT NOT NULL REFERENCES user_api_keys(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    request_id TEXT NOT NULL,
    model TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT '',
    prompt_tokens INT NOT NULL DEFAULT 0,
    completion_tokens INT NOT NULL DEFAULT 0,
    total_tokens INT NOT NULL DEFAULT 0,
    estimated_cost DOUBLE PRECISION NOT NULL DEFAULT 0,
    latency_ms INT NOT NULL DEFAULT 0,
    success BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_key_usage_key_id_created_at ON api_key_usage (key_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_api_key_usage_user_id_created_at ON api_key_usage (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_api_key_usage_request_id ON api_key_usage (request_id);
