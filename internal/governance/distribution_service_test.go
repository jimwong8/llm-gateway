package governance_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"llm-gateway/gateway/internal/governance"
)

func newDistributionServiceForTest(t *testing.T) (*governance.DistributionService, *governance.Store) {
	t.Helper()
	dsn := testPostgresDSN(t)
	store, err := governance.NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return governance.NewDistributionService(store), store
}

func seedRolloutForDistributionTest(t *testing.T, db *sql.DB, rolloutID, policyVersionID, environment string) {
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
) VALUES ($1,$2,$3,'progressive',30,'running','seed','tester',NOW(),NOW())
ON CONFLICT (rollout_id) DO NOTHING
`, rolloutID, policyVersionID, environment)
	if err != nil {
		t.Fatalf("seed rollout error = %v", err)
	}
}

func loadDistributionEventByID(t *testing.T, db *sql.DB, eventID string) (string, string, string, string, map[string]any) {
	t.Helper()
	var (
		policyVersionID sql.NullString
		rolloutID       sql.NullString
		environment     string
		eventType       string
		payloadRaw      []byte
	)
	err := db.QueryRowContext(context.Background(), `
SELECT policy_version_id, rollout_id, environment, event_type, payload
FROM model_distribution_events
WHERE event_id = $1
`, eventID).Scan(&policyVersionID, &rolloutID, &environment, &eventType, &payloadRaw)
	if err != nil {
		t.Fatalf("query distribution event error = %v", err)
	}
	payload := map[string]any{}
	if err := json.Unmarshal(payloadRaw, &payload); err != nil {
		t.Fatalf("unmarshal payload error = %v", err)
	}
	return policyVersionID.String, rolloutID.String, environment, eventType, payload
}

func TestDistributionServiceCreateActivationEvent(t *testing.T) {
	svc, store := newDistributionServiceForTest(t)
	rollout := governance.Rollout{
		ID:                "rollout_distribution_activation",
		PolicyVersionID:   "pv_distribution_activation",
		TargetEnvironment: "prod",
		RolloutMode:       "progressive",
		RolloutPercent:    25,
		TriggeredBy:       "alice",
		StartedAt:         time.Now().UTC(),
	}
	seedRolloutForDistributionTest(t, store.DB(), rollout.ID, rollout.PolicyVersionID, rollout.TargetEnvironment)

	event, err := svc.CreateActivationEvent(context.Background(), rollout)
	if err != nil {
		t.Fatalf("CreateActivationEvent() error = %v", err)
	}
	if event.EventType != governance.DistributionEventActivated {
		t.Fatalf("expected activation event, got %s", event.EventType)
	}

	pvID, rolloutID, env, eventType, payload := loadDistributionEventByID(t, store.DB(), event.ID)
	if pvID != rollout.PolicyVersionID {
		t.Fatalf("unexpected policy_version_id: %s", pvID)
	}
	if rolloutID != rollout.ID {
		t.Fatalf("unexpected rollout_id: %s", rolloutID)
	}
	if env != rollout.TargetEnvironment {
		t.Fatalf("unexpected environment: %s", env)
	}
	if eventType != string(governance.DistributionEventActivated) {
		t.Fatalf("unexpected event_type: %s", eventType)
	}
	if payload["rollout_mode"] != "progressive" {
		t.Fatalf("unexpected payload rollout_mode: %#v", payload["rollout_mode"])
	}
}

func TestDistributionServiceCreateRollbackEvent(t *testing.T) {
	svc, store := newDistributionServiceForTest(t)
	rollout := governance.Rollout{
		ID:                "rollout_distribution_rollback",
		PolicyVersionID:   "pv_distribution_rollback_new",
		TargetEnvironment: "staging",
	}
	seedRolloutForDistributionTest(t, store.DB(), rollout.ID, rollout.PolicyVersionID, rollout.TargetEnvironment)

	restoredID := "pv_distribution_rollback_old"
	revertedID := rollout.PolicyVersionID
	event, err := svc.CreateRollbackEvent(context.Background(), rollout, "bob", "manual rollback", restoredID, revertedID)
	if err != nil {
		t.Fatalf("CreateRollbackEvent() error = %v", err)
	}
	if event.EventType != governance.DistributionEventRollback {
		t.Fatalf("expected rollback event, got %s", event.EventType)
	}

	pvID, rolloutID, env, eventType, payload := loadDistributionEventByID(t, store.DB(), event.ID)
	if pvID != restoredID {
		t.Fatalf("unexpected policy_version_id: %s", pvID)
	}
	if rolloutID != rollout.ID {
		t.Fatalf("unexpected rollout_id: %s", rolloutID)
	}
	if env != rollout.TargetEnvironment {
		t.Fatalf("unexpected environment: %s", env)
	}
	if eventType != string(governance.DistributionEventRollback) {
		t.Fatalf("unexpected event_type: %s", eventType)
	}
	if payload["restored_policy_version_id"] != restoredID {
		t.Fatalf("unexpected restored_policy_version_id payload: %#v", payload["restored_policy_version_id"])
	}
	if payload["reverted_policy_version_id"] != revertedID {
		t.Fatalf("unexpected reverted_policy_version_id payload: %#v", payload["reverted_policy_version_id"])
	}
}
