package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"llm-gateway/gateway/internal/audit"
	"llm-gateway/gateway/internal/controlplane"
	"llm-gateway/gateway/internal/runtime"
)

type auditEventReader interface {
	Events() []audit.ControlPlaneEvent
}

type runtimeEventReader interface {
	Events() []runtime.Event
}

type runtimeReplayPublisher interface {
	PublishIfReleased(version controlplane.ConfigVersion) bool
}

type AdminHandler struct {
	mux             *http.ServeMux
	service         *controlplane.Service
	auditor         auditEventReader
	runtime         runtimeEventReader
	replayPublisher runtimeReplayPublisher
	adminToken      string
}

type createInheritanceDraftRequest struct {
	Module            string `json:"module"`
	TenantID          string `json:"tenant_id"`
	Scope             string `json:"scope"`
	ProjectID         string `json:"project_id,omitempty"`
	SourceEnvironment string `json:"source_environment"`
	TargetEnvironment string `json:"target_environment"`
	Reason            string `json:"reason,omitempty"`
	Actor             string `json:"actor,omitempty"`
}

type releaseDraftRequest struct {
	Module      string `json:"module"`
	TenantID    string `json:"tenant_id"`
	Environment string `json:"environment"`
	Scope       string `json:"scope"`
	ProjectID   string `json:"project_id,omitempty"`
	VersionID   string `json:"version_id"`
	Reason      string `json:"reason,omitempty"`
	Actor       string `json:"actor,omitempty"`
}

type promoteReleasedRequest struct {
	Module            string `json:"module"`
	TenantID          string `json:"tenant_id"`
	SourceEnvironment string `json:"source_environment"`
	TargetEnvironment string `json:"target_environment"`
	Scope             string `json:"scope"`
	ProjectID         string `json:"project_id,omitempty"`
	Reason            string `json:"reason,omitempty"`
	Actor             string `json:"actor,omitempty"`
}

type replayReleasedRequest struct {
	Module      string `json:"module"`
	TenantID    string `json:"tenant_id"`
	Environment string `json:"environment"`
	Scope       string `json:"scope"`
	ProjectID   string `json:"project_id,omitempty"`
	VersionID   string `json:"version_id"`
}

type rollbackReleasedRequest struct {
	Module      string `json:"module"`
	TenantID    string `json:"tenant_id"`
	Environment string `json:"environment"`
	Scope       string `json:"scope"`
	ProjectID   string `json:"project_id,omitempty"`
	VersionID   string `json:"version_id"`
	Reason      string `json:"reason,omitempty"`
	Actor       string `json:"actor,omitempty"`
}

type replayCompensationRequest struct {
	Module      string `json:"module"`
	TenantID    string `json:"tenant_id"`
	Environment string `json:"environment"`
	Scope       string `json:"scope,omitempty"`
	ProjectID   string `json:"project_id,omitempty"`
	Version     string `json:"version,omitempty"`
	VersionID   string `json:"version_id,omitempty"`
}

type inheritanceDraftSourceResponse struct {
	Type              string `json:"type"`
	SourceEnvironment string `json:"source_environment"`
	SourceVersionID   string `json:"source_version_id"`
}

type versionResponse struct {
	VersionID   string                          `json:"version_id"`
	Status      string                          `json:"status"`
	Environment string                          `json:"environment"`
	Source      *inheritanceDraftSourceResponse `json:"source,omitempty"`
}

type auditSummaryResponse struct {
	Total         int            `json:"total"`
	ByType        map[string]int `json:"by_type"`
	ByEnvironment map[string]int `json:"by_environment"`
	ScannedTotal  int            `json:"scanned_total,omitempty"`
	TenantID      string         `json:"tenant_id,omitempty"`
	Environment   string         `json:"environment,omitempty"`
	LatestAt      string         `json:"latest_at,omitempty"`
	OldestAt      string         `json:"oldest_at,omitempty"`
}

type versionSummaryResponse struct {
	Total         int            `json:"total"`
	ByStatus      map[string]int `json:"by_status"`
	ByEnvironment map[string]int `json:"by_environment"`
	BySource      map[string]int `json:"by_source"`
	LatestAt      string         `json:"latest_at,omitempty"`
	OldestAt      string         `json:"oldest_at,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func NewAdminHandler(service *controlplane.Service) *AdminHandler {
	h := &AdminHandler{
		mux:     http.NewServeMux(),
		service: service,
	}
	h.registerRoutes()
	return h
}

func (h *AdminHandler) WithAuditReader(reader auditEventReader) *AdminHandler {
	h.auditor = reader
	return h
}

func (h *AdminHandler) WithRuntimeReader(reader runtimeEventReader) *AdminHandler {
	h.runtime = reader
	return h
}

func (h *AdminHandler) WithRuntimeReplayPublisher(publisher runtimeReplayPublisher) *AdminHandler {
	h.replayPublisher = publisher
	return h
}

func (h *AdminHandler) WithAdminToken(token string) *AdminHandler {
	h.adminToken = token
	return h
}

func (h *AdminHandler) registerRoutes() {
	h.mux.HandleFunc("/admin/inheritance-drafts", h.handleCreateInheritanceDraft)
	h.mux.HandleFunc("/admin/releases", h.handleReleaseDraft)
	h.mux.HandleFunc("/admin/releases/rollback", h.handleRollbackReleased)
	h.mux.HandleFunc("/admin/releases/replay", h.handleReplayReleased)
	h.mux.HandleFunc("/admin/control-plane/compensations/replay", h.handleReplayCompensation)
	h.mux.HandleFunc("/admin/promotions", h.handlePromoteReleased)
	h.mux.HandleFunc("/admin/audit-events", h.handleListAuditEvents)
	h.mux.HandleFunc("/admin/runtime-events", h.handleListRuntimeEvents)
	h.mux.HandleFunc("/admin/config-versions", h.handleListVersions)
	h.mux.HandleFunc("/admin/config-versions/", h.handleGetVersion)
}

func (h *AdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/admin/") && !h.isAuthorizedAdminRequest(r) {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
		return
	}
	h.mux.ServeHTTP(w, r)
}

func (h *AdminHandler) isAuthorizedAdminRequest(r *http.Request) bool {
	if strings.TrimSpace(h.adminToken) == "" {
		return false
	}

	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(authorization, "Bearer ") {
		return false
	}

	token := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
	if token == "" {
		return false
	}

	return token == h.adminToken
}

func (h *AdminHandler) handleCreateInheritanceDraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	var req createInheritanceDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid json body"})
		return
	}
	req.Module = strings.TrimSpace(req.Module)
	req.TenantID = strings.TrimSpace(req.TenantID)
	req.Scope = strings.TrimSpace(req.Scope)
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.SourceEnvironment = strings.TrimSpace(req.SourceEnvironment)
	req.TargetEnvironment = strings.TrimSpace(req.TargetEnvironment)
	req.Reason = strings.TrimSpace(req.Reason)
	req.Actor = strings.TrimSpace(req.Actor)
	if req.Module == "" || req.TenantID == "" || req.Scope == "" || req.SourceEnvironment == "" || req.TargetEnvironment == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing required fields"})
		return
	}
	if req.SourceEnvironment == req.TargetEnvironment {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "source and target environment must differ"})
		return
	}

	version, err := h.service.CreateInheritanceDraft(r.Context(), controlplane.CreateInheritanceDraftInput{
		Module:            req.Module,
		TenantID:          req.TenantID,
		Scope:             req.Scope,
		ProjectID:         req.ProjectID,
		SourceEnvironment: req.SourceEnvironment,
		TargetEnvironment: req.TargetEnvironment,
		Reason:            req.Reason,
		Actor:             req.Actor,
	})
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, controlplane.ErrReleasedConfigNotFound) {
			statusCode = http.StatusNotFound
		}
		writeJSON(w, statusCode, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, buildVersionResponse(version))
}

func (h *AdminHandler) handleReleaseDraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	var req releaseDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid json body"})
		return
	}
	req.Module = strings.TrimSpace(req.Module)
	req.TenantID = strings.TrimSpace(req.TenantID)
	req.Environment = strings.TrimSpace(req.Environment)
	req.Scope = strings.TrimSpace(req.Scope)
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.VersionID = strings.TrimSpace(req.VersionID)
	req.Reason = strings.TrimSpace(req.Reason)
	req.Actor = strings.TrimSpace(req.Actor)
	if req.Module == "" || req.TenantID == "" || req.Environment == "" || req.Scope == "" || req.VersionID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing required fields"})
		return
	}

	version, err := h.service.ReleaseDraft(
		r.Context(),
		req.Module,
		req.TenantID,
		req.Environment,
		req.Scope,
		req.ProjectID,
		req.VersionID,
		req.Actor,
		req.Reason,
	)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, controlplane.ErrVersionNotFound) {
			statusCode = http.StatusNotFound
		}
		writeJSON(w, statusCode, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, buildVersionResponse(version))
}

func (h *AdminHandler) handleReplayReleased(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	if h.replayPublisher == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "runtime replay unavailable"})
		return
	}

	var req replayReleasedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid json body"})
		return
	}

	version, statusCode, errMsg := h.resolveReplayTarget(r.Context(), req, true)
	if statusCode != 0 {
		writeJSON(w, statusCode, errorResponse{Error: errMsg})
		return
	}

	h.replayPublisher.PublishIfReleased(version)
	writeJSON(w, http.StatusOK, buildVersionResponse(version))
}

func (h *AdminHandler) handleRollbackReleased(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	var req rollbackReleasedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid json body"})
		return
	}
	req.Module = strings.TrimSpace(req.Module)
	req.TenantID = strings.TrimSpace(req.TenantID)
	req.Environment = strings.TrimSpace(req.Environment)
	req.Scope = strings.TrimSpace(req.Scope)
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.VersionID = strings.TrimSpace(req.VersionID)
	req.Reason = strings.TrimSpace(req.Reason)
	req.Actor = strings.TrimSpace(req.Actor)
	if req.Module == "" || req.TenantID == "" || req.Environment == "" || req.Scope == "" || req.VersionID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing required fields"})
		return
	}

	version, err := h.service.RollbackReleased(r.Context(), controlplane.RollbackReleasedInput{
		Module:      req.Module,
		TenantID:    req.TenantID,
		Environment: req.Environment,
		Scope:       req.Scope,
		ProjectID:   req.ProjectID,
		VersionID:   req.VersionID,
		Actor:       req.Actor,
		Reason:      req.Reason,
	})
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, controlplane.ErrVersionNotFound) {
			statusCode = http.StatusNotFound
		}
		writeJSON(w, statusCode, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, buildVersionResponse(version))
}

func (h *AdminHandler) handleReplayCompensation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	if h.replayPublisher == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "runtime replay unavailable"})
		return
	}

	var req replayCompensationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid json body"})
		return
	}

	versionID := strings.TrimSpace(req.VersionID)
	if versionID == "" {
		versionID = strings.TrimSpace(req.Version)
	}
	version, statusCode, errMsg := h.resolveReplayTarget(r.Context(), replayReleasedRequest{
		Module:      req.Module,
		TenantID:    req.TenantID,
		Environment: req.Environment,
		Scope:       req.Scope,
		ProjectID:   req.ProjectID,
		VersionID:   versionID,
	}, false)
	if statusCode != 0 {
		writeJSON(w, statusCode, errorResponse{Error: errMsg})
		return
	}

	h.replayPublisher.PublishIfReleased(version)
	writeJSON(w, http.StatusOK, buildVersionResponse(version))
}

func (h *AdminHandler) resolveReplayTarget(ctx context.Context, req replayReleasedRequest, requireScope bool) (controlplane.ConfigVersion, int, string) {
	req.Module = strings.TrimSpace(req.Module)
	req.TenantID = strings.TrimSpace(req.TenantID)
	req.Environment = strings.TrimSpace(req.Environment)
	req.Scope = strings.TrimSpace(req.Scope)
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.VersionID = strings.TrimSpace(req.VersionID)
	if req.Module == "" || req.TenantID == "" || req.Environment == "" || req.VersionID == "" {
		return controlplane.ConfigVersion{}, http.StatusBadRequest, "missing required fields"
	}
	if requireScope && req.Scope == "" {
		return controlplane.ConfigVersion{}, http.StatusBadRequest, "missing required fields"
	}

	versions := h.service.ListVersions(ctx, req.Module, req.TenantID, req.Environment, req.Scope, req.ProjectID)
	matches := make([]controlplane.ConfigVersion, 0, len(versions))
	for _, version := range versions {
		if version.Version == req.VersionID {
			matches = append(matches, version)
		}
	}
	if len(matches) == 0 {
		return controlplane.ConfigVersion{}, http.StatusNotFound, controlplane.ErrVersionNotFound.Error()
	}
	if len(matches) > 1 {
		return controlplane.ConfigVersion{}, http.StatusConflict, "version target is ambiguous; specify scope and project_id"
	}
	if matches[0].Source != controlplane.ConfigStatusReleased {
		return controlplane.ConfigVersion{}, http.StatusConflict, "version is not released"
	}

	return matches[0], 0, ""
}

func (h *AdminHandler) handlePromoteReleased(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	var req promoteReleasedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid json body"})
		return
	}
	req.Module = strings.TrimSpace(req.Module)
	req.TenantID = strings.TrimSpace(req.TenantID)
	req.SourceEnvironment = strings.TrimSpace(req.SourceEnvironment)
	req.TargetEnvironment = strings.TrimSpace(req.TargetEnvironment)
	req.Scope = strings.TrimSpace(req.Scope)
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.Reason = strings.TrimSpace(req.Reason)
	req.Actor = strings.TrimSpace(req.Actor)
	if req.Module == "" || req.TenantID == "" || req.SourceEnvironment == "" || req.TargetEnvironment == "" || req.Scope == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing required fields"})
		return
	}
	if req.SourceEnvironment == req.TargetEnvironment {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "source and target environment must differ"})
		return
	}

	version, err := h.service.PromoteReleased(
		r.Context(),
		req.Module,
		req.TenantID,
		req.SourceEnvironment,
		req.TargetEnvironment,
		req.Scope,
		req.ProjectID,
		req.Actor,
		req.Reason,
	)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, controlplane.ErrReleasedConfigNotFound) {
			statusCode = http.StatusNotFound
		}
		writeJSON(w, statusCode, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, buildVersionResponse(version))
}

func (h *AdminHandler) handleListAuditEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	query := r.URL.Query()
	tenantID := query.Get("tenant_id")
	environment := query.Get("environment")
	if h.auditor == nil {
		if query.Get("summary") == "true" {
			writeJSON(w, http.StatusOK, auditSummaryResponse{Total: 0, ByType: map[string]int{}, ByEnvironment: map[string]int{}, TenantID: tenantID, Environment: environment})
			return
		}
		writeJSON(w, http.StatusOK, []audit.ControlPlaneEvent{})
		return
	}

	all := h.auditor.Events()
	limit := parseAdminLimit(query.Get("limit"))
	filtered := make([]audit.ControlPlaneEvent, 0)
	for _, event := range all {
		if tenantID != "" && event.TenantID != tenantID {
			continue
		}
		if environment != "" && event.Environment != environment {
			continue
		}
		filtered = append(filtered, event)
	}
	if query.Get("summary") == "true" {
		writeJSON(w, http.StatusOK, buildAuditSummary(filtered, len(all), tenantID, environment))
		return
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})
	writeJSON(w, http.StatusOK, applyAuditLimit(filtered, limit))
}

func buildAuditSummary(events []audit.ControlPlaneEvent, scannedTotal int, tenantID, environment string) auditSummaryResponse {
	resp := auditSummaryResponse{
		Total:         len(events),
		ByType:        map[string]int{},
		ByEnvironment: map[string]int{},
		ScannedTotal:  scannedTotal,
		TenantID:      tenantID,
		Environment:   environment,
	}
	if len(events) > 0 {
		latest, oldest := eventRangeFromAudit(events)
		resp.LatestAt = latest.Format(time.RFC3339)
		resp.OldestAt = oldest.Format(time.RFC3339)
	}
	for _, event := range events {
		resp.ByType[event.Type]++
		resp.ByEnvironment[event.Environment]++
	}
	return resp
}

func (h *AdminHandler) handleListRuntimeEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	query := r.URL.Query()
	tenantID := query.Get("tenant_id")
	environment := query.Get("environment")
	if h.runtime == nil {
		if query.Get("summary") == "true" {
			writeJSON(w, http.StatusOK, auditSummaryResponse{Total: 0, ByType: map[string]int{}, ByEnvironment: map[string]int{}, TenantID: tenantID, Environment: environment})
			return
		}
		writeJSON(w, http.StatusOK, []runtime.Event{})
		return
	}

	all := h.runtime.Events()
	limit := parseAdminLimit(query.Get("limit"))
	filtered := make([]runtime.Event, 0)
	for _, event := range all {
		if tenantID != "" && event.Version.TenantID != tenantID {
			continue
		}
		if environment != "" && event.Version.Environment != environment {
			continue
		}
		filtered = append(filtered, event)
	}
	if query.Get("summary") == "true" {
		writeJSON(w, http.StatusOK, buildRuntimeSummary(filtered, len(all), tenantID, environment))
		return
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Version.CreatedAt.After(filtered[j].Version.CreatedAt)
	})
	writeJSON(w, http.StatusOK, applyRuntimeLimit(filtered, limit))
}

func (h *AdminHandler) handleListVersions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	query := r.URL.Query()
	versions := h.service.ListVersions(
		r.Context(),
		query.Get("module"),
		query.Get("tenant_id"),
		query.Get("environment"),
		query.Get("scope"),
		query.Get("project_id"),
	)
	if query.Get("summary") == "true" {
		writeJSON(w, http.StatusOK, buildVersionSummary(versions))
		return
	}

	resp := make([]versionResponse, 0, len(versions))
	for _, version := range versions {
		resp = append(resp, buildVersionResponse(version))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AdminHandler) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	query := r.URL.Query()
	versionID := r.PathValue("versionID")
	version, err := h.service.GetVersion(
		r.Context(),
		query.Get("module"),
		query.Get("tenant_id"),
		query.Get("environment"),
		query.Get("scope"),
		query.Get("project_id"),
		versionID,
	)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, controlplane.ErrVersionNotFound) {
			statusCode = http.StatusNotFound
		}
		writeJSON(w, statusCode, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, buildVersionResponse(version))
}

func parseAdminLimit(raw string) int {
	if raw == "" {
		return 0
	}
	var limit int
	_, err := fmt.Sscanf(raw, "%d", &limit)
	if err != nil || limit <= 0 {
		return 0
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func applyAuditLimit(events []audit.ControlPlaneEvent, limit int) []audit.ControlPlaneEvent {
	if limit <= 0 || limit >= len(events) {
		return events
	}
	return events[:limit]
}

func buildRuntimeSummary(events []runtime.Event, scannedTotal int, tenantID, environment string) auditSummaryResponse {
	resp := auditSummaryResponse{
		Total:         len(events),
		ByType:        map[string]int{},
		ByEnvironment: map[string]int{},
		ScannedTotal:  scannedTotal,
		TenantID:      tenantID,
		Environment:   environment,
	}
	if len(events) > 0 {
		latest, oldest := eventRangeFromRuntime(events)
		resp.LatestAt = latest.Format(time.RFC3339)
		resp.OldestAt = oldest.Format(time.RFC3339)
	}
	for _, event := range events {
		resp.ByType[controlplane.ConfigStatusReleased]++
		resp.ByEnvironment[event.Version.Environment]++
	}
	return resp
}

func buildVersionSummary(versions []controlplane.ConfigVersion) versionSummaryResponse {
	resp := versionSummaryResponse{
		Total:         len(versions),
		ByStatus:      map[string]int{},
		ByEnvironment: map[string]int{},
		BySource:      map[string]int{},
	}
	if len(versions) > 0 {
		latest, oldest := versions[0].CreatedAt.UTC(), versions[0].CreatedAt.UTC()
		for _, version := range versions[1:] {
			createdAt := version.CreatedAt.UTC()
			if createdAt.After(latest) {
				latest = createdAt
			}
			if createdAt.Before(oldest) {
				oldest = createdAt
			}
		}
		resp.LatestAt = latest.Format(time.RFC3339)
		resp.OldestAt = oldest.Format(time.RFC3339)
	}
	for _, version := range versions {
		status := controlplane.ConfigStatusDraft
		if version.Source == controlplane.ConfigStatusReleased {
			status = controlplane.ConfigStatusReleased
		}
		resp.ByStatus[status]++
		resp.ByEnvironment[version.Environment]++

		source := version.Source
		if strings.TrimSpace(source) == "" {
			source = "unspecified"
		}
		resp.BySource[source]++
	}
	return resp
}

func eventRangeFromAudit(events []audit.ControlPlaneEvent) (time.Time, time.Time) {
	latest, oldest := events[0].CreatedAt.UTC(), events[0].CreatedAt.UTC()
	for _, event := range events[1:] {
		createdAt := event.CreatedAt.UTC()
		if createdAt.After(latest) {
			latest = createdAt
		}
		if createdAt.Before(oldest) {
			oldest = createdAt
		}
	}
	return latest, oldest
}

func eventRangeFromRuntime(events []runtime.Event) (time.Time, time.Time) {
	latest, oldest := events[0].Version.CreatedAt.UTC(), events[0].Version.CreatedAt.UTC()
	for _, event := range events[1:] {
		createdAt := event.Version.CreatedAt.UTC()
		if createdAt.After(latest) {
			latest = createdAt
		}
		if createdAt.Before(oldest) {
			oldest = createdAt
		}
	}
	return latest, oldest
}

func applyRuntimeLimit(events []runtime.Event, limit int) []runtime.Event {
	if limit <= 0 || limit >= len(events) {
		return events
	}
	return events[:limit]
}

func buildVersionResponse(version controlplane.ConfigVersion) versionResponse {
	resp := versionResponse{
		VersionID:   version.Version,
		Environment: version.Environment,
	}
	if version.Source == controlplane.ConfigStatusReleased {
		resp.Status = controlplane.ConfigStatusReleased
	} else {
		resp.Status = controlplane.ConfigStatusDraft
	}
	if version.SourceEnvironment != "" && version.SourceVersion != "" {
		resp.Source = &inheritanceDraftSourceResponse{
			Type:              version.Source,
			SourceEnvironment: version.SourceEnvironment,
			SourceVersionID:   version.SourceVersion,
		}
	}
	return resp
}
