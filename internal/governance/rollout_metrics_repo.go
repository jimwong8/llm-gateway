package governance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// RolloutMetricsRepo 负责从 runtime_decision_snapshots 聚合 rollout 指标。
type RolloutMetricsRepo struct {
	db *sql.DB
}

func NewRolloutMetricsRepo(store *Store) *RolloutMetricsRepo {
	if store == nil {
		return nil
	}
	return &RolloutMetricsRepo{db: store.DB()}
}

func (r *RolloutMetricsRepo) Aggregate(ctx context.Context, query RolloutMetricsQuery) (RolloutMetricsSnapshot, error) {
	if r == nil || r.db == nil {
		return RolloutMetricsSnapshot{}, errors.New("rollout metrics repo is not initialized")
	}
	query.RolloutID = strings.TrimSpace(query.RolloutID)
	query.PolicyVersionID = strings.TrimSpace(query.PolicyVersionID)
	if query.RolloutID == "" && query.PolicyVersionID == "" {
		return RolloutMetricsSnapshot{}, errors.New("rollout_id or policy_version_id is required")
	}

	where := make([]string, 0, 2)
	args := make([]any, 0, 2)
	if query.RolloutID != "" {
		args = append(args, query.RolloutID)
		where = append(where, fmt.Sprintf("rollout_id = $%d", len(args)))
	}
	if query.PolicyVersionID != "" {
		args = append(args, query.PolicyVersionID)
		where = append(where, fmt.Sprintf("policy_version_id = $%d", len(args)))
	}

	stmt := fmt.Sprintf(`
SELECT COALESCE(MIN(created_at), NOW()),
       COALESCE(MAX(created_at), NOW()),
       COUNT(1),
       COALESCE(AVG(CASE WHEN success THEN 0 ELSE 1 END), 0),
       COALESCE(AVG(CASE WHEN policy_fallback_used OR system_fallback_used THEN 1 ELSE 0 END), 0),
       COALESCE(SUM(CASE WHEN policy_fallback_used OR system_fallback_used THEN 1 ELSE 0 END), 0),
       COALESCE(AVG(latency_ms), 0)
FROM runtime_decision_snapshots
WHERE %s
`, strings.Join(where, " AND "))

	var (
		windowStart      time.Time
		windowEnd        time.Time
		requests         int64
		errorRate        float64
		fallbackRate     float64
		fallbackRequests int64
		meanLatency      float64
	)
	if err := r.db.QueryRowContext(ctx, stmt, args...).Scan(&windowStart, &windowEnd, &requests, &errorRate, &fallbackRate, &fallbackRequests, &meanLatency); err != nil {
		return RolloutMetricsSnapshot{}, err
	}

	latenciesStmt := fmt.Sprintf(`
SELECT latency_ms
FROM runtime_decision_snapshots
WHERE %s
ORDER BY latency_ms ASC
`, strings.Join(where, " AND "))
	rows, err := r.db.QueryContext(ctx, latenciesStmt, args...)
	if err != nil {
		return RolloutMetricsSnapshot{}, err
	}
	defer rows.Close()

	latencies := make([]int64, 0, requests)
	for rows.Next() {
		var latency int64
		if err := rows.Scan(&latency); err != nil {
			return RolloutMetricsSnapshot{}, err
		}
		latencies = append(latencies, latency)
	}
	if err := rows.Err(); err != nil {
		return RolloutMetricsSnapshot{}, err
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	metrics := RolloutMetricsSnapshot{
		WindowStart:      windowStart.UTC(),
		WindowEnd:        windowEnd.UTC(),
		RequestsTotal:    requests,
		ErrorRate:        errorRate,
		P95LatencyMillis: percentile95(latencies),
		FallbackRequests: fallbackRequests,
		FallbackRate:     fallbackRate,
		MeanCost:         meanLatency,
	}
	return metrics, nil
}

func percentile95(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	idx := int(float64(len(values)-1) * 0.95)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(values) {
		idx = len(values) - 1
	}
	return values[idx]
}
