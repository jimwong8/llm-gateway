CREATE INDEX IF NOT EXISTS idx_model_recommendations_agent_task_env_created
    ON model_recommendations (agent_id, task_type, environment, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_model_recommendations_status_created
    ON model_recommendations (status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_model_approvals_recommendation_created
    ON model_approvals (recommendation_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_model_policy_versions_env_status_created
    ON model_policy_versions (environment, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_model_rollouts_policy_created
    ON model_rollouts (policy_version_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_model_rollouts_env_status_created
    ON model_rollouts (environment, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_governance_audit_logs_event_type_created
    ON governance_audit_logs (event_type, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_governance_audit_logs_entity_created
    ON governance_audit_logs (entity_type, entity_id, created_at DESC);
