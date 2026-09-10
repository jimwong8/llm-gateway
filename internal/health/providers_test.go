package health

import "testing"

func TestSummarizeProvidersCountsByStatus(t *testing.T) {
	statuses := []ProviderStatus{
		{Name: "a", Status: "ok"},
		{Name: "b", Status: "error"},
		{Name: "c", Status: "open"},
		{Name: "d", Status: "disabled"},
		{Name: "e", Status: "mystery"},
	}

	summary := SummarizeProviders(statuses)
	if summary["total"] != 5 {
		t.Fatalf("expected total=5, got %d", summary["total"])
	}
	if summary["ok"] != 1 || summary["error"] != 1 || summary["open"] != 1 || summary["disabled"] != 1 {
		t.Fatalf("unexpected summary counts: %+v", summary)
	}
	if summary["unknown"] != 1 {
		t.Fatalf("expected unknown=1 for mystery status, got %+v", summary)
	}
}
