package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"llm-gateway/gateway/internal/providers"
)

// Council Mode: multi-model parallel consensus (simplified Council Mode,
// arXiv 2604.02923). Model "council" fans the request out to N
// provider-family-diverse models in parallel, then a synthesizer model
// aggregates their answers into a structured consensus response.
//
// Design decisions grounded in the literature survey (2026-09):
//   - Parallel vote + single synthesis round: NeurIPS 2025 proved debate
//     rounds add no expected correctness beyond majority voting, so we do
//     exactly one fan-out + one synthesis. No multi-round debate.
//   - Family diversity beats individual quality: DeepSeek-family +
//     Zhipu-family + Alibaba-family candidates have lower correlated errors
//     than three same-family models.
//   - Degradation is graceful: a candidate that fails (rate gate, 5xx,
//     timeout) is skipped; synthesis requires >=2 usable answers, else the
//     best single answer is returned with X-Council-Degraded headers.
//   - Council is opt-in: only requests with model=council trigger it. AUTO
//     routing is untouched.

const councilDefaultCandidatesEnv = "deepseek-v4-flash,glm-5.2,qwen3.8-flash"
const councilDefaultSynthesizerEnv = "sensenova-6.8-flash-lite"

var councilSynthesizerPrompt = `You are a consensus synthesis model. You will receive several independent answers to the same question from different AI models. Produce a synthesized final answer with EXACTLY these four sections in this order, using the given section headers:

【共识】Points where the models agree (supported by 2+ models).
【分歧】Points where the models disagree; include each model's position briefly.
【独有】Valuable unique insights only one model provided.
【综合】The final comprehensive answer integrating all evidence. It must directly answer the user's question and must not introduce claims absent from the model answers.

Rules: cite which model (A/B/C) supports each point; do not mention that you are synthesizing; keep the user's original language.

`

const councilRequestTimeout = 90 * time.Second
const councilMinAnswers = 2

func (s *Server) chatCompletionsCouncil(w http.ResponseWriter, r *http.Request, req providers.ChatCompletionRequest, requestID string) {
	started := time.Now()

	// ---- Phase 0: resolve candidates (config/env override supported) ----
	candidates := []string{"deepseek-v4-flash", "glm-5.2", "qwen3.8-flash"}
	if v := strings.TrimSpace(s.cfg.CouncilCandidates); v != "" {
		parts := strings.Split(v, ",")
		candidates = make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				candidates = append(candidates, p)
			}
		}
	}
	synth := councilDefaultSynthesizerEnv
	if v := strings.TrimSpace(s.cfg.CouncilSynthesizer); v != "" {
		synth = v
	}

	// ---- Phase 1: parallel fan-out ----
	type councilAnswer struct {
		Model     string
		Channel   string
		Content   string
		Error     string
		LatencyMS int64
	}
	answers := make([]councilAnswer, len(candidates))
	var wg sync.WaitGroup
	for i, m := range candidates {
		wg.Add(1)
		go func(i int, model string) {
			defer wg.Done()
			t0 := time.Now()
			sub := req
			sub.Model = model
			sub.Stream = false
			ctx, cancel := context.WithTimeout(r.Context(), councilRequestTimeout)
			defer cancel()
			resp, channel, err := s.providers.ChatCompletionOnModel(ctx, model, sub)
			answers[i] = councilAnswer{
				Model:     model,
				Channel:   channel,
				LatencyMS: time.Since(t0).Milliseconds(),
			}
			if err != nil {
				answers[i].Error = err.Error()
				return
			}
			if len(resp.Choices) > 0 {
				answers[i].Content = resp.Choices[0].Message.Content
			}
			if strings.TrimSpace(answers[i].Content) == "" {
				answers[i].Error = "empty content"
			}
		}(i, m)
	}
	wg.Wait()

	// ---- Phase 2: collect usable answers ----
	var usable []councilAnswer
	var failed []string
	for _, a := range answers {
		if a.Error == "" && strings.TrimSpace(a.Content) != "" {
			usable = append(usable, a)
		} else {
			failed = append(failed, fmt.Sprintf("%s(%s)", a.Model, a.Error))
		}
	}

	degraded := false
	if len(usable) == 0 {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": map[string]any{
			"message":       "council: all candidate models failed",
			"type":          "upstream_error",
			"failed_models": failed,
			"request_id":    requestID,
		}})
		return
	}
	if len(usable) < councilMinAnswers {
		// Degraded: single best answer pass-through (fastest usable one).
		degraded = true
		sort.Slice(usable, func(a, b int) bool { return usable[a].LatencyMS < usable[b].LatencyMS })
	}

	// Build the final answer content.
	var finalContent string
	var usedModel string
	var synthChannel string
	if degraded {
		finalContent = usable[0].Content
		usedModel = usable[0].Model
		synthChannel = usable[0].Channel
	} else {
		// ---- Phase 3: consensus synthesis ----
		var sb strings.Builder
		labels := []string{"A", "B", "C", "D", "E"}
		for i, a := range usable {
			lbl := fmt.Sprintf("Model-%s", labels[i%len(labels)])
			sb.WriteString(fmt.Sprintf("--- %s (%s) ---\n%s\n\n", lbl, a.Model, a.Content))
		}
		synthReq := providers.ChatCompletionRequest{
			Model: synth,
			Messages: []providers.ChatMessage{{
				Role: "user",
				Content: councilSynthesizerPrompt +
					"Question:\n" + lastUserText(req) +
					"\n\nModel answers:\n\n" + sb.String() +
					"\nSynthesize the final answer now:",
			}},
			MaxTokens: req.MaxTokens,
		}
		ctx, cancel := context.WithTimeout(r.Context(), councilRequestTimeout)
		defer cancel()
		resp, channel, err := s.providers.ChatCompletionOnModel(ctx, synth, synthReq)
		if err == nil && len(resp.Choices) > 0 && strings.TrimSpace(resp.Choices[0].Message.Content) != "" {
			finalContent = resp.Choices[0].Message.Content
			usedModel = synth
			synthChannel = channel
		} else {
			// Synthesis failed: fall back to the longest usable answer.
			sort.Slice(usable, func(a, b int) bool { return len(usable[a].Content) > len(usable[b].Content) })
			finalContent = usable[0].Content
			usedModel = usable[0].Model
			synthChannel = usable[0].Channel
			degraded = true
			if err != nil {
				failed = append(failed, fmt.Sprintf("synthesizer(%s)", err.Error()))
			} else {
				failed = append(failed, "synthesizer(empty)")
			}
		}
	}

	elapsed := time.Since(started).Milliseconds()

	// ---- Emit headers for observability ----
	var modelsUsed []string
	for _, a := range usable {
		modelsUsed = append(modelsUsed, a.Model)
	}
	w.Header().Set("X-Council-Models", strings.Join(modelsUsed, ","))
	w.Header().Set("X-Council-Synth-Model", usedModel)
	w.Header().Set("X-Council-Synth-Channel", synthChannel)
	if degraded {
		w.Header().Set("X-Council-Degraded", "1")
	}
	if len(failed) > 0 {
		w.Header().Set("X-Council-Failed", strings.Join(failed, "; "))
	}
	w.Header().Set("X-Council-Latency-Ms", fmt.Sprintf("%d", elapsed))

	// ---- Build OpenAI-style response ----
	var resp providers.ChatCompletionResponse
	resp.ID = requestID
	resp.Object = "chat.completion"
	resp.Created = time.Now().Unix()
	resp.Model = "council"
	type councilChoice = struct {
		Index   int `json:"index"`
		Message struct {
			Role             string `json:"role"`
			Content          string `json:"content"`
			Reasoning        string `json:"reasoning,omitempty"`
			ReasoningContent string `json:"reasoning_content,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	}
	var choice councilChoice
	choice.Index = 0
	choice.Message.Role = "assistant"
	choice.Message.Content = finalContent
	choice.FinishReason = "stop"
	resp.Choices = append(resp.Choices, choice)

	if req.Stream {
		writeSSECouncil(w, resp)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// writeSSECouncil emits an OpenAI-style SSE stream for the council response.
func writeSSECouncil(w http.ResponseWriter, resp providers.ChatCompletionResponse) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "streaming unsupported"})
		return
	}
	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}
	const chunkSize = 256
	runes := []rune(content)
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		ev := map[string]any{
			"id":      resp.ID,
			"object":  "chat.completion.chunk",
			"created": resp.Created,
			"model":   "council",
			"choices": []map[string]any{{
				"index": 0,
				"delta": map[string]string{"content": string(runes[i:end])},
			}},
		}
		fmt.Fprintf(w, "data: %s\n\n", mustJSON(ev))
		flusher.Flush()
	}
	done := map[string]any{
		"id":      resp.ID,
		"object":  "chat.completion.chunk",
		"created": resp.Created,
		"model":   "council",
		"choices": []map[string]any{{
			"index":         0,
			"delta":         map[string]string{},
			"finish_reason": "stop",
		}},
	}
	fmt.Fprintf(w, "data: %s\n\n", mustJSON(done))
	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// lastUserText extracts the last user message content from the request.
func lastUserText(req providers.ChatCompletionRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" && strings.TrimSpace(req.Messages[i].Content) != "" {
			return req.Messages[i].Content
		}
	}
	return ""
}
