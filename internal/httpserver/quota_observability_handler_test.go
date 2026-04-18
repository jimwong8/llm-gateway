package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"llm-gateway/gateway/internal/config"
)

func TestAdminObservabilityQuotaRouteRegistered(t *testing.T) {
	s := New(config.Config{AdminAPIKey: "k"}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	paths := []string{
		"/admin/observability/quota",
		"/admin/observability/quota/trends",
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

func TestParseQuotaQueryParamsShape(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/admin/observability/quota?tenant_id=t1&window_minutes=15&limit=7", nil)
	q := req.URL.Query()
	if q.Get("tenant_id") != "t1" {
		t.Fatalf("unexpected tenant_id: %s", q.Get("tenant_id"))
	}
	if q.Get("window_minutes") != "15" {
		t.Fatalf("unexpected window_minutes: %s", q.Get("window_minutes"))
	}
	if q.Get("limit") != "7" {
		t.Fatalf("unexpected limit: %s", q.Get("limit"))
	}
}
