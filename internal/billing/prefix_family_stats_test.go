package billing

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// newTestStore opens the billing store against the real database configured for
// the running gateway (POSTGRES_DSN). These tests require a live Postgres on the
// gateway host; they are skipped when the DSN is absent so unit-only runs stay green.
func newTestStore(t *testing.T) *Store {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_DSN not set; skipping integration test")
	}
	store, err := NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return store
}

func TestStore_PrefixFamilyStats_GroupsAndCounts(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	families := []struct {
		fam    string
		reqs   int
		cached int
	}{
		{"pf1_test_" + stamp + "_aaa", 3, 0},
		{"pf1_test_" + stamp + "_bbb", 1, 0},
		{"pf1_test_" + stamp + "_ccc", 5, 1200},
	}
	for _, f := range families {
		for i := 0; i < f.reqs; i++ {
			err := store.Insert(ctx, UsageEvent{
				RequestID:       "r-" + f.fam + "-" + string(rune('a'+i)),
				Model:           "deepseek-v4-flash",
				Provider:        "openai",
				TotalTokens:     100,
				CachedTokens:    f.cached / f.reqs,
				CacheStatus:     "MISS",
				CacheLayer:      "provider_prefix_miss",
				PrefixFamily:    f.fam,
				CacheMissReason: "prefix_changed",
				RouteMode:       "auto",
				RouteTask:       "general",
				Success:         true,
			})
			if err != nil {
				t.Fatalf("insert: %v", err)
			}
		}
	}

	stats, err := store.PrefixFamilyStats(ctx, QueryFilter{Limit: 500})
	if err != nil {
		t.Fatalf("PrefixFamilyStats: %v", err)
	}
	byFam := map[string]PrefixFamilyStatRow{}
	for _, s := range stats {
		byFam[s.PrefixFamily] = s
	}
	// only assert on the families we just inserted (table may have older rows)
	for _, f := range families {
		row, ok := byFam[f.fam]
		if !ok {
			t.Fatalf("family %s missing from stats", f.fam)
		}
		if row.Requests != int64(f.reqs) {
			t.Errorf("%s requests = %d, want %d", f.fam, row.Requests, int64(f.reqs))
		}
		if row.CachedTokens != int64(f.cached) {
			t.Errorf("%s cached = %d, want %d", f.fam, row.CachedTokens, int64(f.cached))
		}
		if f.reqs > 1 && !row.Reusable {
			t.Errorf("%s with %d requests should be Reusable", f.fam, f.reqs)
		}
		if f.reqs == 1 && row.Reusable {
			t.Errorf("%s with 1 request should NOT be Reusable", f.fam)
		}
	}
}
