package quota

import (
	"testing"
)

func TestQuotaSummaryShape(t *testing.T) {
	s := Summary{TenantID: "t1", Used: 3, Limit: 10, Remaining: 7, Rejected: 1, RejectRate: 1.0 / 3.0}
	if s.TenantID != "t1" {
		t.Fatalf("unexpected tenant: %+v", s)
	}
	if s.Remaining < 0 {
		t.Fatalf("remaining should not be negative: %+v", s)
	}
}

func TestTrendPointShape(t *testing.T) {
	p := TrendPoint{Minute: "2026-03-24T12:00:00Z", Used: 5, Rejected: 2, RemainingEstimate: 0}
	if p.Minute == "" {
		t.Fatal("expected minute")
	}
}
