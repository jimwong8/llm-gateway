CREATE TABLE IF NOT EXISTS memory_atoms (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    tags TEXT[] NOT NULL DEFAULT '{}',
    source VARCHAR(64) NOT NULL DEFAULT 'chat',
    importance FLOAT NOT NULL DEFAULT 0.5,
    access_count INT NOT NULL DEFAULT 0,
    last_accessed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_memory_atoms_user_id ON memory_atoms(user_id);
CREATE INDEX idx_memory_atoms_tags ON memory_atoms USING GIN(tags);

CREATE TABLE IF NOT EXISTS memory_scenarios (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    summary TEXT NOT NULL,
    chat_session_id BIGINT REFERENCES chat_sessions(id) ON DELETE SET NULL,
    atom_ids BIGINT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_memory_scenarios_user_id ON memory_scenarios(user_id);

CREATE TABLE IF NOT EXISTS memory_personas (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE UNIQUE,
    preferences JSONB NOT NULL DEFAULT '{}',
    interests TEXT[] NOT NULL DEFAULT '{}',
    communication_style VARCHAR(64) NOT NULL DEFAULT 'neutral',
    expertise_areas TEXT[] NOT NULL DEFAULT '{}',
    summary TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS memory_access_log (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    memory_type VARCHAR(32) NOT NULL,
    memory_id BIGINT NOT NULL,
    chat_session_id BIGINT,
    relevance_score FLOAT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_memory_access_log_user_id ON memory_access_log(user_id);
