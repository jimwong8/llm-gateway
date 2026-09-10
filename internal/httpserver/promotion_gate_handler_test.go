package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPromotionGateRouteShape(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/admin/control-plane/config/promote?source_environment=dev&target_environment=staging&module=policy&tenant_id=t1&version=v1", nil)
	q := req.URL.Query()
	if q.Get("source_environment") != "dev" {
		t.Fatalf("unexpected source environment: %s", q.Get("source_environment"))
	}
	if q.Get("target_environment") != "staging" {
		t.Fatalf("unexpected target environment: %s", q.Get("target_environment"))
	}
	if q.Get("module") != "policy" {
		t.Fatalf("unexpected module: %s", q.Get("module"))
	}
	if q.Get("tenant_id") != "t1" {
		t.Fatalf("unexpected tenant_id: %s", q.Get("tenant_id"))
	}
	if q.Get("version") != "v1" {
		t.Fatalf("unexpected version: %s", q.Get("version"))
	}
}
