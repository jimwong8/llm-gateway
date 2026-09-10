package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"llm-gateway/gateway/internal/audit"
	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/controlplane"
	"llm-gateway/gateway/internal/httpserver"
	"llm-gateway/gateway/internal/policy"
	"llm-gateway/gateway/internal/providers"
	"llm-gateway/gateway/internal/router"
	"llm-gateway/gateway/internal/runtime"
)

const adminToken = "admin-secret"

type versionResponse struct {
	VersionID string `json:"version_id"`
	Status    string `json:"status"`
}

type chatPolicyErrorResponse struct {
	Error struct {
		Message  string `json:"message"`
		Type     string `json:"type"`
		TenantID string `json:"tenant_id"`
		Role     string `json:"role,omitempty"`
		Pattern  string `json:"pattern,omitempty"`
	} `json:"error"`
}

type auditSummaryResponse struct {
	Total  int            `json:"total"`
	ByType map[string]int `json:"by_type"`
}

func main() {
	cfg := config.Config{
		AdminAPIKey:     adminToken,
		DefaultProvider: "mock-primary",
		DefaultModel:    "gpt-4o-mini",
	}
	auditor := audit.NewRecorder()
	runtimeBus := runtime.NewInProcessBus()
	publisher := runtime.NewPublisher().WithBus(runtimeBus)
	manager := runtime.NewManager()
	policyStore := &policy.Store{}
	svc := controlplane.NewService().
		WithAuditRecorder(auditor).
		WithReleasePublisher(publisher)
	registry := providers.NewRegistry(cfg,
		providers.NewMockProvider("mock-primary", "gpt-4o-mini"),
		providers.NewMockProvider("mock-primary", "gpt-4o-mini"),
	)
	modelRouter := router.New(cfg.DefaultProvider, cfg.DefaultModel)
	runtime.SubscribeManagerApplyBridge(runtimeBus, manager, runtime.BuildModuleRuntimeApplyDispatcher(map[string]runtime.ModuleRuntimeApplier{
		"policy": runtime.BuildPolicyReloadApply(
			runtime.BuildPolicyPayloadDrivenApplyWithResolver(policyStore, publisher, svc),
		),
	}))
	server := httpserver.New(cfg, registry, nil, modelRouter, nil, nil, nil, nil, nil, nil, policyStore).
		WithControlPlane(svc, auditor, publisher, manager)
	handler := server.Handler()

	seedPolicyOverlayBaseline(policyStore, "tenant-policy-engine-sensitive", "alice")
	seedPolicyOverlayBaseline(policyStore, "tenant-policy-engine-role", "alice")
	seedPolicyOverlayBaseline(policyStore, "tenant-policy-engine-provider", "alice")

	verifySensitiveRule(handler, policyStore)
	verifyReadonlyDenied(handler, policyStore)
	verifyProviderDenied(handler, policyStore)
	verifyAuditVisible(handler, auditor)

	fmt.Println("verify result: PASS policy engine")
}

func verifySensitiveRule(handler http.Handler, policyStore *policy.Store) {
	tenantID := "tenant-policy-engine-sensitive"
	policyStore.SetSensitiveRulesOverlay(tenantID, []policy.SensitiveRule{{
		TenantID: tenantID,
		Pattern:  "secret",
		Action:   "block",
		Enabled:  true,
	}})

	rr := doChatCompletionsRequest(handler, "alice", `{"tenant_id":"tenant-policy-engine-sensitive","messages":[{"role":"user","content":"my secret should be blocked"}]}`)
	if rr.Code != http.StatusForbidden {
		fail("sensitive rule status", fmt.Errorf("expected 403, got %d body=%s", rr.Code, rr.Body.String()))
	}
	errBody := decodeChatPolicyError(rr.Body.Bytes())
	if errBody.Error.Type != "policy_error" || errBody.Error.Message != "sensitive content blocked" || errBody.Error.Pattern != "secret" {
		fail("sensitive rule body", fmt.Errorf("unexpected error body %+v", errBody))
	}
	fmt.Println("policy engine sensitive rule: PASS status=403 pattern=secret")
}

func verifyReadonlyDenied(handler http.Handler, policyStore *policy.Store) {
	tenantID := "tenant-policy-engine-role"
	policyStore.SetRoleBindingsOverlay(tenantID, []policy.TenantRoleBinding{{
		TenantID: tenantID,
		Subject:  "alice",
		Role:     "readonly",
	}})

	rr := doChatCompletionsRequest(handler, "alice", `{"tenant_id":"tenant-policy-engine-role","messages":[{"role":"user","content":"hello world"}]}`)
	if rr.Code != http.StatusForbidden {
		fail("readonly deny status", fmt.Errorf("expected 403, got %d body=%s", rr.Code, rr.Body.String()))
	}
	errBody := decodeChatPolicyError(rr.Body.Bytes())
	if errBody.Error.Type != "policy_error" || errBody.Error.Message != "role not permitted" || errBody.Error.Role != "readonly" {
		fail("readonly deny body", fmt.Errorf("unexpected error body %+v", errBody))
	}
	fmt.Println("policy engine readonly deny: PASS status=403 role=readonly")
}

func verifyProviderDenied(handler http.Handler, policyStore *policy.Store) {
	tenantID := "tenant-policy-engine-provider"
	policyStore.SetProviderPoliciesOverlay(tenantID, []policy.TenantProviderPolicy{{
		TenantID: tenantID,
		Provider: "openai",
		Mode:     "deny",
		Enabled:  true,
	}})

	rr := doChatCompletionsRequest(handler, "alice", `{"tenant_id":"tenant-policy-engine-provider","candidate_models":["openai/gpt-4o-mini","openai/gpt-4o"],"messages":[{"role":"user","content":"hello world"}]}`)
	if rr.Code != http.StatusForbidden {
		fail("provider deny status", fmt.Errorf("expected 403, got %d body=%s", rr.Code, rr.Body.String()))
	}
	errBody := decodeChatPolicyError(rr.Body.Bytes())
	if errBody.Error.Type != "policy_error" || errBody.Error.Message != "all candidate models denied by provider policy" {
		fail("provider deny body", fmt.Errorf("unexpected error body %+v", errBody))
	}
	fmt.Println("policy engine provider deny: PASS status=403 provider=openai")
}

func verifyAuditVisible(handler http.Handler, auditor *audit.Recorder) {
	auditor.RecordRelease("policy", "tenant-policy-audit", "prod", "cfg_policy_engine_audit", "verify", "policy engine audit visible")

	req := httptest.NewRequest(http.MethodGet, "/admin/audit-events?summary=true&tenant_id=tenant-policy-audit&environment=prod", nil)
	req.Header.Set("X-Admin-Key", adminToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		fail("audit summary status", fmt.Errorf("expected 200, got %d body=%s", rr.Code, rr.Body.String()))
	}
	var summary auditSummaryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &summary); err != nil {
		fail("decode audit summary", err)
	}
	if summary.Total < 1 {
		fail("audit summary total", fmt.Errorf("expected total >= 1, got %+v", summary))
	}
	if summary.ByType[audit.ControlPlaneEventTypeRelease] < 1 {
		fail("audit summary by_type", fmt.Errorf("expected release event in summary, got %+v", summary.ByType))
	}
	fmt.Println("policy engine audit visibility: PASS total=", summary.Total)
}

func doChatCompletionsRequest(handler http.Handler, subject, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+subject)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func seedPolicyOverlayBaseline(store *policy.Store, tenantID, subject string) {
	store.SetAllowedModelsOverlay(tenantID, []string{})
	store.SetRoleBindingsOverlay(tenantID, []policy.TenantRoleBinding{{
		TenantID: tenantID,
		Subject:  subject,
		Role:     "admin",
	}})
	store.SetProviderPoliciesOverlay(tenantID, []policy.TenantProviderPolicy{})
	store.SetSensitiveRulesOverlay(tenantID, []policy.SensitiveRule{})
}

func decodeChatPolicyError(body []byte) chatPolicyErrorResponse {
	var out chatPolicyErrorResponse
	if err := json.Unmarshal(body, &out); err != nil {
		fail("decode chat policy error", err)
	}
	return out
}

func fail(step string, err error) {
	_, _ = fmt.Fprintf(os.Stderr, "verify failed at %s: %v\n", step, err)
	os.Exit(1)
}
