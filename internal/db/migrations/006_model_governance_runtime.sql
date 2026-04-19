CREATE TABLE IF NOT EXISTS runtime_decision_snapshots (
    id BIGSERIAL PRIMARY KEY,
    request_id TEXT NOT NULL UNIQUE,
    policy_version_id TEXT,
    rollout_id TEXT,
    environment TEXT NOT NULL,
    tenant_id TEXT,
    agent_id TEXT NOT NULL,
    task_type TEXT,
    matched_scope_type TEXT,
    matched_scope JSONB NOT NULL DEFAULT '{}'::jsonb,
    resolved_model TEXT NOT NULL,
    fallback_chain JSONB NOT NULL DEFAULT '[]'::jsonb,
    policy_fallback_used BOOLEAN NOT NULL DEFAULT FALSE,
    system_fallback_used BOOLEAN NOT NULL DEFAULT FALSE,
    latency_ms INT NOT NULL DEFAULT 0,
    success BOOLEAN NOT NULL DEFAULT TRUE,
    error_type TEXT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_runtime_decision_snapshots_env_agent_created
    ON runtime_decision_snapshots (environment, agent_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_runtime_decision_snapshots_rollout_created
    ON runtime_decision_snapshots (rollout_id, created_at DESC);
