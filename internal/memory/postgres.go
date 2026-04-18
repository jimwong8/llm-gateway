package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"

	"llm-gateway/gateway/internal/providers"
)

type Item struct {
	Content string
	Role    string
}

type Store struct{ db *sql.DB }

func NewStore(dsn string) (*Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.ensureSchema(context.Background()); err != nil {
		
		return nil, err
	}
	return s, nil
}

func (s *Store) ensureSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS session_memories (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT,
    user_id TEXT,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_session_memories_tenant_user_session_created_at ON session_memories (tenant_id, user_id, session_id, created_at DESC);
`)
	return err
}

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) AppendFromRequest(ctx context.Context, req providers.ChatCompletionRequest) error {
	if strings.TrimSpace(req.SessionID) == "" {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, msg := range req.Messages {
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}
		if msg.Role != "user" && msg.Role != "assistant" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO session_memories (tenant_id, user_id, session_id, role, content) VALUES ($1,$2,$3,$4,$5)`, req.TenantID, req.UserID, req.SessionID, msg.Role, trim(msg.Content)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) Recent(ctx context.Context, tenantID, userID, sessionID string, limit int) ([]Item, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 3
	}
	rows, err := s.db.QueryContext(ctx, `SELECT role, content FROM session_memories WHERE COALESCE(tenant_id, '') = COALESCE($1, '') AND COALESCE(user_id, '') = COALESCE($2, '') AND session_id = $3 ORDER BY created_at DESC LIMIT $4`, tenantID, userID, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.Role, &item.Content); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func InjectMemory(req providers.ChatCompletionRequest, items []Item) providers.ChatCompletionRequest {
	if len(items) == 0 {
		return req
	}
	lines := make([]string, 0, len(items))
	for i := len(items) - 1; i >= 0; i-- {
		lines = append(lines, fmt.Sprintf("- %s: %s", items[i].Role, items[i].Content))
	}
	memoryMessage := providers.ChatMessage{Role: "system", Content: "Session memory:\n" + strings.Join(lines, "\n")}
	req.Messages = append([]providers.ChatMessage{memoryMessage}, req.Messages...)
	return req
}

func trim(s string) string {
	s = strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	if len(s) > 500 {
		return s[:500]
	}
	return s
}
