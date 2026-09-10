package longcontext

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// BlockingChannelSelector is satisfied by *ChannelSelector.
type BlockingChannelSelector interface {
	PickWithTimeout(context.Context) string
}

type CompletedChunkRecorder interface {
	AddCompletedChunk(context.Context, string, int, int, float64) error
}
type MapTaskProcessor struct {
	Claimer          ChunkClaimer
	Selector         *ChannelSelector
	BlockingSelector BlockingChannelSelector
	Caller           ChannelMapCaller
	Results          interface {
		SaveMapResult(context.Context, string, string, string, string, MapResult) error
	}
	Attempts           AttemptRecorder
	Completed          CompletedChunkRecorder
	WorkerID           string
	Lease              time.Duration
	Model              string
	ChannelPickTimeout time.Duration
	MaxRetries         int
}

func (p *MapTaskProcessor) ProcessTask(ctx context.Context, task *Task) error {
	if task == nil {
		return fmt.Errorf("task is required")
	}
	if p.Claimer == nil || p.Selector == nil || p.Caller == nil || p.Results == nil {
		return fmt.Errorf("map task processor dependencies are required")
	}
	lease := p.Lease
	if lease <= 0 {
		lease = time.Minute
	}
	chunk, err := p.Claimer.ClaimChunk(ctx, task.ID, p.WorkerID, lease)
	if err != nil {
		return err
	}
	if chunk == nil {
		return nil
	}
	maxRetries := p.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	if chunk.MapAttempts > maxRetries {
		_ = p.Attempts.MarkChunkFailed(ctx, chunk.ID, task.ID, "max retries exceeded")
		// Do NOT call ReleaseChunk here — MarkChunkFailed already set status='failed'.
		// ReleaseChunk would reset it to 'pending', causing an infinite retry loop.
		return nil
	}
	var channel string
	if p.BlockingSelector != nil && p.ChannelPickTimeout > 0 {
		pickCtx, cancel := context.WithTimeout(ctx, p.ChannelPickTimeout)
		channel = p.BlockingSelector.PickWithTimeout(pickCtx)
		cancel()
	} else {
		channel = p.Selector.Pick(time.Now())
	}
	if channel == "" {
		_ = p.Claimer.ReleaseChunk(ctx, chunk.ID, p.WorkerID)
		return fmt.Errorf("no healthy map channel")
	}
	started := time.Now()
	raw, err := p.Caller.CallMap(ctx, channel, *chunk)
	latency := int(time.Since(started).Milliseconds())
	if err != nil {
		return p.fail(ctx, task, chunk, channel, latency, err)
	}
	var result MapResult
	if err = json.Unmarshal([]byte(raw), &result); err != nil {
		if repaired, ok := RepairMapJSON([]byte(raw)); ok {
			if err2 := json.Unmarshal(repaired, &result); err2 == nil {
				raw = string(repaired)
				err = nil
			}
		}
		if err != nil {
			return p.fail(ctx, task, chunk, channel, latency, fmt.Errorf("map response is not valid JSON: %w", err))
		}
	}
	if err = p.Results.SaveMapResult(ctx, chunk.ID, task.ID, p.Model, channel, result); err != nil {
		return p.fail(ctx, task, chunk, channel, latency, err)
	}
	p.Selector.Success(channel)
	if p.Attempts != nil {
		_ = p.Attempts.RecordAttempt(ctx, Attempt{TaskID: task.ID, ChunkID: chunk.ID, Phase: "mapping", Model: p.Model, Provider: channel, Status: "succeeded", LatencyMs: latency})
	}
	if p.Completed != nil {
		inputTokens := estimateTokens(chunk.Content)
		outputTokens := estimateTokens(raw)
		cost := estimateCost(inputTokens + outputTokens)
		if err = p.Completed.AddCompletedChunk(ctx, task.ID, inputTokens, outputTokens, cost); err != nil {
			return err
		}
	}
	return nil
}
func (p *MapTaskProcessor) fail(ctx context.Context, task *Task, chunk *Chunk, channel string, latency int, err error) error {
	// Extract the real upstream HTTP status (429/403/5xx) so the selector can
	// apply the right cooldown: 429 -> cooldown window, 403 -> long disable,
	// other 5xx -> escalating failures. Defaulting to 500 made every 429 look
	// like a transient 5xx and the selector kept hammering the throttled pool.
	status := 500
	if he, ok := err.(interface{ HTTPStatusCode() int }); ok {
		status = he.HTTPStatusCode()
	} else {
		// Fall back to parsing the wrapped message ("upstream http 429: ...").
		if idx := strings.Index(err.Error(), "upstream http "); idx >= 0 {
			rest := err.Error()[idx+len("upstream http "):]
			var n int
			if _, scanErr := fmt.Sscanf(rest, "%d", &n); scanErr == nil && n >= 400 {
				status = n
			}
		}
	}
	p.Selector.Failure(channel, status, time.Now())
	maxRetries := p.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	if p.Attempts != nil {
		_ = p.Attempts.RecordAttempt(ctx, Attempt{TaskID: task.ID, ChunkID: chunk.ID, Phase: "mapping", Model: p.Model, Provider: channel, Status: "failed", LatencyMs: latency, ErrorType: "map_error", ErrorMessage: err.Error()})
		if chunk.MapAttempts >= maxRetries {
			// Exceeded retry limit: mark this chunk as permanently failed and
			// STOP trying it. We still return nil so the full-task loop can
			// move on to the remaining pending chunks (a single poisoned chunk
			// must not abort the whole task).
			_ = p.Attempts.MarkChunkFailed(ctx, chunk.ID, task.ID, err.Error())
			// Do NOT call ReleaseChunk — it resets status to 'pending' causing infinite retry.
			slog.Warn("chunk permanently failed after retries", "task", task.ID, "chunk", chunk.Ordinal, "channel", channel, "err", err)
			return nil
		}
	}
	// Transient failure: release the chunk back to pending so a subsequent
	// claim can pick a different (non-cooling) channel. Returning nil lets the
	// full-task processor continue the map loop instead of aborting the task.
	_ = p.Claimer.ReleaseChunk(ctx, chunk.ID, p.WorkerID)
	return nil
}

func estimateTokens(text string) int {
	return len([]rune(text)) / 4
}

func estimateCost(tokens int) float64 {
	// Rough estimate: $0.01 per 1K tokens.
	return float64(tokens) * 0.00001
}
