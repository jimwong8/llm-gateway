CREATE TABLE IF NOT EXISTS model_distribution_events (
    id BIGSERIAL PRIMARY KEY,
    event_id TEXT NOT NULL UNIQUE,
    policy_version_id TEXT,
    rollout_id TEXT,
    environment TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS policy_drifts (
    id BIGSERIAL PRIMARY KEY,
    drift_id TEXT NOT NULL UNIQUE,
    environment TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    active_model TEXT,
    recommended_model TEXT,
    drift_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_model_distribution_events_env_created
    ON model_distribution_events (environment, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_model_distribution_events_policy_created
    ON model_distribution_events (policy_version_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_policy_drifts_env_agent_detected
    ON policy_drifts (environment, agent_id, detected_at DESC);

CREATE INDEX IF NOT EXISTS idx_policy_drifts_status_detected
    ON policy_drifts (status, detected_at DESC);
