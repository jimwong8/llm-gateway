CREATE TABLE IF NOT EXISTS prompt_presets (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id VARCHAR(64) NOT NULL DEFAULT '',
    name VARCHAR(128) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    template TEXT NOT NULL,
    variables JSONB NOT NULL DEFAULT '[]',
    tags TEXT[] NOT NULL DEFAULT '{}',
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_prompt_presets_user_id ON prompt_presets(user_id);
CREATE INDEX idx_prompt_presets_tenant_id ON prompt_presets(tenant_id);
CREATE INDEX idx_prompt_presets_user_tenant ON prompt_presets(user_id, tenant_id);

CREATE TABLE IF NOT EXISTS mask_rules (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id VARCHAR(64) NOT NULL DEFAULT '',
    name VARCHAR(128) NOT NULL,
    pattern VARCHAR(255) NOT NULL,
    replace_with VARCHAR(255) NOT NULL DEFAULT '[REDACTED]',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_mask_rules_user_id ON mask_rules(user_id);
CREATE INDEX idx_mask_rules_tenant_id ON mask_rules(tenant_id);
CREATE INDEX idx_mask_rules_user_tenant ON mask_rules(user_id, tenant_id);
