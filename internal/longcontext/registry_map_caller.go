package longcontext

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"llm-gateway/gateway/internal/providers"
)

// RegistryMapCaller adapts the real channel-isolated provider registry to Map.
type RegistryMapCaller struct {
	Registry  *providers.Registry
	Model     string
	MaxTokens int
}

func (c RegistryMapCaller) CallMap(ctx context.Context, channel string, chunk Chunk) (string, error) {
	if c.Registry == nil { return "", fmt.Errorf("provider registry is required") }
	if strings.TrimSpace(channel) == "" { return "", fmt.Errorf("map channel is required") }
	model := strings.TrimSpace(c.Model)
	if model == "" { return "", fmt.Errorf("map model is required") }
	maxTokens := c.MaxTokens
	if maxTokens <= 0 { maxTokens = 4096 }
	prompt := fmt.Sprintf("Output ONLY a single valid JSON object, no markdown fences, no explanation. Schema: {\"claims\":[{\"claim\":\"string\",\"quote\":\"exact substring from source\",\"start_char\":0,\"end_char\":0,\"entities\":[],\"relations\":[],\"confidence\":0.5}],\"chunk_summary\":\"string\",\"open_dependencies\":[],\"conflicts_seen\":[]}. Every quote must be copied verbatim from the source chunk and its span must stay inside the chunk. Source chunk:\n%s", chunk.Content)
	resp, err := c.Registry.ChatCompletionOnChannel(ctx, channel, providers.ChatCompletionRequest{
		Model: model, MaxTokens: maxTokens,
		Messages: []providers.ChatMessage{
			{Role: "system", Content: "You extract grounded facts from document chunks. Reply with one valid JSON object only."},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil { return "", err }
	if len(resp.Choices) == 0 { return "", fmt.Errorf("map provider returned no choices") }
	content := extractFirstJSONObject(resp.Choices[0].Message.Content)
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("map provider returned empty content (finish_reason=%q)", resp.Choices[0].FinishReason)
	}
	// Repair common model JSON deviations (unquoted keys, single quotes,
	// trailing commas, fences) before returning to the validator.
	repaired, _ := RepairMapJSON([]byte(content))
	// Pre-parse to fail with the actual content when the model truncates JSON.
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(repaired, &probe); err != nil {
		head := content
		if len(head) > 300 { head = head[:300] }
		return "", fmt.Errorf("map response is not valid JSON (%v); head=%q", err, head)
	}
	return string(repaired), nil
}

// extractFirstJSONObject tolerates models that wrap JSON in markdown fences or
// add prose, by slicing from the first '{' to the matching final '}'.
func extractFirstJSONObject(s string) string {
	s = strings.TrimSpace(s)
	start := strings.IndexByte(s, '{')
	if start < 0 { return s }
	end := strings.LastIndexByte(s, '}')
	if end <= start { return s }
	return s[start : end+1]
}
