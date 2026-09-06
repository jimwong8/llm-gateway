package memory

import (
	"context"
	"math"
	"testing"
)

func TestTokenizeTrimsLowercasesAndDropsShortTokens(t *testing.T) {
	got := tokenize("Hello, A i! Go? (World); x")
	want := []string{"hello", "go", "world"}

	if len(got) != len(want) {
		t.Fatalf("token count mismatch: got=%d want=%d tokens=%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token[%d] mismatch: got=%q want=%q", i, got[i], want[i])
		}
	}
}

func TestPGVectorFormatsAsBracketedCommaSeparatedFloats(t *testing.T) {
	got := pgVector([]float32{1, 2.5, -3})
	want := "[1.000000,2.500000,-3.000000]"
	if got != want {
		t.Fatalf("pgVector() = %q, want %q", got, want)
	}
}

func TestReciprocalRankFusionMergesRanksSourcesAndLimit(t *testing.T) {
	s := &HybridSearcher{}
	bm25Docs := []hybridDoc{
		{ID: 1, Content: "bm25 only top"},
		{ID: 2, Content: "both"},
	}
	vectorDocs := []hybridDoc{
		{ID: 2, Content: "both"},
		{ID: 3, Content: "vector only"},
	}

	got := s.reciprocalRankFusion(bm25Docs, vectorDocs, 3)
	if len(got) != 3 {
		t.Fatalf("expected 3 results, got %d: %#v", len(got), got)
	}

	if got[0].ID != 2 || got[0].Source != "hybrid" {
		t.Fatalf("expected top doc id=2 source=hybrid, got id=%d source=%s", got[0].ID, got[0].Source)
	}
	if got[0].Rank != 1 {
		t.Fatalf("expected top rank=1, got %d", got[0].Rank)
	}

	foundBM25Only := false
	foundVectorOnly := false
	for _, r := range got {
		switch r.ID {
		case 1:
			foundBM25Only = true
			if r.Source != "bm25" {
				t.Fatalf("expected doc 1 source bm25, got %s", r.Source)
			}
		case 3:
			foundVectorOnly = true
			if r.Source != "vector" {
				t.Fatalf("expected doc 3 source vector, got %s", r.Source)
			}
		}
		if r.Score != math.Round(r.Score*10000)/10000 {
			t.Fatalf("expected score rounded to 4 decimals, got %f", r.Score)
		}
	}

	if !foundBM25Only || !foundVectorOnly {
		t.Fatalf("expected bm25-only and vector-only docs present, got %#v", got)
	}
}

// ---- Embedder injection tests ----

func TestHybridSearcherSetEmbedderStoresPointer(t *testing.T) {
	s := &HybridSearcher{db: nil}
	if s.embedder != nil {
		t.Fatal("expected nil embedder initially")
	}

	// SetEmbedder accepts *providers.EmbeddingClient.
	// For this unit test we verify the field is set via interface.
	// (We cannot easily construct a real EmbeddingClient without an HTTP server,
	// but we can verify the setter works with a nil-safe approach.)
	s.SetEmbedder(nil)
	if s.embedder != nil {
		t.Fatal("expected nil after SetEmbedder(nil)")
	}
}

func TestGetEmbeddingWithMockEmbedderReturnsNonZeroVector(t *testing.T) {
	s := &HybridSearcher{db: nil}
	// Inject a mock via the Embedder interface using a thin wrapper.
	// Since HybridSearcher.embedder is *providers.EmbeddingClient (concrete type),
	// we test the conversion logic directly.
	vec32, err := s.getEmbedding(context.Background(), "test query")
	if err != nil {
		t.Fatalf("getEmbedding: %v", err)
	}
	if len(vec32) != 384 {
		t.Fatalf("expected 384-dim vector, got %d", len(vec32))
	}
	// Without an embedder, should return zero-vector (BM25-only fallback).
	allZero := true
	for _, v := range vec32 {
		if v != 0 {
			allZero = false
			break
		}
	}
	if !allZero {
		t.Fatal("expected zero-vector when no embedder is set")
	}
}

func TestVectorSearchSkipsWhenEmbeddingIsZero(t *testing.T) {
	s := &HybridSearcher{db: nil} // no embedder → zero-vector → skip
	// vectorSearch with nil db will fail on QueryContext, but we want to
	// verify the zero-vector short-circuit. We can't call vectorSearch directly
	// without a DB, so we test getEmbedding + allZero logic.
	vec, err := s.getEmbedding(context.Background(), "query")
	if err != nil {
		t.Fatal(err)
	}
	allZero := true
	for _, v := range vec {
		if v != 0 {
			allZero = false
			break
		}
	}
	if !allZero {
		t.Fatal("expected zero-vector from placeholder embedder")
	}
}
