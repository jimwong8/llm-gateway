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

type mockOAuthStore struct {
	getOrCreateUserFn func(ctx context.Context, provider, providerUserID, email, username, accessToken, refreshToken, encryptionKey string) (*auth.User, *auth.OAuthBinding, error)
	listBindingsFn    func(ctx context.Context, userID int64, encryptionKey string) ([]auth.OAuthBinding, error)
}

func (m *mockOAuthStore) GetOrCreateUserByOAuth(ctx context.Context, provider, providerUserID, email, username, accessToken, refreshToken, encryptionKey string) (*auth.User, *auth.OAuthBinding, error) {
	return m.getOrCreateUserFn(ctx, provider, providerUserID, email, username, accessToken, refreshToken, encryptionKey)
}

func (m *mockOAuthStore) GetOAuthBindingByProvider(ctx context.Context, provider, providerUserID, encryptionKey string) (*auth.OAuthBinding, error) {
	return nil, nil
}

func (m *mockOAuthStore) ListOAuthBindingsByUserID(ctx context.Context, userID int64, encryptionKey string) ([]auth.OAuthBinding, error) {
	return m.listBindingsFn(ctx, userID, encryptionKey)
}

func (m *mockOAuthStore) DeleteOAuthBinding(ctx context.Context, userID int64, provider string) error {
	return nil
}

func newTestServerWithOAuth() *Server {
	cfg := config.Config{
		JWTSecret:          "test-secret-key-at-least-32-characters!!",
		GitHubClientID:     "test-client-id",
		GitHubClientSecret: "test-client-secret",
	}
	s := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	s.userStore = &mockStore{}
	s.oauthStore = &mockOAuthStore{}
	return s
}

func TestOAuthConfig_ReturnsEnabled(t *testing.T) {
	s := newTestServerWithOAuth()
	mux := http.NewServeMux()
	s.mountUserAuthRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/config", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp struct {
		GitHubEnabled bool `json:"github_enabled"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if !resp.GitHubEnabled {
		t.Fatal("expected github_enabled = true")
	}
}

func TestOAuthConfig_ReturnsDisabled(t *testing.T) {
	cfg := config.Config{JWTSecret: "test-secret"}
	s := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	s.userStore = &mockStore{}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/config", nil)
	rr := httptest.NewRecorder()
	s.oauthConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp struct {
		GitHubEnabled bool `json:"github_enabled"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.GitHubEnabled {
		t.Fatal("expected github_enabled = false when not configured")
	}
}

func TestOAuthGitHubLogin_RedirectsToGitHub(t *testing.T) {
	s := newTestServerWithOAuth()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/github", nil)
	rr := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/oauth/github", s.oauthGitHubLogin)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", rr.Code)
	}

	loc := rr.Header().Get("Location")
	if loc == "" {
		t.Fatal("expected Location header")
	}
	if _, err := time.Parse(time.RFC3339, rr.Header().Get("Set-Cookie")); err == nil {
		t.Log("set-cookie present")
	}
}

func TestOAuthGitHubLogin_MissingClientID(t *testing.T) {
	cfg := config.Config{JWTSecret: "test-secret"}
	s := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/github", nil)
	rr := httptest.NewRecorder()
	s.oauthGitHubLogin(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestOAuthCallback_StateMismatch(t *testing.T) {
	s := newTestServerWithOAuth()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/github/callback?code=testcode&state=badstate", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "goodstate"})
	rr := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/oauth/github/callback", s.oauthGitHubCallback)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for state mismatch, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestOAuthCallback_MissingStateCookie(t *testing.T) {
	s := newTestServerWithOAuth()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/github/callback?code=testcode&state=teststate", nil)
	rr := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/oauth/github/callback", s.oauthGitHubCallback)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestOAuthCallback_MissingCode(t *testing.T) {
	s := newTestServerWithOAuth()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/github/callback?state=teststate", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "teststate"})
	rr := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/oauth/github/callback", s.oauthGitHubCallback)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing code, got %d: %s", rr.Code, rr.Body.String())
	}
}

type mockStore struct{}

func (m *mockStore) CreateUser(ctx context.Context, email, username, passwordHash string) (*auth.User, error) {
	return &auth.User{ID: 1, Email: email, Username: username, Role: 1, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (m *mockStore) GetUserByEmail(ctx context.Context, email string) (*auth.User, error) {
	return nil, nil
}

func (m *mockStore) GetUserByID(ctx context.Context, id int64) (*auth.User, error) {
	return &auth.User{ID: id, Email: "test@example.com", Username: "testuser", Role: 1, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (m *mockStore) CreateAPIKey(ctx context.Context, userID int64, keyPrefix, keyHash, name string) (*auth.APIKey, error) {
	return &auth.APIKey{ID: 1, UserID: userID, KeyPrefix: keyPrefix, Name: name, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (m *mockStore) ListAPIKeys(ctx context.Context, userID int64) ([]auth.APIKey, error) {
	return nil, nil
}

func (m *mockStore) RevokeAPIKey(ctx context.Context, userID, keyID int64) error {
	return nil
}

func (m *mockStore) GetAPIKeyByPrefix(ctx context.Context, prefix string) (*auth.APIKey, error) {
	return nil, nil
}

func (m *mockStore) GetAPIKeyByID(ctx context.Context, keyID int64) (*auth.APIKey, error) {
	return nil, nil
}

func (m *mockStore) UpdateAPIKeyLastUsed(ctx context.Context, keyID int64) error {
	return nil
}
