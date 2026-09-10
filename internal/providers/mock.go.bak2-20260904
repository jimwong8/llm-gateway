package providers

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type MockProvider struct {
	defaultModel string
	providerName string
}

func NewMockProvider(providerName, defaultModel string) *MockProvider {
	return &MockProvider{providerName: providerName, defaultModel: defaultModel}
}

func (m *MockProvider) Name() string {
	return m.providerName
}

func (m *MockProvider) ChatCompletion(_ context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	model := req.Model
	if strings.TrimSpace(model) == "" {
		model = m.defaultModel
	}

	if strings.Contains(strings.ToLower(model), "fail") {
		return ChatCompletionResponse{}, fmt.Errorf("mock provider=%s forced failure for model=%s", m.providerName, model)
	}

	promptSummary := summarize(req.Messages)
	content := fmt.Sprintf("mock response from provider=%s model=%s | summary=%s", m.providerName, model, promptSummary)

	resp := ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-mock-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
	}
	resp.Choices = []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	}{{Index: 0, FinishReason: "stop"}}
	resp.Choices[0].Message.Role = "assistant"
	resp.Choices[0].Message.Content = content
	resp.Usage.PromptTokens = len(req.Messages) * 12
	resp.Usage.CompletionTokens = 24
	resp.Usage.TotalTokens = resp.Usage.PromptTokens + resp.Usage.CompletionTokens
	return resp, nil
}

func summarize(messages []ChatMessage) string {
	if len(messages) == 0 {
		return "empty-input"
	}
	var parts []string
	for _, msg := range messages {
		text := strings.TrimSpace(msg.Content)
		if text == "" {
			continue
		}
		if len(text) > 48 {
			text = text[:48]
		}
		parts = append(parts, fmt.Sprintf("%s:%s", msg.Role, text))
		if len(parts) >= 3 {
			break
		}
	}
	if len(parts) == 0 {
		return "empty-input"
	}
	return strings.Join(parts, " | ")
}
