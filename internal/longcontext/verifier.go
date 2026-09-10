package longcontext

import (
	"encoding/json"
	"fmt"
)

// VerificationResult holds the outcome of verifying a synthesized answer.
type VerificationResult struct {
	Pass               bool     `json:"pass"`
	CitationCoverage   float64  `json:"citation_coverage"`
	CitedEvidenceIDs   []string `json:"cited_evidence_ids"`
	InvalidCitations   []string `json:"invalid_citations,omitempty"`
	TotalEvidenceCount int      `json:"total_evidence_count"`
	Errors             []string `json:"errors,omitempty"`
}

// Verifier checks that a synthesized answer's citations are valid and
// citation coverage is above the configured threshold.
type Verifier struct {
	MinCitationCoverage float64
}

// Verify checks the answer text against the evidence set.
// The evidence slice order defines citation indices: [E1]=evidence[0], [E2]=evidence[1], etc.
func (v *Verifier) Verify(answer string, evidence []Evidence) VerificationResult {
	vr := VerificationResult{
		TotalEvidenceCount: len(evidence),
		Errors:             []string{},
	}

	cited := extractCitationIDs(answer)
	vr.CitedEvidenceIDs = cited

	// Build valid citation set from 1-based slice index.
	validCount := len(evidence)

	// Check each cited ID is within evidence range.
	var invalid []string
	for _, cid := range cited {
		// Extract number from [E<n]>
		n := 0
		if _, err := fmt.Sscanf(cid, "[E%d]", &n); err != nil || n < 1 || n > validCount {
			invalid = append(invalid, cid)
		}
	}
	vr.InvalidCitations = invalid

	// Calculate coverage: how many distinct evidence items are cited.
	citedSet := make(map[int]bool)
	for _, cid := range cited {
		n := 0
		if _, err := fmt.Sscanf(cid, "[E%d]", &n); err == nil && n >= 1 && n <= validCount {
			citedSet[n] = true
		}
	}
	if validCount > 0 {
		vr.CitationCoverage = float64(len(citedSet)) / float64(validCount)
	} else {
		vr.CitationCoverage = 1.0
	}

	// Determine pass/fail.
	if len(invalid) > 0 {
		vr.Pass = false
		vr.Errors = append(vr.Errors, "answer contains citations to non-existent evidence")
		return vr
	}

	minCoverage := v.MinCitationCoverage
	if minCoverage <= 0 {
		minCoverage = 0.3
	}

	if vr.CitationCoverage < minCoverage && validCount > 0 {
		vr.Pass = false
		vr.Errors = append(vr.Errors,
			fmt.Sprintf("citation coverage %.2f below threshold %.2f", vr.CitationCoverage, minCoverage))
	} else {
		vr.Pass = true
	}

	return vr
}

// AttachVerificationResult appends verification data to a task's stored result JSON.
func AttachVerificationResult(resultJSON []byte, vr VerificationResult) ([]byte, error) {
	if len(resultJSON) == 0 {
		resultJSON = []byte("{}")
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(resultJSON, &m); err != nil {
		return nil, fmt.Errorf("parse stored result: %w", err)
	}
	vrBytes, err := json.Marshal(vr)
	if err != nil {
		return nil, fmt.Errorf("marshal verification: %w", err)
	}
	m["verification"] = vrBytes
	return json.Marshal(m)
}
