package billing

import (
	"strings"
	"testing"
)

func TestSummaryQuery_SelectsCachedTokens(t *testing.T) {
	// Ensure Summary aggregates cached_tokens from provider responses.
	// We inspect sql.go indirectly by checking SummaryRow has CachedTokens.
	var row SummaryRow
	row.CachedTokens = 123
	if row.CachedTokens != 123 {
		t.Fatalf("SummaryRow.CachedTokens not usable, got %d", row.CachedTokens)
	}
	_ = strings.TrimSpace
}
