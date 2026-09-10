package longcontext

import "testing"

func TestCosineSimilarity(t *testing.T) {
	a := []float64{1, 0}
	b := []float64{0, 1}
	if got := cosineSimilarity(a, b); got != 0 {
		t.Fatalf("orthogonal vectors should score 0, got %f", got)
	}
	c := []float64{1, 1}
	if got := cosineSimilarity(a, c); got <= 0.7 {
		t.Fatalf("expected similarity > 0.7, got %f", got)
	}
}
func TestVectorHybridRetrievalDegeneratesToKeyword(t *testing.T) {
	ev := []Evidence{{Claim: "revenue up", Quote: "revenue up", Confidence: 0.9}}
	out := VectorHybridRetrieval("revenue", nil, ev, 1)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
}
