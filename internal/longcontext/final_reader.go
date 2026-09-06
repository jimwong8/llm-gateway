package longcontext

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"llm-gateway/gateway/internal/providers"
)

// FinalReaderCaller defines the interface for the final-synthesis step.
type FinalReaderCaller interface {
	// SynthesizeGroundedAnswer calls a real model with the evidence context
	// and returns the synthesized answer text.
	SynthesizeGroundedAnswer(ctx context.Context, query string, evidence []Evidence, chunks []string) (string, error)
}

// RegistryFinalReader adapts the channel-isolated provider registry to
// SynthesizeGroundedAnswer.  It sends a carefully designed prompt that
// instructs the model to ONLY use the provided evidence and cite it.
type RegistryFinalReader struct {
	Registry  *providers.Registry
	Model     string
	Channel   string
	MaxTokens int
}

func (r RegistryFinalReader) SynthesizeGroundedAnswer(ctx context.Context, query string, evidence []Evidence, chunks []string) (string, error) {
	if r.Registry == nil {
		return "", fmt.Errorf("provider registry is required")
	}
	model := strings.TrimSpace(r.Model)
	if model == "" {
		return "", fmt.Errorf("final reader model is required")
	}
	channel := strings.TrimSpace(r.Channel)
	if channel == "" {
		return "", fmt.Errorf("final reader channel is required")
	}
	maxTokens := r.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	// Build evidence block.
	var evidenceBlock strings.Builder
	for i, ev := range evidence {
		fmt.Fprintf(&evidenceBlock, "[E%d] %s\n  Quote: %s\n  Confidence: %.2f\n\n", i+1, ev.Claim, ev.Quote, ev.Confidence)
	}

	// Build context block (source chunks for the model to verify if needed).
	var contextBlock strings.Builder
	for i, chunk := range chunks {
		preview := chunk
		if len(preview) > 2000 {
			preview = preview[:2000] + "...(truncated)"
		}
		fmt.Fprintf(&contextBlock, "[Chunk %d]\n%s\n\n", i+1, preview)
	}

	systemPrompt := `You are a precise research assistant. You MUST answer the question using ONLY the provided evidence. Every factual statement in your answer MUST cite its evidence using [E<n] notation. If the evidence does not contain enough information to answer, say so explicitly. Do not fabricate facts.`

	userPrompt := fmt.Sprintf(`Question: %s

Available Evidence:
%s
Source Chunks (for verification):
%s
Provide a concise, accurate answer. Cite evidence with [E<n] for each factual claim.`,
		query, evidenceBlock.String(), contextBlock.String())

	resp, err := r.Registry.ChatCompletionOnChannel(ctx, channel, providers.ChatCompletionRequest{
		Model:    model,
		MaxTokens: maxTokens,
		Messages: []providers.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		return "", fmt.Errorf("final reader call failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("final reader returned no choices")
	}
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("final reader returned empty content")
	}
	return content, nil
}

// FinalReadResult is the JSON payload stored in task.result for a completed
// virtual-long-1m task.
type FinalReadResult struct {
	Query            string            `json:"query"`
	Answer           string            `json:"answer"`
	EvidenceCount    int               `json:"evidence_count"`
	CitedEvidenceIDs []string          `json:"cited_evidence_ids"`
	Conflicts        []ConflictGroup   `json:"conflicts,omitempty"`
	CitationCoverage float64           `json:"citation_coverage"`
}

// MapToFinalResult assembles the Reduce + FinalReader outputs into the
// task.result schema.
func MapToFinalResult(query string, answer string, reduced ReduceResult) FinalReadResult {
	citedIDs := extractCitationIDs(answer)
	return FinalReadResult{
		Query:            query,
		Answer:           answer,
		EvidenceCount:    len(reduced.Claims),
		CitedEvidenceIDs: citedIDs,
		Conflicts:        reduced.Conflicts,
		CitationCoverage: reduced.CitationCoverage,
	}
}

// extractCitationIDs finds all [E1], [E2], etc. references in the answer text.
func extractCitationIDs(answer string) []string {
	var ids []string
	seen := map[string]bool{}
	for i := 1; i <= 100; i++ {
		tag := fmt.Sprintf("[E%d]", i)
		if strings.Contains(answer, tag) && !seen[tag] {
			seen[tag] = true
			ids = append(ids, tag)
		}
	}
	return ids
}

// FinalReadResultJSON serializes a FinalReadResult to a raw JSON byte slice
// suitable for storage in long_context_tasks.result.
func FinalReadResultJSON(result FinalReadResult) json.RawMessage {
	b, _ := json.Marshal(result)
	return b
}
