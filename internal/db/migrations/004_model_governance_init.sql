CREATE TABLE IF NOT EXISTS model_recommendations (
    id BIGSERIAL PRIMARY KEY,
    recommendation_id TEXT NOT NULL UNIQUE,
    agent_id TEXT NOT NULL,
    task_type TEXT NOT NULL,
    environment TEXT NOT NULL,
    recommended_model TEXT NOT NULL,
    candidates JSONB NOT NULL DEFAULT '[]'::jsonb,
    score_breakdown JSONB NOT NULL DEFAULT '{}'::jsonb,
    approval_required BOOLEAN NOT NULL DEFAULT TRUE,
    status TEXT NOT NULL DEFAULT 'pending',
    created_by TEXT NOT NULL DEFAULT 'system',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS model_approvals (
    id BIGSERIAL PRIMARY KEY,
    approval_id TEXT NOT NULL UNIQUE,
    recommendation_id TEXT NOT NULL,
    decision TEXT NOT NULL,
    final_model TEXT,
    approval_reason TEXT,
    approved_by TEXT NOT NULL,
    effective_scope JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS model_policy_versions (
    id BIGSERIAL PRIMARY KEY,
    policy_version_id TEXT NOT NULL UNIQUE,
    environment TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    policy_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_approval_id TEXT,
    created_by TEXT NOT NULL,
    approved_by TEXT,
    approved_at TIMESTAMPTZ,
    activated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS model_rollouts (
    id BIGSERIAL PRIMARY KEY,
    rollout_id TEXT NOT NULL UNIQUE,
    policy_version_id TEXT NOT NULL,
    environment TEXT NOT NULL,
    rollout_mode TEXT NOT NULL,
    rollout_percent INT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'running',
    trigger_reason TEXT,
    triggered_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS governance_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    event_id TEXT NOT NULL UNIQUE,
    event_type TEXT NOT NULL,
    actor_id TEXT,
    entity_type TEXT,
    entity_id TEXT,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
