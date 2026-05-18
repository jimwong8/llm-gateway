CREATE TABLE IF NOT EXISTS chat_sessions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL DEFAULT 'New Chat',
    model VARCHAR(128) NOT NULL DEFAULT 'gpt-4o-mini',
    visibility VARCHAR(20) NOT NULL DEFAULT 'private',
    share_hash VARCHAR(64) UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_sessions_user_id ON chat_sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_chat_sessions_share_hash ON chat_sessions (share_hash);
