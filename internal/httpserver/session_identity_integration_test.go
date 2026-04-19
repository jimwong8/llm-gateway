package httpserver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/providers"
	"llm-gateway/gateway/internal/router"
)

func TestChatCompletionsWritesSessionIDHeaderFromBody(t *testing.T) {
	s := newSessionIDIntegrationServer(t)

	rr := doSessionIDChatRequest(t, s, `{"session_id":"body-session-1","messages":[{"role":"user","content":"hello"}]}`, func(req *http.Request) {})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get(sessionIDHeader); got != "body-session-1" {
		t.Fatalf("expected %s=body-session-1, got %q", sessionIDHeader, got)
	}
}

func TestChatCompletionsReadsLegacySessionIDHeaderAndWritesCanonicalHeader(t *testing.T) {
	s := newSessionIDIntegrationServer(t)

	rr := doSessionIDChatRequest(t, s, `{"messages":[{"role":"user","content":"hello"}]}`, func(req *http.Request) {
		req.Header.Set("X-Session-Id", "legacy-header-session")
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get(sessionIDHeader); got != "legacy-header-session" {
		t.Fatalf("expected %s=legacy-header-session, got %q", sessionIDHeader, got)
	}
}

func TestChatCompletionsGeneratesSessionIDWhenMissing(t *testing.T) {
	s := newSessionIDIntegrationServer(t)

	rr := doSessionIDChatRequest(t, s, `{"messages":[{"role":"user","content":"hello"}]}`, func(req *http.Request) {})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	got := rr.Header().Get(sessionIDHeader)
	if got == "" {
		t.Fatalf("expected generated %s header, got empty", sessionIDHeader)
	}
	if !strings.HasPrefix(got, "oc_") {
		t.Fatalf("expected generated session id prefix oc_, got %q", got)
	}
}

func newSessionIDIntegrationServer(t *testing.T) *Server {
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
	return New(cfg, registry, nil, modelRouter, nil, nil, nil, nil, nil, nil, nil)
}

func doSessionIDChatRequest(t *testing.T, s *Server, body string, mutate func(req *http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if mutate != nil {
		mutate(req)
	}
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	return rr
}
