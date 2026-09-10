package governance_test

import (
	"context"
	"encoding/json"
	"testing"

	"llm-gateway/gateway/internal/governance"
)

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json marshal error = %v", err)
	}
	return string(raw)
}

func seedActivePolicyForDrift(t *testing.T, store *governance.Store, policyVersionID, environment, agentID, modelID string) {
	t.Helper()
	policy := governance.RuntimePolicy{
		Version:      1,
		Environment:  environment,
		DefaultModel: "model-default",
		Agents: map[string]governance.AgentPolicy{
			agentID: {PrimaryModel: modelID},
		},
	}

	_, err := store.DB().ExecContext(context.Background(), `
INSERT INTO model_policy_versions (
    policy_version_id,
    environment,
    status,
    policy_json,
    source_approval_id,
    created_by,
    activated_at,
    created_at
) VALUES ($1,$2,'active',$3::jsonb,'','tester',NOW(),NOW())
`, policyVersionID, environment, mustJSON(t, policy))
	if err != nil {
		t.Fatalf("seed active policy error = %v", err)
	}
}

func seedRecommendationForDrift(t *testing.T, store *governance.Store, recommendationID, environment, agentID, modelID string) {
	t.Helper()
	repo := governance.NewRecommendationRepo(store)
	_, err := repo.SaveRecommendation(context.Background(), governance.Recommendation{
		ID:               recommendationID,
		AgentID:          agentID,
		TaskType:         "code_review",
		Environment:      environment,
		RecommendedModel: modelID,
		Candidates: []governance.CandidateModel{
			{ModelID: modelID, Rank: 1, Composite: 0.95},
		},
		ScoreBreakdown:   governance.ScoreBreakdown{Quality: 0.95},
		ApprovalRequired: true,
		Status:           governance.RecommendationStatusDraft,
	})
	if err != nil {
		t.Fatalf("seed recommendation error = %v", err)
	}
}

func TestDriftServiceDetectModelMismatchCreatesOpenDrift(t *testing.T) {
	dsn := testPostgresDSN(t)
	store, err := governance.NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	environment := "prod"
	agentID := "code-reviewer"
	seedActivePolicyForDrift(t, store, "pv-drift-1", environment, agentID, "model-active")
	seedRecommendationForDrift(t, store, "rec-drift-1", environment, agentID, "model-recommended")

	svc := governance.NewDriftService(store)
	drift, detected, err := svc.DetectModelMismatch(context.Background(), governance.DetectDriftInput{
		TenantID:    "tenant-a",
		Environment: environment,
		AgentID:     agentID,
	})
	if err != nil {
		t.Fatalf("DetectModelMismatch() error = %v", err)
	}
	if !detected {
		t.Fatalf("expected detected=true")
	}
	if drift.Status != governance.PolicyDriftStatusDetected {
		t.Fatalf("expected drift status detected, got %s", drift.Status)
	}
	if drift.CurrentModelID != "model-active" || drift.RecommendedModelID != "model-recommended" {
		t.Fatalf("unexpected drift models: current=%s recommended=%s", drift.CurrentModelID, drift.RecommendedModelID)
	}

	var dbStatus, driftType string
	err = store.DB().QueryRowContext(context.Background(), `
SELECT status, drift_type
FROM policy_drifts
WHERE drift_id = $1
`, drift.ID).Scan(&dbStatus, &driftType)
	if err != nil {
		t.Fatalf("query policy_drifts error = %v", err)
	}
	if dbStatus != "open" {
		t.Fatalf("expected db drift status open, got %s", dbStatus)
	}
	if driftType != "model_mismatch" {
		t.Fatalf("expected drift_type model_mismatch, got %s", driftType)
	}
}

func TestDriftServiceDetectModelMismatchNoDrift(t *testing.T) {
	dsn := testPostgresDSN(t)
	store, err := governance.NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	environment := "staging"
	agentID := "security-reviewer"
	seedActivePolicyForDrift(t, store, "pv-drift-2", environment, agentID, "model-same")
	seedRecommendationForDrift(t, store, "rec-drift-2", environment, agentID, "model-same")

	svc := governance.NewDriftService(store)
	drift, detected, err := svc.DetectModelMismatch(context.Background(), governance.DetectDriftInput{
		Environment: environment,
		AgentID:     agentID,
	})
	if err != nil {
		t.Fatalf("DetectModelMismatch() error = %v", err)
	}
	if detected {
		t.Fatalf("expected detected=false, got drift=%+v", drift)
	}

	var count int
	err = store.DB().QueryRowContext(context.Background(), `
SELECT COUNT(1) FROM policy_drifts WHERE environment = $1 AND agent_id = $2
`, environment, agentID).Scan(&count)
	if err != nil {
		t.Fatalf("count policy_drifts error = %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no drift rows, got %d", count)
	}
}
