package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/providers"
	"llm-gateway/gateway/internal/quota"
	"llm-gateway/gateway/internal/router"
)

func TestChatCompletionsQuotaHeaderUsesLiveLimiterRPM(t *testing.T) {
	cfg := config.Config{
		AdminAPIKey:     "k",
		DefaultProvider: "mock-primary",
		DefaultModel:    "gpt-4o-mini",
		TenantRPM:       5,
	}
	registry := providers.NewRegistry(cfg,
		providers.NewMockProvider("mock-primary", "gpt-4o-mini"),
		providers.NewMockProvider("mock-primary", "gpt-4o-mini"),
	)
	modelRouter := router.New(cfg.DefaultProvider, cfg.DefaultModel)
	limiter := quota.New("127.0.0.1:6379", 5)
	limiter.SetRPM(42)

	s := New(cfg, registry, nil, modelRouter, nil, nil, nil, nil, limiter, nil, nil)

	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"hello"}],"tenant_id":"tenant-a"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 from chat completions, got %d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("X-RateLimit-Limit"); got != "42" {
		t.Fatalf("expected X-RateLimit-Limit to reflect live limiter rpm=42, got %q", got)
	}
}

func TestAdminObservabilityQuotaReportsLiveLimiterRPM(t *testing.T) {
	cfg := config.Config{AdminAPIKey: "k", TenantRPM: 5}
	limiter := quota.New("127.0.0.1:6379", 5)
	limiter.SetRPM(88)
	s := New(cfg, nil, nil, nil, nil, nil, nil, nil, limiter, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/observability/quota?tenant_id=tenant-a", nil)
	req.Header.Set("X-Admin-Key", "k")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 from quota observability, got %d body=%s", rr.Code, rr.Body.String())
	}

	var row map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &row); err != nil {
		t.Fatalf("failed to decode quota summary response: %v", err)
	}
	if got, ok := row["limit"].(float64); !ok || int(got) != 88 {
		t.Fatalf("expected quota summary limit=88 from live limiter rpm, got %#v", row["limit"])
	}
}
