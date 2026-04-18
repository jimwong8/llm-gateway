package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"llm-gateway/gateway/internal/audit"
	"llm-gateway/gateway/internal/controlplane"
	"llm-gateway/gateway/internal/runtime"
)

func parseRFC3339TimeForTest(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("expected RFC3339 time, got %q: %v", value, err)
	}
	return parsed
}

const testAdminToken = "admin-secret"

func newAuthenticatedAdminHandler(service *controlplane.Service) *AdminHandler {
	return NewAdminHandler(service).WithAdminToken(testAdminToken)
}

func authorizeAdminRequest(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
}

func TestAdminHandlerRejectsRequestsWhenTokenNotConfigured(t *testing.T) {
	handler := NewAdminHandler(controlplane.NewService())
	req := httptest.NewRequest(http.MethodGet, "/admin/audit-events", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestAdminHandlerRejectsRequestsWithoutAuthorizationHeader(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	req := httptest.NewRequest(http.MethodGet, "/admin/audit-events", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestAdminHandlerRejectsRequestsWithMalformedAuthorizationHeader(t *testing.T) {
	cases := []struct {
		name          string
		authorization string
	}{
		{name: "basic scheme", authorization: "Basic abc"},
		{name: "missing bearer token", authorization: "Bearer"},
		{name: "empty authorization", authorization: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := newAuthenticatedAdminHandler(controlplane.NewService())
			req := httptest.NewRequest(http.MethodGet, "/admin/audit-events", nil)
			if tc.authorization != "" {
				req.Header.Set("Authorization", tc.authorization)
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d, got %d, body=%s", http.StatusUnauthorized, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestAdminHandlerRejectsRequestsWithWrongToken(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	req := httptest.NewRequest(http.MethodGet, "/admin/audit-events", nil)
	authorizeAdminRequest(req, "wrong-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestAdminHandlerAllowsRequestsWithValidBearerToken(t *testing.T) {
	recorder := audit.NewRecorder()
	recorder.RecordRelease("router", "tenant-a", "prod", "cfg_rel_prod", "release-bot", "promote staging to prod")
	handler := NewAdminHandler(controlplane.NewService()).WithAuditReader(recorder).WithAdminToken(testAdminToken)
	req := httptest.NewRequest(http.MethodGet, "/admin/audit-events", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp []audit.ControlPlaneEvent
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(resp))
	}
}

func TestCreateInheritanceDraftHandler(t *testing.T) {
	svc := controlplane.NewService()
	_, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "staging",
		Scope:       "tenant",
		Version:     "cfg_rel_staging",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "staging released",
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	handler := newAuthenticatedAdminHandler(svc)
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","scope":"tenant","source_environment":"staging","target_environment":"prod","reason":"seed prod candidate from staging","actor":"architect"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/inheritance-drafts", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	var resp versionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != controlplane.ConfigStatusDraft {
		t.Fatalf("expected draft status, got %q", resp.Status)
	}
	if resp.Environment != "prod" {
		t.Fatalf("expected environment prod, got %q", resp.Environment)
	}
	if resp.Source == nil {
		t.Fatalf("expected source metadata for inheritance draft")
	}
	if resp.Source.Type != controlplane.SourceTypeInheritance {
		t.Fatalf("expected source type %q, got %q", controlplane.SourceTypeInheritance, resp.Source.Type)
	}
	if resp.Source.SourceEnvironment != "staging" {
		t.Fatalf("expected source environment staging, got %q", resp.Source.SourceEnvironment)
	}
	if resp.Source.SourceVersionID != "cfg_rel_staging" {
		t.Fatalf("expected source version cfg_rel_staging, got %q", resp.Source.SourceVersionID)
	}
}

func TestGetVersionHandlerReturnsDraftSourceMetadata(t *testing.T) {
	svc := controlplane.NewService()
	_, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "staging",
		Scope:       "tenant",
		Version:     "cfg_rel_staging",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "staging released",
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	draft, err := svc.CreateInheritanceDraft(context.Background(), controlplane.CreateInheritanceDraftInput{
		Module:            "router",
		TenantID:          "tenant-a",
		Scope:             "tenant",
		SourceEnvironment: "staging",
		TargetEnvironment: "prod",
		Reason:            "seed prod candidate from staging",
		Actor:             "architect",
	})
	if err != nil {
		t.Fatalf("CreateInheritanceDraft returned error: %v", err)
	}

	handler := newAuthenticatedAdminHandler(svc)
	req := httptest.NewRequest(
		http.MethodGet,
		"/admin/config-versions/"+draft.Version+"?module=router&tenant_id=tenant-a&environment=prod&scope=tenant",
		nil,
	)
	req.SetPathValue("versionID", draft.Version)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp versionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.VersionID != draft.Version {
		t.Fatalf("expected version %q, got %q", draft.Version, resp.VersionID)
	}
	if resp.Status != controlplane.ConfigStatusDraft {
		t.Fatalf("expected draft status, got %q", resp.Status)
	}
	if resp.Source == nil {
		t.Fatalf("expected draft source metadata")
	}
	if resp.Source.SourceEnvironment != "staging" {
		t.Fatalf("expected source environment staging, got %q", resp.Source.SourceEnvironment)
	}
	if resp.Source.SourceVersionID != "cfg_rel_staging" {
		t.Fatalf("expected source version cfg_rel_staging, got %q", resp.Source.SourceVersionID)
	}
}

func TestGetVersionHandlerReturnsReleasedWithoutInheritanceMetadata(t *testing.T) {
	svc := controlplane.NewService()
	released, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "prod released",
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	handler := newAuthenticatedAdminHandler(svc)
	req := httptest.NewRequest(
		http.MethodGet,
		"/admin/config-versions/"+released.Version+"?module=router&tenant_id=tenant-a&environment=prod&scope=tenant",
		nil,
	)
	req.SetPathValue("versionID", released.Version)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp versionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != controlplane.ConfigStatusReleased {
		t.Fatalf("expected released status, got %q", resp.Status)
	}
	if resp.Source != nil {
		t.Fatalf("expected nil source metadata for released version")
	}
}

func TestCreateInheritanceDraftHandlerReturnsBadRequestOnInvalidJSON(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	req := httptest.NewRequest(http.MethodPost, "/admin/inheritance-drafts", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCreateInheritanceDraftHandlerReturnsBadRequestWhenRequiredFieldsMissing(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","scope":"tenant","target_environment":"prod"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/inheritance-drafts", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestCreateInheritanceDraftHandlerReturnsBadRequestWhenRequiredFieldsBlank(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	body := bytes.NewBufferString(`{"module":"   ","tenant_id":"tenant-a","scope":"tenant","source_environment":"staging","target_environment":"prod"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/inheritance-drafts", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestCreateInheritanceDraftHandlerReturnsNotFoundWhenSourceMissing(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","scope":"tenant","source_environment":"staging","target_environment":"prod"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/inheritance-drafts", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

func TestCreateInheritanceDraftHandlerReturnsBadRequestWhenSourceEqualsTarget(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","scope":"tenant","source_environment":"prod","target_environment":"prod"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/inheritance-drafts", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestGetVersionHandlerReturnsNotFoundWhenVersionMissing(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	req := httptest.NewRequest(
		http.MethodGet,
		"/admin/config-versions/missing?module=router&tenant_id=tenant-a&environment=prod&scope=tenant",
		nil,
	)
	req.SetPathValue("versionID", "missing")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

func TestListVersionsHandlerReturnsFilteredVersionsNewestFirst(t *testing.T) {
	svc := controlplane.NewService()
	_, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "staging",
		Scope:       "tenant",
		Version:     "cfg_rel_staging",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "staging released",
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}
	_, err = svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "prod released",
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}
	_, err = svc.CreateInheritanceDraft(context.Background(), controlplane.CreateInheritanceDraftInput{
		Module:            "router",
		TenantID:          "tenant-a",
		Scope:             "tenant",
		SourceEnvironment: "staging",
		TargetEnvironment: "prod",
		Reason:            "seed prod candidate from staging",
		Actor:             "architect",
	})
	if err != nil {
		t.Fatalf("CreateInheritanceDraft returned error: %v", err)
	}
	_, err = svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "policy",
		TenantID:    "tenant-b",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_policy",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "policy released",
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	handler := newAuthenticatedAdminHandler(svc)
	req := httptest.NewRequest(
		http.MethodGet,
		"/admin/config-versions?module=router&tenant_id=tenant-a&environment=prod&scope=tenant",
		nil,
	)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp []versionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(resp))
	}
	if resp[0].VersionID != "cfg_003" {
		t.Fatalf("expected newest version cfg_003 first, got %q", resp[0].VersionID)
	}
	if resp[0].Status != controlplane.ConfigStatusDraft {
		t.Fatalf("expected first version draft, got %q", resp[0].Status)
	}
	if resp[1].VersionID != "cfg_rel_prod" {
		t.Fatalf("expected older version cfg_rel_prod second, got %q", resp[1].VersionID)
	}
}

func TestListVersionsHandlerReturnsEmptyArrayWhenNoMatch(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	req := httptest.NewRequest(http.MethodGet, "/admin/config-versions?module=router&tenant_id=tenant-a&environment=prod&scope=tenant", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}
	if rr.Body.String() != "[]\n" {
		t.Fatalf("expected empty array response, got %q", rr.Body.String())
	}
}

func TestReleaseDraftHandler(t *testing.T) {
	svc := controlplane.NewService()
	_, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "staging",
		Scope:       "tenant",
		Version:     "cfg_rel_staging",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "staging released",
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}
	_, err = svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod_v1",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "prod released v1",
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}
	createdDraft, err := svc.CreateInheritanceDraft(context.Background(), controlplane.CreateInheritanceDraftInput{
		Module:            "router",
		TenantID:          "tenant-a",
		Scope:             "tenant",
		SourceEnvironment: "staging",
		TargetEnvironment: "prod",
		Reason:            "seed prod candidate from staging",
		Actor:             "architect",
	})
	if err != nil {
		t.Fatalf("CreateInheritanceDraft returned error: %v", err)
	}

	handler := newAuthenticatedAdminHandler(svc)
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod","scope":"tenant","version_id":"` + createdDraft.Version + `","actor":"release-bot","reason":"approve prod draft"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/releases", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp versionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.VersionID != createdDraft.Version {
		t.Fatalf("expected released version %q, got %q", createdDraft.Version, resp.VersionID)
	}
	if resp.Status != controlplane.ConfigStatusReleased {
		t.Fatalf("expected released status, got %q", resp.Status)
	}
}

func TestReplayReleasedHandlerRequiresAuthorization(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService()).WithRuntimeReplayPublisher(runtime.NewPublisher())
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod","scope":"tenant","version_id":"cfg_rel_prod"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/releases/replay", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestReplayCompensationHandlerRequiresAuthorization(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService()).WithRuntimeReplayPublisher(runtime.NewPublisher())
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod","version_id":"cfg_rel_compensate"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/control-plane/compensations/replay", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestReplayReleasedHandlerReturnsServiceUnavailableWithoutReplayPublisher(t *testing.T) {
	svc := controlplane.NewService()
	_, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "prod released",
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	handler := newAuthenticatedAdminHandler(svc)
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod","scope":"tenant","version_id":"cfg_rel_prod"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/releases/replay", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

func TestReplayCompensationHandlerReturnsServiceUnavailableWithoutReplayPublisher(t *testing.T) {
	svc := controlplane.NewService()
	_, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_compensate",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "released for compensation replay",
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	handler := newAuthenticatedAdminHandler(svc)
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod","version_id":"cfg_rel_compensate"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/control-plane/compensations/replay", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

func TestReplayReleasedHandlerReturnsBadRequestWhenRequiredFieldsMissing(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService()).WithRuntimeReplayPublisher(runtime.NewPublisher())
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","scope":"tenant","version_id":"cfg_rel_prod"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/releases/replay", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestReplayReleasedHandlerReturnsConflictWhenVersionNotReleased(t *testing.T) {
	svc := controlplane.NewService()
	_, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_draft_prod",
		Status:      controlplane.ConfigStatusDraft,
		Summary:     "prod draft",
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	publisher := runtime.NewPublisher()
	handler := newAuthenticatedAdminHandler(svc).WithRuntimeReplayPublisher(publisher)
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod","scope":"tenant","version_id":"cfg_draft_prod"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/releases/replay", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusConflict, rr.Code, rr.Body.String())
	}
	if len(publisher.Events()) != 0 {
		t.Fatalf("expected replay to not publish runtime event for non-released version")
	}
}

func TestReplayReleasedHandlerPublishesRuntimeEventForReleasedVersion(t *testing.T) {
	svc := controlplane.NewService()
	_, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "prod released",
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	publisher := runtime.NewPublisher()
	handler := newAuthenticatedAdminHandler(svc).
		WithRuntimeReplayPublisher(publisher).
		WithRuntimeReader(publisher)

	replayBody := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod","scope":"tenant","version_id":"cfg_rel_prod"}`)
	replayReq := httptest.NewRequest(http.MethodPost, "/admin/releases/replay", replayBody)
	replayReq.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(replayReq, testAdminToken)
	replayResp := httptest.NewRecorder()

	handler.ServeHTTP(replayResp, replayReq)

	if replayResp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, replayResp.Code, replayResp.Body.String())
	}

	var replayVersion versionResponse
	if err := json.Unmarshal(replayResp.Body.Bytes(), &replayVersion); err != nil {
		t.Fatalf("failed to decode replay response: %v", err)
	}
	if replayVersion.Status != controlplane.ConfigStatusReleased {
		t.Fatalf("expected replay response status %q, got %q", controlplane.ConfigStatusReleased, replayVersion.Status)
	}

	eventsReq := httptest.NewRequest(http.MethodGet, "/admin/runtime-events", nil)
	authorizeAdminRequest(eventsReq, testAdminToken)
	eventsResp := httptest.NewRecorder()

	handler.ServeHTTP(eventsResp, eventsReq)

	if eventsResp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, eventsResp.Code, eventsResp.Body.String())
	}

	var events []runtime.Event
	if err := json.Unmarshal(eventsResp.Body.Bytes(), &events); err != nil {
		t.Fatalf("failed to decode runtime events response: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 runtime event after replay, got %d", len(events))
	}
	if events[0].Version.Version != "cfg_rel_prod" {
		t.Fatalf("expected replayed runtime event version cfg_rel_prod, got %q", events[0].Version.Version)
	}
	if events[0].Apply.PayloadRef != "released://router/tenant-a/prod/tenant//cfg_rel_prod" {
		t.Fatalf("expected released payload ref for replayed event, got %q", events[0].Apply.PayloadRef)
	}
}

func TestReplayCompensationHandlerPublishesRuntimeEventWithScopeInference(t *testing.T) {
	svc := controlplane.NewService()
	_, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "project",
		ProjectID:   "project-a",
		Version:     "cfg_rel_compensate",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "released for compensation replay",
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	publisher := runtime.NewPublisher()
	handler := newAuthenticatedAdminHandler(svc).
		WithRuntimeReplayPublisher(publisher).
		WithRuntimeReader(publisher)

	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod","version":"cfg_rel_compensate"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/control-plane/compensations/replay", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp versionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.VersionID != "cfg_rel_compensate" || resp.Status != controlplane.ConfigStatusReleased {
		t.Fatalf("unexpected response: %+v", resp)
	}

	events := publisher.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 runtime event after compensation replay, got %d", len(events))
	}
	if events[0].Version.Version != "cfg_rel_compensate" {
		t.Fatalf("expected replayed version cfg_rel_compensate, got %q", events[0].Version.Version)
	}
	if events[0].Version.Scope != "project" || events[0].Version.ProjectID != "project-a" {
		t.Fatalf("expected inferred scope/project from released version, got scope=%q project=%q", events[0].Version.Scope, events[0].Version.ProjectID)
	}
}

func TestReplayCompensationHandlerReturnsConflictWhenTargetAmbiguous(t *testing.T) {
	svc := controlplane.NewService()
	_, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_shared",
		Status:      controlplane.ConfigStatusReleased,
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}
	_, err = svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "project",
		ProjectID:   "project-a",
		Version:     "cfg_rel_shared",
		Status:      controlplane.ConfigStatusReleased,
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	publisher := runtime.NewPublisher()
	handler := newAuthenticatedAdminHandler(svc).WithRuntimeReplayPublisher(publisher)

	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod","version_id":"cfg_rel_shared"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/control-plane/compensations/replay", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusConflict, rr.Code, rr.Body.String())
	}
	if len(publisher.Events()) != 0 {
		t.Fatalf("expected no runtime event when compensation replay target is ambiguous")
	}
}

func TestReplayCompensationHandlerReturnsConflictWhenVersionNotReleased(t *testing.T) {
	svc := controlplane.NewService()
	_, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_draft_compensate",
		Status:      controlplane.ConfigStatusDraft,
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	publisher := runtime.NewPublisher()
	handler := newAuthenticatedAdminHandler(svc).WithRuntimeReplayPublisher(publisher)

	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod","version_id":"cfg_draft_compensate"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/control-plane/compensations/replay", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusConflict, rr.Code, rr.Body.String())
	}
	if len(publisher.Events()) != 0 {
		t.Fatalf("expected no runtime event for non-released compensation target")
	}
}

func TestReplayCompensationHandlerReturnsBadRequestOnInvalidJSON(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService()).WithRuntimeReplayPublisher(runtime.NewPublisher())
	req := httptest.NewRequest(http.MethodPost, "/admin/control-plane/compensations/replay", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestReplayCompensationHandlerReturnsBadRequestWhenRequiredFieldsMissing(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService()).WithRuntimeReplayPublisher(runtime.NewPublisher())
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/control-plane/compensations/replay", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestPromotionHandler(t *testing.T) {
	svc := controlplane.NewService()
	_, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "staging",
		Scope:       "tenant",
		Version:     "cfg_rel_staging",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "staging released",
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}
	_, err = svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod_v1",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "prod released v1",
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	handler := newAuthenticatedAdminHandler(svc)
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","source_environment":"staging","target_environment":"prod","scope":"tenant","actor":"release-bot","reason":"promote staging to prod"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/promotions", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp versionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != controlplane.ConfigStatusReleased {
		t.Fatalf("expected released status, got %q", resp.Status)
	}
	if resp.VersionID == "cfg_rel_staging" {
		t.Fatalf("expected promotion to create a new released version id")
	}
}

func TestRollbackReleasedHandlerCreatesNewReleasedVersion(t *testing.T) {
	svc := controlplane.NewService()
	_, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod_v1",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "prod released v1",
		Config: map[string]string{
			"policy": `{"type":"direct","model":"gpt-4o-mini"}`,
		},
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}

	publisher := runtime.NewPublisher()
	svc.WithReleasePublisher(publisher)
	handler := newAuthenticatedAdminHandler(svc).WithRuntimeReplayPublisher(publisher)
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod","scope":"tenant","version_id":"cfg_rel_prod_v1","actor":"release-bot","reason":"rollback to known good"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/releases/rollback", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp versionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != controlplane.ConfigStatusReleased {
		t.Fatalf("expected released status, got %q", resp.Status)
	}
	if resp.VersionID == "cfg_rel_prod_v1" {
		t.Fatalf("expected rollback to create a new released version id")
	}
	if len(publisher.Events()) != 1 {
		t.Fatalf("expected rollback to publish one runtime event, got %d", len(publisher.Events()))
	}
	if publisher.Events()[0].Version.SourceVersion != "cfg_rel_prod_v1" {
		t.Fatalf("expected rollback event source version cfg_rel_prod_v1, got %q", publisher.Events()[0].Version.SourceVersion)
	}
}

func TestRollbackReleasedHandlerReturnsNotFoundWhenTargetMissing(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod","scope":"tenant","version_id":"missing"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/releases/rollback", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

func TestRollbackReleasedHandlerReturnsBadRequestWhenRequiredFieldsMissing(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod","version_id":"cfg_rel_prod_v1"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/releases/rollback", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestReleaseDraftHandlerReturnsNotFoundWhenVersionMissing(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"prod","scope":"tenant","version_id":"missing","actor":"release-bot","reason":"approve prod draft"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/releases", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

func TestReleaseDraftHandlerReturnsBadRequestWhenRequiredFieldsMissing(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","scope":"tenant","version_id":"missing"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/releases", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestReleaseDraftHandlerReturnsBadRequestWhenRequiredFieldsBlank(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","environment":"   ","scope":"tenant","version_id":"missing"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/releases", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestReleaseDraftHandlerReturnsBadRequestOnInvalidJSON(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	req := httptest.NewRequest(http.MethodPost, "/admin/releases", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestPromotionHandlerReturnsNotFoundWhenSourceReleasedMissing(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","source_environment":"staging","target_environment":"prod","scope":"tenant","actor":"release-bot","reason":"promote staging to prod"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/promotions", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

func TestPromotionHandlerReturnsBadRequestWhenRequiredFieldsMissing(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","target_environment":"prod","scope":"tenant"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/promotions", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestPromotionHandlerReturnsBadRequestWhenRequiredFieldsBlank(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","source_environment":"staging","target_environment":"   ","scope":"tenant"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/promotions", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestPromotionHandlerReturnsBadRequestWhenSourceEqualsTarget(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	body := bytes.NewBufferString(`{"module":"router","tenant_id":"tenant-a","source_environment":"prod","target_environment":"prod","scope":"tenant"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/promotions", body)
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestPromotionHandlerReturnsBadRequestOnInvalidJSON(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	req := httptest.NewRequest(http.MethodPost, "/admin/promotions", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAuditEventsHandler(t *testing.T) {
	recorder := audit.NewRecorder()
	recorder.RecordRelease("router", "tenant-a", "prod", "cfg_rel_prod", "release-bot", "promote staging to prod")
	handler := newAuthenticatedAdminHandler(controlplane.NewService()).WithAuditReader(recorder)
	req := httptest.NewRequest(http.MethodGet, "/admin/audit-events", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp []audit.ControlPlaneEvent
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(resp))
	}
	if resp[0].Type != audit.ControlPlaneEventTypeRelease {
		t.Fatalf("expected event type %q, got %q", audit.ControlPlaneEventTypeRelease, resp[0].Type)
	}
}

func TestAuditEventsHandlerSupportsTenantAndEnvironmentFilter(t *testing.T) {
	recorder := audit.NewRecorder()
	recorder.RecordRelease("router", "tenant-a", "prod", "cfg_rel_prod", "release-bot", "promote staging to prod")
	recorder.RecordRelease("router", "tenant-b", "staging", "cfg_rel_staging", "release-bot", "promote staging to prod")
	handler := newAuthenticatedAdminHandler(controlplane.NewService()).WithAuditReader(recorder)
	req := httptest.NewRequest(http.MethodGet, "/admin/audit-events?tenant_id=tenant-a&environment=prod", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp []audit.ControlPlaneEvent
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 filtered audit event, got %d", len(resp))
	}
	if resp[0].TenantID != "tenant-a" || resp[0].Environment != "prod" {
		t.Fatalf("unexpected filtered audit event: tenant=%s env=%s", resp[0].TenantID, resp[0].Environment)
	}
}

func TestAuditEventsHandlerSupportsLimit(t *testing.T) {
	recorder := audit.NewRecorder()
	recorder.RecordRelease("router", "tenant-a", "prod", "cfg_rel_prod_1", "release-bot", "first")
	recorder.RecordRelease("router", "tenant-a", "prod", "cfg_rel_prod_2", "release-bot", "second")
	handler := newAuthenticatedAdminHandler(controlplane.NewService()).WithAuditReader(recorder)
	req := httptest.NewRequest(http.MethodGet, "/admin/audit-events?limit=1", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp []audit.ControlPlaneEvent
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 limited audit event, got %d", len(resp))
	}
	if resp[0].VersionID != "cfg_rel_prod_2" {
		t.Fatalf("expected newest audit event first, got %q", resp[0].VersionID)
	}
}

func TestAuditEventsHandlerSupportsInvalidAndCappedLimit(t *testing.T) {
	recorder := audit.NewRecorder()
	recorder.RecordRelease("router", "tenant-a", "prod", "cfg_rel_prod_1", "release-bot", "first")
	recorder.RecordRelease("router", "tenant-a", "prod", "cfg_rel_prod_2", "release-bot", "second")
	handler := newAuthenticatedAdminHandler(controlplane.NewService()).WithAuditReader(recorder)

	cases := []struct {
		name          string
		query         string
		expectedCount int
	}{
		{name: "zero treated as unlimited", query: "/admin/audit-events?limit=0", expectedCount: 2},
		{name: "negative treated as unlimited", query: "/admin/audit-events?limit=-1", expectedCount: 2},
		{name: "nonnumeric treated as unlimited", query: "/admin/audit-events?limit=abc", expectedCount: 2},
		{name: "oversized capped but still returns all", query: "/admin/audit-events?limit=999", expectedCount: 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.query, nil)
			authorizeAdminRequest(req, testAdminToken)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
			}
			var resp []audit.ControlPlaneEvent
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if len(resp) != tc.expectedCount {
				t.Fatalf("expected %d audit events, got %d", tc.expectedCount, len(resp))
			}
		})
	}
}

func TestRuntimeEventsHandler(t *testing.T) {
	publisher := runtime.NewPublisher()
	publisher.PublishIfReleased(controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod",
		Source:      controlplane.ConfigStatusReleased,
	})
	handler := newAuthenticatedAdminHandler(controlplane.NewService()).WithRuntimeReader(publisher)
	req := httptest.NewRequest(http.MethodGet, "/admin/runtime-events", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp []runtime.Event
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 runtime event, got %d", len(resp))
	}
	if resp[0].Version.Version != "cfg_rel_prod" {
		t.Fatalf("expected version cfg_rel_prod, got %q", resp[0].Version.Version)
	}
}

func TestRuntimeEventsHandlerSupportsTenantAndEnvironmentFilter(t *testing.T) {
	publisher := runtime.NewPublisher()
	publisher.PublishIfReleased(controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod",
		Source:      controlplane.ConfigStatusReleased,
	})
	publisher.PublishIfReleased(controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-b",
		Environment: "staging",
		Scope:       "tenant",
		Version:     "cfg_rel_staging",
		Source:      controlplane.ConfigStatusReleased,
	})
	handler := newAuthenticatedAdminHandler(controlplane.NewService()).WithRuntimeReader(publisher)
	req := httptest.NewRequest(http.MethodGet, "/admin/runtime-events?tenant_id=tenant-a&environment=prod", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp []runtime.Event
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 filtered runtime event, got %d", len(resp))
	}
	if resp[0].Version.TenantID != "tenant-a" || resp[0].Version.Environment != "prod" {
		t.Fatalf("unexpected filtered runtime event: tenant=%s env=%s", resp[0].Version.TenantID, resp[0].Version.Environment)
	}
}

func TestRuntimeEventsHandlerSupportsLimit(t *testing.T) {
	publisher := runtime.NewPublisher()
	publisher.PublishIfReleased(controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod_1",
		Source:      controlplane.ConfigStatusReleased,
		CreatedAt:   time.Date(2026, 3, 24, 20, 0, 0, 0, time.UTC),
	})
	publisher.PublishIfReleased(controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod_2",
		Source:      controlplane.ConfigStatusReleased,
		CreatedAt:   time.Date(2026, 3, 24, 20, 1, 0, 0, time.UTC),
	})
	handler := newAuthenticatedAdminHandler(controlplane.NewService()).WithRuntimeReader(publisher)
	req := httptest.NewRequest(http.MethodGet, "/admin/runtime-events?limit=1", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp []runtime.Event
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 limited runtime event, got %d", len(resp))
	}
	if resp[0].Version.Version != "cfg_rel_prod_2" {
		t.Fatalf("expected newest runtime event first, got %q", resp[0].Version.Version)
	}
}

func TestRuntimeEventsHandlerSupportsInvalidAndCappedLimit(t *testing.T) {
	publisher := runtime.NewPublisher()
	publisher.PublishIfReleased(controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod_1",
		Source:      controlplane.ConfigStatusReleased,
		CreatedAt:   time.Date(2026, 3, 24, 20, 0, 0, 0, time.UTC),
	})
	publisher.PublishIfReleased(controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod_2",
		Source:      controlplane.ConfigStatusReleased,
		CreatedAt:   time.Date(2026, 3, 24, 20, 1, 0, 0, time.UTC),
	})
	handler := newAuthenticatedAdminHandler(controlplane.NewService()).WithRuntimeReader(publisher)

	cases := []struct {
		name          string
		query         string
		expectedCount int
	}{
		{name: "zero treated as unlimited", query: "/admin/runtime-events?limit=0", expectedCount: 2},
		{name: "negative treated as unlimited", query: "/admin/runtime-events?limit=-1", expectedCount: 2},
		{name: "nonnumeric treated as unlimited", query: "/admin/runtime-events?limit=abc", expectedCount: 2},
		{name: "oversized capped but still returns all", query: "/admin/runtime-events?limit=999", expectedCount: 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.query, nil)
			authorizeAdminRequest(req, testAdminToken)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
			}
			var resp []runtime.Event
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if len(resp) != tc.expectedCount {
				t.Fatalf("expected %d runtime events, got %d", tc.expectedCount, len(resp))
			}
		})
	}
}

func TestAuditEventsHandlerReturnsEmptyArrayWithoutReader(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	req := httptest.NewRequest(http.MethodGet, "/admin/audit-events", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}
	if rr.Body.String() != "[]\n" {
		t.Fatalf("expected empty array response, got %q", rr.Body.String())
	}
}

func TestRuntimeEventsHandlerReturnsEmptyArrayWithoutReader(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	req := httptest.NewRequest(http.MethodGet, "/admin/runtime-events", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}
	if rr.Body.String() != "[]\n" {
		t.Fatalf("expected empty array response, got %q", rr.Body.String())
	}
}

func TestRuntimeEventsHandlerSupportsSummaryView(t *testing.T) {
	publisher := runtime.NewPublisher()
	publisher.PublishIfReleased(controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod_1",
		Source:      controlplane.ConfigStatusReleased,
	})
	publisher.PublishIfReleased(controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod_2",
		Source:      controlplane.ConfigStatusReleased,
	})
	publisher.PublishIfReleased(controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "staging",
		Scope:       "tenant",
		Version:     "cfg_rel_staging",
		Source:      controlplane.ConfigStatusReleased,
	})
	handler := newAuthenticatedAdminHandler(controlplane.NewService()).WithRuntimeReader(publisher)
	req := httptest.NewRequest(http.MethodGet, "/admin/runtime-events?summary=true&tenant_id=tenant-a", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp auditSummaryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode summary response: %v", err)
	}
	if resp.Total != 3 {
		t.Fatalf("expected total 3, got %d", resp.Total)
	}
	if resp.ByType[controlplane.ConfigStatusReleased] != 3 {
		t.Fatalf("expected 3 released runtime events, got %d", resp.ByType[controlplane.ConfigStatusReleased])
	}
	if resp.ByEnvironment["prod"] != 2 {
		t.Fatalf("expected 2 prod runtime events, got %d", resp.ByEnvironment["prod"])
	}
	if resp.ByEnvironment["staging"] != 1 {
		t.Fatalf("expected 1 staging runtime event, got %d", resp.ByEnvironment["staging"])
	}
}

func TestRuntimeEventsHandlerSupportsSummaryViewWithoutReader(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	req := httptest.NewRequest(http.MethodGet, "/admin/runtime-events?summary=true", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp auditSummaryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode summary response: %v", err)
	}
	if resp.Total != 0 {
		t.Fatalf("expected total 0, got %d", resp.Total)
	}
	if len(resp.ByType) != 0 || len(resp.ByEnvironment) != 0 {
		t.Fatalf("expected empty summary maps, got by_type=%v by_environment=%v", resp.ByType, resp.ByEnvironment)
	}
}

func TestRuntimeEventsHandlerSummarySupportsFilterCombination(t *testing.T) {
	publisher := runtime.NewPublisher()
	publisher.PublishIfReleased(controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod_1",
		Source:      controlplane.ConfigStatusReleased,
	})
	publisher.PublishIfReleased(controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "staging",
		Scope:       "tenant",
		Version:     "cfg_rel_staging",
		Source:      controlplane.ConfigStatusReleased,
	})
	publisher.PublishIfReleased(controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-b",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod_other_tenant",
		Source:      controlplane.ConfigStatusReleased,
	})
	handler := newAuthenticatedAdminHandler(controlplane.NewService()).WithRuntimeReader(publisher)
	req := httptest.NewRequest(http.MethodGet, "/admin/runtime-events?summary=true&tenant_id=tenant-a&environment=prod", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp auditSummaryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode summary response: %v", err)
	}
	if resp.Total != 1 {
		t.Fatalf("expected total 1, got %d", resp.Total)
	}
	if resp.ByType[controlplane.ConfigStatusReleased] != 1 {
		t.Fatalf("expected 1 released runtime event, got %d", resp.ByType[controlplane.ConfigStatusReleased])
	}
	if resp.ByEnvironment["prod"] != 1 {
		t.Fatalf("expected 1 prod runtime event, got %d", resp.ByEnvironment["prod"])
	}
	if len(resp.ByEnvironment) != 1 {
		t.Fatalf("expected only 1 environment bucket, got %d", len(resp.ByEnvironment))
	}
}

func TestRuntimeEventsHandlerSummaryRequiresAuthorization(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	req := httptest.NewRequest(http.MethodGet, "/admin/runtime-events?summary=true", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestRuntimeEventsHandlerSummaryIsUnauthorizedWithWrongToken(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	req := httptest.NewRequest(http.MethodGet, "/admin/runtime-events?summary=true", nil)
	authorizeAdminRequest(req, "wrong-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestRuntimeEventsHandlerSummaryRejectsNonGetMethod(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	req := httptest.NewRequest(http.MethodPost, "/admin/runtime-events?summary=true", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusMethodNotAllowed, rr.Code, rr.Body.String())
	}
}

func TestAuditEventsHandlerSupportsSummaryView(t *testing.T) {
	recorder := audit.NewRecorder()
	recorder.RecordRelease("router", "tenant-a", "prod", "cfg_rel_prod_1", "release-bot", "first")
	recorder.RecordRelease("router", "tenant-a", "prod", "cfg_rel_prod_2", "release-bot", "second")
	recorder.RecordInheritanceDraft("router", "tenant-a", "staging", "dev", "cfg_rel_dev", "cfg_draft_staging", "architect", "seed staging")
	handler := newAuthenticatedAdminHandler(controlplane.NewService()).WithAuditReader(recorder)
	req := httptest.NewRequest(http.MethodGet, "/admin/audit-events?summary=true&tenant_id=tenant-a", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp auditSummaryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode summary response: %v", err)
	}
	if resp.Total != 3 {
		t.Fatalf("expected total 3, got %d", resp.Total)
	}
	if resp.ByType[audit.ControlPlaneEventTypeRelease] != 2 {
		t.Fatalf("expected 2 release events, got %d", resp.ByType[audit.ControlPlaneEventTypeRelease])
	}
	if resp.ByType[audit.ControlPlaneEventTypeInheritanceDraft] != 1 {
		t.Fatalf("expected 1 inheritance draft event, got %d", resp.ByType[audit.ControlPlaneEventTypeInheritanceDraft])
	}
	if resp.ByEnvironment["prod"] != 2 {
		t.Fatalf("expected 2 prod events, got %d", resp.ByEnvironment["prod"])
	}
	if resp.ByEnvironment["staging"] != 1 {
		t.Fatalf("expected 1 staging event, got %d", resp.ByEnvironment["staging"])
	}
}

func TestAuditEventsHandlerSupportsSummaryViewWithoutReader(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	req := httptest.NewRequest(http.MethodGet, "/admin/audit-events?summary=true", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp auditSummaryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode summary response: %v", err)
	}
	if resp.Total != 0 {
		t.Fatalf("expected total 0, got %d", resp.Total)
	}
	if len(resp.ByType) != 0 || len(resp.ByEnvironment) != 0 {
		t.Fatalf("expected empty summary maps, got by_type=%v by_environment=%v", resp.ByType, resp.ByEnvironment)
	}
}

func TestAuditEventsHandlerSummaryIgnoresLimitAndReturnsFilteredAggregate(t *testing.T) {
	recorder := audit.NewRecorder()
	recorder.RecordRelease("router", "tenant-a", "prod", "cfg_rel_prod_1", "release-bot", "first")
	recorder.RecordRelease("router", "tenant-a", "prod", "cfg_rel_prod_2", "release-bot", "second")
	recorder.RecordRelease("router", "tenant-b", "staging", "cfg_rel_staging", "release-bot", "third")
	handler := newAuthenticatedAdminHandler(controlplane.NewService()).WithAuditReader(recorder)
	req := httptest.NewRequest(http.MethodGet, "/admin/audit-events?summary=true&tenant_id=tenant-a&limit=1", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp auditSummaryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode summary response: %v", err)
	}
	if resp.Total != 2 {
		t.Fatalf("expected total 2 after tenant filter, got %d", resp.Total)
	}
	if resp.ByEnvironment["prod"] != 2 {
		t.Fatalf("expected 2 prod events, got %d", resp.ByEnvironment["prod"])
	}
}

func TestAuditEventsHandlerSummaryRequiresAuthorization(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	req := httptest.NewRequest(http.MethodGet, "/admin/audit-events?summary=true", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestAuditEventsHandlerSummaryIsUnauthorizedWithWrongToken(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	req := httptest.NewRequest(http.MethodGet, "/admin/audit-events?summary=true", nil)
	authorizeAdminRequest(req, "wrong-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestAuditEventsHandlerSummaryRejectsNonGetMethod(t *testing.T) {
	handler := newAuthenticatedAdminHandler(controlplane.NewService())
	req := httptest.NewRequest(http.MethodPost, "/admin/audit-events?summary=true", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusMethodNotAllowed, rr.Code, rr.Body.String())
	}
}

func TestAuditEventsHandlerSummarySupportsFilterCombination(t *testing.T) {
	recorder := audit.NewRecorder()
	recorder.RecordRelease("router", "tenant-a", "prod", "cfg_rel_prod_1", "release-bot", "first")
	recorder.RecordRelease("router", "tenant-a", "staging", "cfg_rel_staging", "release-bot", "second")
	recorder.RecordInheritanceDraft("router", "tenant-b", "prod", "dev", "cfg_rel_dev", "cfg_draft_prod", "architect", "seed prod")
	handler := newAuthenticatedAdminHandler(controlplane.NewService()).WithAuditReader(recorder)
	req := httptest.NewRequest(http.MethodGet, "/admin/audit-events?summary=true&tenant_id=tenant-a&environment=prod", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp auditSummaryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode summary response: %v", err)
	}
	if resp.Total != 1 {
		t.Fatalf("expected total 1, got %d", resp.Total)
	}
	if resp.ByType[audit.ControlPlaneEventTypeRelease] != 1 {
		t.Fatalf("expected 1 release event, got %d", resp.ByType[audit.ControlPlaneEventTypeRelease])
	}
	if resp.ByEnvironment["prod"] != 1 {
		t.Fatalf("expected 1 prod event, got %d", resp.ByEnvironment["prod"])
	}
}

func TestAuditEventsHandlerSummaryRejectsInvalidLimitAsListModeFallbackOnly(t *testing.T) {
	recorder := audit.NewRecorder()
	recorder.RecordRelease("router", "tenant-a", "prod", "cfg_rel_prod_1", "release-bot", "first")
	recorder.RecordRelease("router", "tenant-a", "prod", "cfg_rel_prod_2", "release-bot", "second")
	handler := newAuthenticatedAdminHandler(controlplane.NewService()).WithAuditReader(recorder)
	req := httptest.NewRequest(http.MethodGet, "/admin/audit-events?summary=true&limit=abc", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp auditSummaryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode summary response: %v", err)
	}
	if resp.Total != 2 {
		t.Fatalf("expected total 2, got %d", resp.Total)
	}
}

func TestAuditEventsHandlerSummaryReturnsOnlySummaryObject(t *testing.T) {
	recorder := audit.NewRecorder()
	recorder.RecordRelease("router", "tenant-a", "prod", "cfg_rel_prod", "release-bot", "promote")
	handler := newAuthenticatedAdminHandler(controlplane.NewService()).WithAuditReader(recorder)
	req := httptest.NewRequest(http.MethodGet, "/admin/audit-events?summary=true", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var raw map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to decode raw summary response: %v", err)
	}
	if _, ok := raw["total"]; !ok {
		t.Fatalf("expected total field in summary response")
	}
	if _, ok := raw["by_type"]; !ok {
		t.Fatalf("expected by_type field in summary response")
	}
	if _, ok := raw["by_environment"]; !ok {
		t.Fatalf("expected by_environment field in summary response")
	}
	if _, ok := raw["items"]; ok {
		t.Fatalf("did not expect list payload field in summary response")
	}
}

func TestAuditEventsHandlerSummaryIncludesOperabilityMetadata(t *testing.T) {
	recorder := audit.NewRecorder()
	recorder.RecordRelease("router", "tenant-a", "prod", "cfg_rel_prod_1", "release-bot", "first")
	recorder.RecordRelease("router", "tenant-a", "prod", "cfg_rel_prod_2", "release-bot", "second")
	recorder.RecordRelease("router", "tenant-b", "staging", "cfg_rel_staging", "release-bot", "third")
	handler := newAuthenticatedAdminHandler(controlplane.NewService()).WithAuditReader(recorder)
	req := httptest.NewRequest(http.MethodGet, "/admin/audit-events?summary=true&tenant_id=tenant-a&environment=prod", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp auditSummaryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode summary response: %v", err)
	}
	if resp.Total != 2 {
		t.Fatalf("expected filtered total 2, got %d", resp.Total)
	}
	if resp.ScannedTotal != 3 {
		t.Fatalf("expected scanned total 3, got %d", resp.ScannedTotal)
	}
	if resp.TenantID != "tenant-a" {
		t.Fatalf("expected tenant metadata tenant-a, got %q", resp.TenantID)
	}
	if resp.Environment != "prod" {
		t.Fatalf("expected environment metadata prod, got %q", resp.Environment)
	}
	if resp.LatestAt == "" || resp.OldestAt == "" {
		t.Fatalf("expected latest_at and oldest_at to be non-empty, got latest=%q oldest=%q", resp.LatestAt, resp.OldestAt)
	}
	latest := parseRFC3339TimeForTest(t, resp.LatestAt)
	oldest := parseRFC3339TimeForTest(t, resp.OldestAt)
	if oldest.After(latest) {
		t.Fatalf("expected oldest_at <= latest_at, got oldest=%s latest=%s", resp.OldestAt, resp.LatestAt)
	}
}

func TestRuntimeEventsHandlerSummaryIncludesOperabilityMetadata(t *testing.T) {
	publisher := runtime.NewPublisher()
	publisher.PublishIfReleased(controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod_1",
		Source:      controlplane.ConfigStatusReleased,
		CreatedAt:   time.Date(2026, 3, 24, 20, 0, 0, 0, time.UTC),
	})
	publisher.PublishIfReleased(controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "prod",
		Scope:       "tenant",
		Version:     "cfg_rel_prod_2",
		Source:      controlplane.ConfigStatusReleased,
		CreatedAt:   time.Date(2026, 3, 24, 20, 1, 0, 0, time.UTC),
	})
	publisher.PublishIfReleased(controlplane.ConfigVersion{
		Module:      "router",
		TenantID:    "tenant-b",
		Environment: "staging",
		Scope:       "tenant",
		Version:     "cfg_rel_staging",
		Source:      controlplane.ConfigStatusReleased,
		CreatedAt:   time.Date(2026, 3, 24, 20, 2, 0, 0, time.UTC),
	})
	handler := newAuthenticatedAdminHandler(controlplane.NewService()).WithRuntimeReader(publisher)
	req := httptest.NewRequest(http.MethodGet, "/admin/runtime-events?summary=true&tenant_id=tenant-a&environment=prod", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp auditSummaryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode summary response: %v", err)
	}
	if resp.Total != 2 {
		t.Fatalf("expected filtered total 2, got %d", resp.Total)
	}
	if resp.ScannedTotal != 3 {
		t.Fatalf("expected scanned total 3, got %d", resp.ScannedTotal)
	}
	if resp.TenantID != "tenant-a" {
		t.Fatalf("expected tenant metadata tenant-a, got %q", resp.TenantID)
	}
	if resp.Environment != "prod" {
		t.Fatalf("expected environment metadata prod, got %q", resp.Environment)
	}
	if resp.LatestAt != "2026-03-24T20:01:00Z" {
		t.Fatalf("expected latest_at 2026-03-24T20:01:00Z, got %q", resp.LatestAt)
	}
	if resp.OldestAt != "2026-03-24T20:00:00Z" {
		t.Fatalf("expected oldest_at 2026-03-24T20:00:00Z, got %q", resp.OldestAt)
	}
}

func TestListVersionsHandlerSupportsSummaryView(t *testing.T) {
	svc := controlplane.NewService()
	_, err := svc.CreateVersion(context.Background(), controlplane.CreateVersionInput{
		Module:      "router",
		TenantID:    "tenant-a",
		Environment: "staging",
		Scope:       "tenant",
		Version:     "cfg_rel_staging",
		Status:      controlplane.ConfigStatusReleased,
		Summary:     "staging released",
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}
	_, err = svc.CreateInheritanceDraft(context.Background(), controlplane.CreateInheritanceDraftInput{
		Module:            "router",
		TenantID:          "tenant-a",
		Scope:             "tenant",
		SourceEnvironment: "staging",
		TargetEnvironment: "prod",
		Reason:            "seed prod",
		Actor:             "architect",
	})
	if err != nil {
		t.Fatalf("CreateInheritanceDraft returned error: %v", err)
	}
	handler := newAuthenticatedAdminHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/admin/config-versions?module=router&tenant_id=tenant-a&scope=tenant&summary=true", nil)
	authorizeAdminRequest(req, testAdminToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp versionSummaryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode summary response: %v", err)
	}
	if resp.Total != 2 {
		t.Fatalf("expected total 2, got %d", resp.Total)
	}
	if resp.ByStatus[controlplane.ConfigStatusReleased] != 1 {
		t.Fatalf("expected 1 released version, got %d", resp.ByStatus[controlplane.ConfigStatusReleased])
	}
	if resp.ByStatus[controlplane.ConfigStatusDraft] != 1 {
		t.Fatalf("expected 1 draft version, got %d", resp.ByStatus[controlplane.ConfigStatusDraft])
	}
	if resp.ByEnvironment["staging"] != 1 || resp.ByEnvironment["prod"] != 1 {
		t.Fatalf("expected env distribution staging=1 prod=1, got %+v", resp.ByEnvironment)
	}
	if resp.BySource[controlplane.ConfigStatusReleased] != 1 {
		t.Fatalf("expected source released count 1, got %d", resp.BySource[controlplane.ConfigStatusReleased])
	}
	if resp.BySource[controlplane.SourceTypeInheritance] != 1 {
		t.Fatalf("expected source inheritance count 1, got %d", resp.BySource[controlplane.SourceTypeInheritance])
	}
	if resp.LatestAt == "" || resp.OldestAt == "" {
		t.Fatalf("expected latest_at and oldest_at to be non-empty")
	}
	latest := parseRFC3339TimeForTest(t, resp.LatestAt)
	oldest := parseRFC3339TimeForTest(t, resp.OldestAt)
	if oldest.After(latest) {
		t.Fatalf("expected oldest_at <= latest_at, got oldest=%s latest=%s", resp.OldestAt, resp.LatestAt)
	}
}
