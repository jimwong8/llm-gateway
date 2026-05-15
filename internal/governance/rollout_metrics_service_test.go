package governance_test

import (
	"context"
	"testing"
	"time"

	"llm-gateway/gateway/internal/governance"
)

func newRolloutMetricsServiceForTest(t *testing.T) (*governance.RolloutMetricsService, *governance.Store) {
	t.Helper()
	dsn := testPostgresDSN(t)
	store, err := governance.NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return governance.NewRolloutMetricsService(store), store
}

func seedRuntimeSnapshotForMetricsTest(t *testing.T, store *governance.Store, snapshot governance.RuntimeDecisionSnapshotWrite) {
	t.Helper()
	repo := governance.NewSnapshotRepo(store)
	if err := repo.Save(context.Background(), snapshot); err != nil {
		t.Fatalf("Save(snapshot) error = %v", err)
	}
}

func TestRolloutMetricsAggregateAndGuardVerdicts(t *testing.T) {
	svc, store := newRolloutMetricsServiceForTest(t)
	now := time.Now().UTC()
	rolloutID := "rollout_metrics_guard"
	versionID := "pv_metrics_guard"
	seedRuntimeSnapshotForMetricsTest(t, store, governance.RuntimeDecisionSnapshotWrite{
		RequestID:          "req-metrics-1",
		RolloutID:          rolloutID,
		PolicyVersionID:    versionID,
		Environment:        "prod",
		TenantID:           "tenant-a",
		AgentID:            "agent-a",
		ResolvedModel:      "model-a",
		LatencyMS:          20,
		PolicyFallbackUsed: true,
		Success:            true,
		CreatedAt:          now.Add(-3 * time.Minute),
	})
	seedRuntimeSnapshotForMetricsTest(t, store, governance.RuntimeDecisionSnapshotWrite{
		RequestID:          "req-metrics-2",
		RolloutID:          rolloutID,
		PolicyVersionID:    versionID,
		Environment:        "prod",
		TenantID:           "tenant-a",
		AgentID:            "agent-a",
		ResolvedModel:      "model-a",
		LatencyMS:          120,
		SystemFallbackUsed: true,
		Success:            false,
		CreatedAt:          now.Add(-2 * time.Minute),
	})
	seedRuntimeSnapshotForMetricsTest(t, store, governance.RuntimeDecisionSnapshotWrite{
		RequestID:       "req-metrics-3",
		RolloutID:       rolloutID,
		PolicyVersionID: versionID,
		Environment:     "prod",
		TenantID:        "tenant-a",
		AgentID:         "agent-a",
		ResolvedModel:   "model-a",
		LatencyMS:       250,
		Success:         true,
		CreatedAt:       now.Add(-1 * time.Minute),
	})

	metrics, err := svc.Aggregate(context.Background(), governance.RolloutMetricsQuery{RolloutID: rolloutID, PolicyVersionID: versionID})
	if err != nil {
		t.Fatalf("Aggregate() error = %v", err)
	}
	if metrics.RequestsTotal != 3 {
		t.Fatalf("expected 3 requests, got %d", metrics.RequestsTotal)
	}
	if metrics.ErrorRate <= 0 || metrics.ErrorRate >= 1 {
		t.Fatalf("unexpected error rate: %f", metrics.ErrorRate)
	}
	if metrics.P95LatencyMillis != 120 {
		t.Fatalf("expected p95 latency 120, got %d", metrics.P95LatencyMillis)
	}
	if metrics.FallbackRequests != 2 {
		t.Fatalf("expected 2 fallback requests, got %d", metrics.FallbackRequests)
	}
	if metrics.FallbackRate < 0.66 || metrics.FallbackRate > 0.67 {
		t.Fatalf("expected fallback rate around 0.6667, got %f", metrics.FallbackRate)
	}

	keep, err := svc.EvaluateGuards(context.Background(), governance.RolloutMetricsQuery{RolloutID: rolloutID, PolicyVersionID: versionID}, governance.RolloutGuardThresholds{
		MinRequests:                       3,
		PauseErrorRateGTE:                 0.5,
		RollbackSuggestedErrorRateGTE:     0.8,
		RollbackRequiredErrorRateGTE:      1.0,
		PauseP95LatencyMillisGTE:          500,
		RollbackSuggestedP95LatencyGTE:    600,
		RollbackRequiredP95LatencyGTE:     700,
		PauseFallbackRateGTE:              0.9,
		RollbackSuggestedFallbackRateGTE:  0.95,
		RollbackRequiredFallbackRateGTE:   1.0,
	})
	if err != nil {
		t.Fatalf("EvaluateGuards(keep) error = %v", err)
	}
	if keep.Verdict != governance.RolloutGuardKeep {
		t.Fatalf("expected keep verdict, got %s", keep.Verdict)
	}

	pauseByErrorRate, err := svc.EvaluateGuards(context.Background(), governance.RolloutMetricsQuery{RolloutID: rolloutID, PolicyVersionID: versionID}, governance.RolloutGuardThresholds{
		PauseErrorRateGTE: 0.2,
	})
	if err != nil {
		t.Fatalf("EvaluateGuards(pause_by_error_rate) error = %v", err)
	}
	if pauseByErrorRate.Verdict != governance.RolloutGuardPause {
		t.Fatalf("expected pause verdict, got %s", pauseByErrorRate.Verdict)
	}

	pauseByP95, err := svc.EvaluateGuards(context.Background(), governance.RolloutMetricsQuery{RolloutID: rolloutID, PolicyVersionID: versionID}, governance.RolloutGuardThresholds{
		PauseP95LatencyMillisGTE: 100,
	})
	if err != nil {
		t.Fatalf("EvaluateGuards(pause_by_p95) error = %v", err)
	}
	if pauseByP95.Verdict != governance.RolloutGuardPause {
		t.Fatalf("expected pause verdict by p95, got %s", pauseByP95.Verdict)
	}

	suggestedByErrorRate, err := svc.EvaluateGuards(context.Background(), governance.RolloutMetricsQuery{RolloutID: rolloutID, PolicyVersionID: versionID}, governance.RolloutGuardThresholds{
		RollbackSuggestedErrorRateGTE: 0.3,
		RollbackRequiredErrorRateGTE:  0.9,
	})
	if err != nil {
		t.Fatalf("EvaluateGuards(rollback_suggested_by_error_rate) error = %v", err)
	}
	if suggestedByErrorRate.Verdict != governance.RolloutGuardRollbackSuggested {
		t.Fatalf("expected rollback_suggested verdict, got %s", suggestedByErrorRate.Verdict)
	}

	requiredByErrorRate, err := svc.EvaluateGuards(context.Background(), governance.RolloutMetricsQuery{RolloutID: rolloutID, PolicyVersionID: versionID}, governance.RolloutGuardThresholds{
		RollbackRequiredErrorRateGTE: 0.3,
	})
	if err != nil {
		t.Fatalf("EvaluateGuards(rollback_required_by_error_rate) error = %v", err)
	}
	if requiredByErrorRate.Verdict != governance.RolloutGuardRollbackRequired {
		t.Fatalf("expected rollback_required verdict, got %s", requiredByErrorRate.Verdict)
	}

	suggestedByP95, err := svc.EvaluateGuards(context.Background(), governance.RolloutMetricsQuery{RolloutID: rolloutID, PolicyVersionID: versionID}, governance.RolloutGuardThresholds{
		RollbackSuggestedP95LatencyGTE: 100,
		RollbackRequiredP95LatencyGTE:  200,
	})
	if err != nil {
		t.Fatalf("EvaluateGuards(rollback_suggested_by_p95) error = %v", err)
	}
	if suggestedByP95.Verdict != governance.RolloutGuardRollbackSuggested {
		t.Fatalf("expected rollback_suggested verdict by p95, got %s", suggestedByP95.Verdict)
	}

	requiredByP95, err := svc.EvaluateGuards(context.Background(), governance.RolloutMetricsQuery{RolloutID: rolloutID, PolicyVersionID: versionID}, governance.RolloutGuardThresholds{
		RollbackRequiredP95LatencyGTE: 100,
	})
	if err != nil {
		t.Fatalf("EvaluateGuards(rollback_required_by_p95) error = %v", err)
	}
	if requiredByP95.Verdict != governance.RolloutGuardRollbackRequired {
		t.Fatalf("expected rollback_required verdict by p95, got %s", requiredByP95.Verdict)
	}

	suggestedByFallback, err := svc.EvaluateGuards(context.Background(), governance.RolloutMetricsQuery{RolloutID: rolloutID, PolicyVersionID: versionID}, governance.RolloutGuardThresholds{
		RollbackSuggestedFallbackRateGTE: 0.6,
		RollbackRequiredFallbackRateGTE:  0.9,
	})
	if err != nil {
		t.Fatalf("EvaluateGuards(rollback_suggested_by_fallback) error = %v", err)
	}
	if suggestedByFallback.Verdict != governance.RolloutGuardRollbackSuggested {
		t.Fatalf("expected rollback_suggested verdict by fallback, got %s", suggestedByFallback.Verdict)
	}

	requiredByFallback, err := svc.EvaluateGuards(context.Background(), governance.RolloutMetricsQuery{RolloutID: rolloutID, PolicyVersionID: versionID}, governance.RolloutGuardThresholds{
		RollbackRequiredFallbackRateGTE: 0.6,
	})
	if err != nil {
		t.Fatalf("EvaluateGuards(rollback_required_by_fallback) error = %v", err)
	}
	if requiredByFallback.Verdict != governance.RolloutGuardRollbackRequired {
		t.Fatalf("expected rollback_required verdict by fallback, got %s", requiredByFallback.Verdict)
	}
}
