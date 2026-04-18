package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/controlplane"
)

type stubRuntimeCompensationReader struct {
	records []controlplane.CompensationRecord
}

func (s *stubRuntimeCompensationReader) CompensationRecords() []controlplane.CompensationRecord {
	return s.records
}

type stubControlplaneCompensationStore struct {
	records []controlplane.CompensationRecord
}

func (s *stubControlplaneCompensationStore) List() []controlplane.CompensationRecord {
	return s.records
}

func TestCompensationRouteRegistered(t *testing.T) {
	s := New(config.Config{AdminAPIKey: "k"}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/control-plane/compensations", nil)
	req.Header.Set("X-Admin-Key", "k")
	s.Handler().ServeHTTP(rr, req)
	if rr.Code == http.StatusNotFound {
		t.Fatalf("expected compensation route to be registered")
	}
}

func TestCompensationResponseShape(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/admin/control-plane/compensations?tenant_id=t1&environment=prod", nil)
	q := req.URL.Query()
	if q.Get("tenant_id") != "t1" {
		t.Fatalf("unexpected tenant_id: %s", q.Get("tenant_id"))
	}
	if q.Get("environment") != "prod" {
		t.Fatalf("unexpected environment: %s", q.Get("environment"))
	}
}

func TestCompensationEndpointReturnsEmptyListByDefault(t *testing.T) {
	s := New(config.Config{AdminAPIKey: "k"}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/control-plane/compensations", nil)
	req.Header.Set("X-Admin-Key", "k")

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body struct {
		Object string                            `json:"object"`
		Data   []controlplane.CompensationRecord `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Object != "list" {
		t.Fatalf("expected object=list, got %q", body.Object)
	}
	if len(body.Data) != 0 {
		t.Fatalf("expected empty data, got %d", len(body.Data))
	}
}

func TestCompensationEndpointReturnsRuntimeAndControlplaneRecords(t *testing.T) {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	s := New(config.Config{AdminAPIKey: "k"}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		WithRuntimeCompensationReader(&stubRuntimeCompensationReader{records: []controlplane.CompensationRecord{{
			Module:          "router",
			TenantID:        "t1",
			Environment:     "prod",
			Version:         "v-runtime",
			FailedStage:     controlplane.FailedStageReload,
			ErrorSummary:    "reload failed",
			SuggestedAction: controlplane.SuggestedActionFor(controlplane.FailedStageReload),
			CreatedAt:       now,
		}}}).
		WithControlplaneCompensationStore(&stubControlplaneCompensationStore{records: []controlplane.CompensationRecord{{
			Module:          "policy",
			TenantID:        "t1",
			Environment:     "prod",
			Version:         "v-control",
			FailedStage:     controlplane.FailedStagePromotionValidation,
			ErrorSummary:    "validation failed",
			SuggestedAction: controlplane.SuggestedActionFor(controlplane.FailedStagePromotionValidation),
			CreatedAt:       now.Add(time.Minute),
		}}})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/control-plane/compensations?tenant_id=t1&environment=prod", nil)
	req.Header.Set("X-Admin-Key", "k")

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body struct {
		Object string                            `json:"object"`
		Data   []controlplane.CompensationRecord `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Object != "list" {
		t.Fatalf("expected object=list, got %q", body.Object)
	}
	if len(body.Data) != 2 {
		t.Fatalf("expected 2 compensation records, got %d", len(body.Data))
	}
	if body.Data[0].Version != "v-control" && body.Data[0].Version != "v-runtime" {
		t.Fatalf("unexpected first record: %+v", body.Data[0])
	}
	if body.Data[1].Version != "v-control" && body.Data[1].Version != "v-runtime" {
		t.Fatalf("unexpected second record: %+v", body.Data[1])
	}
}

func TestCompensationEndpointSortsByCreatedAtDescAndSupportsLimitAndStageFilter(t *testing.T) {
	now := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	s := New(config.Config{AdminAPIKey: "k"}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		WithRuntimeCompensationReader(&stubRuntimeCompensationReader{records: []controlplane.CompensationRecord{{
			Module:          "router",
			TenantID:        "t1",
			Environment:     "prod",
			Version:         "v-runtime-old",
			FailedStage:     controlplane.FailedStageReload,
			ErrorSummary:    "reload failed old",
			SuggestedAction: controlplane.SuggestedActionFor(controlplane.FailedStageReload),
			CreatedAt:       now,
		}, {
			Module:          "router",
			TenantID:        "t1",
			Environment:     "prod",
			Version:         "v-runtime-new",
			FailedStage:     controlplane.FailedStageReload,
			ErrorSummary:    "reload failed new",
			SuggestedAction: controlplane.SuggestedActionFor(controlplane.FailedStageReload),
			CreatedAt:       now.Add(2 * time.Minute),
		}}}).
		WithControlplaneCompensationStore(&stubControlplaneCompensationStore{records: []controlplane.CompensationRecord{{
			Module:          "policy",
			TenantID:        "t1",
			Environment:     "prod",
			Version:         "v-control-mid",
			FailedStage:     controlplane.FailedStagePromotionValidation,
			ErrorSummary:    "validation failed",
			SuggestedAction: controlplane.SuggestedActionFor(controlplane.FailedStagePromotionValidation),
			CreatedAt:       now.Add(time.Minute),
		}}})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/control-plane/compensations?tenant_id=t1&environment=prod&failed_stage=reload_failed&limit=1", nil)
	req.Header.Set("X-Admin-Key", "k")

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body struct {
		Object  string                            `json:"object"`
		Data    []controlplane.CompensationRecord `json:"data"`
		Summary map[string]any                    `json:"summary"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Object != "list" {
		t.Fatalf("expected object=list, got %q", body.Object)
	}
	if len(body.Data) != 1 {
		t.Fatalf("expected 1 record after limit, got %d", len(body.Data))
	}
	if body.Data[0].Version != "v-runtime-new" {
		t.Fatalf("expected newest reload_failed record first, got %+v", body.Data[0])
	}
	if body.Data[0].FailedStage != controlplane.FailedStageReload {
		t.Fatalf("expected failed_stage=%s, got %s", controlplane.FailedStageReload, body.Data[0].FailedStage)
	}
	if body.Summary == nil {
		t.Fatalf("expected summary metadata")
	}
	if got, ok := body.Summary["returned"].(float64); !ok || int(got) != 1 {
		t.Fatalf("expected summary.returned=1, got %#v", body.Summary["returned"])
	}
	if got, ok := body.Summary["filtered_total"].(float64); !ok || int(got) != 2 {
		t.Fatalf("expected summary.filtered_total=2, got %#v", body.Summary["filtered_total"])
	}
	if got, ok := body.Summary["limit"].(float64); !ok || int(got) != 1 {
		t.Fatalf("expected summary.limit=1, got %#v", body.Summary["limit"])
	}
	filters, ok := body.Summary["filters"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary.filters object, got %#v", body.Summary["filters"])
	}
	if filters["failed_stage"] != controlplane.FailedStageReload {
		t.Fatalf("expected failed_stage filter metadata, got %#v", filters["failed_stage"])
	}
}

func TestCompensationEndpointIgnoresInvalidLimit(t *testing.T) {
	now := time.Date(2026, 4, 2, 13, 0, 0, 0, time.UTC)
	s := New(config.Config{AdminAPIKey: "k"}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		WithRuntimeCompensationReader(&stubRuntimeCompensationReader{records: []controlplane.CompensationRecord{{
			Module:          "router",
			TenantID:        "t1",
			Environment:     "prod",
			Version:         "v1",
			FailedStage:     controlplane.FailedStageReload,
			ErrorSummary:    "reload failed",
			SuggestedAction: controlplane.SuggestedActionFor(controlplane.FailedStageReload),
			CreatedAt:       now,
		}, {
			Module:          "router",
			TenantID:        "t1",
			Environment:     "prod",
			Version:         "v2",
			FailedStage:     controlplane.FailedStageReload,
			ErrorSummary:    "reload failed 2",
			SuggestedAction: controlplane.SuggestedActionFor(controlplane.FailedStageReload),
			CreatedAt:       now.Add(time.Minute),
		}}})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/control-plane/compensations?tenant_id=t1&environment=prod&limit=0", nil)
	req.Header.Set("X-Admin-Key", "k")

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body struct {
		Data    []controlplane.CompensationRecord `json:"data"`
		Summary map[string]any                    `json:"summary"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(body.Data) != 2 {
		t.Fatalf("expected invalid limit to be ignored, got %d records", len(body.Data))
	}
	if body.Summary == nil {
		t.Fatalf("expected summary metadata")
	}
	if v, ok := body.Summary["limit"]; ok {
		t.Fatalf("expected no valid limit in summary, got %v", v)
	}
	if got, ok := body.Summary["returned"].(float64); !ok || strconv.Itoa(int(got)) != "2" {
		t.Fatalf("expected summary.returned=2, got %#v", body.Summary["returned"])
	}
}
