package governance_test

import (
	"context"
	"testing"
	"time"

	"llm-gateway/gateway/internal/governance"
)

func newRolloutDashboardServiceForTest(t *testing.T) (*governance.RolloutDashboardService, *governance.Store) {
	t.Helper()
	dsn := testPostgresDSN(t)
	store, err := governance.NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return governance.NewRolloutDashboardService(store), store
}

func seedRolloutForDashboardTest(t *testing.T, store *governance.Store, rolloutID, policyVersionID, environment, status string, percent int, createdAt time.Time) {
	t.Helper()
	_, err := store.DB().ExecContext(context.Background(), `
INSERT INTO model_rollouts (
    rollout_id,
    policy_version_id,
    environment,
    rollout_mode,
    rollout_percent,
    status,
    trigger_reason,
    triggered_by,
    created_at,
    updated_at
) VALUES ($1,$2,$3,'progressive',$4,$5,'seed','tester',$6,$6)
ON CONFLICT (rollout_id) DO UPDATE
SET rollout_percent = EXCLUDED.rollout_percent,
    status = EXCLUDED.status,
    updated_at = EXCLUDED.updated_at
`, rolloutID, policyVersionID, environment, percent, status, createdAt.UTC())
	if err != nil {
		t.Fatalf("seed rollout: %v", err)
	}
}

func TestRolloutDashboardServiceListRows(t *testing.T) {
	svc, store := newRolloutDashboardServiceForTest(t)
	now := time.Now().UTC()

	// Test isolation: clear stale rollouts left behind by sibling tests
	// (e.g. TestRollbackService inserts "rollout_rollback_seam") so that
	// this assertion-by-index test sees only its own seeded rows.
	if _, err := store.DB().ExecContext(context.Background(), `DELETE FROM model_rollouts`); err != nil {
		t.Fatalf("clear model_rollouts: %v", err)
	}

	seedRolloutForDashboardTest(t, store, "rollout_dashboard_1", "pv_dashboard_1", "prod", "running", 10, now.Add(-2*time.Minute))
	seedRolloutForDashboardTest(t, store, "rollout_dashboard_2", "pv_dashboard_2", "prod", "promoted", 80, now.Add(-1*time.Minute))

	snapshotRepo := governance.NewSnapshotRepo(store)
	if err := snapshotRepo.Save(context.Background(), governance.RuntimeDecisionSnapshotWrite{
		RequestID:          "req-dashboard-1",
		RolloutID:          "rollout_dashboard_1",
		PolicyVersionID:    "pv_dashboard_1",
		Environment:        "prod",
		TenantID:           "tenant-a",
		AgentID:            "agent-a",
		ResolvedModel:      "model-a",
		LatencyMS:          120,
		PolicyFallbackUsed: true,
		Success:            true,
		CreatedAt:          now.Add(-90 * time.Second),
	}); err != nil {
		t.Fatalf("Save(snapshot-1) error = %v", err)
	}
	if err := snapshotRepo.Save(context.Background(), governance.RuntimeDecisionSnapshotWrite{
		RequestID:       "req-dashboard-2",
		RolloutID:       "rollout_dashboard_1",
		PolicyVersionID: "pv_dashboard_1",
		Environment:     "prod",
		TenantID:        "tenant-a",
		AgentID:         "agent-a",
		ResolvedModel:   "model-a",
		LatencyMS:       80,
		Success:         false,
		CreatedAt:       now.Add(-80 * time.Second),
	}); err != nil {
		t.Fatalf("Save(snapshot-2) error = %v", err)
	}
	if err := snapshotRepo.Save(context.Background(), governance.RuntimeDecisionSnapshotWrite{
		RequestID:       "req-dashboard-3",
		RolloutID:       "rollout_dashboard_2",
		PolicyVersionID: "pv_dashboard_2",
		Environment:     "prod",
		TenantID:        "tenant-a",
		AgentID:         "agent-a",
		ResolvedModel:   "model-b",
		LatencyMS:       60,
		Success:         true,
		CreatedAt:       now.Add(-70 * time.Second),
	}); err != nil {
		t.Fatalf("Save(snapshot-3) error = %v", err)
	}

	rows, err := svc.ListRows(context.Background(), governance.RolloutDashboardQuery{Limit: 2})
	if err != nil {
		t.Fatalf("ListRows() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].RolloutID != "rollout_dashboard_2" {
		t.Fatalf("expected latest rollout first, got %s", rows[0].RolloutID)
	}
	if rows[0].Percent != 80 || rows[0].Status != "promoted" {
		t.Fatalf("unexpected row[0] summary: %+v", rows[0])
	}
	if rows[0].SampleCount != 1 || rows[0].P95Latency != 60 {
		t.Fatalf("unexpected row[0] metrics: %+v", rows[0])
	}
	if rows[1].RolloutID != "rollout_dashboard_1" {
		t.Fatalf("expected second rollout row for rollout_dashboard_1, got %s", rows[1].RolloutID)
	}
	if rows[1].ErrorRate < 0.49 || rows[1].ErrorRate > 0.51 {
		t.Fatalf("expected row[1] error_rate around 0.5, got %f", rows[1].ErrorRate)
	}
	if rows[1].FallbackRate < 0.49 || rows[1].FallbackRate > 0.51 {
		t.Fatalf("expected row[1] fallback_rate around 0.5, got %f", rows[1].FallbackRate)
	}
	if rows[1].SampleCount != 2 {
		t.Fatalf("expected row[1] sample_count 2, got %d", rows[1].SampleCount)
	}
}
