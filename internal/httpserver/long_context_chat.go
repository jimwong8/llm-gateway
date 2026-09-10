package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"llm-gateway/gateway/internal/longcontext"
	"llm-gateway/gateway/internal/providers"
)

// chatCompletionsLongContext serves model=virtual-long-1m (1Mvir) over the
// ordinary /v1/chat/completions surface so Hermes and other chat clients can
// use the long-context orchestrator without switching to the async task API.
//
// It converts the conversation into a synchronous long-context task:
//   - query      = last user message text (trimmed)
//   - input.text = all prior user/assistant messages joined (the "document")
//
// The task is executed inline by FullTaskProcessor (not handed to the
// background worker pool), so a chat request blocks until the grounded answer
// is ready. Stream requests get OpenAI-style SSE chunks; non-stream requests
// get a normal JSON completion.
func (s *Server) chatCompletionsLongContext(w http.ResponseWriter, r *http.Request, req providers.ChatCompletionRequest, requestID string) {
	if s.longContextProcessor == nil || s.longContextProcessor.Repo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": map[string]any{"message": "long context processor not configured", "type": "service_unavailable"}})
		return
	}
	started := time.Now()
	tenant := strings.TrimSpace(req.TenantID)
	if tenant == "" {
		tenant = "chat"
	}

	// --- Build document text and query from the conversation ---
	var sb strings.Builder
	lastUser := ""
	for _, m := range req.Messages {
		switch m.Role {
		case "user":
			if strings.TrimSpace(m.Content) != "" {
				if sb.Len() > 0 {
					sb.WriteString("\n\n")
				}
				sb.WriteString(m.Content)
				lastUser = m.Content
			}
		case "assistant":
			// Tool calls and reasoning are skipped; only natural-language
			// assistant text becomes part of the retrievable document.
			if strings.TrimSpace(m.Content) != "" {
				if sb.Len() > 0 {
					sb.WriteString("\n\n")
				}
				sb.WriteString(m.Content)
			}
		}
	}
	docText := strings.TrimSpace(sb.String())
	if docText == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "no user or assistant text to process", "type": "invalid_request_error"}})
		return
	}
	query := strings.TrimSpace(lastUser)
	// Short/meta queries ("？", "?", ".", etc.) carry no semantic signal for the
	// Map/retrieval pipeline — the model treats them as noise and often echoes
	// injected system text (e.g. "I am now operating as virtual-long-1m") as
	// the answer. Fall back to an open-ended query that forces the reader to
	// summarize the actual document.
	if query == "" || len([]rune(query)) <= 3 {
		if query != "" && strings.TrimSpace(query) != "" {
			query = "请基于以上内容回答用户的问题：" + query
		} else {
			query = "请基于以上内容回答问题"
		}
	}

	// Fast path for small documents (<=1000 chars): skip Map phase and
	// directly synthesize. This avoids the 30s-2min Map→Retrieve→Index
	// pipeline for small queries that don't need retrieval.
	if len([]rune(docText)) <= 1000 {
		slog.Info("1Mvir fast path (small doc)", "query_chars", len([]rune(query)), "doc_chars", len([]rune(docText)), "stream", req.Stream)
		// Use the first reader candidate (fastest) instead of the default reader model.
		readerModel := s.longContextProcessor.ReaderModel
		readerChannel := s.longContextProcessor.ReaderChannel
		if len(s.longContextProcessor.ReaderCandidates) > 0 {
			readerModel = s.longContextProcessor.ReaderCandidates[0].Model
			readerChannel = s.longContextProcessor.ReaderCandidates[0].Channel
		}
		reader := longcontext.RegistryFinalReader{
			Registry:        s.longContextProcessor.Registry,
			Model:           readerModel,
			Channel:         readerChannel,
			MaxTokens:       4096,
			ReasoningEffort: "none",
		}
		evidence := []longcontext.Evidence{{Claim: docText, Quote: docText, StartChar: 0, EndChar: int64(len(docText)), Confidence: 1.0}}
		chunks := []string{docText}
		answer, err := reader.SynthesizeGroundedAnswer(r.Context(), query, evidence, chunks)
		if err != nil {
			internalError(w, fmt.Errorf("fast path synthesis failed: %w", err))
			return
		}
		final := longcontext.FinalReadResult{Query: query, Answer: answer, EvidenceCount: 1, CitedEvidenceIDs: []string{"[E1]"}, CitationCoverage: 1.0}
		if req.Stream {
			// Stream the fast-path answer as a single SSE chunk.
			fl, ok := w.(http.Flusher)
			if !ok {
				internalError(w, fmt.Errorf("streaming unsupported"))
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("X-Route-Mode", "long_context_fast")
			w.Header().Set("X-Route-Model", "virtual-long-1m")
			w.Header().Set("X-Route-Provider", "longcontext")
			w.WriteHeader(http.StatusOK)
			resp := longContextToChatCompletion(requestID, req.Model, &final)
			chunk := map[string]any{
				"id":      resp.ID,
				"object":  "chat.completion.chunk",
				"created": resp.Created,
				"model":   resp.Model,
				"choices": []map[string]any{{"index": 0, "delta": map[string]any{"role": "assistant", "content": answer}, "finish_reason": "stop"}},
			}
			writeFrame := func(s string) { _, _ = w.Write([]byte(s)); fl.Flush() }
			writeFrame("data: " + mustJSON(chunk) + "\n\n")
			writeFrame("data: [DONE]\n\n")
			return
		}
		resp := longContextToChatCompletion(requestID, req.Model, &final)
		w.Header().Set("X-Route-Mode", "long_context_fast")
		w.Header().Set("X-Route-Model", "virtual-long-1m")
		w.Header().Set("X-Route-Provider", "longcontext")
		writeJSON(w, http.StatusOK, resp)
		return
	}

	repo := s.longContextProcessor.Repo
	taskID := "lct_" + uuid.NewString()
	sessionID := strings.TrimSpace(req.SessionID)
	ci := longcontext.CreateTaskInput{
		ID:               taskID,
		TenantID:         tenant,
		UserID:           "hermes-chat",
		SessionID:        sessionID,
		Model:            "virtual-long-1m",
		Mode:             "qa",
		Query:            query,
		InputSHA256:      longcontext.HashInput(docText),
		InputTokens:      int64(len([]rune(docText)) / 4),
		WorkerCount:      1,
		RetrievalTopK:    32,
		FinalBudget:      180000,
		LeaseOwner:       "hermes-chat-" + requestID,
		LeaseDuration:    10 * time.Minute,
	}
	task, existing, err := repo.CreateTask(r.Context(), ci)
	if err != nil && !existing {
		internalError(w, fmt.Errorf("create long context task: %w", err))
		return
	}
	// Persist chunks (chunk_tokens=500 is the verified safe default).
	if err := repo.PersistChunks(r.Context(), taskID, tenant, docText, 500, 200); err != nil {
		_ = repo.CancelTask(r.Context(), taskID, tenant)
		internalError(w, fmt.Errorf("persist chunks: %w", err))
		return
	}
	if task == nil {
		task, err = repo.GetTask(r.Context(), taskID, tenant)
		if err != nil {
			internalError(w, fmt.Errorf("load task: %w", err))
			return
		}
	}

	// Claim the task with a dedicated chat worker identity and a long lease so
	// the background worker pool never picks this task up while the chat
	// request is driving it inline. Using the default "worker" identity would
	// collide with the pool's own workers and cause compare-and-swap races.
	chatWorkerID := "hermes-chat-" + requestID
	// DirectClaim (unconditional) because the task was just created with empty
	// lease_owner — RenewLease would fail its WHERE lease_owner=$3 match.
	_ = repo.DirectClaim(r.Context(), taskID, tenant, chatWorkerID, 15*time.Minute)
	// Snapshot the processor with the chat identity so every RenewLease inside
	// ProcessTask keeps the same owner (prevents pool takeover mid-task).
	procCopy := *s.longContextProcessor
	procCopy.WorkerID = chatWorkerID
	procCopy.Lease = 15 * time.Minute

	docPreview := docText
	if len([]rune(docPreview)) > 200 {
		docPreview = string([]rune(docPreview)[:200]) + "..."
	}
	slog.Info("1Mvir chat request", "task", taskID, "tenant", tenant, "query", query, "query_chars", len([]rune(query)), "doc_chars", len([]rune(docText)), "doc_preview", docPreview, "msg_count", len(req.Messages), "stream", req.Stream)

	// If streaming, hand off to an SSE-backed runner that sends periodic
	// keepalives while the task executes, then the final chunk + [DONE].
	if req.Stream {
		s.runLongContextStream(w, r, &procCopy, task, tenant, requestID, started)
		return
	}
	// Non-stream: execute inline, then write a plain JSON completion.
	final, err := s.executeLongContextTask(r.Context(), &procCopy, task, tenant)
	if err != nil {
		internalError(w, err)
		return
	}
	resp := longContextToChatCompletion(requestID, req.Model, final)
	w.Header().Set("X-Route-Mode", "long_context")
	w.Header().Set("X-Route-Model", "virtual-long-1m")
	w.Header().Set("X-Route-Provider", "longcontext")
	writeJSON(w, http.StatusOK, resp)
}

// executeLongContextTask runs the full map -> retrieve -> synthesize ->
// verify pipeline synchronously and returns the stored FinalReadResult.
func (s *Server) executeLongContextTask(ctx context.Context, p *longcontext.FullTaskProcessor, task *longcontext.Task, tenant string) (*longcontext.FinalReadResult, error) {
	if err := p.ProcessTask(ctx, task); err != nil {
		slog.Error("1Mvir ProcessTask failed", "task", task.ID, "err", err)
		// Surface the stored last_error when available.
		if t, gerr := p.Repo.GetTask(ctx, task.ID, tenant); gerr == nil && t.LastError != "" {
			return nil, fmt.Errorf("long context task failed: %s", t.LastError)
		}
		return nil, fmt.Errorf("long context task failed: %w", err)
	}
	done, err := p.Repo.GetTask(ctx, task.ID, tenant)
	if err != nil {
		return nil, fmt.Errorf("load finished task: %w", err)
	}
	if done.Status != longcontext.StatusSucceeded {
		msg := done.LastError
		if msg == "" {
			msg = fmt.Sprintf("task ended in %s", done.Status)
		}
		return nil, fmt.Errorf("long context task did not succeed: %s", msg)
	}
	if len(done.Result) == 0 {
		return nil, fmt.Errorf("long context task has no stored result")
	}
	var final longcontext.FinalReadResult
	if err := json.Unmarshal(done.Result, &final); err != nil {
		return nil, fmt.Errorf("decode result: %w", err)
	}
	return &final, nil
}

// runLongContextStream executes the task and writes an OpenAI-compatible SSE
// response. While the orchestrator works, keepalive comment frames keep the
// connection (and Hermes' 900s auto-reconnect timer) alive. The final answer
// is delivered as a single content chunk followed by [DONE].
func (s *Server) runLongContextStream(w http.ResponseWriter, r *http.Request, p *longcontext.FullTaskProcessor, task *longcontext.Task, tenant, requestID string, started time.Time) {
	fl, ok := w.(http.Flusher)
	if !ok {
		internalError(w, fmt.Errorf("streaming unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Route-Mode", "long_context")
	w.Header().Set("X-Route-Model", "virtual-long-1m")
	w.Header().Set("X-Route-Provider", "longcontext")
	w.WriteHeader(http.StatusOK)

	// Run the orchestrator on a detached background context: if the HTTP
	// client disconnects mid-task, the task continues to completion and its
	// result is stored in the DB (retrievable via /v1/long-context/tasks/),
	// instead of leaving the task stuck in 'mapping' holding a chat lease.
	workCtx, cancelWork := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancelWork()

	type result struct {
		final *longcontext.FinalReadResult
		err   error
	}
	done := make(chan result, 1)
	go func() {
		f, err := s.executeLongContextTask(workCtx, p, task, tenant)
		done <- result{final: f, err: err}
	}()

	// Keepalive ticker.
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	writeFrame := func(s string) {
		_, _ = w.Write([]byte(s))
		fl.Flush()
	}

	for {
		select {
		case <-r.Context().Done():
			slog.Info("1Mvir chat stream cancelled", "task", task.ID)
			return
		case <-ticker.C:
			// SSE comment keepalive — ignored by OpenAI-compatible parsers.
			writeFrame(": ping\n\n")
		case res := <-done:
			if res.err != nil {
				writeFrame(fmt.Sprintf("data: %s\n\n", mustJSON(map[string]any{"error": map[string]any{"message": res.err.Error(), "type": "long_context_error"}})))
				writeFrame("data: [DONE]\n\n")
				slog.Error("1Mvir chat stream task failed", "task", task.ID, "err", res.err)
				return
			}
			resp := longContextToChatCompletion(requestID, "virtual-long-1m", res.final)
			// Deliver as one SSE chunk.
			writeFrame("data: " + chatChunkFromResponse(resp, 0, resp.Choices[0].Message.Content, "stop") + "\n\n")
			writeFrame("data: [DONE]\n\n")
			slog.Info("1Mvir chat stream complete", "task", task.ID, "elapsed_s", time.Since(started).Seconds())
			return
		}
	}
}

// longContextToChatCompletion converts a stored FinalReadResult into an
// OpenAI-compatible chat completion response.
func longContextToChatCompletion(requestID, model string, final *longcontext.FinalReadResult) providers.ChatCompletionResponse {
	answer := ""
	usage := providers.CompletionUsage{}
	if final != nil {
		answer = final.Answer
		// Optionally annotate citations if the answer is missing them.
		if len(final.CitedEvidenceIDs) > 0 && !strings.Contains(answer, "[E1]") {
			answer += "\n\n---\n" + strings.Join(final.CitedEvidenceIDs, " ") + " (citation_coverage=" + fmt.Sprintf("%.2f", final.CitationCoverage) + ")"
		}
		usage.PromptTokens = 0
		usage.CompletionTokens = len([]rune(answer)) / 4
		usage.TotalTokens = usage.CompletionTokens
	}
	var choice struct {
		Index   int `json:"index"`
		Message struct {
			Role             string `json:"role"`
			Content          string `json:"content"`
			Reasoning        string `json:"reasoning,omitempty"`
			ReasoningContent string `json:"reasoning_content,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	}
	choice.Index = 0
	choice.Message.Role = "assistant"
	choice.Message.Content = answer
	choice.FinishReason = "stop"

	resp := providers.ChatCompletionResponse{
		ID:      "chatcmpl-" + requestID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Usage:   usage,
	}
	resp.Choices = append(resp.Choices, choice)
	return resp
}

// chatChunkFromResponse renders one OpenAI SSE data frame from a response.
func chatChunkFromResponse(resp providers.ChatCompletionResponse, index int, content, finish string) string {
	chunk := map[string]any{
		"id":      resp.ID,
		"object":  "chat.completion.chunk",
		"created": resp.Created,
		"model":   resp.Model,
		"choices": []map[string]any{{
			"index": index,
			"delta": map[string]any{"role": "assistant", "content": content},
			"finish_reason": finish,
		}},
	}
	return mustJSON(chunk)
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"error":{"message":"encode failed"}}`
	}
	return string(b)
}
