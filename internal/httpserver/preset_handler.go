package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"llm-gateway/gateway/internal/audit"
	"llm-gateway/gateway/internal/memory"
)

type presetStore interface {
	CreatePreset(ctx context.Context, userID int64, tenantID, name, description, template string, variables []string, tags []string, isPublic bool) (*memory.PromptPreset, error)
	ListPresets(ctx context.Context, userID int64, tenantID string, includePublic bool) ([]memory.PromptPreset, error)
	GetPreset(ctx context.Context, presetID int64, tenantID string) (*memory.PromptPreset, error)
	UpdatePreset(ctx context.Context, presetID int64, tenantID, name, description, template string, variables []string, tags []string) (*memory.PromptPreset, error)
	DeletePreset(ctx context.Context, presetID, userID int64, tenantID string) error
	CreateMaskRule(ctx context.Context, userID int64, tenantID, name, pattern, replace string) (*memory.MaskRule, error)
	ListMaskRules(ctx context.Context, userID int64, tenantID string) ([]memory.MaskRule, error)
	DeleteMaskRule(ctx context.Context, ruleID, userID int64, tenantID string) error
	UpdateMaskRule(ctx context.Context, ruleID, userID int64, tenantID, name, pattern, replace string, enabled bool) error
}

func (s *Server) WithPresetStore(store presetStore) *Server {
	s.presetStore = newCachedPresetStore(store)
	return s
}

func (s *Server) mountPresetRoutes(mux *http.ServeMux) {
	if s.presetStore == nil {
		return
	}
	mux.HandleFunc("/api/memory/presets", s.requireUser(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.presetList(w, r)
		case http.MethodPost:
			s.presetCreate(w, r)
		default:
			methodNotAllowed(w, r)
		}
	}))
	mux.HandleFunc("/api/memory/presets/", s.requireUser(func(w http.ResponseWriter, r *http.Request) {
		idStr := strings.TrimPrefix(r.URL.Path, "/api/memory/presets/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			badRequest(w, "invalid preset id")
			return
		}
		switch r.Method {
		case http.MethodGet:
			s.presetGet(w, r, id)
		case http.MethodPut:
			s.presetUpdate(w, r, id)
		case http.MethodDelete:
			s.presetDelete(w, r, id)
		default:
			methodNotAllowed(w, r)
		}
	}))
	mux.HandleFunc("/api/memory/masks", s.requireUser(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.maskList(w, r)
		case http.MethodPost:
			s.maskCreate(w, r)
		default:
			methodNotAllowed(w, r)
		}
	}))
	mux.HandleFunc("/api/memory/masks/", s.requireUser(func(w http.ResponseWriter, r *http.Request) {
		idStr := strings.TrimPrefix(r.URL.Path, "/api/memory/masks/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			badRequest(w, "invalid mask id")
			return
		}
		switch r.Method {
		case http.MethodPut:
			s.maskUpdate(w, r, id)
		case http.MethodDelete:
			s.maskDelete(w, r, id)
		default:
			methodNotAllowed(w, r)
		}
	}))
}

func (s *Server) presetList(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}
	tenantID := TenantIDFromContext(r.Context())
	presets, err := s.presetStore.ListPresets(r.Context(), claims.UserID, tenantID, true)
	if err != nil {
		internalError(w, err)
		return
	}
	if presets == nil {
		presets = []memory.PromptPreset{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": presets})
}

func (s *Server) presetCreate(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}
	var body struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Template    string   `json:"template"`
		Variables   []string `json:"variables"`
		Tags        []string `json:"tags"`
		IsPublic    bool     `json:"is_public"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w, "invalid JSON")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.Template = strings.TrimSpace(body.Template)
	if body.Name == "" || body.Template == "" {
		badRequest(w, "name and template are required")
		return
	}
	tenantID := TenantIDFromContext(r.Context())
	p, err := s.presetStore.CreatePreset(r.Context(), claims.UserID, tenantID, body.Name, body.Description, body.Template, body.Variables, body.Tags, body.IsPublic)
	if err != nil {
		internalError(w, err)
		return
	}
	s.writeAuditAsync(audit.Event{
		RequestPayload: map[string]any{
			"action":    "preset_created",
			"target_id": fmt.Sprintf("%d", p.ID),
			"actor_id":  fmt.Sprintf("%d", claims.UserID),
		},
	})
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) presetGet(w http.ResponseWriter, r *http.Request, id int64) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}
	tenantID := TenantIDFromContext(r.Context())
	p, err := s.presetStore.GetPreset(r.Context(), id, tenantID)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) presetUpdate(w http.ResponseWriter, r *http.Request, id int64) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}
	var body struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Template    string   `json:"template"`
		Variables   []string `json:"variables"`
		Tags        []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w, "invalid JSON")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.Template = strings.TrimSpace(body.Template)
	if body.Name == "" || body.Template == "" {
		badRequest(w, "name and template are required")
		return
	}
	tenantID := TenantIDFromContext(r.Context())
	p, err := s.presetStore.UpdatePreset(r.Context(), id, tenantID, body.Name, body.Description, body.Template, body.Variables, body.Tags)
	if err != nil {
		internalError(w, err)
		return
	}
	s.writeAuditAsync(audit.Event{
		RequestPayload: map[string]any{
			"action":    "preset_updated",
			"target_id": fmt.Sprintf("%d", id),
			"actor_id":  fmt.Sprintf("%d", claims.UserID),
		},
	})
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) presetDelete(w http.ResponseWriter, r *http.Request, id int64) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}
	tenantID := TenantIDFromContext(r.Context())
	if err := s.presetStore.DeletePreset(r.Context(), id, claims.UserID, tenantID); err != nil {
		internalError(w, err)
		return
	}
	s.writeAuditAsync(audit.Event{
		RequestPayload: map[string]any{
			"action":    "preset_deleted",
			"target_id": fmt.Sprintf("%d", id),
			"actor_id":  fmt.Sprintf("%d", claims.UserID),
		},
	})
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) maskList(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}
	tenantID := TenantIDFromContext(r.Context())
	rules, err := s.presetStore.ListMaskRules(r.Context(), claims.UserID, tenantID)
	if err != nil {
		internalError(w, err)
		return
	}
	if rules == nil {
		rules = []memory.MaskRule{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": rules})
}

func (s *Server) maskCreate(w http.ResponseWriter, r *http.Request) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}
	var body struct {
		Name    string `json:"name"`
		Pattern string `json:"pattern"`
		Replace string `json:"replace"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w, "invalid JSON")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.Pattern = strings.TrimSpace(body.Pattern)
	if body.Name == "" || body.Pattern == "" {
		badRequest(w, "name and pattern are required")
		return
	}
	if body.Replace == "" {
		body.Replace = "[REDACTED]"
	}
	tenantID := TenantIDFromContext(r.Context())
	rule, err := s.presetStore.CreateMaskRule(r.Context(), claims.UserID, tenantID, body.Name, body.Pattern, body.Replace)
	if err != nil {
		internalError(w, err)
		return
	}
	s.writeAuditAsync(audit.Event{
		RequestPayload: map[string]any{
			"action":    "mask_created",
			"target_id": fmt.Sprintf("%d", rule.ID),
			"actor_id":  fmt.Sprintf("%d", claims.UserID),
		},
	})
	writeJSON(w, http.StatusCreated, rule)
}

func (s *Server) maskUpdate(w http.ResponseWriter, r *http.Request, id int64) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}
	var body struct {
		Name    string `json:"name"`
		Pattern string `json:"pattern"`
		Replace string `json:"replace"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w, "invalid JSON")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.Pattern = strings.TrimSpace(body.Pattern)
	if body.Name == "" || body.Pattern == "" {
		badRequest(w, "name and pattern are required")
		return
	}
	if body.Replace == "" {
		body.Replace = "[REDACTED]"
	}
	tenantID := TenantIDFromContext(r.Context())
	if err := s.presetStore.UpdateMaskRule(r.Context(), id, claims.UserID, tenantID, body.Name, body.Pattern, body.Replace, body.Enabled); err != nil {
		internalError(w, err)
		return
	}
	s.writeAuditAsync(audit.Event{
		RequestPayload: map[string]any{
			"action":    "mask_updated",
			"target_id": fmt.Sprintf("%d", id),
			"actor_id":  fmt.Sprintf("%d", claims.UserID),
		},
	})
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) maskDelete(w http.ResponseWriter, r *http.Request, id int64) {
	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "not authenticated", "type": "authentication_error"}})
		return
	}
	tenantID := TenantIDFromContext(r.Context())
	if err := s.presetStore.DeleteMaskRule(r.Context(), id, claims.UserID, tenantID); err != nil {
		internalError(w, err)
		return
	}
	s.writeAuditAsync(audit.Event{
		RequestPayload: map[string]any{
			"action":    "mask_deleted",
			"target_id": fmt.Sprintf("%d", id),
			"actor_id":  fmt.Sprintf("%d", claims.UserID),
		},
	})
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
