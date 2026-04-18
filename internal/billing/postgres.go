package billing

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type UsageEvent struct {
	TenantID         string
	UserID           string
	RequestID        string
	Model            string
	Provider         string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	EstimatedCost    float64
	CacheStatus      string
	CacheLayer       string
	RouteMode        string
	RouteProvider    string
	RouteModel       string
	FallbackUsed     bool
	LatencyMs        int
	Success          bool
	ErrorType        string
	ErrorMessage     string
}

type QueryFilter struct {
	TenantID string
	Provider string
	Model    string
	From     time.Time
	To       time.Time
	Limit    int
}

type SummaryRow struct {
	Requests          int64   `json:"requests"`
	PromptTokens      int64   `json:"prompt_tokens"`
	CompletionTokens  int64   `json:"completion_tokens"`
	TotalTokens       int64   `json:"total_tokens"`
	EstimatedCost     float64 `json:"estimated_cost"`
	AvgLatencyMs      float64 `json:"avg_latency_ms"`
	ProviderErrorRate float64 `json:"provider_error_rate"`
	CacheHitRate      float64 `json:"cache_hit_rate"`
}

type CacheBreakdownRow struct {
	CacheStatus string `json:"cache_status"`
	CacheLayer  string `json:"cache_layer"`
	Requests    int64  `json:"requests"`
}

type ProviderBreakdownRow struct {
	Provider          string  `json:"provider"`
	Requests          int64   `json:"requests"`
	PromptTokens      int64   `json:"prompt_tokens"`
	CompletionTokens  int64   `json:"completion_tokens"`
	TotalTokens       int64   `json:"total_tokens"`
	EstimatedCost     float64 `json:"estimated_cost"`
	AvgLatencyMs      float64 `json:"avg_latency_ms"`
	ProviderErrorRate float64 `json:"provider_error_rate"`
}

type HotspotRow struct {
	Key           string  `json:"key"`
	Requests      int64   `json:"requests"`
	TotalTokens   int64   `json:"total_tokens"`
	EstimatedCost float64 `json:"estimated_cost"`
}

type HotspotsResult struct {
	Tenants []HotspotRow `json:"tenants"`
	Models  []HotspotRow `json:"models"`
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
	statements := []string{
		`CREATE TABLE IF NOT EXISTS usage_events (
			id BIGSERIAL PRIMARY KEY,
			tenant_id TEXT,
			user_id TEXT,
			request_id TEXT NOT NULL,
			model TEXT,
			provider TEXT,
			prompt_tokens INT NOT NULL DEFAULT 0,
			completion_tokens INT NOT NULL DEFAULT 0,
			total_tokens INT NOT NULL DEFAULT 0,
			estimated_cost DOUBLE PRECISION NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`ALTER TABLE usage_events ADD COLUMN IF NOT EXISTS cache_status TEXT NOT NULL DEFAULT 'MISS'`,
		`ALTER TABLE usage_events ADD COLUMN IF NOT EXISTS cache_layer TEXT NOT NULL DEFAULT 'none'`,
		`ALTER TABLE usage_events ADD COLUMN IF NOT EXISTS route_mode TEXT NOT NULL DEFAULT 'auto'`,
		`ALTER TABLE usage_events ADD COLUMN IF NOT EXISTS route_provider TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE usage_events ADD COLUMN IF NOT EXISTS route_model TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE usage_events ADD COLUMN IF NOT EXISTS fallback_used BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE usage_events ADD COLUMN IF NOT EXISTS latency_ms INT NOT NULL DEFAULT 0`,
		`ALTER TABLE usage_events ADD COLUMN IF NOT EXISTS success BOOLEAN NOT NULL DEFAULT TRUE`,
		`ALTER TABLE usage_events ADD COLUMN IF NOT EXISTS error_type TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE usage_events ADD COLUMN IF NOT EXISTS error_message TEXT NOT NULL DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_tenant_created_at ON usage_events (tenant_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_request_id ON usage_events (request_id)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_provider_created_at ON usage_events (provider, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_model_created_at ON usage_events (model, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_cache_status_created_at ON usage_events (cache_status, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_success_created_at ON usage_events (success, created_at DESC)`,
	}
	for _, stmt := range statements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) Insert(ctx context.Context, e UsageEvent) error {
	cacheStatus := strings.TrimSpace(e.CacheStatus)
	if cacheStatus == "" {
		cacheStatus = "MISS"
	}
	cacheLayer := strings.TrimSpace(e.CacheLayer)
	if cacheLayer == "" {
		cacheLayer = "none"
	}
	routeMode := strings.TrimSpace(e.RouteMode)
	if routeMode == "" {
		routeMode = "auto"
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO usage_events (
	tenant_id, user_id, request_id, model, provider,
	prompt_tokens, completion_tokens, total_tokens, estimated_cost,
	cache_status, cache_layer, route_mode, route_provider, route_model,
	fallback_used, latency_ms, success, error_type, error_message
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
`, e.TenantID, e.UserID, e.RequestID, e.Model, e.Provider, e.PromptTokens, e.CompletionTokens, e.TotalTokens, e.EstimatedCost, cacheStatus, cacheLayer, routeMode, e.RouteProvider, e.RouteModel, e.FallbackUsed, e.LatencyMs, e.Success, e.ErrorType, e.ErrorMessage)
	return err
}

func (s *Store) Summary(ctx context.Context, filter QueryFilter) (SummaryRow, error) {
	query, args := buildWhere(`
SELECT
	COUNT(*) AS requests,
	COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
	COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
	COALESCE(SUM(total_tokens), 0) AS total_tokens,
	COALESCE(SUM(estimated_cost), 0) AS estimated_cost,
	COALESCE(AVG(latency_ms), 0) AS avg_latency_ms,
	COALESCE(AVG(CASE WHEN error_type = 'provider_error' AND success = FALSE THEN 1.0 ELSE 0.0 END), 0) AS provider_error_rate,
	COALESCE(AVG(CASE WHEN cache_status <> 'MISS' THEN 1.0 ELSE 0.0 END), 0) AS cache_hit_rate
FROM usage_events
`, filter)
	var row SummaryRow
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&row.Requests, &row.PromptTokens, &row.CompletionTokens, &row.TotalTokens, &row.EstimatedCost, &row.AvgLatencyMs, &row.ProviderErrorRate, &row.CacheHitRate)
	return row, err
}

func (s *Store) CacheBreakdown(ctx context.Context, filter QueryFilter) ([]CacheBreakdownRow, error) {
	query, args := buildWhere(`
SELECT cache_status, cache_layer, COUNT(*) AS requests
FROM usage_events
`, filter)
	query += ` GROUP BY cache_status, cache_layer ORDER BY requests DESC, cache_status ASC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CacheBreakdownRow{}
	for rows.Next() {
		var item CacheBreakdownRow
		if err := rows.Scan(&item.CacheStatus, &item.CacheLayer, &item.Requests); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ProviderBreakdown(ctx context.Context, filter QueryFilter) ([]ProviderBreakdownRow, error) {
	query, args := buildWhere(`
SELECT
	provider,
	COUNT(*) AS requests,
	COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
	COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
	COALESCE(SUM(total_tokens), 0) AS total_tokens,
	COALESCE(SUM(estimated_cost), 0) AS estimated_cost,
	COALESCE(AVG(latency_ms), 0) AS avg_latency_ms,
	COALESCE(AVG(CASE WHEN error_type = 'provider_error' AND success = FALSE THEN 1.0 ELSE 0.0 END), 0) AS provider_error_rate
FROM usage_events
`, filter)
	query += ` GROUP BY provider ORDER BY requests DESC, provider ASC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProviderBreakdownRow{}
	for rows.Next() {
		var item ProviderBreakdownRow
		if err := rows.Scan(&item.Provider, &item.Requests, &item.PromptTokens, &item.CompletionTokens, &item.TotalTokens, &item.EstimatedCost, &item.AvgLatencyMs, &item.ProviderErrorRate); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) Hotspots(ctx context.Context, filter QueryFilter) (HotspotsResult, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 10
	}
	tenants, err := s.hotspotByColumn(ctx, filter, "tenant_id", limit)
	if err != nil {
		return HotspotsResult{}, err
	}
	models, err := s.hotspotByColumn(ctx, filter, "model", limit)
	if err != nil {
		return HotspotsResult{}, err
	}
	return HotspotsResult{Tenants: tenants, Models: models}, nil
}

func (s *Store) hotspotByColumn(ctx context.Context, filter QueryFilter, column string, limit int) ([]HotspotRow, error) {
	base := fmt.Sprintf(`
SELECT %s AS key, COUNT(*) AS requests, COALESCE(SUM(total_tokens), 0) AS total_tokens, COALESCE(SUM(estimated_cost), 0) AS estimated_cost
FROM usage_events
`, column)
	query, args := buildWhere(base, filter)
	args = append(args, limit)
	query += fmt.Sprintf(` GROUP BY %s ORDER BY requests DESC, %s ASC LIMIT $%d`, column, column, len(args))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []HotspotRow{}
	for rows.Next() {
		var item HotspotRow
		if err := rows.Scan(&item.Key, &item.Requests, &item.TotalTokens, &item.EstimatedCost); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func buildWhere(base string, filter QueryFilter) (string, []any) {
	clauses := []string{"1=1"}
	args := []any{}
	if strings.TrimSpace(filter.TenantID) != "" {
		args = append(args, filter.TenantID)
		clauses = append(clauses, fmt.Sprintf("tenant_id = $%d", len(args)))
	}
	if strings.TrimSpace(filter.Provider) != "" {
		args = append(args, filter.Provider)
		clauses = append(clauses, fmt.Sprintf("provider = $%d", len(args)))
	}
	if strings.TrimSpace(filter.Model) != "" {
		args = append(args, filter.Model)
		clauses = append(clauses, fmt.Sprintf("model = $%d", len(args)))
	}
	if !filter.From.IsZero() {
		args = append(args, filter.From)
		clauses = append(clauses, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if !filter.To.IsZero() {
		args = append(args, filter.To)
		clauses = append(clauses, fmt.Sprintf("created_at <= $%d", len(args)))
	}
	return base + " WHERE " + strings.Join(clauses, " AND "), args
}
