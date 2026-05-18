package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"llm-gateway/gateway/internal/auth"
)

type userStore interface {
	CreateUser(ctx context.Context, email, username, passwordHash string) (*auth.User, error)
	GetUserByEmail(ctx context.Context, email string) (*auth.User, error)
	GetUserByID(ctx context.Context, id int64) (*auth.User, error)
	CreateAPIKey(ctx context.Context, userID int64, keyPrefix, keyHash, name string) (*auth.APIKey, error)
	ListAPIKeys(ctx context.Context, userID int64) ([]auth.APIKey, error)
	RevokeAPIKey(ctx context.Context, userID, keyID int64) error
}

type userClaimsKey struct{}

func withUserClaims(ctx context.Context, claims *auth.Claims) context.Context {
	return context.WithValue(ctx, userClaimsKey{}, claims)
}

func getUserClaims(ctx context.Context) *auth.Claims {
	claims, _ := ctx.Value(userClaimsKey{}).(*auth.Claims)
	return claims
}

func (s *Server) WithUserStore(store userStore) *Server {
	s.userStore = store
	return s
}

func (s *Server) requireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.userStore == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": map[string]any{"message": "user authentication not available", "type": "service_unavailable"}})
			return
		}

		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "missing or invalid authorization header", "type": "authentication_error"}})
			return
		}

		tokenString := strings.TrimSpace(authHeader[7:])
		if tokenString == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "empty token", "type": "authentication_error"}})
			return
		}

		claims, err := auth.ValidateToken(tokenString, s.cfg.JWTSecret)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "invalid or expired token", "type": "authentication_error"}})
			return
		}

		next(w, r.WithContext(withUserClaims(r.Context(), claims)))
	}
}

func (s *Server) authSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}

	var body struct {
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	body.Username = strings.TrimSpace(body.Username)
	body.Password = strings.TrimSpace(body.Password)

	if body.Email == "" || body.Username == "" || body.Password == "" {
		badRequest(w, "email, username, and password are required")
		return
	}
	if len(body.Password) < 8 {
		badRequest(w, "password must be at least 8 characters")
		return
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		internalError(w, err)
		return
	}

	user, err := s.userStore.CreateUser(r.Context(), body.Email, body.Username, hash)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{"message": "email or username already exists", "type": "conflict_error"}})
			return
		}
		internalError(w, err)
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Email, "user", s.cfg.JWTSecret, 24*time.Hour)
	if err != nil {
		internalError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"token": token,
		"user":  userToResponse(user),
	})
}

func (s *Server) authLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}

	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	body.Password = strings.TrimSpace(body.Password)

	if body.Email == "" || body.Password == "" {
		badRequest(w, "email and password are required")
		return
	}

	user, err := s.userStore.GetUserByEmail(r.Context(), body.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "invalid email or password", "type": "authentication_error"}})
			return
		}
		internalError(w, err)
		return
	}

	if !auth.VerifyPassword(user.PasswordHash, body.Password) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "invalid email or password", "type": "authentication_error"}})
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Email, "user", s.cfg.JWTSecret, 24*time.Hour)
	if err != nil {
		internalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  userToResponse(user),
	})
}

func (s *Server) authMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}

	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}

	user, err := s.userStore.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "user not found", "type": "not_found_error"}})
			return
		}
		internalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, userToResponse(user))
}

func (s *Server) userCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}

	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		body.Name = "default"
	}

	plaintext, prefix, hash := auth.GenerateAPIKey()
	key, err := s.userStore.CreateAPIKey(r.Context(), claims.UserID, prefix, hash, body.Name)
	if err != nil {
		internalError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"key":     plaintext,
		"api_key": apiKeyToResponse(key),
	})
}

func (s *Server) userListAPIKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}

	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}

	keys, err := s.userStore.ListAPIKeys(r.Context(), claims.UserID)
	if err != nil {
		internalError(w, err)
		return
	}

	resp := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		resp = append(resp, apiKeyToResponse(&k))
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": resp})
}

func (s *Server) userRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		methodNotAllowed(w, r)
		return
	}

	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/user/api-keys/")
	if idStr == "" {
		badRequest(w, "api key id required")
		return
	}
	keyID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || keyID <= 0 {
		badRequest(w, "invalid api key id")
		return
	}

	if err := s.userStore.RevokeAPIKey(r.Context(), claims.UserID, keyID); err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "api key not found", "type": "not_found_error"}})
			return
		}
		internalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func userToResponse(u *auth.User) map[string]any {
	return map[string]any{
		"id":         u.ID,
		"email":      u.Email,
		"username":   u.Username,
		"role":       u.Role,
		"status":     u.Status,
		"created_at": u.CreatedAt.Format(time.RFC3339),
	}
}

func apiKeyToResponse(k *auth.APIKey) map[string]any {
	resp := map[string]any{
		"id":         k.ID,
		"name":       k.Name,
		"key_prefix": k.KeyPrefix,
		"status":     k.Status,
		"created_at": k.CreatedAt.Format(time.RFC3339),
	}
	if k.LastUsedAt != nil {
		resp["last_used_at"] = k.LastUsedAt.Format(time.RFC3339)
	}
	return resp
}
