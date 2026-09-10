package httpserver

import (
	"testing"

	"llm-gateway/gateway/internal/billing"
	"llm-gateway/gateway/internal/providers"
	"llm-gateway/gateway/internal/router"
)

func TestBuildUsageEvent_PersistsPrefixFamilyAndMissReason(t *testing.T) {
	resp := providers.ChatCompletionResponse{Usage: providers.CompletionUsage{PromptTokens: 1200}}
	ev := buildUsageEvent("req", providers.ChatCompletionRequest{Model: "m", TenantID: "t"}, router.Decision{Provider: "p", Model: "m"}, "p", "MISS", "provider_prefix_miss", false, true, "", "", 0, resp, "pf1_test", "prefix_changed")
	if ev.PrefixFamily != "pf1_test" || ev.CacheMissReason != "prefix_changed" {
		t.Fatal("cache metadata fields not persisted")
	}
	return
	ev.CacheMissReason = "prefix_changed"
	if ev.PrefixFamily != "pf1_test" || ev.CacheMissReason != "prefix_changed" {
		t.Fatal("cache metadata fields unavailable")
	}
	_ = billing.UsageEvent{}
}
