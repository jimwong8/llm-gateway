package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"llm-gateway/gateway/internal/auth"
	"llm-gateway/gateway/internal/config"
)

type mockUserStore struct {
	userStore
	users []auth.User
	keys  []auth.APIKey
}

func (m *mockUserStore) GetUserByID(_ context.Context, id int64) (*auth.User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return &u, nil
		}
	}
	return nil, nil
}

func (m *mockUserStore) ListAPIKeys(_ context.Context, userID int64) ([]auth.APIKey, error) {
	var out []auth.APIKey
	for _, k := range m.keys {
		if k.UserID == userID {
			out = append(out, k)
		}
	}
	return out, nil
}

func newTestServerWithUserStore(us userStore) *Server {
	cfg := config.Config{JWTSecret: "test-jwt-secret-at-least-32-chars!!"}
	s := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if us != nil {
		s.WithUserStore(us)
	}
	return s
}

func genToken(t *testing.T, userID int64, email, role string) string {
	t.Helper()
	tk, err := auth.GenerateToken(userID, email, role, "test-jwt-secret-at-least-32-chars!!", 1*time.Hour)
	if err != nil {
		t.Fatalf("gen token: %v", err)
	}
	return tk
}

func TestUserDashboard_RequiresAuth(t *testing.T) {
	s := newTestServerWithUserStore(nil)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/user/dashboard", nil)
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307 (redirect), got %d", rr.Code)
	}
}

func TestUserDashboard_ReturnsEmptyData(t *testing.T) {
	s := newTestServerWithUserStore(&mockUserStore{
		users: []auth.User{{ID: 1, Email: "t@t.com", Username: "t", Role: 1, Status: "active"}},
		keys:  []auth.APIKey{},
	})

	token := genToken(t, 1, "t@t.com", "user")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/user/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := resp["summary"]; !ok {
		t.Fatal("missing summary")
	}
	if _, ok := resp["recent_api_keys"]; !ok {
		t.Fatal("missing recent_api_keys")
	}
	if _, ok := resp["model_distribution"]; !ok {
		t.Fatal("missing model_distribution")
	}
}

func TestUserDashboard_WithKeys(t *testing.T) {
	s := newTestServerWithUserStore(&mockUserStore{
		users: []auth.User{{ID: 1, Email: "a@b.com", Username: "ab", Role: 1, Status: "active"}},
		keys: []auth.APIKey{
			{ID: 10, UserID: 1, KeyPrefix: "sk-test", Name: "My Key", Status: "active", CreatedAt: time.Now()},
		},
	})

	token := genToken(t, 1, "a@b.com", "user")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/user/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	keys := resp["recent_api_keys"].([]any)
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
}

func TestUserDashboard_MethodNotAllowed(t *testing.T) {
	s := newTestServerWithUserStore(&mockUserStore{users: []auth.User{{ID: 1, Email: "t@t.com", Username: "t"}}})
	token := genToken(t, 1, "t@t.com", "user")

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/api/user/dashboard", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		s.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("method %s: expected 405, got %d", method, rr.Code)
		}
	}
}

func TestUserUsage_RequiresAuth(t *testing.T) {
	s := newTestServerWithUserStore(nil)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/user/usage", nil)
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307 (redirect), got %d", rr.Code)
	}
}

func TestUserUsage_ReturnsData(t *testing.T) {
	s := newTestServerWithUserStore(&mockUserStore{users: []auth.User{{ID: 1, Email: "t@t.com", Username: "t"}}})

	token := genToken(t, 1, "t@t.com", "user")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/user/usage?days=7", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, ok := resp["data"].([]any)
	if !ok {
		t.Fatal("expected data array")
	}
	if data == nil {
		t.Fatal("data should be non-nil array")
	}
}

func TestUserUsage_MethodNotAllowed(t *testing.T) {
	s := newTestServerWithUserStore(&mockUserStore{users: []auth.User{{ID: 1, Email: "t@t.com", Username: "t"}}})
	token := genToken(t, 1, "t@t.com", "user")

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/api/user/usage", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		s.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("method %s: expected 405, got %d", method, rr.Code)
		}
	}
}
