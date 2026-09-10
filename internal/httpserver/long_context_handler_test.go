package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLongContextHandlerDisabled(t *testing.T) {
	h := NewLongContextHandler(nil, false, 1024)
	req := httptest.NewRequest(http.MethodPost, "/v1/long-context/tasks", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}
