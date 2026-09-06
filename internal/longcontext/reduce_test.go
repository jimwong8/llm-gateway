package longcontext

import "testing"

func TestReduceClaimsDeduplicatesAndPreservesQuotes(t *testing.T) {
	result, err := ReduceClaims([]ReducedClaim{
		{Claim:"Revenue increased", Quote:"revenue increased 10%", ChunkID:"c1", Confidence:.8},
		{Claim:" revenue   increased ", Quote:"revenue increased 10%", ChunkID:"c2", Confidence:.9},
	})
	if err != nil { t.Fatal(err) }
	if len(result.Claims) != 1 { t.Fatalf("claims=%d, want 1", len(result.Claims)) }
	if result.Claims[0].Quote != "revenue increased 10%" { t.Fatal("quote was not preserved") }
	if result.CitationCoverage != 1 { t.Fatalf("coverage=%v", result.CitationCoverage) }
	if result.Status != "passed" { t.Fatalf("status=%q", result.Status) }
}

func TestReduceClaimsGroupsConflictingQuotes(t *testing.T) {
	result, err := ReduceClaims([]ReducedClaim{
		{Claim:"Revenue increased", Quote:"revenue increased 10%", ChunkID:"c1", Confidence:.8},
		{Claim:"revenue increased", Quote:"revenue increased 4%", ChunkID:"c2", Confidence:.7},
	})
	if err != nil { t.Fatal(err) }
	if len(result.Conflicts) != 1 { t.Fatalf("conflicts=%d, want 1", len(result.Conflicts)) }
	if len(result.Conflicts[0].Claims) != 2 { t.Fatal("conflict claims lost") }
}

func TestReduceClaimsRejectsUncitedClaim(t *testing.T) {
	result, err := ReduceClaims([]ReducedClaim{{Claim:"unsupported conclusion"}})
	if err == nil { t.Fatal("accepted uncited claim") }
	if result.Status != "rejected" || result.UncitedClaims != 1 { t.Fatalf("result=%+v", result) }
}
