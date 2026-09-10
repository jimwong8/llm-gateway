package auth

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type APIKeyUsageEvent struct {
	KeyID            int64
	UserID           int64
	RequestID        string
	Model            string
	Provider         string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	EstimatedCost    float64
	LatencyMs        int
	Success          bool
}

type APIKeyUsageSummary struct {
	KeyID            int64   `json:"key_id"`
	TotalRequests    int64   `json:"total_requests"`
	TotalPromptTokens  int64 `json:"total_prompt_tokens"`
	TotalCompletionTokens int64 `json:"total_completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	TotalCost        float64 `json:"total_cost"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
}

type APIKeyUsageStore struct {
	db *sql.DB
}

func NewAPIKeyUsageStore(db *sql.DB) *APIKeyUsageStore {
	return &APIKeyUsageStore{db: db}
}

func (s *APIKeyUsageStore) ensureSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS api_key_usage (
			id BIGSERIAL PRIMARY KEY,
			key_id BIGINT NOT NULL REFERENCES user_api_keys(id) ON DELETE CASCADE,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			request_id TEXT NOT NULL,
			model TEXT NOT NULL DEFAULT '',
			provider TEXT NOT NULL DEFAULT '',
			prompt_tokens INT NOT NULL DEFAULT 0,
			completion_tokens INT NOT NULL DEFAULT 0,
			total_tokens INT NOT NULL DEFAULT 0,
			estimated_cost DOUBLE PRECISION NOT NULL DEFAULT 0,
			latency_ms INT NOT NULL DEFAULT 0,
			success BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_api_key_usage_key_id_created_at ON api_key_usage (key_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_api_key_usage_user_id_created_at ON api_key_usage (user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_api_key_usage_request_id ON api_key_usage (request_id)`,
	}
	for _, stmt := range statements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *APIKeyUsageStore) Insert(ctx context.Context, e APIKeyUsageEvent) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO api_key_usage (key_id, user_id, request_id, model, provider, prompt_tokens, completion_tokens, total_tokens, estimated_cost, latency_ms, success)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		e.KeyID, e.UserID, e.RequestID, e.Model, e.Provider,
		e.PromptTokens, e.CompletionTokens, e.TotalTokens, e.EstimatedCost,
		e.LatencyMs, e.Success)
	return err
}

type UsageStatsFilter struct {
	KeyID  int64
	UserID int64
	From   time.Time
	To     time.Time
}

func (s *APIKeyUsageStore) Summary(ctx context.Context, filter UsageStatsFilter) (*APIKeyUsageSummary, error) {
	query := `
SELECT
	key_id,
	COUNT(*) AS total_requests,
	COALESCE(SUM(prompt_tokens), 0) AS total_prompt_tokens,
	COALESCE(SUM(completion_tokens), 0) AS total_completion_tokens,
	COALESCE(SUM(total_tokens), 0) AS total_tokens,
	COALESCE(SUM(estimated_cost), 0) AS total_cost,
	COALESCE(AVG(latency_ms), 0) AS avg_latency_ms
FROM api_key_usage
`
	clauses := []string{}
	args := []any{}
	if filter.KeyID > 0 {
		args = append(args, filter.KeyID)
		clauses = append(clauses, fmt.Sprintf("key_id = $%d", len(args)))
	}
	if filter.UserID > 0 {
		args = append(args, filter.UserID)
		clauses = append(clauses, fmt.Sprintf("user_id = $%d", len(args)))
	}
	if !filter.From.IsZero() {
		args = append(args, filter.From)
		clauses = append(clauses, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if !filter.To.IsZero() {
		args = append(args, filter.To)
		clauses = append(clauses, fmt.Sprintf("created_at <= $%d", len(args)))
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " GROUP BY key_id"

	var summary APIKeyUsageSummary
	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&summary.KeyID, &summary.TotalRequests, &summary.TotalPromptTokens,
		&summary.TotalCompletionTokens, &summary.TotalTokens, &summary.TotalCost,
		&summary.AvgLatencyMs,
	)
	if err == sql.ErrNoRows {
		if filter.KeyID > 0 {
			return &APIKeyUsageSummary{KeyID: filter.KeyID}, nil
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &summary, nil
}

type APIKeyUsageRow struct {
	ID               int64   `json:"id"`
	KeyID            int64   `json:"key_id"`
	RequestID        string  `json:"request_id"`
	Model            string  `json:"model"`
	Provider         string  `json:"provider"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	EstimatedCost    float64 `json:"estimated_cost"`
	LatencyMs        int     `json:"latency_ms"`
	Success          bool    `json:"success"`
	CreatedAt        string  `json:"created_at"`
}

func (s *APIKeyUsageStore) UsageHistory(ctx context.Context, keyID int64, limit int) ([]APIKeyUsageRow, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, key_id, request_id, model, provider, prompt_tokens, completion_tokens, total_tokens, estimated_cost, latency_ms, success, created_at
FROM api_key_usage
WHERE key_id = $1
ORDER BY created_at DESC
LIMIT $2`, keyID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]APIKeyUsageRow, 0, limit)
	for rows.Next() {
		var row APIKeyUsageRow
		var createdAt time.Time
		if err := rows.Scan(&row.ID, &row.KeyID, &row.RequestID, &row.Model, &row.Provider,
			&row.PromptTokens, &row.CompletionTokens, &row.TotalTokens, &row.EstimatedCost,
			&row.LatencyMs, &row.Success, &createdAt); err != nil {
			return nil, err
		}
		row.CreatedAt = createdAt.Format(time.RFC3339)
		out = append(out, row)
	}
	return out, rows.Err()
}
