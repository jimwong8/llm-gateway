package httpserver

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"time"
)

type usageLogRow struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	Provider       string    `json:"provider"`
	Model          string    `json:"model"`
	PromptTokens   int       `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens    int       `json:"total_tokens"`
	CostCents      int64     `json:"cost_cents"`
	StatusCode     int       `json:"status_code"`
	DurationMs     int       `json:"duration_ms"`
	CreatedAt      time.Time `json:"created_at"`
}

type usageLogStore interface {
	ListUserLogs(ctx context.Context, userID int64, limit, offset int) ([]usageLogRow, int, error)
	GetUserCostTrend(ctx context.Context, userID int64, days int) ([]costTrendPoint, error)
}

type costTrendPoint struct {
	Date      string `json:"date"`
	CostCents int64  `json:"cost_cents"`
	Tokens    int    `json:"tokens"`
	Requests  int    `json:"requests"`
}

func (s *Server) userUsageLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	logs, total, err := s.usageLogStore.ListUserLogs(r.Context(), claims.UserID, limit, offset)
	if err != nil {
		internalError(w, err)
		return
	}

	resp := map[string]any{
		"object": "list",
		"data":   logs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) userCostTrend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}

	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days <= 0 {
		days = 30
	}

	trend, err := s.usageLogStore.GetUserCostTrend(r.Context(), claims.UserID, days)
	if err != nil {
		internalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": trend, "days": days})
}

type sqlUsageLogStore struct {
	db *sql.DB
}

func (s *sqlUsageLogStore) ListUserLogs(ctx context.Context, userID int64, limit, offset int) ([]usageLogRow, int, error) {
	var total int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_logs WHERE user_id = $1`, userID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, user_id, provider, model, prompt_tokens, completion_tokens, total_tokens, cost_cents, status_code, duration_ms, created_at
FROM usage_logs WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []usageLogRow
	for rows.Next() {
		var l usageLogRow
		if err := rows.Scan(&l.ID, &l.UserID, &l.Provider, &l.Model, &l.PromptTokens, &l.CompletionTokens, &l.TotalTokens, &l.CostCents, &l.StatusCode, &l.DurationMs, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}
	return logs, total, rows.Err()
}

func (s *sqlUsageLogStore) GetUserCostTrend(ctx context.Context, userID int64, days int) ([]costTrendPoint, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT DATE(created_at) as date, SUM(cost_cents) as cost_cents, SUM(total_tokens) as tokens, COUNT(*) as requests
FROM usage_logs WHERE user_id = $1 AND created_at > NOW() - ($2 || ' days')::interval
GROUP BY DATE(created_at) ORDER BY date ASC`, userID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trend []costTrendPoint
	for rows.Next() {
		var p costTrendPoint
		if err := rows.Scan(&p.Date, &p.CostCents, &p.Tokens, &p.Requests); err != nil {
			return nil, err
		}
		trend = append(trend, p)
	}
	return trend, rows.Err()
}
