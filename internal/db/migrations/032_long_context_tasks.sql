-- virtual-long-1m Phase 1 durable task state.
CREATE TABLE IF NOT EXISTS long_context_tasks (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    session_id TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL,
    mode TEXT NOT NULL,
    query TEXT NOT NULL,
    status TEXT NOT NULL,
    phase TEXT NOT NULL,
    input_sha256 TEXT NOT NULL,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    chunk_count INTEGER NOT NULL DEFAULT 0,
    completed_chunks INTEGER NOT NULL DEFAULT 0,
    worker_count INTEGER NOT NULL DEFAULT 1,
    retrieval_top_k INTEGER NOT NULL DEFAULT 1,
    final_context_budget INTEGER NOT NULL DEFAULT 16000,
    max_cost DOUBLE PRECISION,
    used_tokens BIGINT NOT NULL DEFAULT 0,
    estimated_cost DOUBLE PRECISION NOT NULL DEFAULT 0,
    retry_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    result JSONB,
    lease_owner TEXT NOT NULL DEFAULT '',
    lease_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_long_context_tasks_tenant_idempotency
    ON long_context_tasks (tenant_id, idempotency_key)
    WHERE idempotency_key <> '';
CREATE INDEX IF NOT EXISTS idx_long_context_tasks_claim
    ON long_context_tasks (status, lease_until, created_at);

CREATE TABLE IF NOT EXISTS long_context_chunks (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES long_context_tasks(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL,
    start_char BIGINT NOT NULL,
    end_char BIGINT NOT NULL,
    start_token BIGINT NOT NULL,
    end_token BIGINT NOT NULL,
    content TEXT NOT NULL,
    content_sha256 TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    map_attempts INTEGER NOT NULL DEFAULT 0,
    map_result JSONB,
    map_model TEXT NOT NULL DEFAULT '',
    map_provider TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (task_id, ordinal)
);
CREATE INDEX IF NOT EXISTS idx_long_context_chunks_task_status
    ON long_context_chunks (task_id, status, ordinal);

CREATE TABLE IF NOT EXISTS long_context_evidence (
    id BIGSERIAL PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES long_context_tasks(id) ON DELETE CASCADE,
    chunk_id TEXT NOT NULL REFERENCES long_context_chunks(id) ON DELETE CASCADE,
    claim TEXT NOT NULL,
    quote TEXT NOT NULL,
    start_char BIGINT NOT NULL,
    end_char BIGINT NOT NULL,
    entities JSONB NOT NULL DEFAULT '[]'::jsonb,
    relations JSONB NOT NULL DEFAULT '[]'::jsonb,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0,
    verification_status TEXT NOT NULL DEFAULT 'unverified',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_long_context_evidence_task ON long_context_evidence (task_id, confidence DESC);

CREATE TABLE IF NOT EXISTS long_context_attempts (
    id BIGSERIAL PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES long_context_tasks(id) ON DELETE CASCADE,
    chunk_id TEXT,
    phase TEXT NOT NULL,
    model TEXT NOT NULL,
    provider TEXT NOT NULL,
    request_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    estimated_cost DOUBLE PRECISION NOT NULL DEFAULT 0,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    error_type TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_long_context_attempts_task ON long_context_attempts (task_id, created_at DESC);
