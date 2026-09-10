package audit

import "testing"

func TestEventStructCarriesStructuredRoutingAndPreprocessFields(t *testing.T) {
	event := Event{
		RequestID:            "req-1",
		RouteMode:            "auto",
		RouteTask:            "summarization",
		RouteTier:            "domestic",
		RouteModel:           "deepseek-chat",
		RouteProvider:        "deepseek-domestic",
		RouteReason:          "task-aware routing preferred domestic tier",
		RouteScore:           "0.9000",
		CacheStatus:          "MISS",
		FallbackUsed:         false,
		PreprocessApplied:    true,
		CanonicalHash:        "abc123",
		SummaryApplied:       true,
		SummaryRatio:         0.42,
		TaskHint:             "summarization",
		Complexity:           "simple",
		ComplexityConfidence: 0.91,
		RequestPayload:       map[string]any{"hello": "world"},
		ResponsePayload:      map[string]any{"ok": true},
	}

	if event.RouteTier != "domestic" {
		t.Fatalf("unexpected route tier: %s", event.RouteTier)
	}
	if !event.PreprocessApplied || !event.SummaryApplied {
		t.Fatalf("expected preprocess flags to be true")
	}
	if event.TaskHint != "summarization" || event.Complexity != "simple" {
		t.Fatalf("unexpected task/classification fields: %#v", event)
	}
}
