-- Extract shadow tables from postgres.go ensureSchema() into a formal migration.
-- These tables were previously created inline by Store.ensureSchema() at runtime.

CREATE TABLE IF NOT EXISTS session_memories (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT,
    user_id TEXT,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_session_memories_tenant_user_session_created_at ON session_memories (tenant_id, user_id, session_id, created_at DESC);

CREATE TABLE IF NOT EXISTS conversations (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT,
    user_id TEXT,
    session_id TEXT NOT NULL,
    status TEXT,
    last_seq BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_conversations_session_id ON conversations (session_id);
CREATE INDEX IF NOT EXISTS idx_conversations_tenant_user_session ON conversations (tenant_id, user_id, session_id);

CREATE TABLE IF NOT EXISTS messages (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL,
    conversation_id BIGINT REFERENCES conversations(id),
    seq BIGINT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    search_vector tsvector GENERATED ALWAYS AS (to_tsvector('english', content)) STORED,
    token_count INTEGER,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_messages_session_seq UNIQUE (session_id, seq)
);
CREATE INDEX IF NOT EXISTS idx_messages_conversation_seq ON messages (conversation_id, seq);
CREATE INDEX IF NOT EXISTS idx_messages_session_created_at ON messages (session_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_search_vector ON messages USING GIN (search_vector);

CREATE TABLE IF NOT EXISTS session_summaries (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT,
    user_id TEXT,
    session_id TEXT NOT NULL,
    current_goal TEXT NOT NULL DEFAULT '',
    completed_items JSONB NOT NULL DEFAULT '[]'::jsonb,
    open_items JSONB NOT NULL DEFAULT '[]'::jsonb,
    key_decisions JSONB NOT NULL DEFAULT '[]'::jsonb,
    blockers JSONB NOT NULL DEFAULT '[]'::jsonb,
    source_message_seq BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_session_summaries_session_id ON session_summaries (session_id);
CREATE INDEX IF NOT EXISTS idx_session_summaries_tenant_user ON session_summaries (tenant_id, user_id);
CREATE INDEX IF NOT EXISTS idx_session_summaries_updated_at ON session_summaries (updated_at DESC);

CREATE TABLE IF NOT EXISTS user_preferences (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT,
    user_id TEXT,
    preference_key TEXT NOT NULL,
    preference_value TEXT NOT NULL,
    source_text TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_preferences_tenant_user_key ON user_preferences (COALESCE(tenant_id, ''), COALESCE(user_id, ''), preference_key);
CREATE INDEX IF NOT EXISTS idx_user_preferences_tenant_user_updated_at ON user_preferences (tenant_id, user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS project_facts (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT,
    user_id TEXT,
    fact_key TEXT NOT NULL,
    fact_value TEXT NOT NULL,
    source_text TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    superseded_by BIGINT REFERENCES project_facts(id),
    source_message_seq BIGINT NOT NULL DEFAULT 0,
    last_verified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_project_facts_active_tenant_user_key ON project_facts (COALESCE(tenant_id, ''), COALESCE(user_id, ''), fact_key) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_project_facts_tenant_user_key_status ON project_facts (tenant_id, user_id, fact_key, status);

CREATE TABLE IF NOT EXISTS candidate_facts (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT,
    user_id TEXT,
    fact_key TEXT NOT NULL,
    fact_value TEXT NOT NULL,
    source_text TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    source_message_seq BIGINT NOT NULL DEFAULT 0,
    confirmation_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_candidate_facts_confirmation_count_non_negative CHECK (confirmation_count >= 0)
);
CREATE INDEX IF NOT EXISTS idx_candidate_facts_tenant_user_status ON candidate_facts (tenant_id, user_id, status);
CREATE INDEX IF NOT EXISTS idx_candidate_facts_tenant_user_key_status ON candidate_facts (tenant_id, user_id, fact_key, status);
