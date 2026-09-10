package longcontext

import (
	"math"
)

// cosineSimilarity returns the cosine similarity of two non-zero vectors.
// Vectors of differing length are padded with zeros. Returns 0 if either
// vector has zero norm.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	var dot, na, nb float64
	for i := 0; i < n; i++ {
		var ai, bi float64
		if i < len(a) {
			ai = a[i]
		}
		if i < len(b) {
			bi = b[i]
		}
		dot += ai * bi
		na += ai * ai
		nb += bi * bi
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// VectorHybridRetrieval selects the top-k evidence items using a weighted
// combination of keyword overlap and vector cosine similarity. If queryVec is
// nil or evidence items lack embeddings, it degrades to keyword-only scoring.
func VectorHybridRetrieval(query string, queryVec []float64, evidence []Evidence, topK int) []Evidence {
	if topK <= 0 || len(evidence) == 0 {
		return evidence
	}
	hasVectors := queryVec != nil
	if hasVectors {
		for _, ev := range evidence {
			if len(ev.Embedding) == 0 {
				hasVectors = false
				break
			}
		}
	}
	queryTerms := tokenize(query)
	seen := map[string]bool{}
	var deduped []Evidence
	for _, ev := range evidence {
		key := ev.Claim + "\x00" + ev.Quote
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, ev)
	}

	type scored struct {
		ev    Evidence
		score float64
	}
	var items []scored
	for _, ev := range deduped {
		score := scoreEvidence(queryTerms, ev)
		if hasVectors {
			vecScore := cosineSimilarity(queryVec, ev.Embedding)
			// Weight: 0.4 keyword + 0.6 vector, normalized to roughly [0,1].
			score = 0.4*score + 0.6*vecScore
		}
		items = append(items, scored{ev, score})
	}

	for i := 0; i < topK && i < len(items)-1; i++ {
		best := i
		for j := i + 1; j < len(items); j++ {
			if items[j].score > items[best].score {
				best = j
			}
		}
		items[i], items[best] = items[best], items[i]
	}
	n := topK
	if n > len(items) {
		n = len(items)
	}
	out := make([]Evidence, n)
	for i := 0; i < n; i++ {
		out[i] = items[i].ev
	}
	return out
}
