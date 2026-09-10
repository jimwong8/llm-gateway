package providers

import (
	"context"
	"strings"
	"testing"
)

func TestMockProviderName(t *testing.T) {
	p := NewMockProvider("mock-test", "test-model")
	if p.Name() != "mock-test" {
		t.Errorf("Name() = %q, want %q", p.Name(), "mock-test")
	}
}

func TestMockProviderChatCompletion(t *testing.T) {
	p := NewMockProvider("mock-test", "default-model")

	req := ChatCompletionRequest{
		Model: "test-model",
		Messages: []ChatMessage{
			{Role: "user", Content: "hello"},
		},
	}

	resp, err := p.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("ChatCompletion() error: %v", err)
	}

	if resp.ID == "" {
		t.Error("ChatCompletion() returned empty ID")
	}
	if resp.Object != "chat.completion" {
		t.Errorf("ChatCompletion() object = %q, want %q", resp.Object, "chat.completion")
	}
	if resp.Model != "test-model" {
		t.Errorf("ChatCompletion() model = %q, want %q", resp.Model, "test-model")
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("ChatCompletion() choices = %d, want 1", len(resp.Choices))
	}
	if resp.Choices[0].Message.Role != "assistant" {
		t.Errorf("ChatCompletion() role = %q, want %q", resp.Choices[0].Message.Role, "assistant")
	}
	if !strings.Contains(resp.Choices[0].Message.Content, "mock response") {
		t.Errorf("ChatCompletion() content = %q, want to contain 'mock response'", resp.Choices[0].Message.Content)
	}
	if resp.Choices[0].FinishReason != "stop" {
		t.Errorf("ChatCompletion() finish_reason = %q, want %q", resp.Choices[0].FinishReason, "stop")
	}
	if resp.Usage.PromptTokens <= 0 {
		t.Errorf("ChatCompletion() prompt_tokens = %d, want > 0", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens <= 0 {
		t.Errorf("ChatCompletion() completion_tokens = %d, want > 0", resp.Usage.CompletionTokens)
	}
	if resp.Usage.TotalTokens != resp.Usage.PromptTokens+resp.Usage.CompletionTokens {
		t.Errorf("ChatCompletion() total_tokens = %d, want %d", resp.Usage.TotalTokens, resp.Usage.PromptTokens+resp.Usage.CompletionTokens)
	}
}

func TestMockProviderDefaultModel(t *testing.T) {
	p := NewMockProvider("mock-test", "fallback-model")

	req := ChatCompletionRequest{
		Messages: []ChatMessage{{Role: "user", Content: "hello"}},
	}

	resp, err := p.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("ChatCompletion() error: %v", err)
	}
	if resp.Model != "fallback-model" {
		t.Errorf("ChatCompletion() model = %q, want %q (fallback)", resp.Model, "fallback-model")
	}
}

func TestMockProviderFailure(t *testing.T) {
	p := NewMockProvider("mock-fail", "fail-model")

	req := ChatCompletionRequest{
		Model: "fail-model",
		Messages: []ChatMessage{{Role: "user", Content: "hello"}},
	}

	_, err := p.ChatCompletion(context.Background(), req)
	if err == nil {
		t.Error("ChatCompletion() with fail model should return error")
	}
	if !strings.Contains(err.Error(), "forced failure") {
		t.Errorf("ChatCompletion() error = %q, want to contain 'forced failure'", err.Error())
	}
}

func TestMockProviderEmptyMessages(t *testing.T) {
	p := NewMockProvider("mock-test", "test-model")

	req := ChatCompletionRequest{
		Messages: []ChatMessage{},
	}

	resp, err := p.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("ChatCompletion() error: %v", err)
	}
	if !strings.Contains(resp.Choices[0].Message.Content, "empty-input") {
		t.Errorf("ChatCompletion() content = %q, want to contain 'empty-input'", resp.Choices[0].Message.Content)
	}
}

func TestMockProviderNilMessages(t *testing.T) {
	p := NewMockProvider("mock-test", "test-model")

	req := ChatCompletionRequest{}

	resp, err := p.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("ChatCompletion() error: %v", err)
	}
	if !strings.Contains(resp.Choices[0].Message.Content, "empty-input") {
		t.Errorf("ChatCompletion() content = %q, want to contain 'empty-input'", resp.Choices[0].Message.Content)
	}
}

func TestMockProviderContentSummary(t *testing.T) {
	p := NewMockProvider("mock-test", "test-model")

	// 长消息应被截断
	longContent := strings.Repeat("a", 100)
	req := ChatCompletionRequest{
		Messages: []ChatMessage{
			{Role: "user", Content: longContent},
			{Role: "assistant", Content: "short"},
		},
	}

	resp, err := p.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("ChatCompletion() error: %v", err)
	}
	// 内容中应包含 role 前缀
	if !strings.Contains(resp.Choices[0].Message.Content, "user:") {
		t.Errorf("ChatCompletion() content should contain role prefix")
	}
}

func TestMockProviderMultipleMessages(t *testing.T) {
	p := NewMockProvider("mock-test", "test-model")

	req := ChatCompletionRequest{
		Messages: []ChatMessage{
			{Role: "system", Content: "You are a helpful assistant"},
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi there"},
			{Role: "user", Content: "how are you"},
		},
	}

	resp, err := p.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("ChatCompletion() error: %v", err)
	}
	// 应只取前 3 条消息做摘要
	content := resp.Choices[0].Message.Content
	if !strings.Contains(content, "system:") {
		t.Errorf("ChatCompletion() summary should contain first message role")
	}
}

func TestProviderType(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"mock-test", "mock"},
		{"mock-code", "mock"},
		{"openai", "openai"},
		{"OPENAI", "openai"},
		{"custom-provider", "provider"},
		{"anthropic", "provider"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := providerType(tt.name)
			if got != tt.expected {
				t.Errorf("providerType(%q) = %q, want %q", tt.name, got, tt.expected)
			}
		})
	}
}

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"openai", true},
		{"anthropic", true},
		{"mock-test", false},
		{"mock-code", false},
		{"custom", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRetry(tt.name)
			if got != tt.expected {
				t.Errorf("shouldRetry(%q) = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

func TestMax(t *testing.T) {
	tests := []struct {
		a, b, expected int
	}{
		{1, 2, 2},
		{2, 1, 2},
		{0, 0, 0},
		{-1, 1, 1},
		{5, 5, 5},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := max(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("max(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}
