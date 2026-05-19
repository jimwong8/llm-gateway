package httpserver

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"llm-gateway/gateway/internal/auth"
)

type emailStore interface {
	CreateVerificationToken(ctx context.Context, userID int64, token, tokenType string, expiresAt time.Time) error
	VerifyEmailToken(ctx context.Context, token string) (int64, error)
	UpdateUserPassword(ctx context.Context, userID int64, hashedPassword string) error
	GetUserByEmail(ctx context.Context, email string) (*auth.User, error)
}

type emailService interface {
	SendVerificationEmail(to, token, baseURL string) error
	SendPasswordResetEmail(to, token, baseURL string) error
}

func (s *Server) WithEmailStore(store emailStore) *Server {
	s.emailStore = store
	return s
}

func (s *Server) WithEmailService(svc emailService) *Server {
	s.emailService = svc
	return s
}

func (s *Server) emailVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		badRequest(w, "token required")
		return
	}
	userID, err := s.emailStore.VerifyEmailToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "invalid or expired token", "type": "invalid_token"}})
			return
		}
		internalError(w, err)
		return
	}
	slog.Info("email verified", "user_id", userID)
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "message": "email verified"})
}

func (s *Server) emailResendVerification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}
	user, err := s.emailStore.GetUserByEmail(r.Context(), claims.Email)
	if err != nil {
		internalError(w, err)
		return
	}
	token, err := generateSecureToken()
	if err != nil {
		internalError(w, err)
		return
	}
	if err := s.emailStore.CreateVerificationToken(r.Context(), user.ID, token, "verification", time.Now().Add(24*time.Hour)); err != nil {
		internalError(w, err)
		return
	}
	baseURL := s.frontendBaseURL(r)
	if err := s.emailService.SendVerificationEmail(user.Email, token, baseURL); err != nil {
		slog.Error("failed to send verification email", "err", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "message": "verification email sent"})
}

func (s *Server) forgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		badRequest(w, "email required")
		return
	}
	user, err := s.emailStore.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "message": "if the email exists, a reset link has been sent"})
		return
	}
	token, err := generateSecureToken()
	if err != nil {
		internalError(w, err)
		return
	}
	if err := s.emailStore.CreateVerificationToken(r.Context(), user.ID, token, "password_reset", time.Now().Add(1*time.Hour)); err != nil {
		internalError(w, err)
		return
	}
	baseURL := s.frontendBaseURL(r)
	if err := s.emailService.SendPasswordResetEmail(user.Email, token, baseURL); err != nil {
		slog.Error("failed to send password reset email", "err", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "message": "if the email exists, a reset link has been sent"})
}

func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "invalid request body")
		return
	}
	req.Token = strings.TrimSpace(req.Token)
	if req.Token == "" || len(req.Password) < 8 {
		badRequest(w, "token required and password must be at least 8 characters")
		return
	}
	userID, err := s.emailStore.VerifyEmailToken(r.Context(), req.Token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "invalid or expired token", "type": "invalid_token"}})
			return
		}
		internalError(w, err)
		return
	}
	hashed, err := auth.HashPassword(req.Password)
	if err != nil {
		internalError(w, err)
		return
	}
	if err := s.emailStore.UpdateUserPassword(r.Context(), userID, hashed); err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "message": "password reset successful"})
}

func generateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Server) frontendBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.HasPrefix(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = "localhost:8080"
	}
	return scheme + "://" + host + "/admin/ui"
}
