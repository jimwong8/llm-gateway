package governance_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"llm-gateway/gateway/internal/governance"
)

func newVersionServiceForTest(t *testing.T) (*governance.VersionService, *governance.Store) {
	t.Helper()
	dsn := testPostgresDSN(t)
	store, err := governance.NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return governance.NewVersionService(store), store
}

func seedRecommendationForVersionTest(t *testing.T, db *sql.DB, agentID, env, model string) string {
	t.Helper()
	recID := "rec_seed_" + model + "_" + env + "_" + agentID
	_, err := db.Exec(`
INSERT INTO model_recommendations (
    recommendation_id,
    agent_id,
    task_type,
    environment,
    recommended_model,
    candidates,
    score_breakdown,
    approval_required,
    status,
    created_by,
    created_at,
    updated_at
) VALUES ($1,$2,'code_review',$3,$4,'[]'::jsonb,'{}'::jsonb,TRUE,'ready','tester',NOW(),NOW())
ON CONFLICT (recommendation_id) DO NOTHING
`, recID, agentID, env, model)
	if err != nil {
		t.Fatalf("seed recommendation error = %v", err)
	}
	return recID
}

func seedApprovalForVersionTest(t *testing.T, db *sql.DB, recommendationID, environment, decision, finalModel string) string {
	t.Helper()
	approvalID := "approval_seed_" + recommendationID + "_" + decision
	scope := governance.EffectiveScope{Scope: "agent", Environment: environment}
	rawScope, err := json.Marshal(scope)
	if err != nil {
		t.Fatalf("marshal scope error = %v", err)
	}
	_, err = db.Exec(`
INSERT INTO model_approvals (
    approval_id,
    recommendation_id,
    decision,
    final_model,
    approval_reason,
    approved_by,
    effective_scope,
    created_at
) VALUES ($1,$2,$3,NULLIF($4,''),'seed','alice',$5::jsonb,NOW())
ON CONFLICT (approval_id) DO NOTHING
`, approvalID, recommendationID, decision, finalModel, string(rawScope))
	if err != nil {
		t.Fatalf("seed approval error = %v", err)
	}
	return approvalID
}

func TestCreatePolicyVersionFromApproval(t *testing.T) {
	svc, store := newVersionServiceForTest(t)
	ctx := context.Background()

	recID := seedRecommendationForVersionTest(t, store.DB(), "code-reviewer", "prod", "model-b")
	approvalID := seedApprovalForVersionTest(t, store.DB(), recID, "prod", "approved", "")

	version, err := svc.CreateFromApproval(ctx, approvalID, "alice")
	if err != nil {
		t.Fatalf("CreateFromApproval() error = %v", err)
	}
	if version.Status != governance.PolicyVersionDraft {
		t.Fatalf("unexpected status: %s", version.Status)
	}
	if version.Policy.Environment != "prod" {
		t.Fatalf("unexpected policy environment: %s", version.Policy.Environment)
	}
	agentPolicy, ok := version.Policy.Agents["code-reviewer"]
	if !ok {
		t.Fatalf("expected agent policy for code-reviewer")
	}
	if agentPolicy.PrimaryModel != "model-b" {
		t.Fatalf("unexpected primary model: %s", agentPolicy.PrimaryModel)
	}

	approved, err := svc.Approve(ctx, version.ID, "alice")
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if approved.Status != governance.PolicyVersionApproved {
		t.Fatalf("expected approved status, got %s", approved.Status)
	}

	active, err := svc.Activate(ctx, version.ID)
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if active.Status != governance.PolicyVersionActive {
		t.Fatalf("expected active status, got %s", active.Status)
	}
	if active.ActivatedAt.IsZero() {
		t.Fatalf("expected activated_at to be set")
	}
}

func TestVersionActivationRequiresApprovalAndSingleActivePerEnvironment(t *testing.T) {
	svc, store := newVersionServiceForTest(t)
	ctx := context.Background()

	recID1 := seedRecommendationForVersionTest(t, store.DB(), "code-reviewer", "staging", "model-a")
	approvalID1 := seedApprovalForVersionTest(t, store.DB(), recID1, "staging", "approved", "")
	v1, err := svc.CreateFromApproval(ctx, approvalID1, "alice")
	if err != nil {
		t.Fatalf("CreateFromApproval(v1) error = %v", err)
	}
	if _, err := svc.Activate(ctx, v1.ID); err == nil {
		t.Fatalf("expected activate draft to fail")
	}
	if _, err := svc.Approve(ctx, v1.ID, "alice"); err != nil {
		t.Fatalf("Approve(v1) error = %v", err)
	}
	if _, err := svc.Activate(ctx, v1.ID); err != nil {
		t.Fatalf("Activate(v1) error = %v", err)
	}

	recID2 := seedRecommendationForVersionTest(t, store.DB(), "code-reviewer", "staging", "model-c")
	approvalID2 := seedApprovalForVersionTest(t, store.DB(), recID2, "staging", "approved", "")
	v2, err := svc.CreateFromApproval(ctx, approvalID2, "bob")
	if err != nil {
		t.Fatalf("CreateFromApproval(v2) error = %v", err)
	}
	if _, err := svc.Approve(ctx, v2.ID, "bob"); err != nil {
		t.Fatalf("Approve(v2) error = %v", err)
	}
	v2Active, err := svc.Activate(ctx, v2.ID)
	if err != nil {
		t.Fatalf("Activate(v2) error = %v", err)
	}
	if v2Active.Status != governance.PolicyVersionActive {
		t.Fatalf("expected v2 active, got %s", v2Active.Status)
	}

	v1After, err := svc.Get(ctx, v1.ID)
	if err != nil {
		t.Fatalf("Get(v1) error = %v", err)
	}
	if v1After.Status != governance.PolicyVersionSuperseded {
		t.Fatalf("expected v1 superseded, got %s", v1After.Status)
	}
}

func TestGetPolicyVersionDiffPrefersSourceThenPrevious(t *testing.T) {
	svc, store := newVersionServiceForTest(t)
	ctx := context.Background()

	recID1 := seedRecommendationForVersionTest(t, store.DB(), "diff-agent", "qa", "model-x")
	approvalID1 := seedApprovalForVersionTest(t, store.DB(), recID1, "qa", "approved", "")

	v1, err := svc.CreateFromApproval(ctx, approvalID1, "alice")
	if err != nil {
		t.Fatalf("CreateFromApproval(v1) error = %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	v2, err := svc.CreateFromApproval(ctx, approvalID1, "alice")
	if err != nil {
		t.Fatalf("CreateFromApproval(v2) error = %v", err)
	}

	diffBySource, err := svc.GetDiff(ctx, v2.ID)
	if err != nil {
		t.Fatalf("GetDiff(v2) error = %v", err)
	}
	if diffBySource.BaseType != "source" {
		t.Fatalf("expected base_type=source, got %s", diffBySource.BaseType)
	}
	if diffBySource.BaseVersion == nil || diffBySource.BaseVersion.ID != v1.ID {
		t.Fatalf("expected source base version %s, got %+v", v1.ID, diffBySource.BaseVersion)
	}

	recID2 := seedRecommendationForVersionTest(t, store.DB(), "diff-agent", "qa", "model-y")
	approvalID2 := seedApprovalForVersionTest(t, store.DB(), recID2, "qa", "approved", "")
	time.Sleep(2 * time.Millisecond)
	v3, err := svc.CreateFromApproval(ctx, approvalID2, "bob")
	if err != nil {
		t.Fatalf("CreateFromApproval(v3) error = %v", err)
	}

	diffByPrevious, err := svc.GetDiff(ctx, v3.ID)
	if err != nil {
		t.Fatalf("GetDiff(v3) error = %v", err)
	}
	if diffByPrevious.BaseType != "previous" {
		t.Fatalf("expected base_type=previous, got %s", diffByPrevious.BaseType)
	}
	if diffByPrevious.BaseVersion == nil || diffByPrevious.BaseVersion.ID != v2.ID {
		t.Fatalf("expected previous base version %s, got %+v", v2.ID, diffByPrevious.BaseVersion)
	}
	if len(diffByPrevious.Changes) == 0 {
		t.Fatalf("expected non-empty changes for previous diff")
	}
}
