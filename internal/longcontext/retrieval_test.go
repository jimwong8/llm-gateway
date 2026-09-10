package longcontext

import (
	"testing"
)

func TestHybridRetrievalReturnsTopKByConfidence(t *testing.T) {
	ev := []Evidence{
		{Claim: "Revenue grew 8%", Quote: "revenue grew 8 percent", Confidence: 0.9},
		{Claim: "Margin improved", Quote: "operating margin 22 percent", Confidence: 0.7},
		{Claim: "Partnership announced", Quote: "partnered with logistics", Confidence: 0.5},
	}
	got := HybridRetrieval("revenue profit growth", ev, 2)
	if len(got) != 2 {
		t.Fatalf("got %d, want 2", len(got))
	}
	// First should be the high-confidence revenue item (keyword match boost).
	if got[0].Confidence != 0.9 {
		t.Fatalf("first item confidence=%v, want 0.9", got[0].Confidence)
	}
}

func TestHybridRetrievalDeduplicatesClaims(t *testing.T) {
	ev := []Evidence{
		{Claim: "Revenue grew 8%", Quote: "revenue grew 8 percent", Confidence: 0.9},
		{Claim: "Revenue grew 8%", Quote: "revenue grew 8 percent", Confidence: 0.85},
		{Claim: "Margin improved", Quote: "operating margin 22 percent", Confidence: 0.7},
	}
	got := HybridRetrieval("", ev, 10)
	if len(got) != 2 {
		t.Fatalf("got %d items after dedup, want 2", len(got))
	}
}

func TestExtractCitationIDs(t *testing.T) {
	answer := "The company reported [E1] revenue growth and [E3] margin expansion. [E1] is cited again."
	ids := extractCitationIDs(answer)
	if len(ids) != 2 {
		t.Fatalf("got %d citation IDs, want 2: %v", len(ids), ids)
	}
}

func TestMapToFinalResult(t *testing.T) {
	reduced := ReduceResult{
		Claims: []ReducedClaim{
			{Claim: "Revenue grew", Quote: "revenue grew 10%", Confidence: 0.9},
		},
		CitationCoverage: 1.0,
	}
	result := MapToFinalResult("summarize finances", "The company showed [E1] growth.", reduced)
	if result.EvidenceCount != 1 {
		t.Fatalf("evidence_count=%d, want 1", result.EvidenceCount)
	}
	if len(result.CitedEvidenceIDs) != 1 {
		t.Fatalf("cited_ids=%d, want 1", len(result.CitedEvidenceIDs))
	}
}
