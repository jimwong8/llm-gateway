package governance_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"llm-gateway/gateway/internal/governance"
)

func newRollbackServiceForTest(t *testing.T) (*governance.RollbackService, *governance.Store) {
	t.Helper()
	dsn := testPostgresDSN(t)
	store, err := governance.NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return governance.NewRollbackService(store), store
}

func TestRollbackExecuteRestoresPreviousActiveVersionAndCreatesDistributionEvent(t *testing.T) {
	svc, store := newRollbackServiceForTest(t)
	ctx := context.Background()
	previousID := "pv_rollback_previous"
	currentID := "pv_rollback_current"
	seedPolicyVersionForRolloutTest(t, store.DB(), previousID, "prod", governance.PolicyVersionSuperseded)
	seedPolicyVersionForRolloutTest(t, store.DB(), currentID, "prod", governance.PolicyVersionApproved)

	rolloutSvc := governance.NewRolloutService(store)
	rollout, _, err := rolloutSvc.Start(ctx, governance.StartRolloutInput{
		PolicyVersionID: currentID,
		TriggeredBy:     "alice",
		RolloutPercent:  25,
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	result, err := svc.Execute(ctx, governance.ExecuteRollbackInput{
		RolloutID: rollout.ID,
		Actor:     "ops-bot",
		Reason:    "error budget exceeded",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Rollout.Status != governance.RolloutStatusRolledBack {
		t.Fatalf("expected rollout rolled_back, got %s", result.Rollout.Status)
	}
	if result.RestoredPolicyVersionID != previousID {
		t.Fatalf("expected restored version %s, got %s", previousID, result.RestoredPolicyVersionID)
	}
	if result.RevertedPolicyVersionID != currentID {
		t.Fatalf("expected reverted version %s, got %s", currentID, result.RevertedPolicyVersionID)
	}
	if result.DistributionEvent.EventType != governance.DistributionEventRollback {
		t.Fatalf("expected rollback distribution event, got %s", result.DistributionEvent.EventType)
	}

	var previousStatus, currentStatus string
	if err := store.DB().QueryRowContext(ctx, `SELECT status FROM model_policy_versions WHERE policy_version_id = $1`, previousID).Scan(&previousStatus); err != nil {
		t.Fatalf("query previous status: %v", err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT status FROM model_policy_versions WHERE policy_version_id = $1`, currentID).Scan(&currentStatus); err != nil {
		t.Fatalf("query current status: %v", err)
	}
	if previousStatus != string(governance.PolicyVersionActive) {
		t.Fatalf("expected previous version active, got %s", previousStatus)
	}
	if currentStatus != string(governance.PolicyVersionRolledBack) {
		t.Fatalf("expected current version rolled_back, got %s", currentStatus)
	}

	events := loadDistributionEventsByRollout(t, store, rollout.ID)
	if len(events) != 2 {
		t.Fatalf("expected activation and rollback events, got %#v", events)
	}
	if events[0] != string(governance.DistributionEventActivated) || events[1] != string(governance.DistributionEventRollback) {
		t.Fatalf("unexpected event order: %#v", events)
	}
}

type rollbackRepoStub struct {
	executeFn func(ctx context.Context, rolloutID string) (governance.Rollout, string, string, error)
}

func (s rollbackRepoStub) Execute(ctx context.Context, rolloutID string) (governance.Rollout, string, string, error) {
	if s.executeFn == nil {
		return governance.Rollout{}, "", "", nil
	}
	return s.executeFn(ctx, rolloutID)
}

type versionSwitcherStub struct {
	activateFn func(ctx context.Context, versionID string) (governance.PolicyVersion, error)
}

func (s versionSwitcherStub) ActivateVersion(ctx context.Context, versionID string) (governance.PolicyVersion, error) {
	if s.activateFn == nil {
		return governance.PolicyVersion{}, nil
	}
	return s.activateFn(ctx, versionID)
}

func seedRolloutForRollbackServiceTest(t *testing.T, db *sql.DB, rolloutID, policyVersionID, environment string, status governance.RolloutStatus) {
	t.Helper()
	_, err := db.Exec(`
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
) VALUES ($1,$2,$3,'progressive',50,$4,'seed','tester',NOW(),NOW())
ON CONFLICT (rollout_id) DO NOTHING
`, rolloutID, policyVersionID, environment, string(status))
	if err != nil {
		t.Fatalf("seed rollout: %v", err)
	}
}

func TestRollbackServiceExecuteRequiresVersionActivationViaSeam(t *testing.T) {
	dsn := testPostgresDSN(t)
	store, err := governance.NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	rolloutID := "rollout_rollback_seam"
	seedRolloutForRollbackServiceTest(t, store.DB(), rolloutID, "pv_current", "prod", governance.RolloutStatusRunning)

	repo := rollbackRepoStub{
		executeFn: func(ctx context.Context, id string) (governance.Rollout, string, string, error) {
			return governance.Rollout{ID: id, TargetEnvironment: "prod", Status: governance.RolloutStatusRolledBack}, "pv_restored", "pv_current", nil
		},
	}
	calledVersionID := ""
	versions := versionSwitcherStub{
		activateFn: func(ctx context.Context, versionID string) (governance.PolicyVersion, error) {
			calledVersionID = versionID
			return governance.PolicyVersion{ID: versionID, Status: governance.PolicyVersionActive, Environment: "prod"}, nil
		},
	}
	svc := governance.NewRollbackServiceWithRepo(repo, governance.NewDistributionRepo(store), versions)

	result, err := svc.Execute(context.Background(), governance.ExecuteRollbackInput{RolloutID: rolloutID, Actor: "ops-bot", Reason: "forced rollback"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if calledVersionID != "pv_restored" {
		t.Fatalf("expected ActivateVersion called with pv_restored, got %s", calledVersionID)
	}
	if result.RestoredPolicyVersionID != "pv_restored" {
		t.Fatalf("expected restored id pv_restored, got %s", result.RestoredPolicyVersionID)
	}
	if result.DistributionEvent.EventType != governance.DistributionEventRollback {
		t.Fatalf("expected rollback distribution event, got %s", result.DistributionEvent.EventType)
	}
	payloadRaw, err := json.Marshal(result.DistributionEvent.Payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	payload := map[string]any{}
	if err := json.Unmarshal(payloadRaw, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["restored_policy_version_id"] != "pv_restored" {
		t.Fatalf("unexpected restored id in payload: %#v", payload)
	}
}

func TestRollbackServiceExecuteReturnsErrorWhenVersionSeamFails(t *testing.T) {
	dsn := testPostgresDSN(t)
	store, err := governance.NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	svc := governance.NewRollbackServiceWithRepo(
		rollbackRepoStub{executeFn: func(ctx context.Context, rolloutID string) (governance.Rollout, string, string, error) {
			return governance.Rollout{ID: rolloutID, TargetEnvironment: "prod", Status: governance.RolloutStatusRolledBack}, "pv_restored", "pv_current", nil
		}},
		governance.NewDistributionRepo(store),
		versionSwitcherStub{activateFn: func(ctx context.Context, versionID string) (governance.PolicyVersion, error) {
			return governance.PolicyVersion{}, errors.New("activate failed")
		}},
	)

	_, err = svc.Execute(context.Background(), governance.ExecuteRollbackInput{RolloutID: "rollout_missing", Actor: "ops-bot"})
	if err == nil {
		t.Fatalf("expected version seam activation error")
	}
}
