package providers

import (
	"encoding/json"
	"testing"
)

func TestChatCompletionResponse_ParsesPromptCacheUsage(t *testing.T) {
	var resp ChatCompletionResponse
	raw := []byte(`{"usage":{"prompt_tokens":4096,"completion_tokens":32,"total_tokens":4128,"prompt_tokens_details":{"cached_tokens":3072},"cache_write_tokens":64}}`)
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Usage.PromptTokensDetails.CachedTokens != 3072 {
		t.Fatalf("PromptTokensDetails.CachedTokens=%d, want 3072", resp.Usage.PromptTokensDetails.CachedTokens)
	}
	if resp.Usage.CacheWriteTokens != 64 {
		t.Fatalf("CacheWriteTokens=%d, want 64", resp.Usage.CacheWriteTokens)
	}
}
