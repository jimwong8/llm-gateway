package governance_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"llm-gateway/gateway/internal/governance"
)

func newRolloutServiceForTest(t *testing.T) (*governance.RolloutService, *governance.Store) {
	t.Helper()
	dsn := testPostgresDSN(t)
	store, err := governance.NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return governance.NewRolloutService(store), store
}

func seedPolicyVersionForRolloutTest(t *testing.T, db *sql.DB, versionID, environment string, status governance.PolicyVersionStatus) {
	t.Helper()
	policy := governance.RuntimePolicy{Version: time.Now().UTC().UnixNano(), Environment: environment, DefaultModel: "model-seed"}
	policyRaw, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("marshal policy: %v", err)
	}
	_, err = db.Exec(`
INSERT INTO model_policy_versions (
    policy_version_id,
    environment,
    status,
    policy_json,
    created_by,
    approved_by,
    approved_at,
    activated_at,
    created_at
) VALUES ($1,$2,$3,$4::jsonb,'tester','tester',NOW(),CASE WHEN $3 = 'active' THEN NOW() ELSE NULL END,NOW())
ON CONFLICT (policy_version_id) DO NOTHING
`, versionID, environment, string(status), string(policyRaw))
	if err != nil {
		t.Fatalf("seed policy version: %v", err)
	}
}

func loadDistributionEventsByRollout(t *testing.T, store *governance.Store, rolloutID string) []string {
	t.Helper()
	rows, err := store.DB().QueryContext(context.Background(), `
SELECT event_type
FROM model_distribution_events
WHERE rollout_id = $1
ORDER BY created_at ASC, id ASC
`, rolloutID)
	if err != nil {
		t.Fatalf("query distribution events: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var eventType string
		if err := rows.Scan(&eventType); err != nil {
			t.Fatalf("scan distribution event: %v", err)
		}
		out = append(out, eventType)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate distribution events: %v", err)
	}
	return out
}

func TestRolloutStartAndPromote(t *testing.T) {
	svc, store := newRolloutServiceForTest(t)
	ctx := context.Background()
	versionID := "pv_rollout_start_promote"
	seedPolicyVersionForRolloutTest(t, store.DB(), versionID, "prod", governance.PolicyVersionApproved)

	rollout, event, err := svc.Start(ctx, governance.StartRolloutInput{
		PolicyVersionID: versionID,
		RolloutMode:     "progressive",
		RolloutPercent:  10,
		TriggerReason:   "initial rollout",
		TriggeredBy:     "alice",
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if rollout.Status != governance.RolloutStatusRunning {
		t.Fatalf("expected rollout running, got %s", rollout.Status)
	}
	if rollout.RolloutPercent != 10 {
		t.Fatalf("expected rollout percent 10, got %d", rollout.RolloutPercent)
	}
	if event.EventType != governance.DistributionEventActivated {
		t.Fatalf("expected activation distribution event, got %s", event.EventType)
	}

	promoted, err := svc.Promote(ctx, governance.PromoteRolloutInput{RolloutID: rollout.ID, RolloutPercent: 60, GuardSummary: "healthy"})
	if err != nil {
		t.Fatalf("Promote() error = %v", err)
	}
	if promoted.Status != governance.RolloutStatusRunning {
		t.Fatalf("expected rollout running after partial promote, got %s", promoted.Status)
	}
	if promoted.RolloutPercent != 60 {
		t.Fatalf("expected rollout percent 60, got %d", promoted.RolloutPercent)
	}

	promoted, err = svc.Promote(ctx, governance.PromoteRolloutInput{RolloutID: rollout.ID, RolloutPercent: 100, GuardSummary: "healthy"})
	if err != nil {
		t.Fatalf("Promote() error = %v", err)
	}
	if promoted.Status != governance.RolloutStatusPromoted {
		t.Fatalf("expected rollout promoted, got %s", promoted.Status)
	}
	if promoted.RolloutPercent != 100 {
		t.Fatalf("expected rollout percent 100, got %d", promoted.RolloutPercent)
	}

	var versionStatus string
	err = store.DB().QueryRowContext(ctx, `SELECT status FROM model_policy_versions WHERE policy_version_id = $1`, versionID).Scan(&versionStatus)
	if err != nil {
		t.Fatalf("query version status: %v", err)
	}
	if versionStatus != string(governance.PolicyVersionActive) {
		t.Fatalf("expected started version active, got %s", versionStatus)
	}

	events := loadDistributionEventsByRollout(t, store, rollout.ID)
	if len(events) != 1 || events[0] != string(governance.DistributionEventActivated) {
		t.Fatalf("unexpected distribution events: %#v", events)
	}
}

func TestRolloutStartRequiresApprovedVersion(t *testing.T) {
	svc, store := newRolloutServiceForTest(t)
	versionID := "pv_rollout_requires_approved"
	seedPolicyVersionForRolloutTest(t, store.DB(), versionID, "staging", governance.PolicyVersionDraft)

	_, _, err := svc.Start(context.Background(), governance.StartRolloutInput{
		PolicyVersionID: versionID,
		TriggeredBy:     "alice",
	})
	if err == nil {
		t.Fatalf("expected Start() to fail for non-approved version")
	}
}
