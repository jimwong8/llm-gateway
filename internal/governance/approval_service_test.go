package governance_test

import (
	"context"
	"strings"
	"testing"

	"llm-gateway/gateway/internal/governance"
)

func newApprovalServiceForTest(t *testing.T) *governance.ApprovalService {
	t.Helper()
	dsn := testPostgresDSN(t)
	store, err := governance.NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return governance.NewApprovalService(store)
}

func seedRecommendation(t *testing.T, store *governance.Store, recommendationID string, status governance.RecommendationStatus) {
	t.Helper()
	repo := governance.NewRecommendationRepo(store)
	_, err := repo.SaveRecommendation(context.Background(), governance.Recommendation{
		ID:               recommendationID,
		AgentID:          "security-reviewer",
		TaskType:         "security_audit",
		Environment:      "prod",
		RecommendedModel: "model-a",
		Candidates: []governance.CandidateModel{
			{ModelID: "model-a", Rank: 1, Composite: 0.91},
			{ModelID: "model-b", Rank: 2, Composite: 0.87},
		},
		ScoreBreakdown:   governance.ScoreBreakdown{Quality: 0.9, Cost: 0.7, Safety: 0.95},
		ApprovalRequired: true,
		Status:           status,
	})
	if err != nil {
		t.Fatalf("SaveRecommendation() error = %v", err)
	}
}

func fetchRecommendationStatus(t *testing.T, store *governance.Store, recommendationID string) string {
	t.Helper()
	var status string
	err := store.DB().QueryRowContext(context.Background(), `
SELECT status
FROM model_recommendations
WHERE recommendation_id = $1
`, recommendationID).Scan(&status)
	if err != nil {
		t.Fatalf("query recommendation status error = %v", err)
	}
	return status
}

func TestApproveRecommendationWithOverride(t *testing.T) {
	svc := newApprovalServiceForTest(t)
	ctx := context.Background()
	recommendationID := "rec-approve-override"
	seedRecommendation(t, svc.Store(), recommendationID, governance.RecommendationStatusPending)

	approval, err := svc.Decide(ctx, governance.ApprovalInput{
		RecommendationID: recommendationID,
		Decision:         governance.ApprovalDecisionOverridden,
		FinalModel:       "model-c",
		ApprovalReason:   "cost too high",
		ApprovedBy:       "alice",
		EffectiveScope: governance.EffectiveScope{
			Scope:       "agent",
			Environment: "prod",
		},
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if approval.Status != governance.ApprovalStatusOverridden {
		t.Fatalf("expected status overridden, got %s", approval.Status)
	}
	if approval.FinalModel != "model-c" {
		t.Fatalf("unexpected final model: %s", approval.FinalModel)
	}
	status := fetchRecommendationStatus(t, svc.Store(), recommendationID)
	if status != string(governance.RecommendationStatusApproved) {
		t.Fatalf("expected recommendation status approved, got %s", status)
	}
}

func TestApproveRecommendationRequiresPendingStatus(t *testing.T) {
	svc := newApprovalServiceForTest(t)
	ctx := context.Background()
	recommendationID := "rec-non-pending"
	seedRecommendation(t, svc.Store(), recommendationID, governance.RecommendationStatusApproved)

	_, err := svc.Decide(ctx, governance.ApprovalInput{
		RecommendationID: recommendationID,
		Decision:         governance.ApprovalDecisionApproved,
		ApprovedBy:       "alice",
		EffectiveScope: governance.EffectiveScope{
			Scope:       "agent",
			Environment: "prod",
		},
	})
	if err == nil {
		t.Fatalf("expected error for non-pending recommendation")
	}
	if !strings.Contains(err.Error(), governance.ErrApprovalRecommendationNotPending.Error()) {
		t.Fatalf("expected non-pending error, got %v", err)
	}
}

func TestApproveRecommendationOverrideRequiresFinalModel(t *testing.T) {
	svc := newApprovalServiceForTest(t)
	recommendationID := "rec-override-without-final"
	seedRecommendation(t, svc.Store(), recommendationID, governance.RecommendationStatusPending)

	_, err := svc.Decide(context.Background(), governance.ApprovalInput{
		RecommendationID: recommendationID,
		Decision:         governance.ApprovalDecisionOverridden,
		ApprovedBy:       "alice",
		EffectiveScope: governance.EffectiveScope{
			Scope:       "agent",
			Environment: "prod",
		},
	})
	if err == nil {
		t.Fatalf("expected error for missing final model")
	}
	if !strings.Contains(err.Error(), "final_model is required") {
		t.Fatalf("expected final_model validation error, got %v", err)
	}
}

func TestApproveRecommendationRejectRequiresReason(t *testing.T) {
	svc := newApprovalServiceForTest(t)
	recommendationID := "rec-reject-no-reason"
	seedRecommendation(t, svc.Store(), recommendationID, governance.RecommendationStatusPending)

	_, err := svc.Decide(context.Background(), governance.ApprovalInput{
		RecommendationID: recommendationID,
		Decision:         governance.ApprovalDecisionRejected,
		ApprovedBy:       "alice",
		EffectiveScope: governance.EffectiveScope{
			Scope:       "agent",
			Environment: "prod",
		},
	})
	if err == nil {
		t.Fatalf("expected error for missing reject reason")
	}
	if !strings.Contains(err.Error(), "approval_reason is required") {
		t.Fatalf("expected approval_reason validation error, got %v", err)
	}
}

func TestRejectRecommendationTransitionsToRejected(t *testing.T) {
	svc := newApprovalServiceForTest(t)
	ctx := context.Background()
	recommendationID := "rec-reject-ok"
	seedRecommendation(t, svc.Store(), recommendationID, governance.RecommendationStatusPending)

	approval, err := svc.Decide(ctx, governance.ApprovalInput{
		RecommendationID: recommendationID,
		Decision:         governance.ApprovalDecisionRejected,
		ApprovalReason:   "quality regression",
		ApprovedBy:       "alice",
		EffectiveScope: governance.EffectiveScope{
			Scope:       "agent",
			Environment: "prod",
		},
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if approval.Status != governance.ApprovalStatusRejected {
		t.Fatalf("expected status rejected, got %s", approval.Status)
	}
	status := fetchRecommendationStatus(t, svc.Store(), recommendationID)
	if status != string(governance.RecommendationStatusRejected) {
		t.Fatalf("expected recommendation status rejected, got %s", status)
	}
}
