package admin

import (
	"context"
	"testing"
)

func TestUsageRow_HasCachedTokensFields(t *testing.T) {
	var row UsageRow
	row.CachedTokens = 2048
	row.CacheWriteTokens = 64
	if row.CachedTokens != 2048 {
		t.Fatalf("CachedTokens=%d, want 2048", row.CachedTokens)
	}
	if row.CacheWriteTokens != 64 {
		t.Fatalf("CacheWriteTokens=%d, want 64", row.CacheWriteTokens)
	}
	_ = context.Background()
}
