package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"llm-gateway/gateway/internal/longcontext"
)

type longContextChunkPersister interface {
	PersistChunks(context.Context, string, string, string, int, int) error
}

type LongContextHandler struct {
	repo          longcontext.Repository
	enabled       bool
	maxInputBytes int
}

func NewLongContextHandler(repo longcontext.Repository, enabled bool, maxInputBytes int) *LongContextHandler {
	if maxInputBytes <= 0 {
		maxInputBytes = 32 << 20
	}
	return &LongContextHandler{repo: repo, enabled: enabled, maxInputBytes: maxInputBytes}
}

func (h *LongContextHandler) Repo() longcontext.Repository { return h.repo }

func (h *LongContextHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.enabled {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "long context is disabled", "type": "long_context_disabled"}})
		return
	}
	if h.repo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": map[string]any{"message": "long context repository unavailable", "type": "service_unavailable"}})
		return
	}
	base := strings.TrimSuffix("/v1/long-context/tasks", "/")
	path := strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/"), base)
	if path == "" {
		switch r.Method {
		case http.MethodPost:
			h.create(w, r)
			return
		case http.MethodGet:
			h.list(w, r)
			return
		}
		methodNotAllowed(w, r)
		return
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 1 && r.Method == http.MethodGet {
		h.get(w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "cancel" && r.Method == http.MethodPost {
		h.cancel(w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "result" && r.Method == http.MethodGet {
		h.result(w, r, parts[0])
		return
	}
	methodNotAllowed(w, r)
}

type longContextCreateRequest struct {
	Model string `json:"model"`
	Mode  string `json:"mode"`
	Query string `json:"query"`
	Input struct {
		Text string `json:"text"`
	} `json:"input"`
	Options struct {
		WorkerCount        int      `json:"worker_count"`
		RetrievalTopK      int      `json:"retrieval_top_k"`
		FinalContextBudget int      `json:"final_context_budget"`
		ChunkTokens        int      `json:"chunk_tokens"`
		ChunkOverlapTokens int      `json:"chunk_overlap_tokens"`
		MaxCost            *float64 `json:"max_cost"`
	} `json:"options"`
	TenantID  string `json:"tenant_id"`
	SessionID string `json:"session_id"`
}

func (h *LongContextHandler) create(w http.ResponseWriter, r *http.Request) {
	var in longContextCreateRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, int64(h.maxInputBytes)+1024*1024))
	if err := dec.Decode(&in); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	if in.Model == "" {
		in.Model = "virtual-long-1m"
	}
	if in.Mode == "" {
		in.Mode = "qa"
	}
	if in.Options.WorkerCount == 0 {
		in.Options.WorkerCount = 1
	}
	if in.Options.RetrievalTopK == 0 {
		in.Options.RetrievalTopK = 32
	}
	if in.Options.FinalContextBudget == 0 {
		in.Options.FinalContextBudget = 180000
	}
	if strings.TrimSpace(in.TenantID) == "" {
		badRequest(w, "tenant_id is required")
		return
	}
	if err := longcontext.ValidateCreate(longcontext.CreateTaskInput{Model: in.Model, Mode: in.Mode, Query: in.Query, WorkerCount: in.Options.WorkerCount, RetrievalTopK: in.Options.RetrievalTopK, FinalBudget: in.Options.FinalContextBudget}, h.maxInputBytes, len(in.Input.Text)); err != nil {
		badRequest(w, err.Error())
		return
	}
	// Enforce per-tenant concurrency limit before accepting the task.
	if pr, ok := h.repo.(*longcontext.PostgresRepository); ok {
		if err := longcontext.EnforceTenantConcurrency(r.Context(), pr, strings.TrimSpace(in.TenantID)); err != nil {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": map[string]any{"message": err.Error(), "type": "concurrency_limit"}})
			return
		}
	}
	claims := getUserClaims(r.Context())
	userID := "admin"
	if claims != nil && claims.UserID != 0 {
		userID = fmt.Sprintf("%d", claims.UserID)
	}
	ci := longcontext.CreateTaskInput{ID: "lct_" + uuid.NewString(), TenantID: strings.TrimSpace(in.TenantID), UserID: userID, SessionID: strings.TrimSpace(in.SessionID), IdempotencyKey: strings.TrimSpace(r.Header.Get("Idempotency-Key")), Model: in.Model, Mode: in.Mode, Query: in.Query, InputSHA256: longcontext.HashInput(in.Input.Text), InputTokens: int64(len(in.Input.Text) / 4), WorkerCount: in.Options.WorkerCount, RetrievalTopK: in.Options.RetrievalTopK, FinalBudget: in.Options.FinalContextBudget, MaxCost: in.Options.MaxCost}
	t, existing, err := h.repo.CreateTask(r.Context(), ci)
	if err == nil && !existing {
		if p, ok := h.repo.(longContextChunkPersister); ok {
			chunkTokens, overlap := in.Options.ChunkTokens, in.Options.ChunkOverlapTokens
			if chunkTokens == 0 {
				chunkTokens = 16000
			}
			if overlap == 0 {
				overlap = 800
			}
			if perr := p.PersistChunks(r.Context(), t.ID, ci.TenantID, in.Input.Text, chunkTokens, overlap); perr != nil {
				_ = h.repo.CancelTask(r.Context(), t.ID, ci.TenantID)
				err = fmt.Errorf("persist chunks: %w", perr)
			}
		}
	}
	if err != nil {
		if strings.Contains(err.Error(), "idempotency") {
			writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{"message": err.Error(), "type": "idempotency_conflict"}})
		} else {
			internalError(w, err)
		}
		return
	}
	status := http.StatusAccepted
	if existing {
		status = http.StatusOK
	}
	u := "/v1/long-context/tasks/" + t.ID
	w.Header().Set("Location", u)
	writeJSON(w, status, map[string]any{"id": t.ID, "status": t.Status, "phase": t.Phase, "model": t.Model, "status_url": u, "result_url": u + "/result"})
}
func (h *LongContextHandler) list(w http.ResponseWriter, r *http.Request) {
	tenant := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	if tenant == "" {
		badRequest(w, "tenant_id is required")
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	tasks, total, err := h.repo.ListTasks(r.Context(), tenant, limit, offset)
	if err != nil {
		internalError(w, err)
		return
	}
	items := make([]map[string]any, len(tasks))
	for i, t := range tasks {
		items[i] = taskResponse(t)
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": items, "total": total, "limit": limit, "offset": offset})
}

func (h *LongContextHandler) task(w http.ResponseWriter, r *http.Request, id string) (*longcontext.Task, bool) {
	tenant := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	if tenant == "" {
		badRequest(w, "tenant_id is required")
		return nil, false
	}
	t, err := h.repo.GetTask(r.Context(), id, tenant)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "task not found", "type": "not_found"}})
		return nil, false
	}
	return t, true
}
func (h *LongContextHandler) get(w http.ResponseWriter, r *http.Request, id string) {
	if t, ok := h.task(w, r, id); ok {
		writeJSON(w, http.StatusOK, taskResponse(t))
	}
}
func (h *LongContextHandler) cancel(w http.ResponseWriter, r *http.Request, id string) {
	t, ok := h.task(w, r, id)
	if !ok {
		return
	}
	if err := h.repo.CancelTask(r.Context(), t.ID, t.TenantID); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{"message": err.Error(), "type": "cancel_error"}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": t.ID, "status": longcontext.StatusCanceled})
}
func (h *LongContextHandler) result(w http.ResponseWriter, r *http.Request, id string) {
	t, ok := h.task(w, r, id)
	if !ok {
		return
	}
	if t.Status != longcontext.StatusSucceeded {
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{"message": "task result is not ready", "type": "result_not_ready"}, "status": t.Status})
		return
	}
	writeJSON(w, http.StatusOK, t.Result)
}
func taskResponse(t *longcontext.Task) map[string]any {
	return map[string]any{"id": t.ID, "status": t.Status, "phase": t.Phase, "model": t.Model, "chunk_count": t.ChunkCount, "completed_chunks": t.CompletedChunks, "used_tokens": t.UsedTokens, "estimated_cost": t.EstimatedCost, "retry_count": t.RetryCount, "last_error": t.LastError, "result": t.Result, "created_at": t.CreatedAt, "updated_at": t.UpdatedAt}
}
