package main

import (
	"bytes"
	"context"
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

func main() {
	ctx := context.Background()
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

	seedPolicyOverlayBaseline(policyStore, "tenant-chat-policy", "alice")
	seedPolicyOverlayBaseline(policyStore, "tenant-chat-role", "alice")
	seedPolicyOverlayBaseline(policyStore, "tenant-chat-provider", "alice")
	seedPolicyOverlayBaseline(policyStore, "tenant-chat-model", "alice")
	seedPolicyOverlayBaseline(policyStore, "tenant-chat-baseline", "alice")

	verifyPolicySensitiveReplayEnforcesRequestPath(ctx, svc, handler)
	verifyPolicyRoleDeny(handler, policyStore)
	verifyPolicyProviderDenyAllCandidates(handler, policyStore)
	verifyPolicyPreferredModelDeny(handler, policyStore)
	verifyPolicyAllowBaseline(handler)

	fmt.Println("verify result: PASS chat policy replay/deny-role/deny-provider/deny-model/allow-baseline")
}

func verifyPolicySensitiveReplayEnforcesRequestPath(ctx context.Context, svc *controlplane.Service, handler http.Handler) {
	released, err := svc.CreateVersion(ctx, controlplane.CreateVersionInput{
		Module:      "policy",
		TenantID:    "tenant-chat-policy",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_verify_chat_policy_sensitive",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "verify chat policy sensitive replay target",
		Config: map[string]string{
			"sensitive/rules": `[{"pattern":"secret","action":"block","enabled":true}]`,
		},
	})
	if err != nil {
		fail("create chat policy sensitive replay target", err)
	}

	resp, status := postVersion(handler, "/admin/releases/replay", map[string]any{
		"module":      "policy",
		"tenant_id":   "tenant-chat-policy",
		"environment": "prod",
		"scope":       "tenant",
		"version_id":  released.Version,
	})
	if status != http.StatusOK {
		fail("replay released chat policy version", fmt.Errorf("unexpected status %d", status))
	}
	if resp.VersionID != released.Version || resp.Status != controlplane.ConfigStatusReleased {
		fail("verify chat policy replay response", fmt.Errorf("unexpected response %+v", resp))
	}

	rr := doChatCompletionsRequest(handler, "alice", `{"tenant_id":"tenant-chat-policy","messages":[{"role":"user","content":"my secret should be blocked"}]}`)
	if rr.Code != http.StatusForbidden {
		fail("verify chat policy deny status", fmt.Errorf("expected 403, got %d body=%s", rr.Code, rr.Body.String()))
	}
	errBody := decodeChatPolicyError(rr.Body.Bytes())
	if errBody.Error.Type != "policy_error" || errBody.Error.Message != "sensitive content blocked" || errBody.Error.Pattern != "secret" {
		fail("verify chat policy deny body", fmt.Errorf("unexpected error body %+v", errBody))
	}

	fmt.Println("policy enforcement replay: PASS", released.Version, "tenant=tenant-chat-policy")
	fmt.Println("request path denied: PASS status=403 type=policy_error reason=sensitive content blocked")
}

func verifyPolicyAllowBaseline(handler http.Handler) {
	rr := doChatCompletionsRequest(handler, "alice", `{"tenant_id":"tenant-chat-baseline","messages":[{"role":"user","content":"hello world"}]}`)
	if rr.Code == http.StatusForbidden {
		fail("verify chat policy baseline allow", fmt.Errorf("expected non-403 baseline response, got %d body=%s", rr.Code, rr.Body.String()))
	}

	fmt.Println("request path allow baseline: PASS status=200")
}

func verifyPolicyRoleDeny(handler http.Handler, policyStore *policy.Store) {
	policyStore.SetRoleBindingsOverlay("tenant-chat-role", []policy.TenantRoleBinding{{
		TenantID: "tenant-chat-role",
		Subject:  "alice",
		Role:     "readonly",
	}})

	rr := doChatCompletionsRequest(handler, "alice", `{"tenant_id":"tenant-chat-role","messages":[{"role":"user","content":"hello world"}]}`)
	if rr.Code != http.StatusForbidden {
		fail("verify chat policy role deny status", fmt.Errorf("expected 403, got %d body=%s", rr.Code, rr.Body.String()))
	}
	errBody := decodeChatPolicyError(rr.Body.Bytes())
	if errBody.Error.Type != "policy_error" || errBody.Error.Message != "role not permitted" || errBody.Error.Role != "readonly" {
		fail("verify chat policy role deny body", fmt.Errorf("unexpected error body %+v", errBody))
	}

	fmt.Println("request path role deny: PASS status=403 type=policy_error reason=role not permitted")
}

func verifyPolicyProviderDenyAllCandidates(handler http.Handler, policyStore *policy.Store) {
	policyStore.SetProviderPoliciesOverlay("tenant-chat-provider", []policy.TenantProviderPolicy{{
		TenantID: "tenant-chat-provider",
		Provider: "openai",
		Mode:     "deny",
		Enabled:  true,
	}})

	rr := doChatCompletionsRequest(handler, "alice", `{"tenant_id":"tenant-chat-provider","candidate_models":["openai/gpt-4o-mini","openai/gpt-4o"],"messages":[{"role":"user","content":"hello world"}]}`)
	if rr.Code != http.StatusForbidden {
		fail("verify chat policy provider deny status", fmt.Errorf("expected 403, got %d body=%s", rr.Code, rr.Body.String()))
	}
	errBody := decodeChatPolicyError(rr.Body.Bytes())
	if errBody.Error.Type != "policy_error" || errBody.Error.Message != "all candidate models denied by provider policy" {
		fail("verify chat policy provider deny body", fmt.Errorf("unexpected error body %+v", errBody))
	}

	fmt.Println("request path provider deny-all: PASS status=403 type=policy_error reason=all candidate models denied by provider policy")
}

func verifyPolicyPreferredModelDeny(handler http.Handler, policyStore *policy.Store) {
	policyStore.SetAllowedModelsOverlay("tenant-chat-model", []string{"gpt-4o-mini"})

	rr := doChatCompletionsRequest(handler, "alice", `{"tenant_id":"tenant-chat-model","preferred_model":"claude-3-5-sonnet","messages":[{"role":"user","content":"hello world"}]}`)
	if rr.Code != http.StatusForbidden {
		fail("verify chat policy preferred model deny status", fmt.Errorf("expected 403, got %d body=%s", rr.Code, rr.Body.String()))
	}
	errBody := decodeChatPolicyError(rr.Body.Bytes())
	if errBody.Error.Type != "policy_error" || errBody.Error.Message != "preferred model not allowed for tenant" {
		fail("verify chat policy preferred model deny body", fmt.Errorf("unexpected error body %+v", errBody))
	}

	fmt.Println("request path preferred model deny: PASS status=403 type=policy_error reason=preferred model not allowed for tenant")
}

func postVersion(handler http.Handler, path string, payload map[string]any) (versionResponse, int) {
	body, err := json.Marshal(payload)
	if err != nil {
		fail("marshal admin request body", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Key", adminToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		return versionResponse{}, rr.Code
	}
	var resp versionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		fail("decode admin version response", err)
	}
	return resp, rr.Code
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
