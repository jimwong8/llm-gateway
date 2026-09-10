package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type Event struct {
	RequestID          string         `json:"request_id"`
	RouteMode          string         `json:"route_mode"`
	RouteTask          string         `json:"route_task"`
	RouteModel         string         `json:"route_model"`
	RouteProvider      string         `json:"route_provider"`
	RouteReason        string         `json:"route_reason"`
	RouteScore         string         `json:"route_score"`
	RouteTier          string         `json:"route_tier"`
	CacheStatus        string         `json:"cache_status"`
	FallbackUsed       bool           `json:"fallback_used"`
	PreprocessApplied  bool           `json:"preprocess_applied"`
	CanonicalHash      string         `json:"canonical_hash"`
	SummaryApplied     bool           `json:"summary_applied"`
	SummaryRatio       float64        `json:"summary_ratio"`
	TaskHint           string         `json:"task_hint"`
	Complexity         string         `json:"complexity"`
	ComplexityConfidence float64      `json:"complexity_confidence"`
	RequestPayload     map[string]any `json:"request_payload"`
	ResponsePayload    map[string]any `json:"response_payload"`
}

type BusinessAuditEvent struct {
	TenantID   string `json:"tenant_id"`
	Action     string `json:"action"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	ActorID    string `json:"actor_id"`
}

type Store struct{ db *sql.DB }

func NewStore(dsn string) (*Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db}
	if err := store.ensureSchema(context.Background()); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) ensureSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS request_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    request_id TEXT NOT NULL,
    route_mode TEXT,
    route_task TEXT,
    route_model TEXT,
    route_provider TEXT,
    route_reason TEXT,
    route_score TEXT,
    cache_status TEXT,
    fallback_used BOOLEAN DEFAULT FALSE,
    request_payload JSONB,
    response_payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_request_audit_logs_created_at ON request_audit_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_request_audit_logs_request_id ON request_audit_logs (request_id);

CREATE TABLE IF NOT EXISTS business_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT,
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id TEXT,
    actor_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_business_audit_logs_tenant_created_at ON business_audit_logs (tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_business_audit_logs_target ON business_audit_logs (target_type, target_id);
`)
	return err
}

func (s *Store) Insert(ctx context.Context, event Event) error {
	req, err := json.Marshal(event.RequestPayload)
	if err != nil {
		return fmt.Errorf("marshal request payload: %w", err)
	}
	resp, err := json.Marshal(event.ResponsePayload)
	if err != nil {
		return fmt.Errorf("marshal response payload: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO request_audit_logs (
 request_id, route_mode, route_task, route_model, route_provider, route_reason, route_score, cache_status, fallback_used, request_payload, response_payload
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
`, event.RequestID, event.RouteMode, event.RouteTask, event.RouteModel, event.RouteProvider, event.RouteReason, event.RouteScore, event.CacheStatus, event.FallbackUsed, req, resp)
	return err
}

func (s *Store) InsertBusinessAudit(ctx context.Context, event BusinessAuditEvent) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO business_audit_logs (
 tenant_id, action, target_type, target_id, actor_id
) VALUES ($1,$2,$3,$4,$5)
`, event.TenantID, event.Action, event.TargetType, event.TargetID, event.ActorID)
	return err
}

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) DeleteOldEvents(ctx context.Context, retentionDays int) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
DELETE FROM request_audit_logs WHERE created_at < NOW() - INTERVAL '1 day' * $1
`, retentionDays)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

type AuditEventRecord struct {
	ID             int64     `json:"id"`
	RequestID      string    `json:"request_id"`
	RouteMode      string    `json:"route_mode"`
	RouteTask      string    `json:"route_task"`
	RouteModel     string    `json:"route_model"`
	RouteProvider  string    `json:"route_provider"`
	RouteReason    string    `json:"route_reason"`
	RouteScore     string    `json:"route_score"`
	CacheStatus    string    `json:"cache_status"`
	FallbackUsed   bool      `json:"fallback_used"`
	RequestPayload string    `json:"request_payload"`
	CreatedAt      time.Time `json:"created_at"`
}

func (s *Store) ExportTenantData(ctx context.Context, tenantID string) ([]AuditEventRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, request_id, route_mode, route_task, route_model, route_provider, route_reason, route_score, cache_status, fallback_used, request_payload::text, created_at
FROM request_audit_logs
WHERE request_payload->>'tenant_id' = $1
ORDER BY created_at DESC
LIMIT 10000
`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []AuditEventRecord
	for rows.Next() {
		var r AuditEventRecord
		if err := rows.Scan(&r.ID, &r.RequestID, &r.RouteMode, &r.RouteTask, &r.RouteModel, &r.RouteProvider, &r.RouteReason, &r.RouteScore, &r.CacheStatus, &r.FallbackUsed, &r.RequestPayload, &r.CreatedAt); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (s *Store) SearchEvents(ctx context.Context, tenantID, provider, fromDate, toDate string, limit, offset int) ([]AuditEventRecord, error) {
	query := `
SELECT id, request_id, route_mode, route_task, route_model, route_provider, route_reason, route_score, cache_status, fallback_used, request_payload::text, created_at
FROM request_audit_logs
WHERE 1=1
`
	args := []interface{}{}
	argIdx := 1
	if tenantID != "" {
		query += fmt.Sprintf(" AND request_payload->>'tenant_id' = $%d", argIdx)
		args = append(args, tenantID)
		argIdx++
	}
	if provider != "" {
		query += fmt.Sprintf(" AND route_provider = $%d", argIdx)
		args = append(args, provider)
		argIdx++
	}
	if fromDate != "" {
		query += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, fromDate)
		argIdx++
	}
	if toDate != "" {
		query += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, toDate)
		argIdx++
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []AuditEventRecord
	for rows.Next() {
		var r AuditEventRecord
		if err := rows.Scan(&r.ID, &r.RequestID, &r.RouteMode, &r.RouteTask, &r.RouteModel, &r.RouteProvider, &r.RouteReason, &r.RouteScore, &r.CacheStatus, &r.FallbackUsed, &r.RequestPayload, &r.CreatedAt); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}
