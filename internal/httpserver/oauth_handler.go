package httpserver

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"llm-gateway/gateway/internal/auth"
)

const (
	oauthStateCookie     = "oauth_state"
	oauthStateExpiry     = 10 * time.Minute
	oauthEncryptionKeyID = "oauth-token-encryption"
)

type oauthStore interface {
	GetOrCreateUserByOAuth(ctx context.Context, provider, providerUserID, email, username, accessToken, refreshToken, encryptionKey string) (*auth.User, *auth.OAuthBinding, error)
	GetOAuthBindingByProvider(ctx context.Context, provider, providerUserID, encryptionKey string) (*auth.OAuthBinding, error)
	ListOAuthBindingsByUserID(ctx context.Context, userID int64, encryptionKey string) ([]auth.OAuthBinding, error)
	DeleteOAuthBinding(ctx context.Context, userID int64, provider string) error
}

func (s *Server) WithOAuthStore(store oauthStore) *Server {
	s.oauthStore = store
	return s
}

func (s *Server) getOAuthEncryptionKey() string {
	if len(s.cfg.JWTSecret) >= 32 {
		return s.cfg.JWTSecret[:32]
	}
	key := s.cfg.JWTSecret
	for len(key) < 32 {
		key += key
	}
	return key[:32]
}

func (s *Server) oauthGitHubLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if s.cfg.GitHubClientID == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": map[string]any{"message": "GitHub OAuth not configured", "type": "service_unavailable"}})
		return
	}

	state, err := generateState()
	if err != nil {
		internalError(w, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/",
		Expires:  time.Now().Add(oauthStateExpiry),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	redirectURI := s.oauthRedirectURI(r)
	authURL := fmt.Sprintf("https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&state=%s&scope=user:email",
		s.cfg.GitHubClientID, url.QueryEscape(redirectURI), state)

	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Server) oauthGitHubCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}

	code := strings.TrimSpace(r.URL.Query().Get("code"))
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	if code == "" || state == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "missing code or state parameter", "type": "invalid_request_error"}})
		return
	}

	cookie, err := r.Cookie(oauthStateCookie)
	if err != nil || cookie.Value == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "missing state cookie", "type": "invalid_request_error"}})
		return
	}

	if cookie.Value != state {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{"message": "state mismatch - possible CSRF attack", "type": "csrf_error"}})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:    oauthStateCookie,
		Value:   "",
		Path:    "/",
		Expires: time.Unix(0, 0),
	})

	accessToken, err := s.exchangeGitHubCode(r.Context(), code)
	if err != nil {
		slog.Error("github token exchange failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": "failed to exchange code for token", "type": "oauth_error"}})
		return
	}

	ghUser, err := s.getGitHubUser(r.Context(), accessToken)
	if err != nil {
		slog.Error("failed to get github user", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": "failed to get GitHub user info", "type": "oauth_error"}})
		return
	}

	email := ghUser.Email
	if email == "" {
		email, err = s.getGitHubPrimaryEmail(r.Context(), accessToken)
		if err != nil {
			slog.Error("failed to get github email", "err", err)
		}
	}
	if email == "" {
		email = fmt.Sprintf("%s@users.noreply.github.com", ghUser.Login)
	}

	user, _, err := s.oauthStore.GetOrCreateUserByOAuth(
		r.Context(),
		"github",
		strconv.FormatInt(ghUser.ID, 10),
		email,
		ghUser.Login,
		accessToken,
		"",
		s.getOAuthEncryptionKey(),
	)
	if err != nil {
		slog.Error("failed to create/find user via oauth", "err", err)
		internalError(w, err)
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Email, "user", s.cfg.JWTSecret, 24*time.Hour)
	if err != nil {
		internalError(w, err)
		return
	}

	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		writeJSON(w, http.StatusOK, map[string]any{
			"token": token,
			"user":  userToResponse(user),
		})
		return
	}

	frontendURL := fmt.Sprintf("/admin/ui/login?token=%s", url.QueryEscape(token))
	http.Redirect(w, r, frontendURL, http.StatusFound)
}

type githubUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Email string `json:"email"`
}

type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func (s *Server) exchangeGitHubCode(ctx context.Context, code string) (string, error) {
	data := url.Values{
		"client_id":     {s.cfg.GitHubClientID},
		"client_secret": {s.cfg.GitHubClientSecret},
		"code":          {code},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.Error != "" {
		return "", fmt.Errorf("github oauth error: %s - %s", result.Error, result.ErrorDesc)
	}
	if result.AccessToken == "" {
		return "", errors.New("empty access token from github")
	}
	return result.AccessToken, nil
}

func (s *Server) getGitHubUser(ctx context.Context, accessToken string) (*githubUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var user githubUser
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, err
	}
	if user.ID == 0 {
		return nil, errors.New("invalid github user response")
	}
	return &user, nil
}

func (s *Server) getGitHubPrimaryEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var emails []githubEmail
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	if len(emails) > 0 {
		return emails[0].Email, nil
	}
	return "", nil
}

func (s *Server) oauthConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"github_enabled": s.cfg.GitHubClientID != "",
	})
}

func (s *Server) oauthRedirectURI(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.HasPrefix(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = "localhost:8080"
	}
	return fmt.Sprintf("%s://%s/api/auth/oauth/github/callback", scheme, host)
}

func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Server) oauthListBindings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}

	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}

	bindings, err := s.oauthStore.ListOAuthBindingsByUserID(r.Context(), claims.UserID, s.getOAuthEncryptionKey())
	if err != nil {
		internalError(w, err)
		return
	}

	resp := make([]map[string]any, 0, len(bindings))
	for _, b := range bindings {
		resp = append(resp, map[string]any{
			"id":              b.ID,
			"provider":        b.Provider,
			"created_at":      b.CreatedAt.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": resp})
}

func (s *Server) oauthDeleteBinding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		methodNotAllowed(w, r)
		return
	}

	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}

	provider := strings.TrimPrefix(r.URL.Path, "/api/user/oauth/")
	if provider == "" {
		badRequest(w, "provider required")
		return
	}

	if err := s.oauthStore.DeleteOAuthBinding(r.Context(), claims.UserID, provider); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "binding not found", "type": "not_found_error"}})
			return
		}
		internalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
