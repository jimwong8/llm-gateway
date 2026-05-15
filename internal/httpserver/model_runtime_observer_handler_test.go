package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"llm-gateway/gateway/internal/governance"
)

func testRuntimeObserverPostgresDSN(t *testing.T) string {
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

func seedObserverActivePolicy(t *testing.T, db *sql.DB, environment, policyVersionID string, activatedAt time.Time) {
	t.Helper()
	policyRaw, err := json.Marshal(governance.RuntimePolicy{
		Version:      1,
		Environment:  environment,
		DefaultModel: "gpt-4o-mini",
	})
	if err != nil {
		t.Fatalf("marshal policy: %v", err)
	}
	_, err = db.ExecContext(context.Background(), `
INSERT INTO model_policy_versions (
	policy_version_id,
	environment,
	status,
	policy_json,
	created_by,
	approved_by,
	created_at,
	approved_at,
	activated_at
) VALUES ($1, $2, 'active', $3::jsonb, 'tester', 'tester', $4, $4, $4)
ON CONFLICT (policy_version_id) DO NOTHING
`, policyVersionID, environment, string(policyRaw), activatedAt.UTC())
	if err != nil {
		t.Fatalf("seed active policy failed: %v", err)
	}
}

func seedObserverRuntimeDecision(t *testing.T, db *sql.DB, requestID, environment, policyVersionID, resolvedModel string, createdAt time.Time) {
	t.Helper()
	matchedScopeRaw := `{"environment":"` + environment + `"}`
	fallbackRaw := `[]`
	_, err := db.ExecContext(context.Background(), `
INSERT INTO runtime_decision_snapshots (
	request_id,
	policy_version_id,
	rollout_id,
	environment,
	tenant_id,
	agent_id,
	task_type,
	matched_scope_type,
	matched_scope,
	resolved_model,
	fallback_chain,
	policy_fallback_used,
	system_fallback_used,
	success,
	created_at
) VALUES ($1, $2, 'ro-observer', $3, 'tenant-observer', 'agent-observer', 'chat', 'environment', $4::jsonb, $5, $6::jsonb, false, false, true, $7)
ON CONFLICT (request_id) DO NOTHING
`, requestID, policyVersionID, environment, matchedScopeRaw, resolvedModel, fallbackRaw, createdAt.UTC())
	if err != nil {
		t.Fatalf("seed runtime decision failed: %v", err)
	}
}

func seedObserverDistributionEvent(t *testing.T, db *sql.DB, eventID, environment, policyVersionID string, createdAt time.Time) {
	t.Helper()
	payloadRaw := `{"rollout_percent":100,"triggered_by":"observer-test"}`
	_, err := db.ExecContext(context.Background(), `
INSERT INTO model_distribution_events (
	event_id,
	policy_version_id,
	rollout_id,
	environment,
	event_type,
	payload,
	created_at
) VALUES ($1, $2, 'ro-observer', $3, 'policy_distribution.activated', $4::jsonb, $5)
ON CONFLICT (event_id) DO NOTHING
`, eventID, policyVersionID, environment, payloadRaw, createdAt.UTC())
	if err != nil {
		t.Fatalf("seed distribution event failed: %v", err)
	}
}

func TestModelRuntimeHandlerRuntimeObserverEndpoint(t *testing.T) {
	dsn := testRuntimeObserverPostgresDSN(t)
	store, err := governance.NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	environment := "observer-it"
	policyVersionID := "pv_observer_it_1"
	requestID := fmt.Sprintf("req_observer_it_%d", time.Now().UnixNano())
	eventID := "distribution_observer_it_1"
	baseTime := time.Now().UTC().Add(-2 * time.Minute)

	seedObserverActivePolicy(t, store.DB(), environment, policyVersionID, baseTime)
	seedObserverRuntimeDecision(t, store.DB(), requestID, environment, policyVersionID, "gpt-4o-mini", baseTime.Add(30*time.Second))
	seedObserverDistributionEvent(t, store.DB(), eventID, environment, policyVersionID, baseTime.Add(time.Minute))

	resolver := governance.NewRuntimeResolver(store)
	_, err = resolver.Resolve(context.Background(), governance.ResolveInput{
		RequestID:   fmt.Sprintf("req_observer_cache_prime_%d", time.Now().UnixNano()),
		Environment: environment,
		AgentID:     "agent-observer",
		TenantID:    "tenant-observer",
	})
	if err != nil {
		t.Fatalf("prime resolver cache failed: %v", err)
	}
	resolver.InvalidateCache(environment)

	handler := NewModelRuntimeHandler().WithResolver(resolver).WithQueryer(store.DB()).WithTimeNow(func() time.Time {
		return time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/governance/runtime-observer?environment="+environment+"&limit=5", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode observer payload failed: %v", err)
	}

	if payload["environment"] != environment {
		t.Fatalf("unexpected environment: %v", payload["environment"])
	}
	activePolicy, _ := payload["active_policy"].(map[string]any)
	if activePolicy["version_id"] != policyVersionID {
		t.Fatalf("unexpected active policy version: %v", activePolicy["version_id"])
	}

	cache, _ := payload["cache"].(map[string]any)
	if cache["invalidation_count"].(float64) < 1 {
		t.Fatalf("expected invalidation_count >= 1, got %v", cache["invalidation_count"])
	}

	facts, _ := payload["facts"].(map[string]any)
	runtimeDecisions, _ := facts["runtime_decisions"].([]any)
	distributionEvents, _ := facts["distribution_events"].([]any)
	if len(runtimeDecisions) == 0 {
		t.Fatalf("expected runtime decisions facts")
	}
	if len(distributionEvents) == 0 {
		t.Fatalf("expected distribution events facts")
	}
}
