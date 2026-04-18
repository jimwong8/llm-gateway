package providers

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"llm-gateway/gateway/internal/config"
)

type stubProvider struct {
	name      string
	response  ChatCompletionResponse
	err       error
	callCount int
}

func (s *stubProvider) ChatCompletion(_ context.Context, _ ChatCompletionRequest) (ChatCompletionResponse, error) {
	s.callCount++
	if s.err != nil {
		return ChatCompletionResponse{}, s.err
	}
	return s.response, nil
}

func (s *stubProvider) Name() string { return s.name }

func TestMockProviderUsesDefaultModelAndSummarizesMessages(t *testing.T) {
	provider := NewMockProvider("mock-main", "default-model")

	resp, err := provider.ChatCompletion(context.Background(), ChatCompletionRequest{
		Messages: []ChatMessage{
			{Role: "user", Content: "  hello world  "},
			{Role: "assistant", Content: "reply"},
		},
	})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if resp.Model != "default-model" {
		t.Fatalf("resp.Model = %q, want %q", resp.Model, "default-model")
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("len(resp.Choices) = %d, want 1", len(resp.Choices))
	}
	content := resp.Choices[0].Message.Content
	if !strings.Contains(content, "provider=mock-main") {
		t.Fatalf("content %q does not include provider name", content)
	}
	if !strings.Contains(content, "summary=user:hello world | assistant:reply") {
		t.Fatalf("content %q does not include expected summary", content)
	}
	if resp.Usage.PromptTokens != 24 {
		t.Fatalf("PromptTokens = %d, want 24", resp.Usage.PromptTokens)
	}
	if resp.Usage.TotalTokens != resp.Usage.PromptTokens+resp.Usage.CompletionTokens {
		t.Fatalf("TotalTokens = %d, want %d", resp.Usage.TotalTokens, resp.Usage.PromptTokens+resp.Usage.CompletionTokens)
	}
}

func TestMockProviderReturnsForcedFailure(t *testing.T) {
	provider := NewMockProvider("mock-main", "default-model")

	_, err := provider.ChatCompletion(context.Background(), ChatCompletionRequest{Model: "fail-model"})
	if err == nil {
		t.Fatal("ChatCompletion() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "forced failure") {
		t.Fatalf("error = %q, want forced failure", err.Error())
	}
}

func TestRegistryResolveAndHealthStateTransitions(t *testing.T) {
	fallback := &stubProvider{name: "fallback"}
	primary := &stubProvider{
		name:     "OpenAI ",
		response: ChatCompletionResponse{Model: "ok-model"},
	}
	registry := NewRegistry(config.Config{
		ProviderFailureThreshold: 2,
		ProviderOpenSeconds:      1,
	}, fallback, primary)

	resolved := registry.Resolve(" openai ")
	if resolved != primary {
		t.Fatalf("Resolve() returned %T, want primary provider", resolved)
	}
	if registry.Resolve("missing") != fallback {
		t.Fatal("Resolve() should return fallback for missing provider")
	}

	registry.recordFailure("openai", 10*time.Millisecond, errors.New("boom-1"))
	statuses := registry.HealthStatuses()
	if len(statuses) != 2 {
		t.Fatalf("len(HealthStatuses()) = %d, want 2", len(statuses))
	}
	var openAI ProviderHealth
	for _, status := range statuses {
		if strings.EqualFold(status.Name, "OpenAI ") {
			openAI = status
			break
		}
	}
	if openAI.Status != "error" {
		t.Fatalf("status after first failure = %q, want %q", openAI.Status, "error")
	}
	if openAI.FailureCount != 1 {
		t.Fatalf("FailureCount = %d, want 1", openAI.FailureCount)
	}

	registry.recordFailure("openai", 10*time.Millisecond, errors.New("boom-2"))
	if err := registry.beforeCall("openai"); err == nil {
		t.Fatal("beforeCall() error = nil, want open circuit error")
	}

	registry.recordSuccess("openai", 5*time.Millisecond)
	if err := registry.beforeCall("openai"); err != nil {
		t.Fatalf("beforeCall() after success error = %v, want nil", err)
	}
}

func TestHelperFunctions(t *testing.T) {
	if got := summarize(nil); got != "empty-input" {
		t.Fatalf("summarize(nil) = %q, want %q", got, "empty-input")
	}
	if got := summarize([]ChatMessage{{Role: "user", Content: "   "}}); got != "empty-input" {
		t.Fatalf("summarize(blank) = %q, want %q", got, "empty-input")
	}
	long := strings.Repeat("a", 60)
	got := summarize([]ChatMessage{{Role: "user", Content: long}})
	want := "user:" + strings.Repeat("a", 48)
	if got != want {
		t.Fatalf("summarize(long) = %q, want %q", got, want)
	}

	if got := providerType(" mock-primary "); got != "mock" {
		t.Fatalf("providerType(mock) = %q, want %q", got, "mock")
	}
	if got := providerType("my-openai-provider"); got != "openai" {
		t.Fatalf("providerType(openai) = %q, want %q", got, "openai")
	}
	if got := providerType("custom"); got != "provider" {
		t.Fatalf("providerType(default) = %q, want %q", got, "provider")
	}

	if shouldRetry("mock-main") {
		t.Fatal("shouldRetry(mock-main) = true, want false")
	}
	if !shouldRetry("openai") {
		t.Fatal("shouldRetry(openai) = false, want true")
	}
	if got := max(2, 1); got != 2 {
		t.Fatalf("max(2, 1) = %d, want 2", got)
	}
	if got := max(1, 2); got != 2 {
		t.Fatalf("max(1, 2) = %d, want 2", got)
	}
}
