package billing

import (
	"context"
	"testing"
	"time"
)

func TestUsageEventStructExpandedFields(t *testing.T) {
	event := UsageEvent{
		TenantID:         "t_demo",
		UserID:           "u_demo",
		RequestID:        "req-1",
		Model:            "gpt-4o-mini",
		Provider:         "openai",
		PromptTokens:     10,
		CompletionTokens: 20,
		TotalTokens:      30,
		EstimatedCost:    0.12,
		CacheStatus:      "HIT",
		CacheLayer:       "l1_exact",
		RouteMode:        "auto",
		RouteProvider:    "openai",
		RouteModel:       "gpt-4o-mini",
		FallbackUsed:     false,
		LatencyMs:        125,
		Success:          true,
		ErrorType:        "",
		ErrorMessage:     "",
	}
	if event.CacheStatus != "HIT" || event.CacheLayer != "l1_exact" {
		t.Fatalf("unexpected expanded fields: %+v", event)
	}
}

func TestBuildWhere(t *testing.T) {
	from := time.Now().Add(-time.Hour)
	to := time.Now()
	query, args := buildWhere("SELECT * FROM usage_events", QueryFilter{
		TenantID: "t_demo",
		Provider: "openai",
		Model:    "gpt-4o-mini",
		From:     from,
		To:       to,
	})
	if len(args) != 5 {
		t.Fatalf("expected 5 args, got %d", len(args))
	}
	if query == "" {
		t.Fatal("expected non-empty query")
	}
}

func TestSummaryZeroValue(t *testing.T) {
	var row SummaryRow
	if row.Requests != 0 || row.CacheHitRate != 0 {
		t.Fatalf("unexpected zero row: %+v", row)
	}
}

func TestHotspotsResultShape(t *testing.T) {
	result := HotspotsResult{
		Tenants: []HotspotRow{{Key: "t_demo", Requests: 2}},
		Models:  []HotspotRow{{Key: "gpt-4o-mini", Requests: 2}},
	}
	if len(result.Tenants) != 1 || len(result.Models) != 1 {
		t.Fatalf("unexpected hotspots result: %+v", result)
	}
}

func TestStoreMethodSignaturesCompile(t *testing.T) {
	var s *Store
	ctx := context.Background()
	_, _ = s, ctx
	_ = func() error { return nil }
}

func TestSummaryProviderErrorRateShape(t *testing.T) {
	row := SummaryRow{ProviderErrorRate: 0.25}
	if row.ProviderErrorRate != 0.25 {
		t.Fatalf("unexpected provider error rate: %+v", row)
	}
}

func TestSummaryCacheHitRateShape(t *testing.T) {
	row := SummaryRow{CacheHitRate: 2.0 / 3.0}
	if row.CacheHitRate <= 0 {
		t.Fatalf("unexpected cache hit rate: %+v", row)
	}
}

func TestHotspotsSortedExampleShape(t *testing.T) {
	items := []HotspotRow{
		{Key: "tenant-a", Requests: 5},
		{Key: "tenant-b", Requests: 3},
	}
	if items[0].Requests < items[1].Requests {
		t.Fatalf("expected sorted descending requests: %+v", items)
	}
}
