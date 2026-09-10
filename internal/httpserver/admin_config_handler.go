package httpserver

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"llm-gateway/gateway/internal/adminconfig"
)

type AdminConfigHandler struct {
	store *adminconfig.Store
}

func NewAdminConfigHandler(store *adminconfig.Store) *AdminConfigHandler {
	return &AdminConfigHandler{store: store}
}

func (h *AdminConfigHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/admin/config")
	path = strings.TrimSuffix(path, "/")

	switch {
	case path == "/site" || path == "":
		switch r.Method {
		case http.MethodGet:
			h.handleGetSiteConfig(w, r)
		case http.MethodPut:
			h.handleUpdateSiteConfig(w, r)
		default:
			methodNotAllowed(w, r)
		}

	case path == "/jwt/rotate":
		if r.Method != http.MethodPost {
			methodNotAllowed(w, r)
			return
		}
		h.handleRotateJWT(w, r)

	case path == "/versions":
		switch r.Method {
		case http.MethodGet:
			h.handleListVersions(w, r)
		case http.MethodPost:
			h.handleCreateVersion(w, r)
		default:
			methodNotAllowed(w, r)
		}

	case path == "/versions/export":
		if r.Method != http.MethodGet {
			methodNotAllowed(w, r)
			return
		}
		h.handleExportVersions(w, r)

	case path == "/versions/import":
		if r.Method != http.MethodPost {
			methodNotAllowed(w, r)
			return
		}
		h.handleImportVersions(w, r)

	case strings.HasPrefix(path, "/versions/"):
		parts := strings.Split(strings.TrimPrefix(path, "/versions/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing version id"})
			return
		}
		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid version id"})
			return
		}

		if len(parts) == 1 && r.Method == http.MethodGet {
			h.handleGetVersion(w, r, id)
			return
		}

		if len(parts) == 2 {
			action := parts[1]
			if r.Method != http.MethodPost {
				methodNotAllowed(w, r)
				return
			}
			switch action {
			case "publish":
				h.handlePublishVersion(w, r, id)
			case "rollback":
				h.handleRollbackVersion(w, r, id)
			default:
				writeJSON(w, http.StatusBadRequest, errorResponse{Error: "unknown action: " + action})
			}
			return
		}

		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid path"})

	default:
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
	}
}

func (h *AdminConfigHandler) handleGetSiteConfig(w http.ResponseWriter, _ *http.Request) {
	cfg := h.store.GetSiteConfig()
	resp := map[string]any{
		"site_name":           cfg.SiteName,
		"logo_url":            cfg.LogoURL,
		"smtp_host":           cfg.SMTPHost,
		"smtp_port":           cfg.SMTPPort,
		"smtp_user":           cfg.SMTPUser,
		"smtp_from":           cfg.SMTPFrom,
		"allow_registration":  cfg.AllowRegistration,
		"default_user_role":   cfg.DefaultUserRole,
		"default_user_quota":  cfg.DefaultUserQuota,
		"updated_at":          cfg.UpdatedAt.Format(time.RFC3339),
		"updated_by":          cfg.UpdatedBy,
	}
	if cfg.JWTSecretRotatedAt != nil {
		resp["jwt_secret_rotated_at"] = cfg.JWTSecretRotatedAt.Format(time.RFC3339)
	}
	resp["jwt_secret_configured"] = cfg.JWTSecret != ""
	writeJSON(w, http.StatusOK, resp)
}

func (h *AdminConfigHandler) handleUpdateSiteConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SiteName          string `json:"site_name"`
		LogoURL           string `json:"logo_url"`
		SMTPHost          string `json:"smtp_host"`
		SMTPPort          int    `json:"smtp_port"`
		SMTPUser          string `json:"smtp_user"`
		SMTPPass          string `json:"smtp_pass"`
		SMTPFrom          string `json:"smtp_from"`
		AllowRegistration *bool  `json:"allow_registration"`
		DefaultUserRole   string `json:"default_user_role"`
		DefaultUserQuota  int64  `json:"default_user_quota"`
		UpdatedBy         string `json:"updated_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}

	updatedBy := strings.TrimSpace(body.UpdatedBy)
	if updatedBy == "" {
		updatedBy = "admin"
	}

	cfg := adminconfig.SiteConfig{
		SiteName:          strings.TrimSpace(body.SiteName),
		LogoURL:           strings.TrimSpace(body.LogoURL),
		SMTPHost:          strings.TrimSpace(body.SMTPHost),
		SMTPPort:          body.SMTPPort,
		SMTPUser:          strings.TrimSpace(body.SMTPUser),
		SMTPPass:          strings.TrimSpace(body.SMTPPass),
		SMTPFrom:          strings.TrimSpace(body.SMTPFrom),
		DefaultUserRole:   strings.TrimSpace(body.DefaultUserRole),
		DefaultUserQuota:  body.DefaultUserQuota,
	}
	if body.AllowRegistration != nil {
		cfg.AllowRegistration = *body.AllowRegistration
	}

	updated := h.store.UpdateSiteConfig(cfg, updatedBy)
	resp := map[string]any{
		"site_name":           updated.SiteName,
		"logo_url":            updated.LogoURL,
		"smtp_host":           updated.SMTPHost,
		"smtp_port":           updated.SMTPPort,
		"smtp_user":           updated.SMTPUser,
		"smtp_from":           updated.SMTPFrom,
		"allow_registration":  updated.AllowRegistration,
		"default_user_role":   updated.DefaultUserRole,
		"default_user_quota":  updated.DefaultUserQuota,
		"updated_at":          updated.UpdatedAt.Format(time.RFC3339),
		"updated_by":          updated.UpdatedBy,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AdminConfigHandler) handleRotateJWT(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UpdatedBy string `json:"updated_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		body.UpdatedBy = ""
	}

	updatedBy := strings.TrimSpace(body.UpdatedBy)
	if updatedBy == "" {
		updatedBy = "admin"
	}

	newSecret, rotatedAt := h.store.RotateJWTSecret(updatedBy)
	writeJSON(w, http.StatusOK, map[string]any{
		"jwt_secret":              newSecret,
		"jwt_secret_rotated_at":   rotatedAt.Format(time.RFC3339),
		"message": "JWT secret rotated. Existing tokens remain valid until expiry (grace period).",
	})
}

func (h *AdminConfigHandler) handleListVersions(w http.ResponseWriter, _ *http.Request) {
	snapshots := h.store.ListSnapshots()
	resp := make([]map[string]any, 0, len(snapshots))
	for _, snap := range snapshots {
		item := map[string]any{
			"id":              snap.ID,
			"version":         snap.Version,
			"status":          snap.Status,
			"notes":           snap.Notes,
			"created_by":      snap.CreatedBy,
			"created_at":      snap.CreatedAt.Format(time.RFC3339),
		}
		if snap.PublishedAt != nil {
			item["published_at"] = snap.PublishedAt.Format(time.RFC3339)
		}
		if snap.RolledBackAt != nil {
			item["rolled_back_at"] = snap.RolledBackAt.Format(time.RFC3339)
		}
		resp = append(resp, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": resp})
}

func (h *AdminConfigHandler) handleCreateVersion(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Version        string `json:"version"`
		ConfigSnapshot string `json:"config_snapshot"`
		Notes          string `json:"notes"`
		CreatedBy      string `json:"created_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}

	version := strings.TrimSpace(body.Version)
	if version == "" {
		version = fmt.Sprintf("v%d", time.Now().Unix())
	}
	createdBy := strings.TrimSpace(body.CreatedBy)
	if createdBy == "" {
		createdBy = "admin"
	}

	snap := h.store.CreateSnapshot(version, body.ConfigSnapshot, strings.TrimSpace(body.Notes), createdBy)
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":              snap.ID,
		"version":         snap.Version,
		"status":          snap.Status,
		"notes":           snap.Notes,
		"created_by":      snap.CreatedBy,
		"created_at":      snap.CreatedAt.Format(time.RFC3339),
	})
}

func (h *AdminConfigHandler) handleGetVersion(w http.ResponseWriter, _ *http.Request, id int64) {
	snap, ok := h.store.GetSnapshot(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "snapshot not found"})
		return
	}
	resp := map[string]any{
		"id":              snap.ID,
		"version":         snap.Version,
		"status":          snap.Status,
		"config_snapshot": snap.ConfigSnapshot,
		"notes":           snap.Notes,
		"created_by":      snap.CreatedBy,
		"created_at":      snap.CreatedAt.Format(time.RFC3339),
	}
	if snap.PublishedAt != nil {
		resp["published_at"] = snap.PublishedAt.Format(time.RFC3339)
	}
	if snap.RolledBackAt != nil {
		resp["rolled_back_at"] = snap.RolledBackAt.Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AdminConfigHandler) handlePublishVersion(w http.ResponseWriter, _ *http.Request, id int64) {
	snap, err := h.store.PublishSnapshot(id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	resp := map[string]any{
		"id":            snap.ID,
		"version":       snap.Version,
		"status":        snap.Status,
		"published_at":  snap.PublishedAt.Format(time.RFC3339),
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AdminConfigHandler) handleRollbackVersion(w http.ResponseWriter, _ *http.Request, id int64) {
	snap, err := h.store.RollbackSnapshot(id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	resp := map[string]any{
		"id":             snap.ID,
		"version":        snap.Version,
		"status":         snap.Status,
		"rolled_back_at": snap.RolledBackAt.Format(time.RFC3339),
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AdminConfigHandler) handleExportVersions(w http.ResponseWriter, _ *http.Request) {
	snapshots := h.store.ExportSnapshots()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=config-snapshots-export.json")
	json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": snapshots})
}

func (h *AdminConfigHandler) handleImportVersions(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Data []adminconfig.ConfigSnapshot `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	if len(body.Data) == 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "no snapshots to import"})
		return
	}
	h.store.ImportSnapshots(body.Data)
	writeJSON(w, http.StatusOK, map[string]any{"imported": len(body.Data)})
}

func generateJWTSecret() string {
	bytes := make([]byte, 32)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
