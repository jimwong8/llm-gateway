package preprocess

import (
	"context"
	"fmt"
	"strings"

	"llm-gateway/gateway/internal/providers"
)

type Summarizer interface {
	Apply(ctx context.Context, req providers.ChatCompletionRequest) (providers.ChatCompletionRequest, SummaryMeta, error)
}

type ModelBackedSummarizer struct {
	provider        providers.Provider
	model           string
	triggerMessages int
	maxRecentTurns  int
	fallback        Summarizer
}

func NewModelBackedSummarizer(provider providers.Provider, model string, triggerMessages, maxRecentTurns int, fallback Summarizer) *ModelBackedSummarizer {
	if triggerMessages <= 0 {
		triggerMessages = 20
	}
	if maxRecentTurns <= 0 {
		maxRecentTurns = 6
	}
	if fallback == nil {
		fallback = NewPlaceholderSummarizer(triggerMessages, maxRecentTurns)
	}
	return &ModelBackedSummarizer{
		provider:        provider,
		model:           strings.TrimSpace(model),
		triggerMessages: triggerMessages,
		maxRecentTurns:  maxRecentTurns,
		fallback:        fallback,
	}
}

func (s *ModelBackedSummarizer) Apply(ctx context.Context, req providers.ChatCompletionRequest) (providers.ChatCompletionRequest, SummaryMeta, error) {
	meta := SummaryMeta{}
	originalTokens := estimateMessageTokens(req.Messages)
	meta.OriginalTokenEstimate = originalTokens
	meta.ReducedTokenEstimate = originalTokens

	if len(req.Messages) < s.triggerMessages {
		return req, meta, nil
	}
	if s.provider == nil || strings.TrimSpace(s.model) == "" {
		return s.fallback.Apply(ctx, req)
	}

	recentWindowMessages := s.maxRecentTurns * 2
	if recentWindowMessages > len(req.Messages) {
		recentWindowMessages = len(req.Messages)
	}
	history := req.Messages[:len(req.Messages)-recentWindowMessages]
	preserved := req.Messages[len(req.Messages)-recentWindowMessages:]
	if len(history) == 0 {
		return req, meta, nil
	}

	prompt := buildSummaryPrompt(history)
	resp, err := s.provider.ChatCompletion(ctx, providers.ChatCompletionRequest{
		Model: s.model,
		Messages: []providers.ChatMessage{{Role: "user", Content: prompt}},
	})
	if err != nil || len(resp.Choices) == 0 {
		return s.fallback.Apply(ctx, req)
	}
	summaryText := strings.TrimSpace(resp.Choices[0].Message.Content)
	if summaryText == "" {
		return s.fallback.Apply(ctx, req)
	}

	processed := req
	processed.Messages = make([]providers.ChatMessage, 0, 1+len(preserved))
	processed.Messages = append(processed.Messages, providers.ChatMessage{
		Role:    "system",
		Content: "[conversation summary]\n" + summaryText,
	})
	processed.Messages = append(processed.Messages, preserved...)

	reducedTokens := estimateMessageTokens(processed.Messages)
	if originalTokens > 0 {
		meta.CompressionRatio = float64(reducedTokens) / float64(originalTokens)
	}
	meta.Applied = true
	meta.ReducedTokenEstimate = reducedTokens
	meta.SummaryText = summaryText
	return processed, meta, nil
}

type PlaceholderSummarizer struct {
	triggerMessages int
	maxRecentTurns  int
}

func NewPlaceholderSummarizer(triggerMessages, maxRecentTurns int) *PlaceholderSummarizer {
	if triggerMessages <= 0 {
		triggerMessages = 20
	}
	if maxRecentTurns <= 0 {
		maxRecentTurns = 6
	}
	return &PlaceholderSummarizer{triggerMessages: triggerMessages, maxRecentTurns: maxRecentTurns}
}

func (s *PlaceholderSummarizer) Apply(_ context.Context, req providers.ChatCompletionRequest) (providers.ChatCompletionRequest, SummaryMeta, error) {
	meta := SummaryMeta{}
	originalTokens := estimateMessageTokens(req.Messages)
	meta.OriginalTokenEstimate = originalTokens

	if len(req.Messages) < s.triggerMessages {
		meta.ReducedTokenEstimate = originalTokens
		return req, meta, nil
	}

	recentWindowMessages := s.maxRecentTurns * 2
	if recentWindowMessages > len(req.Messages) {
		recentWindowMessages = len(req.Messages)
	}
	preserved := req.Messages[len(req.Messages)-recentWindowMessages:]
	reducedTokens := estimateMessageTokens(preserved)
	if originalTokens > 0 {
		meta.CompressionRatio = float64(reducedTokens) / float64(originalTokens)
	}
	meta.Applied = true
	meta.ReducedTokenEstimate = reducedTokens
	meta.SummaryText = "placeholder-summary"

	return req, meta, nil
}

type NoopSummarizer struct{}

func NewNoopSummarizer() *NoopSummarizer {
	return &NoopSummarizer{}
}

func (s *NoopSummarizer) Apply(_ context.Context, req providers.ChatCompletionRequest) (providers.ChatCompletionRequest, SummaryMeta, error) {
	return req, SummaryMeta{}, nil
}

func buildSummaryPrompt(messages []providers.ChatMessage) string {
	var builder strings.Builder
	builder.WriteString("Summarize the following conversation history into concise factual context. Preserve decisions, constraints, tasks, and relevant facts. Avoid stylistic filler.\n\n")
	for _, msg := range messages {
		builder.WriteString(fmt.Sprintf("%s: %s\n", strings.TrimSpace(msg.Role), strings.TrimSpace(msg.Content)))
	}
	return builder.String()
}

func estimateMessageTokens(messages []providers.ChatMessage) int {
	total := 0
	for _, msg := range messages {
		total += len(msg.Content)/4 + 4
	}
	return total
}
