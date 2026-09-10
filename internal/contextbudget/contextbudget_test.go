package contextbudget

import "testing"

func TestEstimateIncludesMessagesToolsAndOutputReserve(t *testing.T) {
	got := Estimate(Request{
		Messages: []Message{{Role: "system", Content: "12345678"}, {Role: "user", Content: "123456789012"}},
		ToolsJSON: "12345678",
		MaxOutputTokens: 100,
	})
	if got != 115 {
		t.Fatalf("Estimate()=%d, want 115", got)
	}
}

func TestFitsUsesSafetyMargin(t *testing.T) {
	if Fits(900, 1000, 0.9) {
		t.Fatal("expected request at the safety boundary to be rejected")
	}
	if !Fits(899, 1000, 0.9) {
		t.Fatal("expected request below the safety boundary to fit")
	}
}
