package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/memory"
)

type memoryAdminStoreStub struct {
	listCandidateFactsResp []memory.CandidateFact
	listCandidateFactsErr  error
	listProjectFactsResp   []memory.ProjectFact
	listProjectFactsErr    error

	listCandidateTenantID string
	listCandidateUserID   string
	listCandidateStatus   string

	listProjectTenantID string
	listProjectUserID   string
	listProjectStatus   string

	confirmTenantID string
	confirmUserID   string
	confirmFactKey  string
	confirmResp     *memory.CandidateFact
	confirmErr      error

	rejectTenantID string
	rejectUserID   string
	rejectFactKey  string
	rejectResp     *memory.CandidateFact
	rejectErr      error

	promoteTenantID string
	promoteUserID   string
	promoteFactKey  string
	promoteResp     *memory.CandidateFact
	promoteErr      error
}

func (s *memoryAdminStoreStub) ListCandidateFacts(_ context.Context, tenantID, userID, status string) ([]memory.CandidateFact, error) {
	s.listCandidateTenantID = tenantID
	s.listCandidateUserID = userID
	s.listCandidateStatus = status
	return s.listCandidateFactsResp, s.listCandidateFactsErr
}

func (s *memoryAdminStoreStub) ListProjectFacts(_ context.Context, tenantID, userID, status string) ([]memory.ProjectFact, error) {
	s.listProjectTenantID = tenantID
	s.listProjectUserID = userID
	s.listProjectStatus = status
	return s.listProjectFactsResp, s.listProjectFactsErr
}

func (s *memoryAdminStoreStub) ConfirmCandidateFact(_ context.Context, tenantID, userID, factKey string) (*memory.CandidateFact, error) {
	s.confirmTenantID = tenantID
	s.confirmUserID = userID
	s.confirmFactKey = factKey
	return s.confirmResp, s.confirmErr
}

func (s *memoryAdminStoreStub) RejectCandidateFact(_ context.Context, tenantID, userID, factKey string) (*memory.CandidateFact, error) {
	s.rejectTenantID = tenantID
	s.rejectUserID = userID
	s.rejectFactKey = factKey
	return s.rejectResp, s.rejectErr
}

func (s *memoryAdminStoreStub) PromoteCandidateFact(_ context.Context, tenantID, userID, factKey string) (*memory.CandidateFact, error) {
	s.promoteTenantID = tenantID
	s.promoteUserID = userID
	s.promoteFactKey = factKey
	return s.promoteResp, s.promoteErr
}

func TestMemoryAdminHandlerEndpoints(t *testing.T) {
	store := &memoryAdminStoreStub{
		listCandidateFactsResp: []memory.CandidateFact{{ID: 1, TenantID: "t1", UserID: "u1", Key: "repo", Value: "mono", Status: "pending"}},
		listProjectFactsResp:   []memory.ProjectFact{{ID: 2, TenantID: "t1", UserID: "u1", Key: "stack", Value: "go", Status: "superseded"}},
		confirmResp:            &memory.CandidateFact{ID: 1, TenantID: "t1", UserID: "u1", Key: "repo", Value: "mono", Status: "confirmed"},
		rejectResp:             &memory.CandidateFact{ID: 1, TenantID: "t1", UserID: "u1", Key: "repo", Value: "mono", Status: "rejected"},
		promoteResp:            &memory.CandidateFact{ID: 1, TenantID: "t1", UserID: "u1", Key: "repo", Value: "mono", Status: "promoted"},
	}
	h := NewMemoryAdminHandler(store)

	candidateListReq := httptest.NewRequest(http.MethodGet, "/admin/memory/candidate-facts?tenant_id=t1&user_id=u1&status=pending", nil)
	candidateListResp := httptest.NewRecorder()
	h.ServeHTTP(candidateListResp, candidateListReq)
	if candidateListResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", candidateListResp.Code, candidateListResp.Body.String())
	}
	if store.listCandidateTenantID != "t1" || store.listCandidateUserID != "u1" || store.listCandidateStatus != "pending" {
		t.Fatalf("unexpected candidate list args: tenant=%s user=%s status=%s", store.listCandidateTenantID, store.listCandidateUserID, store.listCandidateStatus)
	}

	projectListReq := httptest.NewRequest(http.MethodGet, "/admin/memory/project-facts?tenant_id=t1&user_id=u1&status=superseded", nil)
	projectListResp := httptest.NewRecorder()
	h.ServeHTTP(projectListResp, projectListReq)
	if projectListResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", projectListResp.Code, projectListResp.Body.String())
	}
	if store.listProjectTenantID != "t1" || store.listProjectUserID != "u1" || store.listProjectStatus != "superseded" {
		t.Fatalf("unexpected project list args: tenant=%s user=%s status=%s", store.listProjectTenantID, store.listProjectUserID, store.listProjectStatus)
	}

	actions := []struct {
		path        string
		wantStatus  string
		gotTenantID func() string
		gotUserID   func() string
		gotFactKey  func() string
	}{
		{path: "/admin/memory/candidate-facts/repo/confirm", wantStatus: "confirmed", gotTenantID: func() string { return store.confirmTenantID }, gotUserID: func() string { return store.confirmUserID }, gotFactKey: func() string { return store.confirmFactKey }},
		{path: "/admin/memory/candidate-facts/repo/reject", wantStatus: "rejected", gotTenantID: func() string { return store.rejectTenantID }, gotUserID: func() string { return store.rejectUserID }, gotFactKey: func() string { return store.rejectFactKey }},
		{path: "/admin/memory/candidate-facts/repo/promote", wantStatus: "promoted", gotTenantID: func() string { return store.promoteTenantID }, gotUserID: func() string { return store.promoteUserID }, gotFactKey: func() string { return store.promoteFactKey }},
	}

	for _, tc := range actions {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewBufferString(`{"tenant_id":"t1","user_id":"u1"}`))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
			}
			if tc.gotTenantID() != "t1" || tc.gotUserID() != "u1" || tc.gotFactKey() != "repo" {
				t.Fatalf("unexpected action args: tenant=%s user=%s key=%s", tc.gotTenantID(), tc.gotUserID(), tc.gotFactKey())
			}

			var fact memory.CandidateFact
			if err := json.Unmarshal(rr.Body.Bytes(), &fact); err != nil {
				t.Fatalf("decode response failed: %v", err)
			}
			if fact.Status != tc.wantStatus {
				t.Fatalf("expected status %s, got %s", tc.wantStatus, fact.Status)
			}
		})
	}
}

func TestMemoryAdminHandlerBatchActions(t *testing.T) {
	store := &memoryAdminStoreStub{
		confirmResp: &memory.CandidateFact{ID: 1, TenantID: "t1", UserID: "u1", Key: "repo", Value: "mono", Status: "confirmed"},
		rejectResp:  &memory.CandidateFact{ID: 2, TenantID: "t1", UserID: "u1", Key: "stack", Value: "go", Status: "rejected"},
		promoteResp: &memory.CandidateFact{ID: 3, TenantID: "t1", UserID: "u1", Key: "cache", Value: "redis", Status: "promoted"},
	}
	h := NewMemoryAdminHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/admin/memory/candidate-facts/actions/confirm", bytes.NewBufferString(`{"items":[{"tenant_id":"t1","user_id":"u1","fact_key":"repo"},{"tenant_id":"t1","user_id":"u1","fact_key":"repo"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var response candidateFactBatchActionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if response.Action != "confirm" {
		t.Fatalf("expected action confirm, got %q", response.Action)
	}
	if response.SuccessCount != 1 || response.FailureCount != 0 {
		t.Fatalf("unexpected batch counts: success=%d failure=%d", response.SuccessCount, response.FailureCount)
	}
	if len(response.Results) != 1 {
		t.Fatalf("expected deduped single result, got %d", len(response.Results))
	}
	if response.Results[0].Fact == nil || response.Results[0].Status != "confirmed" {
		t.Fatalf("expected successful fact response, got %#v", response.Results[0])
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/memory/candidate-facts/actions/reject", bytes.NewBufferString(`{"items":[{"tenant_id":"t1","user_id":"u1","fact_key":"stack"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for reject batch, got %d body=%s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode reject response failed: %v", err)
	}
	if response.Action != "reject" || response.SuccessCount != 1 || response.Results[0].Status != "rejected" {
		t.Fatalf("unexpected reject batch response: %#v", response)
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/memory/candidate-facts/actions/promote", bytes.NewBufferString(`{"items":[{"tenant_id":"t1","user_id":"u1","fact_key":"cache"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for promote batch, got %d body=%s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode promote response failed: %v", err)
	}
	if response.Action != "promote" || response.SuccessCount != 1 || response.Results[0].Status != "promoted" {
		t.Fatalf("unexpected promote batch response: %#v", response)
	}

	store.confirmResp = nil
	store.confirmErr = memory.ErrCandidateFactNotFound
	req = httptest.NewRequest(http.MethodPost, "/admin/memory/candidate-facts/actions/confirm", bytes.NewBufferString(`{"items":[{"tenant_id":"t1","user_id":"u1","fact_key":"missing"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error response failed: %v", err)
	}
	if response.SuccessCount != 0 || response.FailureCount != 1 {
		t.Fatalf("unexpected batch counts after failure: success=%d failure=%d", response.SuccessCount, response.FailureCount)
	}
	if response.Results[0].Error == nil || response.Results[0].Error.Message == "" {
		t.Fatalf("expected batch error payload, got %#v", response.Results[0])
	}
}

func TestServerMountsMemoryAdminRoutes(t *testing.T) {
	store := &memoryAdminStoreStub{
		listCandidateFactsResp: []memory.CandidateFact{{ID: 1, TenantID: "t1", UserID: "u1", Key: "repo", Value: "mono", Status: "pending"}},
		confirmResp:            &memory.CandidateFact{ID: 1, TenantID: "t1", UserID: "u1", Key: "repo", Value: "mono", Status: "confirmed"},
	}
	s := New(config.Config{AdminAPIKey: "k"}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		WithMemoryAdminHandler(NewMemoryAdminHandler(store))

	unauthReq := httptest.NewRequest(http.MethodGet, "/admin/memory/candidate-facts?tenant_id=t1&user_id=u1", nil)
	unauthResp := httptest.NewRecorder()
	s.Handler().ServeHTTP(unauthResp, unauthReq)
	if unauthResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", unauthResp.Code)
	}

	unauthBatchReq := httptest.NewRequest(http.MethodPost, "/admin/memory/candidate-facts/actions/confirm", bytes.NewBufferString(`{"items":[{"tenant_id":"t1","user_id":"u1","fact_key":"repo"}]}`))
	unauthBatchReq.Header.Set("Content-Type", "application/json")
	unauthBatchResp := httptest.NewRecorder()
	s.Handler().ServeHTTP(unauthBatchResp, unauthBatchReq)
	if unauthBatchResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthorized batch route, got %d", unauthBatchResp.Code)
	}

	authReq := httptest.NewRequest(http.MethodGet, "/admin/memory/candidate-facts?tenant_id=t1&user_id=u1", nil)
	authReq.Header.Set("X-Admin-Key", "k")
	authResp := httptest.NewRecorder()
	s.Handler().ServeHTTP(authResp, authReq)
	if authResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", authResp.Code, authResp.Body.String())
	}

	batchReq := httptest.NewRequest(http.MethodPost, "/admin/memory/candidate-facts/actions/confirm", bytes.NewBufferString(`{"items":[{"tenant_id":"t1","user_id":"u1","fact_key":"repo"}]}`))
	batchReq.Header.Set("Content-Type", "application/json")
	batchReq.Header.Set("X-Admin-Key", "k")
	batchResp := httptest.NewRecorder()
	s.Handler().ServeHTTP(batchResp, batchReq)
	if batchResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for batch route, got %d body=%s", batchResp.Code, batchResp.Body.String())
	}
}

func TestMemoryAdminHandlerValidationAndErrors(t *testing.T) {
	t.Run("candidate list method not allowed", func(t *testing.T) {
		h := NewMemoryAdminHandler(&memoryAdminStoreStub{})
		req := httptest.NewRequest(http.MethodPost, "/admin/memory/candidate-facts", bytes.NewBufferString(`{}`))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("project list method not allowed", func(t *testing.T) {
		h := NewMemoryAdminHandler(&memoryAdminStoreStub{})
		req := httptest.NewRequest(http.MethodPost, "/admin/memory/project-facts", bytes.NewBufferString(`{}`))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("candidate action method not allowed", func(t *testing.T) {
		h := NewMemoryAdminHandler(&memoryAdminStoreStub{})
		req := httptest.NewRequest(http.MethodGet, "/admin/memory/candidate-facts/repo/confirm", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("candidate batch action method not allowed", func(t *testing.T) {
		h := NewMemoryAdminHandler(&memoryAdminStoreStub{})
		req := httptest.NewRequest(http.MethodGet, "/admin/memory/candidate-facts/actions/confirm", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("candidate action invalid json", func(t *testing.T) {
		h := NewMemoryAdminHandler(&memoryAdminStoreStub{})
		req := httptest.NewRequest(http.MethodPost, "/admin/memory/candidate-facts/repo/confirm", bytes.NewBufferString(`{"tenant_id":"t1","user_id":`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("candidate batch action invalid json", func(t *testing.T) {
		h := NewMemoryAdminHandler(&memoryAdminStoreStub{})
		req := httptest.NewRequest(http.MethodPost, "/admin/memory/candidate-facts/actions/confirm", bytes.NewBufferString(`{"items":`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("candidate action missing user id", func(t *testing.T) {
		h := NewMemoryAdminHandler(&memoryAdminStoreStub{})
		req := httptest.NewRequest(http.MethodPost, "/admin/memory/candidate-facts/repo/confirm", bytes.NewBufferString(`{"tenant_id":"t1"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("candidate batch action missing items", func(t *testing.T) {
		h := NewMemoryAdminHandler(&memoryAdminStoreStub{})
		req := httptest.NewRequest(http.MethodPost, "/admin/memory/candidate-facts/actions/confirm", bytes.NewBufferString(`{"items":[]}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("candidate batch action missing fact key", func(t *testing.T) {
		h := NewMemoryAdminHandler(&memoryAdminStoreStub{})
		req := httptest.NewRequest(http.MethodPost, "/admin/memory/candidate-facts/actions/confirm", bytes.NewBufferString(`{"items":[{"tenant_id":"t1","user_id":"u1"}]}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("candidate action route not found", func(t *testing.T) {
		h := NewMemoryAdminHandler(&memoryAdminStoreStub{})
		req := httptest.NewRequest(http.MethodPost, "/admin/memory/candidate-facts/repo", bytes.NewBufferString(`{"tenant_id":"t1","user_id":"u1"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("candidate batch action route not found", func(t *testing.T) {
		h := NewMemoryAdminHandler(&memoryAdminStoreStub{})
		req := httptest.NewRequest(http.MethodPost, "/admin/memory/candidate-facts/actions", bytes.NewBufferString(`{"items":[{"tenant_id":"t1","user_id":"u1","fact_key":"repo"}]}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("candidate action maps transition error to bad request", func(t *testing.T) {
		store := &memoryAdminStoreStub{confirmErr: memory.ErrInvalidCandidateFactTransition}
		h := NewMemoryAdminHandler(store)
		req := httptest.NewRequest(http.MethodPost, "/admin/memory/candidate-facts/repo/confirm", bytes.NewBufferString(`{"tenant_id":"t1","user_id":"u1"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("candidate action maps not found to 404", func(t *testing.T) {
		store := &memoryAdminStoreStub{rejectErr: memory.ErrCandidateFactNotFound}
		h := NewMemoryAdminHandler(store)
		req := httptest.NewRequest(http.MethodPost, "/admin/memory/candidate-facts/repo/reject", bytes.NewBufferString(`{"tenant_id":"t1","user_id":"u1"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("candidate action maps unknown error to 500", func(t *testing.T) {
		store := &memoryAdminStoreStub{promoteErr: errors.New("db down")}
		h := NewMemoryAdminHandler(store)
		req := httptest.NewRequest(http.MethodPost, "/admin/memory/candidate-facts/repo/promote", bytes.NewBufferString(`{"tenant_id":"t1","user_id":"u1"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d body=%s", rr.Code, rr.Body.String())
		}
	})
}
