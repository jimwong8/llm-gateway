CREATE TABLE IF NOT EXISTS evaluation_datasets (
    id BIGSERIAL PRIMARY KEY,
    dataset_id TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    task_type TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by TEXT NOT NULL DEFAULT 'system',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS evaluation_scoring_formulas (
    id BIGSERIAL PRIMARY KEY,
    formula_id TEXT NOT NULL UNIQUE,
    version TEXT NOT NULL,
    formula_json JSONB NOT NULL,
    created_by TEXT NOT NULL DEFAULT 'system',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS evaluation_runs (
    id BIGSERIAL PRIMARY KEY,
    run_id TEXT NOT NULL UNIQUE,
    dataset_id TEXT NOT NULL,
    formula_id TEXT,
    agent_id TEXT NOT NULL,
    task_type TEXT NOT NULL,
    environment TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS evaluation_results (
    id BIGSERIAL PRIMARY KEY,
    result_id TEXT NOT NULL UNIQUE,
    run_id TEXT NOT NULL,
    model TEXT NOT NULL,
    sample_count INT NOT NULL DEFAULT 0,
    final_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    metrics JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_evaluation_runs_agent_task_env_created
    ON evaluation_runs (agent_id, task_type, environment, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_evaluation_results_run_score
    ON evaluation_results (run_id, final_score DESC);
