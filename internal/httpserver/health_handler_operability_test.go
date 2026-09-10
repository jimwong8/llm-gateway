package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/controlplane"
	"llm-gateway/gateway/internal/providers"
	"llm-gateway/gateway/internal/runtime"
)

func TestAdminHealthIncludesProviderRuntimeAndCompensationSummary(t *testing.T) {
	cfg := config.Config{AdminAPIKey: "k", MockMode: true}
	registry := providers.NewRegistry(cfg, providers.NewMockProvider("mock-fallback", "gpt-4o-mini"), providers.NewMockProvider("mock-openai", "gpt-4o-mini"))

	manager := runtime.NewManager()
	manager.MarkEventSeen("router", "v1", time.Now().UTC())
	manager.SetStatus("router", "error", "reload failed")
	manager.MarkEventSeen("quota", "v2", time.Now().UTC())
	manager.SetStatus("quota", "ok", "")

	runtimeReader := &stubRuntimeCompensationReader{records: []controlplane.CompensationRecord{{
		Module:          "router",
		TenantID:        "t1",
		Environment:     "prod",
		Version:         "v1",
		FailedStage:     controlplane.FailedStageReload,
		ErrorSummary:    "reload failed",
		SuggestedAction: controlplane.SuggestedActionFor(controlplane.FailedStageReload),
		CreatedAt:       time.Now().UTC(),
	}}}
	controlStore := &stubControlplaneCompensationStore{records: []controlplane.CompensationRecord{{
		Module:          "policy",
		TenantID:        "t1",
		Environment:     "prod",
		Version:         "v2",
		FailedStage:     controlplane.FailedStagePromotionValidation,
		ErrorSummary:    "validation failed",
		SuggestedAction: controlplane.SuggestedActionFor(controlplane.FailedStagePromotionValidation),
		CreatedAt:       time.Now().UTC(),
	}}}

	s := New(cfg, registry, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		WithRuntimeCompensationReader(runtimeReader).
		WithControlplaneCompensationStore(controlStore)
	s.runtimeManager = manager

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/health", nil)
	req.Header.Set("X-Admin-Key", "k")

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	providerSummary, ok := body["provider_summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected provider_summary object, got %#v", body["provider_summary"])
	}
	if got, ok := providerSummary["total"].(float64); !ok || int(got) == 0 {
		t.Fatalf("expected provider_summary.total > 0, got %#v", providerSummary["total"])
	}

	runtimeSummary, ok := body["runtime_summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected runtime_summary object, got %#v", body["runtime_summary"])
	}
	if got, ok := runtimeSummary["status_total"].(float64); !ok || int(got) != 2 {
		t.Fatalf("expected runtime_summary.status_total=2, got %#v", runtimeSummary["status_total"])
	}
	if got, ok := runtimeSummary["ok"].(float64); !ok || int(got) != 1 {
		t.Fatalf("expected runtime_summary.ok=1, got %#v", runtimeSummary["ok"])
	}
	if got, ok := runtimeSummary["error"].(float64); !ok || int(got) != 1 {
		t.Fatalf("expected runtime_summary.error=1, got %#v", runtimeSummary["error"])
	}

	compensationStats, ok := body["compensation_stats"].(map[string]any)
	if !ok {
		t.Fatalf("expected compensation_stats object, got %#v", body["compensation_stats"])
	}
	if got, ok := compensationStats["total"].(float64); !ok || int(got) != 2 {
		t.Fatalf("expected compensation total=2, got %#v", compensationStats["total"])
	}
	if got, ok := compensationStats["runtime"].(float64); !ok || int(got) != 1 {
		t.Fatalf("expected compensation runtime=1, got %#v", compensationStats["runtime"])
	}
	if got, ok := compensationStats["controlplane"].(float64); !ok || int(got) != 1 {
		t.Fatalf("expected compensation controlplane=1, got %#v", compensationStats["controlplane"])
	}
}
