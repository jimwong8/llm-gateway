CREATE TABLE IF NOT EXISTS model_rollbacks (
    id BIGSERIAL PRIMARY KEY,
    rollback_id TEXT NOT NULL UNIQUE,
    rollout_id TEXT NOT NULL,
    environment TEXT NOT NULL,
    actor TEXT NOT NULL,
    reason TEXT,
    restored_policy_version_id TEXT NOT NULL,
    reverted_policy_version_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_model_rollbacks_rollout_created
    ON model_rollbacks (rollout_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_model_rollbacks_env_created
    ON model_rollbacks (environment, created_at DESC);
