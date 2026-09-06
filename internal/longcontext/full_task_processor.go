package longcontext

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"llm-gateway/gateway/internal/providers"
)

// FullTaskProcessor drives a single virtual-long-1m task through its entire
// lifecycle: queued -> ingesting -> mapping -> indexing -> retrieving ->
// synthesizing -> verifying -> succeeded. It is intended for background worker
// pools, not synchronous admin calls.
type FullTaskProcessor struct {
	Repo          *PostgresRepository
	MapChannel    string
	MapModel      string
	ReaderChannel string
	ReaderModel   string
	ReaderTopK    int
	Registry      *providers.Registry
	Lease         time.Duration
}

func (p *FullTaskProcessor) ProcessTask(ctx context.Context, task *Task) error {
	if p.Repo == nil || p.Registry == nil {
		return fmt.Errorf("full task processor dependencies are required")
	}
	if task == nil {
		return fmt.Errorf("task is required")
	}
	// Adopt defaults.
	if p.Lease <= 0 {
		p.Lease = time.Minute
	}
	if p.ReaderTopK <= 0 {
		p.ReaderTopK = 8
	}
	readerChannel := p.ReaderChannel
	if readerChannel == "" {
		readerChannel = p.MapChannel
	}
	readerModel := p.ReaderModel
	if readerModel == "" {
		readerModel = p.MapModel
	}

	// Renew task lease so concurrent workers/admin runs know this task is
	// being handled.
	if err := p.Repo.RenewLease(ctx, task.ID, "worker", p.Lease); err != nil {
		return fmt.Errorf("claim task lease: %w", err)
	}

	// Phase 1: ingest.
	if task.Status == StatusQueued {
		if err := p.Repo.TransitionTask(ctx, task.ID, StatusQueued, StatusIngesting, PhaseIngesting); err != nil {
			return fmt.Errorf("transition to ingesting: %w", err)
		}
		task.Status = StatusIngesting
		task.Phase = PhaseIngesting
	}
	if task.Status == StatusIngesting {
		if err := p.Repo.TransitionTask(ctx, task.ID, StatusIngesting, StatusMapping, PhaseMapping); err != nil {
			return fmt.Errorf("transition to mapping: %w", err)
		}
		task.Status = StatusMapping
		task.Phase = PhaseMapping
	}

	// Phase 2: map all pending chunks (skipped for small docs with SkipMap).
	if strategy.SkipMap && task.Status == StatusMapping {
		slog.Info("1Mvir skip map for small document", "task", task.ID)
		if err := p.Repo.TransitionTask(ctx, task.ID, StatusMapping, StatusIndexing, PhaseIndexing); err != nil {
			return fmt.Errorf("transition to indexing (skip map): %w", err)
		}
	}
	if task.Status == StatusMapping && !strategy.SkipMap {

		mapProcessor := MapTaskProcessor{
			Claimer:   p.Repo,
			Selector:  NewChannelSelector([]string{p.MapChannel}, 30*time.Second),
			Caller:    RegistryMapCaller{Registry: p.Registry, Model: p.MapModel, MaxTokens: 4096},
			Results:   p.Repo,
			Attempts:  p.Repo,
			Completed: p.Repo,
			WorkerID:  "worker-" + task.ID,
			Lease:     p.Lease,
			Model:     p.MapModel,
		}
		for {
			if err := p.Repo.RenewLease(ctx, task.ID, "worker", p.Lease); err != nil {
				return fmt.Errorf("renew task lease during map: %w", err)
			}
			err := mapProcessor.ProcessTask(ctx, task)
			if err != nil {
				return fmt.Errorf("map chunk: %w", err)
			}
			var n int
			if cerr := p.Repo.CountPendingChunks(ctx, task.ID, &n); cerr != nil || n == 0 {
				break
			}
		}
	}

	// Phase 3: synthesize answer if reader config is present.
	if task.Status == StatusMapping && readerModel != "" {
		if err := p.synthesize(ctx, task, readerChannel, readerModel); err != nil {
			return fmt.Errorf("synthesize: %w", err)
		}
	}

	return nil
}

func (p *FullTaskProcessor) synthesize(ctx context.Context, task *Task, readerChannel, readerModel string) error {
	phases := []struct{ from, to Status; ph Phase }{
		{StatusMapping, StatusIndexing, PhaseIndexing},
		{StatusIndexing, StatusRetrieving, PhaseRetrieving},
		{StatusRetrieving, StatusSynthesizing, PhaseSynthesizing},
	}
	for _, ph := range phases {
		if err := p.Repo.TransitionTask(ctx, task.ID, ph.from, ph.to, ph.ph); err != nil {
			return fmt.Errorf("transition to %s: %w", ph.to, err)
		}
	}

	evidence, err := p.Repo.GetEvidenceByTask(ctx, task.ID)
	if err != nil {
		return fmt.Errorf("fetch evidence: %w", err)
	}
	if len(evidence) == 0 {
		if err := p.Repo.TransitionTask(ctx, task.ID, StatusSynthesizing, StatusSucceeded, PhaseVerifying); err != nil {
			return fmt.Errorf("transition to succeeded: %w", err)
		}
		return p.Repo.StoreResult(ctx, task.ID, FinalReadResultJSON(FinalReadResult{Query: task.Query, Answer: "No evidence available for this query."}))
	}

	selected := HybridRetrieval(task.Query, evidence, p.ReaderTopK)
	chunkIDs := map[string]bool{}
	for _, ev := range selected {
		chunkIDs[ev.ChunkID] = true
	}
	var chunks []string
	for cid := range chunkIDs {
		c, cerr := p.Repo.GetChunkContent(ctx, cid)
		if cerr != nil {
			continue
		}
		chunks = append(chunks, c)
	}

	reader := RegistryFinalReader{
		Registry:  p.Registry,
		Model:     readerModel,
		Channel:   readerChannel,
		MaxTokens: 4096,
	}
	answer, err := reader.SynthesizeGroundedAnswer(ctx, task.Query, selected, chunks)
	if err != nil {
		return fmt.Errorf("final reader: %w", err)
	}

	reducedClaims := make([]ReducedClaim, len(selected))
	for i, ev := range selected {
		reducedClaims[i] = ReducedClaim{
			Claim: ev.Claim, Quote: ev.Quote, ChunkID: ev.ChunkID,
			StartChar: ev.StartChar, EndChar: ev.EndChar, Confidence: ev.Confidence,
		}
	}
	reduced, _ := ReduceClaims(reducedClaims)
	finalResult := MapToFinalResult(task.Query, answer, reduced)
	if err := p.Repo.StoreResult(ctx, task.ID, FinalReadResultJSON(finalResult)); err != nil {
		return fmt.Errorf("store result: %w", err)
	}

	if err := p.Repo.TransitionTask(ctx, task.ID, StatusSynthesizing, StatusVerifying, PhaseVerifying); err != nil {
		return fmt.Errorf("transition to verifying: %w", err)
	}
	if err := p.Repo.TransitionTask(ctx, task.ID, StatusVerifying, StatusSucceeded, PhaseVerifying); err != nil {
		return fmt.Errorf("transition to succeeded: %w", err)
	}
	return nil
}
// AdaptiveStrategy determines pipeline parameters based on document size
// and query complexity. It is computed once at the start of ProcessTask.
type adaptiveStrategy struct {
	SkipMap          bool
	ReaderTopK       int
	ReaderCandidates int
	MapTimeout       time.Duration
	SynthTimeout     time.Duration
}

func computeAdaptive(inputTokens int64, query string) adaptiveStrategy {
	switch {
	case inputTokens < 2000:
		// Small document (<2K tokens): single chunk, skip Map overhead,
		// minimal reader race. Latency target: <10s.
		return adaptiveStrategy{
			SkipMap:          true,
			ReaderTopK:       4,
			ReaderCandidates: 2,
			MapTimeout:       30 * time.Second,
			SynthTimeout:     60 * time.Second,
		}
	case inputTokens < 100000:
		// Medium document (2K-100K tokens): standard pipeline.
		return adaptiveStrategy{
			SkipMap:          false,
			ReaderTopK:       16,
			ReaderCandidates: 6,
			MapTimeout:       3 * time.Minute,
			SynthTimeout:     5 * time.Minute,
		}
	default:
		// Large document (>100K tokens): deeper retrieval, more reader candidates.
		return adaptiveStrategy{
			SkipMap:          false,
			ReaderTopK:       32,
			ReaderCandidates: 8,
			MapTimeout:       5 * time.Minute,
			SynthTimeout:     8 * time.Minute,
		}
	}
}


