package httpserver

import (
	"testing"

	"llm-gateway/gateway/internal/providers"
)

func TestDeriveCacheLayer_ProviderPrefixWhenCached(t *testing.T) {
	// L1/L2 hits keep their layer
	if got := deriveCacheLayer("HIT", "l1_exact", providers.ChatCompletionResponse{}); got != "l1_exact" {
		t.Fatalf("l1: got %s", got)
	}
	if got := deriveCacheLayer("SEMANTIC_HIT", "l2_semantic", providers.ChatCompletionResponse{}); got != "l2_semantic" {
		t.Fatalf("l2: got %s", got)
	}
	// MISS but provider returned cached tokens -> provider_prefix
	hit := providers.ChatCompletionResponse{Usage: providers.CompletionUsage{PromptTokensDetails: providers.PromptTokensDetails{CachedTokens: 2048}}}
	if got := deriveCacheLayer("MISS", "none", hit); got != "provider_prefix" {
		t.Fatalf("provider prefix: got %s", got)
	}
	// MISS and no cached tokens -> provider_prefix_miss
	miss := providers.ChatCompletionResponse{Usage: providers.CompletionUsage{}}
	if got := deriveCacheLayer("MISS", "none", miss); got != "provider_prefix_miss" {
		t.Fatalf("provider miss: got %s", got)
	}
}
