package governance_test

import (
	"context"
	"testing"

	"llm-gateway/gateway/internal/governance"
)

func newRollbackRecordRepoForTest(t *testing.T) (*governance.RollbackRecordRepo, *governance.Store) {
	t.Helper()
	dsn := testPostgresDSN(t)
	store, err := governance.NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return governance.NewRollbackRecordRepo(store), store
}

func TestRollbackRecordRepoCreateListGet(t *testing.T) {
	repo, store := newRollbackRecordRepoForTest(t)
	ctx := context.Background()
	seedPolicyVersionForRolloutTest(t, store.DB(), "pv_rr_prev", "prod", governance.PolicyVersionSuperseded)
	seedPolicyVersionForRolloutTest(t, store.DB(), "pv_rr_cur", "prod", governance.PolicyVersionActive)
	seedRolloutForRollbackServiceTest(t, store.DB(), "ro_rr_1", "pv_rr_cur", "prod", governance.RolloutStatusRolledBack)

	record, err := repo.Create(ctx, governance.ExecuteRollbackInput{RolloutID: "ro_rr_1", Actor: "ops", Reason: "manual"}, governance.ExecuteRollbackResult{
		Rollout:                 governance.Rollout{ID: "ro_rr_1", TargetEnvironment: "prod", Status: governance.RolloutStatusRolledBack},
		RestoredPolicyVersionID: "pv_rr_prev",
		RevertedPolicyVersionID: "pv_rr_cur",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if record.ID == "" || record.RolloutID != "ro_rr_1" {
		t.Fatalf("unexpected record: %+v", record)
	}

	items, err := repo.List(ctx, 20)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) == 0 {
		t.Fatalf("expected at least one rollback record")
	}

	got, err := repo.Get(ctx, record.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ID != record.ID || got.RestoredPolicyVersionID != "pv_rr_prev" {
		t.Fatalf("unexpected get result: %+v", got)
	}
}

func TestRollbackRecordRepoGetNotFound(t *testing.T) {
	repo, _ := newRollbackRecordRepoForTest(t)
	_, err := repo.Get(context.Background(), "not-exist")
	if err == nil {
		t.Fatalf("expected not found error")
	}
	if err != governance.ErrRollbackNotFound {
		t.Fatalf("expected ErrRollbackNotFound, got %v", err)
	}
}
