CREATE TABLE IF NOT EXISTS site_config (
    id BIGSERIAL PRIMARY KEY,
    site_name VARCHAR(255) NOT NULL DEFAULT 'LLM Gateway',
    logo_url VARCHAR(512) NOT NULL DEFAULT '',
    jwt_secret VARCHAR(512) NOT NULL DEFAULT '',
    jwt_secret_rotated_at TIMESTAMPTZ,
    smtp_host VARCHAR(255) NOT NULL DEFAULT '',
    smtp_port INTEGER NOT NULL DEFAULT 587,
    smtp_user VARCHAR(255) NOT NULL DEFAULT '',
    smtp_pass VARCHAR(512) NOT NULL DEFAULT '',
    smtp_from VARCHAR(255) NOT NULL DEFAULT '',
    allow_registration BOOLEAN NOT NULL DEFAULT true,
    default_user_role VARCHAR(20) NOT NULL DEFAULT 'user',
    default_user_quota BIGINT NOT NULL DEFAULT 1000000,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255) NOT NULL DEFAULT ''
);

INSERT INTO site_config (site_name, jwt_secret)
VALUES ('LLM Gateway', '')
ON CONFLICT DO NOTHING;
