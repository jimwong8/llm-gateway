package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"llm-gateway/gateway/internal/memory"
)

type memoryAdminStore interface {
	ListCandidateFacts(ctx context.Context, tenantID, userID, status string) ([]memory.CandidateFact, error)
	ListProjectFacts(ctx context.Context, tenantID, userID, status string) ([]memory.ProjectFact, error)
	ConfirmCandidateFact(ctx context.Context, tenantID, userID, factKey string) (*memory.CandidateFact, error)
	RejectCandidateFact(ctx context.Context, tenantID, userID, factKey string) (*memory.CandidateFact, error)
	PromoteCandidateFact(ctx context.Context, tenantID, userID, factKey string) (*memory.CandidateFact, error)
}

type MemoryAdminHandler struct {
	store memoryAdminStore
}

func NewMemoryAdminHandler(store memoryAdminStore) *MemoryAdminHandler {
	return &MemoryAdminHandler{store: store}
}

func (h *MemoryAdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "memory store unavailable"})
		return
	}

	path := r.URL.Path
	switch {
	case path == "/admin/memory/candidate-facts":
		h.handleCandidateFacts(w, r)
	case path == "/admin/memory/project-facts":
		h.handleProjectFacts(w, r)
	case strings.HasPrefix(path, "/admin/memory/candidate-facts/actions/"):
		h.handleCandidateFactBatchActions(w, r)
	case strings.HasPrefix(path, "/admin/memory/candidate-facts/"):
		h.handleCandidateFactActions(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "route not found", "type": "not_found_error", "path": path}})
	}
}

func (h *MemoryAdminHandler) handleCandidateFacts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	rows, err := h.store.ListCandidateFacts(r.Context(), tenantID, userID, status)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"object":    "list",
		"tenant_id": tenantID,
		"user_id":   userID,
		"status":    status,
		"data":      rows,
	})
}

func (h *MemoryAdminHandler) handleProjectFacts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	rows, err := h.store.ListProjectFacts(r.Context(), tenantID, userID, status)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"object":    "list",
		"tenant_id": tenantID,
		"user_id":   userID,
		"status":    status,
		"data":      rows,
	})
}

type candidateFactActionRequest struct {
	TenantID string `json:"tenant_id"`
	UserID   string `json:"user_id"`
}

type candidateFactBatchActionItem struct {
	TenantID string `json:"tenant_id"`
	UserID   string `json:"user_id"`
	FactKey  string `json:"fact_key"`
}

type candidateFactBatchActionRequest struct {
	Items []candidateFactBatchActionItem `json:"items"`
}

type memoryAdminErrorPayload struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

type candidateFactBatchActionResult struct {
	FactKey  string                   `json:"fact_key"`
	TenantID string                   `json:"tenant_id,omitempty"`
	UserID   string                   `json:"user_id,omitempty"`
	Status   string                   `json:"status,omitempty"`
	Fact     *memory.CandidateFact    `json:"fact,omitempty"`
	Error    *memoryAdminErrorPayload `json:"error,omitempty"`
}

type candidateFactBatchActionResponse struct {
	Action       string                           `json:"action"`
	SuccessCount int                              `json:"success_count"`
	FailureCount int                              `json:"failure_count"`
	Results      []candidateFactBatchActionResult `json:"results"`
}

func (h *MemoryAdminHandler) handleCandidateFactActions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/admin/memory/candidate-facts/"), "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "route not found", "type": "not_found_error", "path": r.URL.Path}})
		return
	}

	factKey := strings.TrimSpace(parts[0])
	action := strings.TrimSpace(parts[1])

	var body candidateFactActionRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	body.TenantID = strings.TrimSpace(body.TenantID)
	body.UserID = strings.TrimSpace(body.UserID)
	if body.UserID == "" {
		badRequest(w, "user_id is required")
		return
	}

	fact, err := h.executeCandidateFactAction(r.Context(), action, body.TenantID, body.UserID, factKey)
	if err != nil {
		writeMemoryAdminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, fact)
}

func (h *MemoryAdminHandler) handleCandidateFactBatchActions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/admin/memory/candidate-facts/actions/"), "/")
	if len(parts) != 1 || strings.TrimSpace(parts[0]) == "" {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "route not found", "type": "not_found_error", "path": r.URL.Path}})
		return
	}
	action := strings.TrimSpace(parts[0])

	var body candidateFactBatchActionRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}

	items := normalizeCandidateFactBatchItems(body.Items)
	if len(items) == 0 {
		badRequest(w, "at least one candidate fact item is required")
		return
	}

	response := candidateFactBatchActionResponse{
		Action:  action,
		Results: make([]candidateFactBatchActionResult, 0, len(items)),
	}
	for _, item := range items {
		if item.UserID == "" {
			badRequest(w, "user_id is required")
			return
		}
		if item.FactKey == "" {
			badRequest(w, "fact_key is required")
			return
		}

		result := candidateFactBatchActionResult{
			FactKey:  item.FactKey,
			TenantID: item.TenantID,
			UserID:   item.UserID,
		}
		fact, err := h.executeCandidateFactAction(r.Context(), action, item.TenantID, item.UserID, item.FactKey)
		if err != nil {
			_, payload := memoryAdminErrorResponse(err)
			result.Error = &payload
			response.FailureCount++
			response.Results = append(response.Results, result)
			continue
		}
		result.Status = fact.Status
		result.Fact = fact
		response.SuccessCount++
		response.Results = append(response.Results, result)
	}

	writeJSON(w, http.StatusOK, response)
}

func normalizeCandidateFactBatchItems(items []candidateFactBatchActionItem) []candidateFactBatchActionItem {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	cleaned := make([]candidateFactBatchActionItem, 0, len(items))
	for _, item := range items {
		item.TenantID = strings.TrimSpace(item.TenantID)
		item.UserID = strings.TrimSpace(item.UserID)
		item.FactKey = strings.TrimSpace(item.FactKey)
		key := item.TenantID + "\x00" + item.UserID + "\x00" + item.FactKey
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		cleaned = append(cleaned, item)
	}
	return cleaned
}

func (h *MemoryAdminHandler) executeCandidateFactAction(ctx context.Context, action, tenantID, userID, factKey string) (*memory.CandidateFact, error) {
	switch action {
	case "confirm":
		return h.store.ConfirmCandidateFact(ctx, tenantID, userID, factKey)
	case "reject":
		return h.store.RejectCandidateFact(ctx, tenantID, userID, factKey)
	case "promote":
		return h.store.PromoteCandidateFact(ctx, tenantID, userID, factKey)
	default:
		return nil, errors.New("route not found")
	}
}

func writeMemoryAdminError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	status, payload := memoryAdminErrorResponse(err)
	writeJSON(w, status, map[string]any{"error": payload})
}

func memoryAdminErrorResponse(err error) (int, memoryAdminErrorPayload) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, memory.ErrCandidateFactNotFound):
		status = http.StatusNotFound
	case errors.Is(err, memory.ErrInvalidCandidateFactTransition):
		status = http.StatusBadRequest
	case err != nil && err.Error() == "route not found":
		status = http.StatusNotFound
	}
	return status, memoryAdminErrorPayload{Message: err.Error(), Type: "memory_governance_error"}
}
