package longcontext

import (
	"context"
	"testing"
)

func TestEnforceTenantConcurrency(t *testing.T) {
	r := &PostgresRepository{db: nil}
	// With nil db, CountRunningTasksByTenant will fail on query; we only test
	// that the helper signature compiles and returns an error for nil db.
	err := EnforceTenantConcurrency(context.Background(), r, "tenant-a")
	if err == nil {
		t.Fatal("expected error for nil db")
	}
}

func TestEstimateCost(t *testing.T) {
	if got := estimateCost(1000); got != 0.01 {
		t.Fatalf("estimateCost(1000)=%.4f, want 0.01", got)
	}
}
