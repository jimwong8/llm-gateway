package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"llm-gateway/gateway/internal/config"
)

func TestAdminUI_Index(t *testing.T) {
	s := New(config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/ui", nil)
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("expected html content-type, got %s", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "LLM Gateway Admin Console") {
		t.Fatalf("expected admin console html body")
	}
	if !strings.Contains(body, "/admin/ui/assets/") {
		t.Fatalf("expected built asset path in html body")
	}
}

func TestAdminUI_Assets(t *testing.T) {
	s := New(config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	indexRR := httptest.NewRecorder()
	indexReq := httptest.NewRequest(http.MethodGet, "/admin/ui", nil)
	s.Handler().ServeHTTP(indexRR, indexReq)

	body := indexRR.Body.String()
	marker := "/admin/ui/assets/"
	start := strings.Index(body, marker)
	if start == -1 {
		t.Fatalf("expected asset path in html body")
	}
	end := strings.Index(body[start:], "\"")
	if end == -1 {
		t.Fatalf("expected asset path closing quote")
	}
	assetPath := body[start : start+end]

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, assetPath, nil)
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "application/javascript") && !strings.Contains(ct, "text/css") {
		t.Fatalf("expected asset content-type, got %s", ct)
	}
}

func TestAdminUI_SPAFallback(t *testing.T) {
	s := New(config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/ui/dashboard", nil)
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("expected html content-type, got %s", ct)
	}
	if !strings.Contains(rr.Body.String(), "LLM Gateway Admin Console") {
		t.Fatalf("expected admin console html body")
	}
}
