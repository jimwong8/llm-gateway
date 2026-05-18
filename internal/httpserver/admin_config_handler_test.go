package httpserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"llm-gateway/gateway/internal/adminconfig"
	"llm-gateway/gateway/internal/config"
)

func TestAdminConfigHandlerSiteConfig(t *testing.T) {
	store := adminconfig.NewStore()
	h := NewAdminConfigHandler(store)

	t.Run("get default site config", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/config/site", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if resp["site_name"] != "LLM Gateway" {
			t.Fatalf("expected site_name LLM Gateway, got %v", resp["site_name"])
		}
		if resp["jwt_secret_configured"] != false {
			t.Fatalf("expected jwt_secret_configured false, got %v", resp["jwt_secret_configured"])
		}
	})

	t.Run("update site config", func(t *testing.T) {
		body := `{"site_name":"My Gateway","smtp_host":"smtp.example.com","smtp_port":465,"allow_registration":false,"default_user_role":"admin","default_user_quota":500000,"updated_by":"test"}`
		req := httptest.NewRequest(http.MethodPut, "/admin/config/site", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if resp["site_name"] != "My Gateway" {
			t.Fatalf("expected site_name My Gateway, got %v", resp["site_name"])
		}
		if resp["allow_registration"] != false {
			t.Fatalf("expected allow_registration false, got %v", resp["allow_registration"])
		}
		if resp["default_user_role"] != "admin" {
			t.Fatalf("expected default_user_role admin, got %v", resp["default_user_role"])
		}
		if resp["updated_by"] != "test" {
			t.Fatalf("expected updated_by test, got %v", resp["updated_by"])
		}
	})

	t.Run("update site config partial", func(t *testing.T) {
		h2 := NewAdminConfigHandler(adminconfig.NewStore())
		h2.store.UpdateSiteConfig(adminconfig.SiteConfig{SiteName: "Original", LogoURL: "old.png", DefaultUserRole: "user"}, "a")

		body := `{"logo_url":"new.png","updated_by":"partial"}`
		req := httptest.NewRequest(http.MethodPut, "/admin/config/site", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h2.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if resp["site_name"] != "Original" {
			t.Fatalf("expected site_name to stay Original, got %v", resp["site_name"])
		}
		if resp["logo_url"] != "new.png" {
			t.Fatalf("expected logo_url new.png, got %v", resp["logo_url"])
		}
	})

	t.Run("invalid json returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/admin/config/site", bytes.NewBufferString(`{invalid`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/config/site", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405, got %d body=%s", rr.Code, rr.Body.String())
		}

		req = httptest.NewRequest(http.MethodDelete, "/admin/config/site", nil)
		rr = httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405, got %d body=%s", rr.Code, rr.Body.String())
		}
	})
}

func TestAdminConfigHandlerJWTRotate(t *testing.T) {
	store := adminconfig.NewStore()
	h := NewAdminConfigHandler(store)

	t.Run("rotate jwt secret", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/config/jwt/rotate", bytes.NewBufferString(`{"updated_by":"test"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		secret, ok := resp["jwt_secret"].(string)
		if !ok || len(secret) == 0 {
			t.Fatalf("expected non-empty jwt_secret, got %v", resp["jwt_secret"])
		}
		if len(secret) != 64 {
			t.Fatalf("expected 64-char hex secret, got %d chars", len(secret))
		}
		if resp["jwt_secret_rotated_at"] == nil || resp["jwt_secret_rotated_at"] == "" {
			t.Fatalf("expected jwt_secret_rotated_at, got nil")
		}
	})

	t.Run("jwt secret appears as configured after rotation", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/config/site", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if resp["jwt_secret_configured"] != true {
			t.Fatalf("expected jwt_secret_configured true, got %v", resp["jwt_secret_configured"])
		}
		if _, exists := resp["jwt_secret"]; exists {
			t.Fatalf("jwt_secret should not be returned in site config response")
		}
	})

	t.Run("rotate method not allowed for non-post", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/config/jwt/rotate", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("rotate with empty body defaults to admin", func(t *testing.T) {
		store2 := adminconfig.NewStore()
		h2 := NewAdminConfigHandler(store2)
		req := httptest.NewRequest(http.MethodPost, "/admin/config/jwt/rotate", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h2.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("rotate with invalid body defaults to admin", func(t *testing.T) {
		h2 := NewAdminConfigHandler(adminconfig.NewStore())
		req := httptest.NewRequest(http.MethodPost, "/admin/config/jwt/rotate", bytes.NewBufferString(`{invalid`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h2.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
		}
	})
}

func TestAdminConfigHandlerVersions(t *testing.T) {
	store := adminconfig.NewStore()
	h := NewAdminConfigHandler(store)

	t.Run("list versions returns empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/config/versions", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		data, ok := resp["data"].([]any)
		if !ok || len(data) != 0 {
			t.Fatalf("expected empty data, got %v", resp["data"])
		}
	})

	t.Run("create snapshot version", func(t *testing.T) {
		body := `{"version":"v1","config_snapshot":"{\"key\":\"value\"}","notes":"test snapshot","created_by":"tester"}`
		req := httptest.NewRequest(http.MethodPost, "/admin/config/versions", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d body=%s", rr.Code, rr.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if resp["version"] != "v1" {
			t.Fatalf("expected version v1, got %v", resp["version"])
		}
		if resp["status"] != "draft" {
			t.Fatalf("expected status draft, got %v", resp["status"])
		}
		if resp["created_by"] != "tester" {
			t.Fatalf("expected created_by tester, got %v", resp["created_by"])
		}
		id, ok := resp["id"]
		if !ok || id.(float64) <= 0 {
			t.Fatalf("expected valid id, got %v", id)
		}
	})

	t.Run("create snapshot auto-generates version", func(t *testing.T) {
		body := `{"config_snapshot":"{}","notes":"auto version"}`
		req := httptest.NewRequest(http.MethodPost, "/admin/config/versions", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d body=%s", rr.Code, rr.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		version, ok := resp["version"].(string)
		if !ok || !strings.HasPrefix(version, "v") {
			t.Fatalf("expected auto-generated version starting with v, got %v", resp["version"])
		}
	})

	t.Run("create snapshot invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/config/versions", bytes.NewBufferString(`{invalid`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("list versions returns created snapshots", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/config/versions", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		data, ok := resp["data"].([]any)
		if !ok || len(data) < 2 {
			t.Fatalf("expected at least 2 snapshots, got %d", len(data))
		}
	})

	t.Run("list versions method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/admin/config/versions", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405, got %d body=%s", rr.Code, rr.Body.String())
		}
	})
}

func TestAdminConfigHandlerVersionByID(t *testing.T) {
	store := adminconfig.NewStore()
	store.CreateSnapshot("v-test", `{"key":"val"}`, "test notes", "admin")
	h := NewAdminConfigHandler(store)

	t.Run("get snapshot by id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/config/versions/1", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if resp["version"] != "v-test" {
			t.Fatalf("expected version v-test, got %v", resp["version"])
		}
		if resp["config_snapshot"] != `{"key":"val"}` {
			t.Fatalf("expected config_snapshot, got %v", resp["config_snapshot"])
		}
	})

	t.Run("get non-existent snapshot", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/config/versions/9999", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("get snapshot with invalid id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/config/versions/abc", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
		}
	})
}

func TestAdminConfigHandlerPublishAndRollback(t *testing.T) {
	store := adminconfig.NewStore()
	store.CreateSnapshot("v-pub", "{}", "publish test", "admin")
	h := NewAdminConfigHandler(store)

	t.Run("publish draft snapshot", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/config/versions/1/publish", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if resp["status"] != "published" {
			t.Fatalf("expected status published, got %v", resp["status"])
		}
		if resp["published_at"] == nil || resp["published_at"] == "" {
			t.Fatalf("expected published_at, got nil")
		}
	})

	t.Run("publish already published returns error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/config/versions/1/publish", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("rollback published snapshot", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/config/versions/1/rollback", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if resp["status"] != "rolled_back" {
			t.Fatalf("expected status rolled_back, got %v", resp["status"])
		}
		if resp["rolled_back_at"] == nil || resp["rolled_back_at"] == "" {
			t.Fatalf("expected rolled_back_at, got nil")
		}
	})

	t.Run("rollback already rolled back returns error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/config/versions/1/rollback", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("publish non-existent snapshot", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/config/versions/9999/publish", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("publish method not allowed for get", func(t *testing.T) {
		store2 := adminconfig.NewStore()
		store2.CreateSnapshot("v2", "{}", "", "admin")
		h2 := NewAdminConfigHandler(store2)
		req := httptest.NewRequest(http.MethodGet, "/admin/config/versions/1/publish", nil)
		rr := httptest.NewRecorder()
		h2.ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405, got %d body=%s", rr.Code, rr.Body.String())
		}
	})
}

func TestAdminConfigHandlerExportImport(t *testing.T) {
	store := adminconfig.NewStore()
	store.CreateSnapshot("v-export", `{"key":"val"}`, "export test", "admin")
	h := NewAdminConfigHandler(store)

	t.Run("export snapshots", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/config/versions/export", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		data, ok := resp["data"].([]any)
		if !ok || len(data) != 1 {
			t.Fatalf("expected 1 snapshot, got %d", len(data))
		}
	})

	t.Run("export method not allowed for post", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/config/versions/export", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("import snapshots", func(t *testing.T) {
		store2 := adminconfig.NewStore()
		h2 := NewAdminConfigHandler(store2)
		body := `{"data":[{"version":"v-imported","config_snapshot":"{}","notes":"imported","created_by":"importer","status":"draft"}]}`
		req := httptest.NewRequest(http.MethodPost, "/admin/config/versions/import", bytes.NewBufferString(body))
		rr := httptest.NewRecorder()
		h2.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if resp["imported"] != float64(1) {
			t.Fatalf("expected imported 1, got %v", resp["imported"])
		}
	})

	t.Run("import empty data returns error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/config/versions/import", bytes.NewBufferString(`{"data":[]}`))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("import invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/config/versions/import", bytes.NewBufferString(`{invalid`))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("import method not allowed for get", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/config/versions/import", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405, got %d body=%s", rr.Code, rr.Body.String())
		}
	})
}

func TestAdminConfigHandlerNotFoundRoutes(t *testing.T) {
	store := adminconfig.NewStore()
	h := NewAdminConfigHandler(store)

	t.Run("unknown path returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/config/unknown", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("trailing slash matches versions list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/config/versions/", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("unknown version action", func(t *testing.T) {
		h2 := NewAdminConfigHandler(adminconfig.NewStore())
		req := httptest.NewRequest(http.MethodPost, "/admin/config/versions/1/unknown", nil)
		rr := httptest.NewRecorder()
		h2.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
		}
	})
}

func TestServerMountsAdminConfigRoutes(t *testing.T) {
	store := adminconfig.NewStore()
	h := NewAdminConfigHandler(store)
	s := New(config.Config{AdminAPIKey: "k"}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		WithAdminConfigHandler(h)

	t.Run("unauthorized access returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/config/site", nil)
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("authorized site config returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/config/site", nil)
		req.Header.Set("X-Admin-Key", "k")
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("authorized jwt rotate returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/config/jwt/rotate", bytes.NewBufferString(`{}`))
		req.Header.Set("X-Admin-Key", "k")
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("authorized versions list returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/config/versions", nil)
		req.Header.Set("X-Admin-Key", "k")
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("unauthorized versions returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/config/versions", nil)
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("nil handler does not mount routes", func(t *testing.T) {
		s2 := New(config.Config{AdminAPIKey: "k"}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		req := httptest.NewRequest(http.MethodGet, "/admin/config/site", nil)
		req.Header.Set("X-Admin-Key", "k")
		rr := httptest.NewRecorder()
		s2.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404 (routes not mounted), got %d", rr.Code)
		}
	})
}

func TestAdminConfigHandlerRoundTripPublishRollbackLifecycle(t *testing.T) {
	store := adminconfig.NewStore()
	h := NewAdminConfigHandler(store)

	t.Run("full lifecycle: create -> publish -> rollback", func(t *testing.T) {
		createBody := `{"version":"v-lifecycle","config_snapshot":"{\"setting\":\"enabled\"}","notes":"full lifecycle","created_by":"lifecycle"}`
		req := httptest.NewRequest(http.MethodPost, "/admin/config/versions", bytes.NewBufferString(createBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("create failed: %d body=%s", rr.Code, rr.Body.String())
		}

		var created map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		id := created["id"].(float64)

		pubReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/config/versions/%.0f/publish", id), nil)
		pubRR := httptest.NewRecorder()
		h.ServeHTTP(pubRR, pubReq)
		if pubRR.Code != http.StatusOK {
			t.Fatalf("publish failed: %d body=%s", pubRR.Code, pubRR.Body.String())
		}

		var published map[string]any
		if err := json.Unmarshal(pubRR.Body.Bytes(), &published); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if published["status"] != "published" {
			t.Fatalf("expected published, got %v", published["status"])
		}

		rollReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/config/versions/%.0f/rollback", id), nil)
		rollRR := httptest.NewRecorder()
		h.ServeHTTP(rollRR, rollReq)
		if rollRR.Code != http.StatusOK {
			t.Fatalf("rollback failed: %d body=%s", rollRR.Code, rollRR.Body.String())
		}

		var rolledBack map[string]any
		if err := json.Unmarshal(rollRR.Body.Bytes(), &rolledBack); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if rolledBack["status"] != "rolled_back" {
			t.Fatalf("expected rolled_back, got %v", rolledBack["status"])
		}
	})
}
