package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"llm-gateway/gateway/internal/auth"
	"llm-gateway/gateway/internal/billing"
)

type userStore interface {
	CreateUser(ctx context.Context, email, username, passwordHash string) (*auth.User, error)
	GetUserByEmail(ctx context.Context, email string) (*auth.User, error)
	GetUserByID(ctx context.Context, id int64) (*auth.User, error)
	CreateAPIKey(ctx context.Context, userID int64, keyPrefix, keyHash, name string) (*auth.APIKey, error)
	ListAPIKeys(ctx context.Context, userID int64) ([]auth.APIKey, error)
	RevokeAPIKey(ctx context.Context, userID, keyID int64) error
	GetAPIKeyByPrefix(ctx context.Context, prefix string) (*auth.APIKey, error)
	GetAPIKeyByID(ctx context.Context, keyID int64) (*auth.APIKey, error)
	UpdateAPIKeyLastUsed(ctx context.Context, keyID int64) error
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

func (s *Server) withOptionalUserAPIKey(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, apiKeyID := s.authenticateUserAPIKey(r)
		if user != nil {
			ctx := context.WithValue(r.Context(), userClaimsKey{}, &auth.Claims{
				UserID: user.ID,
				Email:  user.Email,
				Role:   "user",
			})
			if apiKeyID > 0 {
				ctx = withAPIKeyID(ctx, apiKeyID)
			}
			next(w, r.WithContext(ctx))
			return
		}
		next(w, r)
	}
}

func (s *Server) authenticateUserAPIKey(r *http.Request) (*auth.User, int64) {
	if s.userStore == nil {
		return nil, 0
	}
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return nil, 0
	}
	tokenString := strings.TrimSpace(authHeader[7:])
	if tokenString == "" || !strings.HasPrefix(tokenString, "sk-") {
		return nil, 0
	}

	prefix := tokenString
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}

	key, err := s.userStore.GetAPIKeyByPrefix(r.Context(), prefix)
	if err != nil {
		return nil, 0
	}
	if key.Status != "active" {
		return nil, 0
	}
	if !auth.VerifyAPIKey(tokenString, key.KeyHash) {
		return nil, 0
	}

	user, err := s.userStore.GetUserByID(r.Context(), key.UserID)
	if err != nil {
		return nil, 0
	}

	if err := s.userStore.UpdateAPIKeyLastUsed(r.Context(), key.ID); err != nil {
		slog.Warn("failed to update api key last_used", "key_id", key.ID, "err", err)
	}

	return user, key.ID
}

func (s *Server) authSignup(w http.ResponseWriter, r *http.Request) {
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
		item := apiKeyToResponse(&k)
		if s.apiKeyUsageStore != nil {
			summary, err := s.apiKeyUsageStore.Summary(r.Context(), auth.UsageStatsFilter{KeyID: k.ID})
			if err == nil && summary != nil {
				item["usage"] = map[string]any{
					"total_requests":            summary.TotalRequests,
					"total_tokens":              summary.TotalTokens,
					"total_prompt_tokens":       summary.TotalPromptTokens,
					"total_completion_tokens":   summary.TotalCompletionTokens,
					"total_cost":                summary.TotalCost,
					"avg_latency_ms":            summary.AvgLatencyMs,
				}
			}
		}
		resp = append(resp, item)
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
		"rpm_limit":  k.RPMILimit,
		"created_at": k.CreatedAt.Format(time.RFC3339),
	}
	if k.LastUsedAt != nil {
		resp["last_used_at"] = k.LastUsedAt.Format(time.RFC3339)
	}
	return resp
}

func (s *Server) userDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}

	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}

	userIDStr := strconv.FormatInt(claims.UserID, 10)

	summary := billing.SummaryRow{}
	if s.billing != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		row, err := s.billing.Summary(ctx, billing.QueryFilter{
			UserID: userIDStr,
			From:   time.Now().AddDate(0, 0, -30),
		})
		cancel()
		if err == nil {
			summary = row
		}

		todayStart := time.Now().UTC().Truncate(24 * time.Hour)
		ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
		todayRow, err := s.billing.Summary(ctx, billing.QueryFilter{
			UserID: userIDStr,
			From:   todayStart,
		})
		cancel()
		if err == nil {
			summary.Requests = todayRow.Requests
			summary.TotalTokens = todayRow.TotalTokens
		}
	}

	recentKeys := []map[string]any{}
	if s.userStore != nil {
		keys, err := s.userStore.ListAPIKeys(r.Context(), claims.UserID)
		if err == nil {
			limit := 5
			if len(keys) > limit {
				keys = keys[:limit]
			}
			for _, k := range keys {
				recentKeys = append(recentKeys, apiKeyToResponse(&k))
			}
		}
	}

	modelDist := []billing.HotspotRow{}
	if s.billing != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		rows, err := s.billing.ModelBreakdown(ctx, billing.QueryFilter{
			UserID: userIDStr,
			From:   time.Now().AddDate(0, 0, -30),
		})
		cancel()
		if err == nil {
			modelDist = rows
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"summary":            summary,
		"recent_api_keys":    recentKeys,
		"model_distribution": modelDist,
	})
}

func (s *Server) userUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}

	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}

	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 30 {
			days = parsed
		}
	}

	userIDStr := strconv.FormatInt(claims.UserID, 10)

	data := []billing.DailyUsageRow{}
	if s.billing != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		rows, err := s.billing.DailyUsage(ctx, billing.QueryFilter{
			UserID: userIDStr,
			From:   time.Now().AddDate(0, 0, -days),
		})
		cancel()
		if err == nil {
			data = rows
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

func (s *Server) userAPIKeyUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}

	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 30 {
			days = parsed
		}
	}

	keys, err := s.userStore.ListAPIKeys(r.Context(), claims.UserID)
	if err != nil {
		internalError(w, err)
		return
	}

	result := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		item := map[string]any{
			"id":         k.ID,
			"name":       k.Name,
			"key_prefix": k.KeyPrefix,
		}
		if s.apiKeyUsageStore != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			summary, err := s.apiKeyUsageStore.Summary(ctx, auth.UsageStatsFilter{
				KeyID: k.ID,
				From:  time.Now().AddDate(0, 0, -days),
			})
			cancel()
			if err == nil && summary != nil {
				item["usage"] = summary
			} else {
				item["usage"] = &auth.APIKeyUsageSummary{KeyID: k.ID}
			}
		}
		result = append(result, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": result})
}

func (s *Server) userAPIKeyUsageByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/user/api-keys/")
	keyIDStr := strings.TrimSuffix(path, "/usage")
	keyID, err := strconv.ParseInt(keyIDStr, 10, 64)
	if err != nil || keyID <= 0 {
		badRequest(w, "invalid api key id")
		return
	}

	limit := parseLimit(r, 20)

	history := []auth.APIKeyUsageRow{}
	if s.apiKeyUsageStore != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		rows, err := s.apiKeyUsageStore.UsageHistory(ctx, keyID, limit)
		cancel()
		if err == nil {
			history = rows
		}
	}

	summary := &auth.APIKeyUsageSummary{KeyID: keyID}
	if s.apiKeyUsageStore != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		summary, err = s.apiKeyUsageStore.Summary(ctx, auth.UsageStatsFilter{KeyID: keyID})
		cancel()
		if err != nil {
			summary = &auth.APIKeyUsageSummary{KeyID: keyID}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"summary": summary,
		"history": history,
	})
}
