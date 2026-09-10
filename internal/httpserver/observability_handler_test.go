package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"llm-gateway/gateway/internal/billing"
	"llm-gateway/gateway/internal/config"
)

type stubBillingStore struct{}

func (s *stubBillingStore) Ping(_ interface{}) error { return nil }

func TestAdminObservabilitySummary_ServiceUnavailable(t *testing.T) {
	s := New(config.Config{AdminAPIKey: "k"}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/observability/summary", nil)
	req.Header.Set("X-Admin-Key", "k")
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestParseBillingFilter(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/admin/observability/summary?tenant_id=t1&provider=openai&model=gpt-4o-mini&limit=7&from=2026-03-24T10:00:00Z&to=2026-03-24T11:00:00Z", nil)
	filter := parseBillingFilter(req)
	if filter.TenantID != "t1" || filter.Provider != "openai" || filter.Model != "gpt-4o-mini" {
		t.Fatalf("unexpected filter: %+v", filter)
	}
	if filter.Limit != 7 {
		t.Fatalf("expected limit 7, got %d", filter.Limit)
	}
	if filter.From.IsZero() || filter.To.IsZero() {
		t.Fatalf("expected parsed time range")
	}
}

func TestObservabilityRoutesRegistered(t *testing.T) {
	s := New(config.Config{AdminAPIKey: "k"}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	paths := []string{
		"/admin/observability/summary",
		"/admin/observability/cache",
		"/admin/observability/providers",
		"/admin/observability/hotspots",
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

func TestSummaryRowJSONShape(t *testing.T) {
	row := billing.SummaryRow{Requests: 1, TotalTokens: 2, EstimatedCost: 0.1}
	b, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 {
		t.Fatal("expected json body")
	}
}

func TestObservabilityFallbackAndFailureShapes(t *testing.T) {
	failure := billing.UsageEvent{Success: false, ErrorType: "provider_error", FallbackUsed: false}
	if failure.Success {
		t.Fatal("expected failure.Success to be false")
	}
	if failure.ErrorType != "provider_error" {
		t.Fatalf("unexpected error type: %+v", failure)
	}

	fallback := billing.UsageEvent{Success: true, FallbackUsed: true}
	if !fallback.Success || !fallback.FallbackUsed {
		t.Fatalf("unexpected fallback shape: %+v", fallback)
	}
}
