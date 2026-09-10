package longcontext

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// MapClaim is the only result shape accepted from a map worker.
type MapClaim struct {
	Claim      string          `json:"claim"`
	Quote      string          `json:"quote"`
	StartChar  int64           `json:"start_char"`
	EndChar    int64           `json:"end_char"`
	Entities   json.RawMessage `json:"entities"`
	Relations  json.RawMessage `json:"relations"`
	Confidence float64         `json:"confidence"`
}

type MapResult struct {
	Claims           []MapClaim `json:"claims"`
	ChunkSummary     string     `json:"chunk_summary"`
	OpenDependencies []string   `json:"open_dependencies"`
	ConflictsSeen    []string   `json:"conflicts_seen"`
}

// ValidateMapResult validates a MapResult against the source chunk. Quote
// matching tolerates case and whitespace differences via findQuoteSpan. When
// the model supplies a span outside the chunk's absolute range, the span is
// recovered from the quote's actual position (chunk-relative offset plus the
// chunk's absolute start) before accepting.
func ValidateMapResult(result MapResult, chunk Chunk) error {
	if strings.TrimSpace(result.ChunkSummary) == "" && len(result.Claims) == 0 {
		return fmt.Errorf("map result has no summary or claims")
	}
	for i, claim := range result.Claims {
		if strings.TrimSpace(claim.Claim) == "" || strings.TrimSpace(claim.Quote) == "" {
			return fmt.Errorf("claim %d missing claim or quote", i)
		}
		start, end := claim.StartChar, claim.EndChar
		if !quoteMatchesChunk(chunk.Content, claim.Quote) || start < chunk.StartChar || end > chunk.EndChar || end <= start {
			rs, re, ok := findQuoteSpanRune(chunk.Content, claim.Quote)
			if ok {
				start = chunk.StartChar + rs
				end = chunk.StartChar + re
				// Replace quote with actual source text at the found span (byte slice).
				bs, be, _ := findQuoteSpan(chunk.Content, claim.Quote)
				if bs >= 0 && be > bs && be <= int64(len(chunk.Content)) {
					claim.Quote = string(chunk.Content[bs:be])
				} else {
					claim.Quote = string([]rune(chunk.Content)[rs:re])
				}
			} else {
				// Quote is not verbatim in the chunk and cannot be recovered:
				// this claim has no verifiable evidence. Reject instead of
				// silently widening the span to the whole chunk (anti-hallucination).
				slog.Warn("claim quote not in source", "i", i)
				return fmt.Errorf("claim %d quote is not verbatim in chunk source", i)
			}
		}
		if start < chunk.StartChar || end > chunk.EndChar || end <= start {
			return fmt.Errorf("claim %d source span is outside chunk", i)
		}
		if claim.Confidence < 0 || claim.Confidence > 1 {
			return fmt.Errorf("claim %d confidence out of range", i)
		}
	}
	return nil
}
