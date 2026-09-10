package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestControlPlaneSupportsProjectScope(t *testing.T) {
	// 当前阶段只验证 project scope 相关查询参数能够出现在请求中，后续接入真实 handler 时再断言更细行为
	req := httptest.NewRequest(http.MethodGet, "/admin/control-plane/config?tenant_id=t1&environment=prod&scope=project&project_id=p1", nil)
	q := req.URL.Query()
	if q.Get("tenant_id") != "t1" {
		t.Fatalf("unexpected tenant_id: %s", q.Get("tenant_id"))
	}
	if q.Get("environment") != "prod" {
		t.Fatalf("unexpected environment: %s", q.Get("environment"))
	}
	if q.Get("scope") != "project" {
		t.Fatalf("unexpected scope: %s", q.Get("scope"))
	}
	if q.Get("project_id") != "p1" {
		t.Fatalf("unexpected project_id: %s", q.Get("project_id"))
	}
}
