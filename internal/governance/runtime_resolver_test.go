package governance_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"llm-gateway/gateway/internal/governance"
)

func newRuntimeResolverForTest(t *testing.T) *governance.RuntimeResolver {
	t.Helper()
	dsn := testPostgresDSN(t)
	store, err := governance.NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return governance.NewRuntimeResolver(store)
}

type runtimePolicySeed struct {
	Environment   string
	AgentID       string
	PrimaryModel  string
	FallbackChain []string
	DefaultModel  string
}

func seedActivePolicy(t *testing.T, store *governance.Store, seed runtimePolicySeed) string {
	t.Helper()
	policyID := "pv_rt_" + time.Now().UTC().Format("20060102150405.000000000")

	policy := governance.RuntimePolicy{
		Version:      1,
		Environment:  seed.Environment,
		DefaultModel: seed.DefaultModel,
		Agents:       map[string]governance.AgentPolicy{},
	}
	if seed.AgentID != "" {
		policy.Agents[seed.AgentID] = governance.AgentPolicy{
			PrimaryModel:  seed.PrimaryModel,
			FallbackChain: seed.FallbackChain,
		}
	}
	policyRaw, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("marshal policy: %v", err)
	}

	db := store.DB()
	if db == nil {
		t.Fatalf("store db is nil")
	}

	if _, err := db.ExecContext(context.Background(), `
UPDATE model_policy_versions
SET status = 'superseded'
WHERE environment = $1 AND status = 'active'
`, seed.Environment); err != nil {
		t.Fatalf("clear active policy: %v", err)
	}

	_, err = db.ExecContext(context.Background(), `
INSERT INTO model_policy_versions (
	policy_version_id,
	environment,
	status,
	policy_json,
	created_by,
	activated_at,
	created_at
) VALUES ($1, $2, 'active', $3::jsonb, 'tester', NOW(), NOW())
`, policyID, seed.Environment, string(policyRaw))
	if err != nil {
		t.Fatalf("seed active policy: %v", err)
	}
	return policyID
}

func snapshotExists(t *testing.T, store *governance.Store, requestID string) bool {
	t.Helper()
	var exists bool
	err := store.DB().QueryRowContext(context.Background(), `
SELECT EXISTS (
	SELECT 1 FROM runtime_decision_snapshots WHERE request_id = $1
)
`, requestID).Scan(&exists)
	if err != nil {
		t.Fatalf("query snapshot exists: %v", err)
	}
	return exists
}

func loadSnapshotFlags(t *testing.T, store *governance.Store, requestID string) (bool, bool, string, []string) {
	t.Helper()
	var (
		policyFallback bool
		systemFallback bool
		matchedScope   sql.NullString
		fallbackRaw    []byte
	)
	err := store.DB().QueryRowContext(context.Background(), `
SELECT policy_fallback_used, system_fallback_used, matched_scope_type, fallback_chain
FROM runtime_decision_snapshots
WHERE request_id = $1
`, requestID).Scan(&policyFallback, &systemFallback, &matchedScope, &fallbackRaw)
	if err != nil {
		t.Fatalf("load snapshot flags: %v", err)
	}
	fallbacks := make([]string, 0)
	if len(fallbackRaw) > 0 {
		if err := json.Unmarshal(fallbackRaw, &fallbacks); err != nil {
			t.Fatalf("unmarshal fallback chain: %v", err)
		}
	}
	scope := ""
	if matchedScope.Valid {
		scope = matchedScope.String
	}
	return policyFallback, systemFallback, scope, fallbacks
}

func TestResolveRuntimePolicyAndWriteSnapshot(t *testing.T) {
	svc := newRuntimeResolverForTest(t)
	seedActivePolicy(t, svc.Store(), runtimePolicySeed{
		Environment:   "prod",
		AgentID:       "security-reviewer",
		PrimaryModel:  "model-x",
		FallbackChain: []string{"model-y"},
		DefaultModel:  "model-default",
	})

	decision, err := svc.Resolve(context.Background(), governance.ResolveInput{
		RequestID:   "11111111-1111-1111-1111-111111111111",
		Environment: "prod",
		AgentID:     "security-reviewer",
		TenantID:    "tenant-a",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if decision.ResolvedModel != "model-x" {
		t.Fatalf("unexpected model: %s", decision.ResolvedModel)
	}
	if decision.MatchedScopeType != "agent" {
		t.Fatalf("unexpected matched scope type: %s", decision.MatchedScopeType)
	}
	if decision.PolicyFallbackUsed {
		t.Fatalf("expected policy_fallback_used=false")
	}
	if decision.SystemFallbackUsed {
		t.Fatalf("expected system_fallback_used=false")
	}
	if !snapshotExists(t, svc.Store(), decision.RequestID) {
		t.Fatalf("expected snapshot persisted")
	}

	policyFallback, systemFallback, matchedScope, fallbackChain := loadSnapshotFlags(t, svc.Store(), decision.RequestID)
	if policyFallback {
		t.Fatalf("snapshot policy_fallback_used should be false")
	}
	if systemFallback {
		t.Fatalf("snapshot system_fallback_used should be false")
	}
	if matchedScope != "agent" {
		t.Fatalf("unexpected snapshot matched_scope_type: %s", matchedScope)
	}
	if len(fallbackChain) != 1 || fallbackChain[0] != "model-y" {
		t.Fatalf("unexpected snapshot fallback chain: %#v", fallbackChain)
	}
}

func TestResolveUsesPolicyFallbackWhenPrimaryModelMissing(t *testing.T) {
	svc := newRuntimeResolverForTest(t)
	seedActivePolicy(t, svc.Store(), runtimePolicySeed{
		Environment:   "staging",
		AgentID:       "security-reviewer",
		PrimaryModel:  "",
		FallbackChain: []string{"model-fallback-a", "model-fallback-b"},
	})

	decision, err := svc.Resolve(context.Background(), governance.ResolveInput{
		RequestID:   "22222222-2222-2222-2222-222222222222",
		Environment: "staging",
		AgentID:     "security-reviewer",
		TenantID:    "tenant-b",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if decision.ResolvedModel != "model-fallback-a" {
		t.Fatalf("unexpected resolved model: %s", decision.ResolvedModel)
	}
	if !decision.PolicyFallbackUsed {
		t.Fatalf("expected policy fallback used")
	}
	if decision.SystemFallbackUsed {
		t.Fatalf("expected system fallback not used")
	}
	if len(decision.FallbackChain) != 1 || decision.FallbackChain[0] != "model-fallback-b" {
		t.Fatalf("unexpected fallback chain: %#v", decision.FallbackChain)
	}

	policyFallback, systemFallback, matchedScope, _ := loadSnapshotFlags(t, svc.Store(), decision.RequestID)
	if !policyFallback || systemFallback {
		t.Fatalf("unexpected snapshot fallback flags: policy=%v system=%v", policyFallback, systemFallback)
	}
	if matchedScope != "agent" {
		t.Fatalf("unexpected snapshot matched scope: %s", matchedScope)
	}
}

func TestResolveUsesSystemFallbackWhenNoPolicyScopeMatch(t *testing.T) {
	svc := newRuntimeResolverForTest(t)
	seedActivePolicy(t, svc.Store(), runtimePolicySeed{
		Environment:  "qa",
		AgentID:      "other-agent",
		DefaultModel: "",
	})

	decision, err := svc.Resolve(context.Background(), governance.ResolveInput{
		RequestID:           "33333333-3333-3333-3333-333333333333",
		Environment:         "qa",
		AgentID:             "security-reviewer",
		TenantID:            "tenant-c",
		SystemFallbackModel: "model-system-safe",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if decision.ResolvedModel != "model-system-safe" {
		t.Fatalf("unexpected resolved model: %s", decision.ResolvedModel)
	}
	if decision.PolicyFallbackUsed {
		t.Fatalf("expected policy fallback false")
	}
	if !decision.SystemFallbackUsed {
		t.Fatalf("expected system fallback true")
	}
	if decision.MatchedScopeType != "system_fallback" {
		t.Fatalf("unexpected matched scope type: %s", decision.MatchedScopeType)
	}

	policyFallback, systemFallback, matchedScope, fallbackChain := loadSnapshotFlags(t, svc.Store(), decision.RequestID)
	if policyFallback || !systemFallback {
		t.Fatalf("unexpected snapshot fallback flags: policy=%v system=%v", policyFallback, systemFallback)
	}
	if matchedScope != "system_fallback" {
		t.Fatalf("unexpected snapshot matched scope: %s", matchedScope)
	}
	if len(fallbackChain) != 0 {
		t.Fatalf("expected empty fallback chain, got %#v", fallbackChain)
	}
}
