package governance_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"llm-gateway/gateway/internal/governance"
)

func testPostgresDSN(t *testing.T) string {
	t.Helper()

	if dsn := strings.TrimSpace(os.Getenv("POSTGRES_DSN")); dsn != "" {
		return dsn
	}
	if dsn := strings.TrimSpace(os.Getenv("GOVERNANCE_TEST_POSTGRES_DSN")); dsn != "" {
		return dsn
	}
	t.Skip("skip integration test: set POSTGRES_DSN or GOVERNANCE_TEST_POSTGRES_DSN")
	return ""
}

func TestGovernanceStoreBootstrapsSchema(t *testing.T) {
	dsn := testPostgresDSN(t)
	store, err := governance.NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	tables := []string{
		"model_recommendations",
		"model_approvals",
		"model_policy_versions",
		"model_rollouts",
		"runtime_decision_snapshots",
		"evaluation_runs",
		"policy_drifts",
	}

	ctx := context.Background()
	for _, table := range tables {
		if !store.TableExists(ctx, table) {
			t.Fatalf("expected table %s to exist", table)
		}
	}
}
