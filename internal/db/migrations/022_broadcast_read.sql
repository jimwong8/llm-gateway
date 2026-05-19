CREATE TABLE IF NOT EXISTS broadcast_reads (
    id BIGSERIAL PRIMARY KEY,
    broadcast_id BIGINT NOT NULL REFERENCES broadcasts(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    read_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_broadcast_reads_unique ON broadcast_reads (broadcast_id, user_id);
CREATE INDEX IF NOT EXISTS idx_broadcast_reads_user_id ON broadcast_reads (user_id);
