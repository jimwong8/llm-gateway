package longcontext

import (
	"fmt"
	"sort"
	"strings"
)

// ReducedClaim is a grounded claim selected from Map output.
type ReducedClaim struct {
	Claim      string  `json:"claim"`
	Quote      string  `json:"quote"`
	ChunkID    string  `json:"chunk_id"`
	StartChar  int64   `json:"start_char"`
	EndChar    int64   `json:"end_char"`
	Confidence float64 `json:"confidence"`
}

type ConflictGroup struct {
	Key    string         `json:"key"`
	Claims []ReducedClaim `json:"claims"`
}

type ReduceResult struct {
	Claims           []ReducedClaim  `json:"claims"`
	Conflicts        []ConflictGroup `json:"conflicts"`
	CitationCoverage float64         `json:"citation_coverage"`
	UncitedClaims    int             `json:"uncited_claims"`
	Status           string          `json:"status"`
}

// ReduceClaims deduplicates grounded claims and groups contradictory claims.
// It never invents text: every returned claim must carry its original quote.
func ReduceClaims(inputs []ReducedClaim) (ReduceResult, error) {
	result := ReduceResult{Status: "passed"}
	seen := map[string]bool{}
	groups := map[string][]ReducedClaim{}
	for _, claim := range inputs {
		if strings.TrimSpace(claim.Claim) == "" || strings.TrimSpace(claim.Quote) == "" {
			result.UncitedClaims++
			continue
		}
		key := normalizeClaim(claim.Claim)
		if key == "" {
			result.UncitedClaims++
			continue
		}
		dedupKey := key + "\x00" + strings.TrimSpace(claim.Quote)
		if seen[dedupKey] {
			continue
		}
		seen[dedupKey] = true
		result.Claims = append(result.Claims, claim)
		groups[key] = append(groups[key], claim)
	}
	for key, claims := range groups {
		quotes := map[string]bool{}
		for _, c := range claims {
			quotes[strings.TrimSpace(c.Quote)] = true
		}
		if len(quotes) > 1 {
			result.Conflicts = append(result.Conflicts, ConflictGroup{Key: key, Claims: claims})
		}
	}
	sort.Slice(result.Conflicts, func(i, j int) bool { return result.Conflicts[i].Key < result.Conflicts[j].Key })
	if len(result.Claims) > 0 {
		result.CitationCoverage = float64(len(result.Claims)-result.UncitedClaims) / float64(len(result.Claims))
	}
	if result.UncitedClaims > 0 {
		result.Status = "rejected"
		return result, fmt.Errorf("reduce contains %d uncited claims", result.UncitedClaims)
	}
	return result, nil
}

func normalizeClaim(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}
