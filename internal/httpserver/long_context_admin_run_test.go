package httpserver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminLongContextRunDisabled(t *testing.T) {
	srv := &Server{longContextHandler: NewLongContextHandler(nil, false, 1024)}
	body := bytes.NewBufferString(`{"task_id":"lct_x","tenant_id":"t","channel":"c","model":"m"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/long-context/run", body)
	rec := httptest.NewRecorder()
	srv.adminLongContextRun(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAdminLongContextRunMissingParams(t *testing.T) {
	srv := &Server{longContextHandler: NewLongContextHandler(nil, true, 1024)}
	body := bytes.NewBufferString(`{"task_id":"lct_x"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/long-context/run", body)
	rec := httptest.NewRecorder()
	srv.adminLongContextRun(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAdminLongContextRunRepoUnavailable(t *testing.T) {
	srv := &Server{longContextHandler: NewLongContextHandler(nil, true, 1024)}
	body := bytes.NewBufferString(`{"task_id":"lct_x","tenant_id":"t","channel":"c","model":"m"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/long-context/run", body)
	rec := httptest.NewRecorder()
	srv.adminLongContextRun(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", rec.Code)
	}
}
