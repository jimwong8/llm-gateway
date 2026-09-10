package httpserver

import (
	"testing"

	"llm-gateway/gateway/internal/billing"
	"llm-gateway/gateway/internal/providers"
	"llm-gateway/gateway/internal/router"
)

func TestBuildUsageEvent_CarriesProviderCacheTokens(t *testing.T) {
	resp := providers.ChatCompletionResponse{
		Model: "gpt-4o-mini",
		Usage: providers.CompletionUsage{
			PromptTokens:        4096,
			CompletionTokens:    32,
			TotalTokens:         4128,
			CacheWriteTokens:    64,
			PromptTokensDetails: providers.PromptTokensDetails{CachedTokens: 3072},
		},
	}
	ev := buildUsageEvent(
		"req-1",
		providers.ChatCompletionRequest{Model: "gpt-4o-mini", UserID: "u1", TenantID: "t1"},
		router.Decision{Provider: "openai", Model: "gpt-4o-mini", RouteMode: "priority", Task: "chat"},
		"openai",
		"MISS",
		"provider_prefix_unknown",
		false,
		true,
		"",
		"",
		0,
		resp,
		"pf1_sample",
		"prefix_changed",
	)
	if ev.CachedTokens != 3072 {
		t.Fatalf("CachedTokens=%d, want 3072", ev.CachedTokens)
	}
	if ev.CacheWriteTokens != 64 {
		t.Fatalf("CacheWriteTokens=%d, want 64", ev.CacheWriteTokens)
	}
	if ev.TotalTokens != 4128 {
		t.Fatalf("TotalTokens=%d, want 4128", ev.TotalTokens)
	}
	_ = billing.UsageEvent{}
}
