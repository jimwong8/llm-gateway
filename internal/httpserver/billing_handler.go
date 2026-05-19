package httpserver

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"llm-gateway/gateway/internal/auth"
	"llm-gateway/gateway/internal/billing"
)

func (s *Server) mountBillingRoutes(mux *http.ServeMux) {
	if s.billingService == nil {
		return
	}
	mux.HandleFunc("/api/billing/balance", s.requireUser(s.userBillingBalance))
	mux.HandleFunc("/api/billing/ledger", s.requireUser(s.userBillingLedger))
	mux.HandleFunc("/api/admin/billing/pricing", s.requireAdmin(s.adminBillingPricing))
	mux.HandleFunc("/api/admin/billing/credit", s.requireAdmin(s.adminBillingCredit))
}

func (s *Server) userBillingBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "not authenticated"})
		return
	}
	userID := userIDStr(claims)
	bal, err := s.billingService.GetBalance(r.Context(), userID)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"balance": bal, "currency": "USD"})
}

func (s *Server) userBillingLedger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "not authenticated"})
		return
	}
	userID := userIDStr(claims)
	filter := billing.LedgerFilter{
		UserID: userID,
		Limit:  parseLimit(r, 50),
		Offset: parseOffset(r),
	}
	if t := strings.TrimSpace(r.URL.Query().Get("type")); t != "" {
		filter.Type = t
	}
	if from := strings.TrimSpace(r.URL.Query().Get("from")); from != "" {
		if ts, err := time.Parse(time.RFC3339, from); err == nil {
			filter.From = ts
		}
	}
	if to := strings.TrimSpace(r.URL.Query().Get("to")); to != "" {
		if ts, err := time.Parse(time.RFC3339, to); err == nil {
			filter.To = ts
		}
	}
	entries, err := s.billingService.ListLedger(r.Context(), filter)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": entries})
}

func (s *Server) adminBillingPricing(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := s.billingService.AllPricing(r.Context())
		if err != nil {
			internalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": rows})
	case http.MethodPost:
		var body struct {
			Provider      string  `json:"provider"`
			Model         string  `json:"model"`
			InputPrice1K  float64 `json:"input_price_per_1k"`
			OutputPrice1K float64 `json:"output_price_per_1k"`
			IsDefault     bool    `json:"is_default"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			badRequest(w, "invalid JSON body")
			return
		}
		if strings.TrimSpace(body.Provider) == "" {
			badRequest(w, "provider is required")
			return
		}
		if err := s.billingService.UpsertPricing(r.Context(), body.Provider, body.Model, body.InputPrice1K, body.OutputPrice1K, body.IsDefault); err != nil {
			internalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	default:
		methodNotAllowed(w, r)
	}
}

func (s *Server) adminBillingCredit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	var body struct {
		UserID      string  `json:"user_id"`
		Amount      float64 `json:"amount"`
		ReferenceID string  `json:"reference_id"`
		Description string  `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	if strings.TrimSpace(body.UserID) == "" || body.Amount <= 0 {
		badRequest(w, "user_id and positive amount are required")
		return
	}
	if body.ReferenceID == "" {
		badRequest(w, "reference_id is required (idempotency)")
		return
	}
	entry, err := s.billingService.Credit(r.Context(), body.UserID, body.Amount, "admin_topup", body.Description, body.ReferenceID)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func userIDStr(claims *auth.Claims) string {
	return strconv.FormatInt(claims.UserID, 10)
}
