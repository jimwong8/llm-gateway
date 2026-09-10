package cache

import (
	"testing"

	"llm-gateway/gateway/internal/providers"
)

func TestBuildPrefixFamily_ExcludesCurrentUserTail(t *testing.T) {
	base := providers.ChatCompletionRequest{
		Model: "deepseek-v4-flash", TenantID: "tenant-a",
		Messages: []providers.ChatMessage{
			{Role: "system", Content: "stable system rules"},
			{Role: "user", Content: "first turn"},
			{Role: "assistant", Content: "first answer"},
			{Role: "user", Content: "question A"},
		},
	}
	other := base
	other.Messages = append([]providers.ChatMessage(nil), base.Messages...)
	other.Messages[len(other.Messages)-1].Content = "question B"
	if got, want := BuildPrefixFamily(base), BuildPrefixFamily(other); got != want {
		t.Fatalf("current user tail changed prefix family: %q != %q", got, want)
	}
}

func TestBuildPrefixFamily_IsolatesTenantAndSystem(t *testing.T) {
	base := providers.ChatCompletionRequest{
		Model: "model-a", TenantID: "tenant-a",
		Messages: []providers.ChatMessage{{Role: "system", Content: "rules"}, {Role: "user", Content: "q"}},
	}
	otherTenant := base
	otherTenant.TenantID = "tenant-b"
	otherSystem := base
	otherSystem.Messages = append([]providers.ChatMessage(nil), base.Messages...)
	otherSystem.Messages[0].Content = "different rules"
	if BuildPrefixFamily(base) == BuildPrefixFamily(otherTenant) {
		t.Fatal("different tenants shared prefix family")
	}
	if BuildPrefixFamily(base) == BuildPrefixFamily(otherSystem) {
		t.Fatal("different system prompts shared prefix family")
	}
}
