package longcontext

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ReuseMappedChunks copies map results from previous tasks that processed the
// identical chunk content (same content_sha256). This avoids re-spending
// upstream TPM quota on repeat requests for the same document. Chunks whose
// reused result fails validation fall back to normal map processing.
//
// Returns the number of chunks that were satisfied from prior results.
func (p *FullTaskProcessor) ReuseMappedChunks(ctx context.Context, task *Task) (int, error) {
	// Load this task's chunks.
	chunks, err := p.Repo.LoadChunks(ctx, task.ID)
	if err != nil {
		return 0, fmt.Errorf("load chunks for reuse: %w", err)
	}
	if len(chunks) == 0 {
		return 0, nil
	}

	reused := 0
	for _, ch := range chunks {
		if ch.Status == "mapped" || ch.ContentHash == "" {
			continue
		}
		// Find a mapped chunk with identical content in another task.
		src, err := p.Repo.FindMappedChunkByHash(ctx, ch.ContentHash, task.ID)
		if err != nil || src == nil {
			continue // no prior result — leave for normal map
		}

		var result MapResult
		if err := json.Unmarshal(src.MapResult, &result); err != nil {
			continue // corrupt prior result — leave for normal map
		}
		// Validate the reused result against THIS task's chunk (same content
		// hash ⇒ same spans, so validation must pass for a sane result).
		if err := ValidateMapResult(result, ch); err != nil {
			continue
		}

		// Copy the result into this task's chunk.
		if err := p.Repo.CopyMappedResult(ctx, ch.ID, task.ID, src.MapModel, src.MapProvider, src.MapResult); err != nil {
			continue // DB error — leave for normal map
		}
		_ = p.Repo.AddCompletedChunk(ctx, task.ID, estimateTokens(ch.Content), estimateTokens(string(src.MapResult)), 0)
		reused++
	}
	return reused, nil
}

// keep time import used even if not referenced elsewhere in future edits
var _ = time.Now
