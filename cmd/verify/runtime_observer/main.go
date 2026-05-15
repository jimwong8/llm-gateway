package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"llm-gateway/gateway/internal/governance"
	"llm-gateway/gateway/internal/httpserver"
)

const adminToken = "admin-secret"

func main() {
	dsn := runtimeObserverVerifyDSN()
	if dsn == "" {
		fmt.Println("runtime observer verify skipped: set POSTGRES_DSN or GOVERNANCE_TEST_POSTGRES_DSN")
		return
	}

	store, err := governance.NewStore(dsn)
	if err != nil {
		fail("create governance store", err)
	}

	environment := "verify-runtime-observer"
	policyVersionID := "pv_verify_runtime_observer"
	requestID := "req_verify_runtime_observer"
	eventID := "event_verify_runtime_observer"
	baseTime := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)

	seedActivePolicy(store.DB(), environment, policyVersionID, baseTime)
	seedRuntimeDecision(store.DB(), requestID, environment, policyVersionID, "model-a", baseTime.Add(30*time.Second))
	seedDistributionEvent(store.DB(), eventID, environment, policyVersionID, baseTime.Add(time.Minute))

	resolver := governance.NewRuntimeResolver(store)
	_, err = resolver.Resolve(context.Background(), governance.ResolveInput{
		RequestID:   "req_verify_runtime_observer_cache_prime",
		Environment: environment,
		AgentID:     "agent-verify",
		TenantID:    "tenant-verify",
	})
	if err != nil {
		fail("prime resolver cache", err)
	}
	resolver.InvalidateCache(environment)

	handler := httpserver.NewModelRuntimeHandler().
		WithResolver(resolver).
		WithQueryer(store.DB()).
		WithTimeNow(func() time.Time { return baseTime.Add(2 * time.Minute) })

	verifyRuntimeDecisions(handler)
	verifyDistributionEvents(handler)
	verifyRuntimeObserver(handler, environment, policyVersionID, requestID, eventID)

	fmt.Println("verify result: PASS runtime observer query closure")
}

func runtimeObserverVerifyDSN() string {
	if dsn := strings.TrimSpace(os.Getenv("POSTGRES_DSN")); dsn != "" {
		return dsn
	}
	if dsn := strings.TrimSpace(os.Getenv("GOVERNANCE_TEST_POSTGRES_DSN")); dsn != "" {
		return dsn
	}
	return ""
}

func seedActivePolicy(db *sql.DB, environment, policyVersionID string, activatedAt time.Time) {
	policyRaw, err := json.Marshal(governance.RuntimePolicy{
		Version:      1,
		Environment:  environment,
		DefaultModel: "model-a",
	})
	if err != nil {
		fail("marshal active policy", err)
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
) VALUES ($1, $2, 'active', $3::jsonb, 'verify', 'verify', $4, $4, $4)
ON CONFLICT (policy_version_id) DO NOTHING
`, policyVersionID, environment, string(policyRaw), activatedAt.UTC())
	if err != nil {
		fail("seed active policy", err)
	}
}

func seedRuntimeDecision(db *sql.DB, requestID, environment, policyVersionID, resolvedModel string, createdAt time.Time) {
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
) VALUES ($1, $2, 'ro-verify', $3, 'tenant-verify', 'agent-verify', 'chat', 'environment', $4::jsonb, $5, $6::jsonb, false, false, true, $7)
ON CONFLICT (request_id) DO NOTHING
`, requestID, policyVersionID, environment, matchedScopeRaw, resolvedModel, fallbackRaw, createdAt.UTC())
	if err != nil {
		fail("seed runtime decision", err)
	}
}

func seedDistributionEvent(db *sql.DB, eventID, environment, policyVersionID string, createdAt time.Time) {
	payloadRaw := `{"rollout_percent":100,"triggered_by":"verify"}`
	_, err := db.ExecContext(context.Background(), `
INSERT INTO model_distribution_events (
	event_id,
	policy_version_id,
	rollout_id,
	environment,
	event_type,
	payload,
	created_at
) VALUES ($1, $2, 'ro-verify', $3, 'policy_distribution.activated', $4::jsonb, $5)
ON CONFLICT (event_id) DO NOTHING
`, eventID, policyVersionID, environment, payloadRaw, createdAt.UTC())
	if err != nil {
		fail("seed distribution event", err)
	}
}

func verifyRuntimeDecisions(handler http.Handler) {
	req := httptest.NewRequest(http.MethodGet, "/admin/governance/runtime-decisions?limit=5", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		fail("runtime decisions status", fmt.Errorf("expected 200, got %d body=%s", rr.Code, rr.Body.String()))
	}
	var payload struct {
		Object string                   `json:"object"`
		Data   []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		fail("decode runtime decisions payload", err)
	}
	if payload.Object != "list" || len(payload.Data) == 0 {
		fail("runtime decisions payload shape", fmt.Errorf("unexpected payload %+v", payload))
	}
	fmt.Println("runtime decisions query: PASS count=", len(payload.Data))
}

func verifyDistributionEvents(handler http.Handler) {
	req := httptest.NewRequest(http.MethodGet, "/admin/governance/distribution-events?limit=5", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		fail("distribution events status", fmt.Errorf("expected 200, got %d body=%s", rr.Code, rr.Body.String()))
	}
	var payload struct {
		Object string                   `json:"object"`
		Data   []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		fail("decode distribution events payload", err)
	}
	if payload.Object != "list" || len(payload.Data) == 0 {
		fail("distribution events payload shape", fmt.Errorf("unexpected payload %+v", payload))
	}
	fmt.Println("distribution events query: PASS count=", len(payload.Data))
}

func verifyRuntimeObserver(handler http.Handler, environment, policyVersionID, requestID, eventID string) {
	req := httptest.NewRequest(http.MethodGet, "/admin/governance/runtime-observer?environment="+environment+"&limit=5", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		fail("runtime observer status", fmt.Errorf("expected 200, got %d body=%s", rr.Code, rr.Body.String()))
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		fail("decode runtime observer payload", err)
	}
	if payload["environment"] != environment {
		fail("runtime observer environment", fmt.Errorf("unexpected environment %v", payload["environment"]))
	}
	activePolicy, _ := payload["active_policy"].(map[string]interface{})
	if activePolicy["version_id"] != policyVersionID {
		fail("runtime observer active policy", fmt.Errorf("unexpected version id %v", activePolicy["version_id"]))
	}
	facts, _ := payload["facts"].(map[string]interface{})
	runtimeDecisions, _ := facts["runtime_decisions"].([]interface{})
	distributionEvents, _ := facts["distribution_events"].([]interface{})
	if len(runtimeDecisions) == 0 || len(distributionEvents) == 0 {
		fail("runtime observer facts", fmt.Errorf("unexpected facts payload %+v", facts))
	}
	fmt.Println("runtime observer query: PASS request_id=", requestID, "event_id=", eventID)
}

func fail(step string, err error) {
	_, _ = fmt.Fprintf(os.Stderr, "verify failed at %s: %v\n", step, err)
	os.Exit(1)
}
