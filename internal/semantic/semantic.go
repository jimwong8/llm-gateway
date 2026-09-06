package semantic

import (
	"context"

	"llm-gateway/gateway/internal/providers"
)

// L2Cache defines the semantic cache interface with fuzzy matching.
type L2Cache interface {
	Search(ctx context.Context, req providers.ChatCompletionRequest) (*SearchHit, error)
	Upsert(ctx context.Context, req providers.ChatCompletionRequest, resp providers.ChatCompletionResponse) error
	EnsureCollection(ctx context.Context) error
	SetEmbedder(embedder EmbeddingClient)
	SetPersistence(path string)
}

// SearchHit represents a semantic cache hit.
type SearchHit struct {
	Score     float64
	Response  providers.ChatCompletionResponse
	Prompt    string
	Model     string
	TenantID  string
	UserID    string
	SessionID string
}
