package httpserver

import (
	"strings"

	"llm-gateway/gateway/internal/providers"
)

const (
	pcMaxPromptChars   = 32000
	pcMaxSystemChars   = 4000
	pcMaxHistoryRounds = 10
)

func CompressPrompt(req *providers.ChatCompletionRequest) (*providers.ChatCompletionRequest, bool) {
	totalChars := 0
	for _, m := range req.Messages {
		totalChars += len(m.Content)
	}
	if totalChars <= pcMaxPromptChars {
		return req, false
	}

	compressed := *req
	compressed.Messages = make([]providers.ChatMessage, 0, len(req.Messages))

	for _, m := range req.Messages {
		if m.Role == "system" && len(m.Content) > pcMaxSystemChars {
			compressed.Messages = append(compressed.Messages, providers.ChatMessage{
				Role:    "system",
				Content: m.Content[:pcMaxSystemChars] + "\n...[truncated]",
			})
		} else if m.Role == "system" {
			compressed.Messages = append(compressed.Messages, m)
		}
	}

	var history []providers.ChatMessage
	for _, m := range req.Messages {
		if m.Role != "system" {
			history = append(history, m)
		}
	}
	if len(history) > pcMaxHistoryRounds*2 {
		history = history[len(history)-pcMaxHistoryRounds*2:]
	}
	compressed.Messages = append(compressed.Messages, history...)

	return &compressed, true
}

func EstimateTokens(text string) int {
	return len([]rune(text)) / 4
}

func CompressSystemPrompt(content string, maxTokens int) string {
	if EstimateTokens(content) <= maxTokens {
		return content
	}
	lines := strings.Split(content, "\n")
	var kept []string
	used := 0
	for _, line := range lines {
		tokens := EstimateTokens(line)
		if used+tokens > maxTokens {
			kept = append(kept, "\n...[truncated]")
			break
		}
		kept = append(kept, line)
		used += tokens
	}
	return strings.Join(kept, "\n")
}
