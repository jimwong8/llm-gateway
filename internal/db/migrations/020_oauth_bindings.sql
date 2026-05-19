CREATE TABLE IF NOT EXISTS oauth_bindings (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider VARCHAR(64) NOT NULL,
    provider_user_id VARCHAR(255) NOT NULL,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_oauth_bindings_provider_user ON oauth_bindings (provider, provider_user_id);
CREATE INDEX IF NOT EXISTS idx_oauth_bindings_user_id ON oauth_bindings (user_id);
