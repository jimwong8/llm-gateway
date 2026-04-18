package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/policy"
	"llm-gateway/gateway/internal/providers"
)

func TestPolicyEngineAdminRoutesStillRegistered(t *testing.T) {
	s := New(config.Config{AdminAPIKey: "k"}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	paths := []string{
		"/admin/health",
		"/admin/usage",
		"/admin/audit",
		"/admin/policies/models",
	}
	for _, path := range paths {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Admin-Key", "k")
		s.Handler().ServeHTTP(rr, req)
		if rr.Code == http.StatusNotFound {
			t.Fatalf("expected route %s to be registered", path)
		}
	}
}

func TestPolicyShapesCompile(t *testing.T) {
	role := policy.TenantRoleBinding{TenantID: "t1", Subject: "alice", Role: "admin"}
	provider := policy.TenantProviderPolicy{TenantID: "t1", Provider: "openai", Mode: "deny", Enabled: true}
	rule := policy.SensitiveRule{TenantID: "t1", Pattern: "secret", Action: "block", Enabled: true}
	if role.Role == "" || provider.Mode == "" || rule.Action == "" {
		t.Fatalf("unexpected policy shape: %+v %+v %+v", role, provider, rule)
	}
}

func TestPolicyEngineAdminRoleHelpers(t *testing.T) {
	if !roleAllowsMethod("admin", http.MethodPost) {
		t.Fatal("admin should allow write")
	}
	if !roleAllowsMethod("operator", http.MethodPost) {
		t.Fatal("operator should allow write")
	}
	if roleAllowsMethod("readonly", http.MethodPost) {
		t.Fatal("readonly should not allow write")
	}
	if !roleAllowsMethod("readonly", http.MethodGet) {
		t.Fatal("readonly should allow read")
	}
}

func TestPolicyEngineSensitiveHelper(t *testing.T) {
	req := providers.ChatCompletionRequest{Messages: []providers.ChatMessage{{Role: "user", Content: "this contains secret material"}}}
	rules := []policy.SensitiveRule{{TenantID: "t1", Pattern: "secret", Action: "block", Enabled: true}}
	matched, ok := containsSensitive(req, rules)
	if !ok || matched != "secret" {
		t.Fatalf("expected sensitive match, got matched=%q ok=%v", matched, ok)
	}
}

func TestPolicyEngineSensitiveBlockShape(t *testing.T) {
	req := providers.ChatCompletionRequest{Messages: []providers.ChatMessage{{Role: "user", Content: "secret should be blocked"}}}
	rules := []policy.SensitiveRule{{TenantID: "t1", Pattern: "secret", Action: "block", Enabled: true}}
	matched, ok := containsSensitive(req, rules)
	if !ok || matched != "secret" {
		t.Fatalf("expected sensitive block match, got matched=%q ok=%v", matched, ok)
	}
}
