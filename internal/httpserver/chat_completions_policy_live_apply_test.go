package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/policy"
	"llm-gateway/gateway/internal/providers"
	"llm-gateway/gateway/internal/router"
)

type chatPolicyErrorResponse struct {
	Error struct {
		Message  string `json:"message"`
		Type     string `json:"type"`
		TenantID string `json:"tenant_id"`
		Role     string `json:"role,omitempty"`
		Pattern  string `json:"pattern,omitempty"`
	} `json:"error"`
}

func TestChatCompletionsPolicySensitiveBlockReturnsForbidden(t *testing.T) {
	subject := "alice"
	tenantID := "tenant-a"
	store := newPolicyOverlayStore(t, tenantID, subject)
	store.SetSensitiveRulesOverlay(tenantID, []policy.SensitiveRule{{
		TenantID: tenantID,
		Pattern:  "secret",
		Action:   "block",
		Enabled:  true,
	}})

	s := newPolicyAwareChatServer(t, store)
	rr := doChatCompletionsRequest(t, s, subject, `{"tenant_id":"tenant-a","messages":[{"role":"user","content":"my secret should be blocked"}]}`)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for sensitive block, got %d body=%s", rr.Code, rr.Body.String())
	}
	errBody := decodeChatPolicyError(t, rr)
	if errBody.Error.Type != "policy_error" {
		t.Fatalf("expected error.type=policy_error, got %q", errBody.Error.Type)
	}
	if errBody.Error.Message != "sensitive content blocked" {
		t.Fatalf("expected sensitive block message, got %q", errBody.Error.Message)
	}
	if errBody.Error.Pattern != "secret" {
		t.Fatalf("expected matched pattern=secret, got %q", errBody.Error.Pattern)
	}
	if errBody.Error.TenantID != tenantID {
		t.Fatalf("expected tenant_id=%q, got %q", tenantID, errBody.Error.TenantID)
	}
}

func TestChatCompletionsPolicyRoleDenyReturnsForbidden(t *testing.T) {
	subject := "readonly-user"
	tenantID := "tenant-a"
	store := newPolicyOverlayStore(t, tenantID, subject)
	store.SetRoleBindingsOverlay(tenantID, []policy.TenantRoleBinding{{
		TenantID: tenantID,
		Subject:  subject,
		Role:     "readonly",
	}})

	s := newPolicyAwareChatServer(t, store)
	rr := doChatCompletionsRequest(t, s, subject, `{"tenant_id":"tenant-a","messages":[{"role":"user","content":"hello"}]}`)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for role deny, got %d body=%s", rr.Code, rr.Body.String())
	}
	errBody := decodeChatPolicyError(t, rr)
	if errBody.Error.Type != "policy_error" {
		t.Fatalf("expected error.type=policy_error, got %q", errBody.Error.Type)
	}
	if errBody.Error.Message != "role not permitted" {
		t.Fatalf("expected role deny message, got %q", errBody.Error.Message)
	}
	if errBody.Error.Role != "readonly" {
		t.Fatalf("expected role=readonly, got %q", errBody.Error.Role)
	}
	if errBody.Error.TenantID != tenantID {
		t.Fatalf("expected tenant_id=%q, got %q", tenantID, errBody.Error.TenantID)
	}
}

func TestChatCompletionsPolicyProviderDenyAllCandidatesReturnsForbidden(t *testing.T) {
	subject := "alice"
	tenantID := "tenant-a"
	store := newPolicyOverlayStore(t, tenantID, subject)
	store.SetProviderPoliciesOverlay(tenantID, []policy.TenantProviderPolicy{{
		TenantID: tenantID,
		Provider: "openai",
		Mode:     "deny",
		Enabled:  true,
	}})

	s := newPolicyAwareChatServer(t, store)
	rr := doChatCompletionsRequest(t, s, subject, `{"tenant_id":"tenant-a","candidate_models":["openai/gpt-4o-mini","openai/gpt-4o"],"messages":[{"role":"user","content":"hello"}]}`)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for provider deny all candidates, got %d body=%s", rr.Code, rr.Body.String())
	}
	errBody := decodeChatPolicyError(t, rr)
	if errBody.Error.Type != "policy_error" {
		t.Fatalf("expected error.type=policy_error, got %q", errBody.Error.Type)
	}
	if errBody.Error.Message != "all candidate models denied by provider policy" {
		t.Fatalf("expected provider deny-all message, got %q", errBody.Error.Message)
	}
	if errBody.Error.TenantID != tenantID {
		t.Fatalf("expected tenant_id=%q, got %q", tenantID, errBody.Error.TenantID)
	}
}

func TestChatCompletionsPolicyPreferredModelDeniedReturnsForbidden(t *testing.T) {
	subject := "alice"
	tenantID := "tenant-a"
	store := newPolicyOverlayStore(t, tenantID, subject)
	store.SetAllowedModelsOverlay(tenantID, []string{"gpt-4o-mini"})

	s := newPolicyAwareChatServer(t, store)
	rr := doChatCompletionsRequest(t, s, subject, `{"tenant_id":"tenant-a","preferred_model":"claude-3-5-sonnet","messages":[{"role":"user","content":"hello"}]}`)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for preferred model deny, got %d body=%s", rr.Code, rr.Body.String())
	}
	errBody := decodeChatPolicyError(t, rr)
	if errBody.Error.Type != "policy_error" {
		t.Fatalf("expected error.type=policy_error, got %q", errBody.Error.Type)
	}
	if errBody.Error.Message != "preferred model not allowed for tenant" {
		t.Fatalf("expected preferred model deny message, got %q", errBody.Error.Message)
	}
	if errBody.Error.TenantID != tenantID {
		t.Fatalf("expected tenant_id=%q, got %q", tenantID, errBody.Error.TenantID)
	}
}

func newPolicyAwareChatServer(t *testing.T, store *policy.Store) *Server {
	t.Helper()
	cfg := config.Config{
		DefaultProvider: "mock-primary",
		DefaultModel:    "gpt-4o-mini",
	}
	registry := providers.NewRegistry(cfg,
		providers.NewMockProvider("mock-primary", "gpt-4o-mini"),
		providers.NewMockProvider("mock-primary", "gpt-4o-mini"),
	)
	modelRouter := router.New(cfg.DefaultProvider, cfg.DefaultModel)
	return New(cfg, registry, nil, modelRouter, nil, nil, nil, nil, nil, nil, store)
}

func newPolicyOverlayStore(t *testing.T, tenantID, subject string) *policy.Store {
	t.Helper()
	store := &policy.Store{}
	store.SetAllowedModelsOverlay(tenantID, []string{})
	store.SetRoleBindingsOverlay(tenantID, []policy.TenantRoleBinding{{
		TenantID: tenantID,
		Subject:  subject,
		Role:     "admin",
	}})
	store.SetProviderPoliciesOverlay(tenantID, []policy.TenantProviderPolicy{})
	store.SetSensitiveRulesOverlay(tenantID, []policy.SensitiveRule{})
	return store
}

func doChatCompletionsRequest(t *testing.T, s *Server, subject, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+subject)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	return rr
}

func decodeChatPolicyError(t *testing.T, rr *httptest.ResponseRecorder) chatPolicyErrorResponse {
	t.Helper()
	var out chatPolicyErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("failed to decode response body as policy error: %v body=%s", err, rr.Body.String())
	}
	return out
}
