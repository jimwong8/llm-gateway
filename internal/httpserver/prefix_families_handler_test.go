package httpserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"llm-gateway/gateway/internal/billing"
	"llm-gateway/gateway/internal/config"
)

func TestAdminObservabilityPrefixFamilies_ServesList(t *testing.T) {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_DSN not set")
	}
	store, err := billing.NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	srv := New(config.Config{AdminAPIKey: "ok0115ok"}, nil, nil, nil, nil, nil, nil, store, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/observability/prefix-families?limit=5", nil)
	req.Header.Set("Authorization", "Bearer ok0115ok")
	rec := httptest.NewRecorder()
	srv.adminObservabilityPrefixFamilies(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "prefix_family") {
		t.Fatalf("response missing prefix_family field: %s", rec.Body.String())
	}
}
