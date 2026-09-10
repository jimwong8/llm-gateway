package longcontext

import (
	"context"
	"fmt"
	"time"
)

type ChunkClaimer interface {
	ClaimChunk(context.Context, string, string, time.Duration) (*Chunk, error)
	ReleaseChunk(context.Context, string, string) error
}
type ChannelMapCaller interface {
	CallMap(context.Context, string, Chunk) (string, error)
}
type AttemptRecorder interface {
	RecordAttempt(context.Context, Attempt) error
	MarkChunkFailed(context.Context, string, string, string) error
}
type SingleMapRunner struct {
	Claimer  ChunkClaimer
	Selector *ChannelSelector
	Caller   ChannelMapCaller
	Attempts AttemptRecorder
	WorkerID string
	Lease    time.Duration
}

func (r *SingleMapRunner) RunOnce(ctx context.Context, taskID string) error {
	if r.Claimer == nil || r.Selector == nil || r.Caller == nil {
		return fmt.Errorf("map runner dependencies are required")
	}
	lease := r.Lease
	if lease <= 0 {
		lease = time.Minute
	}
	c, err := r.Claimer.ClaimChunk(ctx, taskID, r.WorkerID, lease)
	if err != nil {
		return err
	}
	if c == nil {
		return nil
	}
	now := time.Now()
	cid := r.Selector.Pick(now)
	if cid == "" {
		_ = r.Claimer.ReleaseChunk(ctx, c.ID, r.WorkerID)
		return fmt.Errorf("no healthy map channel")
	}
	started := time.Now()
	_, err = r.Caller.CallMap(ctx, cid, *c)
	latency := int(time.Since(started).Milliseconds())
	if err != nil {
		r.Selector.Failure(cid, 500, now)
		if r.Attempts != nil {
			_ = r.Attempts.RecordAttempt(ctx, Attempt{TaskID: taskID, ChunkID: c.ID, Phase: "mapping", Model: "", Provider: cid, Status: "failed", LatencyMs: latency, ErrorType: "provider_error", ErrorMessage: err.Error()})
			_ = r.Attempts.MarkChunkFailed(ctx, c.ID, taskID, err.Error())
		}
		_ = r.Claimer.ReleaseChunk(ctx, c.ID, r.WorkerID)
		return err
	}
	r.Selector.Success(cid)
	if r.Attempts != nil {
		_ = r.Attempts.RecordAttempt(ctx, Attempt{TaskID: taskID, ChunkID: c.ID, Phase: "mapping", Model: "", Provider: cid, Status: "succeeded", LatencyMs: latency})
	}
	return nil
}
