package longcontext

import (
	"context"
	"encoding/json"
	"fmt"
)

type MapProvider interface {
	ChatCompletion(context.Context, string, string, int) (string, error)
}

type MapResultStore interface {
	SaveMapResult(context.Context, string, string, string, string, MapResult) error
}

type MapWorker struct {
	Provider     MapProvider
	Store        MapResultStore
	Model        string
	ProviderName string
	MaxTokens    int
}

func (w MapWorker) Process(ctx context.Context, chunk Chunk) error {
	if w.Provider == nil || w.Store == nil {
		return fmt.Errorf("map worker dependencies are required")
	}
	maxTokens := w.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2000
	}
	prompt := fmt.Sprintf("Return JSON only with keys claims, chunk_summary, open_dependencies, conflicts_seen. Every claim quote must be copied verbatim from the source. Source chunk:\n%s", chunk.Content)
	raw, err := w.Provider.ChatCompletion(ctx, w.Model, prompt, maxTokens)
	if err != nil {
		return fmt.Errorf("map provider: %w", err)
	}
	var result MapResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return fmt.Errorf("map response is not valid JSON: %w", err)
	}
	if err := ValidateMapResult(result, chunk); err != nil {
		return fmt.Errorf("map response validation: %w", err)
	}
	return w.Store.SaveMapResult(ctx, chunk.ID, chunk.TaskID, w.Model, w.ProviderName, result)
}
