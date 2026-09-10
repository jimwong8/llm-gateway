package longcontext

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"llm-gateway/gateway/internal/providers"
)

// FullTaskProcessor drives a single virtual-long-1m task through its entire
// lifecycle: queued -> ingesting -> mapping -> indexing -> retrieving ->
// synthesizing -> verifying -> succeeded. It is intended for background worker
// pools, not synchronous admin calls.
type FullTaskProcessor struct {
	Repo             *PostgresRepository
	MapChannel       string
	MapChannels      []string
	MapModel         string
	ReaderChannel    string
	ReaderModel      string
	ReaderTopK       int
	ReaderCandidates []ReaderCandidate // model@channel pairs for parallel final-reader race
	WorkerID         string            // lease identity; defaults to "worker"
	Registry         *providers.Registry
	Lease            time.Duration
	MapTimeout       time.Duration
	SynthTimeout     time.Duration
	Embedder         *providers.EmbeddingClient
}

func (p *FullTaskProcessor) mapChannelList() []string {
	if len(p.MapChannels) > 0 {
		return p.MapChannels
	}
	return []string{p.MapChannel}
}

// adaptiveStrategy determines pipeline parameters based on document size
// and query complexity. Computed once at the start of ProcessTask.
type adaptiveStrategy struct {
	ReaderTopK       int
	ReaderCandidates int
	MapTimeout       time.Duration
	SynthTimeout     time.Duration
}

func computeAdaptive(inputTokens int64, query string) adaptiveStrategy {
	switch {
	case inputTokens < 2000:
		// Small document (<2K tokens): single chunk, minimal reader race.
		// Latency target: <10s.
		return adaptiveStrategy{
			ReaderTopK:       4,
			ReaderCandidates: 2,
			MapTimeout:       30 * time.Second,
			SynthTimeout:     60 * time.Second,
		}
	case inputTokens < 100000:
		// Medium document (2K-100K tokens): standard pipeline.
		return adaptiveStrategy{
			ReaderTopK:       16,
			ReaderCandidates: 6,
			MapTimeout:       3 * time.Minute,
			SynthTimeout:     5 * time.Minute,
		}
	default:
		// Large document (>100K tokens): deeper retrieval, more reader candidates.
		return adaptiveStrategy{
			ReaderTopK:       32,
			ReaderCandidates: 8,
			MapTimeout:       5 * time.Minute,
			SynthTimeout:     8 * time.Minute,
		}
	}
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
	if err := p.Repo.RenewLease(ctx, task.ID, p.workerID(), p.Lease); err != nil {
		return fmt.Errorf("claim task lease: %w", err)
	}

	// Adaptive strategy: adjust pipeline parameters based on document size.
	strategy := computeAdaptive(task.InputTokens, task.Query)
	if strategy.ReaderTopK > 0 {
		p.ReaderTopK = strategy.ReaderTopK
	}
	if strategy.ReaderCandidates > 0 && len(p.ReaderCandidates) > strategy.ReaderCandidates {
		p.ReaderCandidates = p.ReaderCandidates[:strategy.ReaderCandidates]
	}
	if strategy.MapTimeout > 0 {
		p.MapTimeout = strategy.MapTimeout
	}
	if strategy.SynthTimeout > 0 {
		p.SynthTimeout = strategy.SynthTimeout
	}
	slog.Info("1Mvir adaptive strategy", "input_tokens", task.InputTokens, "top_k", p.ReaderTopK, "reader_candidates", len(p.ReaderCandidates), "map_timeout", p.MapTimeout.String(), "synth_timeout", p.SynthTimeout.String())

	// Cost guard: abort if estimated cost already exceeds the task's max_cost.
	if task.MaxCost != nil && task.EstimatedCost > *task.MaxCost {
		_ = p.Repo.CancelTask(ctx, task.ID, task.TenantID)
		_ = p.Repo.TransitionTask(ctx, task.ID, task.Status, StatusFailed, PhaseQueued)
		return fmt.Errorf("estimated cost %.4f exceeds max_cost %.4f", task.EstimatedCost, *task.MaxCost)
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

	// Phase 1.5: reuse mapped results from prior tasks with identical chunk
	// content — avoids re-spending upstream TPM on repeat requests.
	if reused, rerr := p.ReuseMappedChunks(ctx, task); rerr == nil && reused > 0 {
		slog.Info("1Mvir chunks reused from prior tasks", "task", task.ID, "reused", reused)
	}

	// Phase 2: map all pending chunks concurrently.
	if task.Status == StatusMapping {
		mapTimeout := p.MapTimeout
		if mapTimeout <= 0 {
			mapTimeout = 3 * time.Minute
		}
		mapCtx, cancel := context.WithTimeout(ctx, mapTimeout)
		defer cancel()
		sem := make(chan struct{}, 1)
		var wg sync.WaitGroup
		var mu sync.Mutex
		var firstErr error
		selector := NewChannelSelector(p.mapChannelList(), 30*time.Second)
		go func() {
			ticker := time.NewTicker(p.Lease / 2)
			defer ticker.Stop()
			for range ticker.C {
				_ = p.Repo.RenewLease(ctx, task.ID, p.workerID(), p.Lease)
			}
		}()
		for {
			var n int
			if cerr := p.Repo.CountPendingChunks(mapCtx, task.ID, &n); cerr != nil || n == 0 {
				break
			}
			sem <- struct{}{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				mp := MapTaskProcessor{
					Claimer:    p.Repo,
					Selector:   selector,
					Caller:     RegistryMapCaller{Registry: p.Registry, Model: p.MapModel, MaxTokens: 16384, ReasoningEffort: "none", ResponseFormat: map[string]any{"type": "json_object"}},
					Results:    p.Repo,
					Attempts:   p.Repo,
					Completed:  p.Repo,
					WorkerID:   fmt.Sprintf("worker-%s-%d", task.ID, time.Now().UnixNano()),
					Lease:      p.Lease,
					Model:      p.MapModel,
					MaxRetries: 8,
				}
				if err := mp.ProcessTask(mapCtx, task); err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		if firstErr != nil {
			return fmt.Errorf("map chunk: %w", firstErr)
		}
	}
	// Phase 3: synthesize answer if reader config is present.
	if task.Status == StatusMapping && readerModel != "" {
		synthTimeout := p.SynthTimeout
		if synthTimeout <= 0 {
			synthTimeout = 5 * time.Minute
		}
		synthCtx, cancel := context.WithTimeout(ctx, synthTimeout)
		defer cancel()
		if err := p.synthesize(synthCtx, task, readerChannel, readerModel); err != nil {
			return fmt.Errorf("synthesize: %w", err)
		}
	}

	return nil
}

func (p *FullTaskProcessor) synthesize(ctx context.Context, task *Task, readerChannel, readerModel string) error {
	phases := []struct {
		from, to Status
		ph       Phase
	}{
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
		if err := p.Repo.TransitionTask(ctx, task.ID, StatusSynthesizing, StatusVerifying, PhaseVerifying); err != nil {
			return fmt.Errorf("transition to verifying: %w", err)
		}
		if err := p.Repo.TransitionTask(ctx, task.ID, StatusVerifying, StatusSucceeded, PhaseVerifying); err != nil {
			return fmt.Errorf("transition to succeeded: %w", err)
		}
		return p.Repo.StoreResult(ctx, task.ID, FinalReadResultJSON(FinalReadResult{Query: task.Query, Answer: "No evidence available for this query."}))
	}

	if p.Embedder != nil {
		// Backfill missing evidence embeddings.
		var need []int64
		var idx []int
		for i := range evidence {
			if len(evidence[i].Embedding) == 0 {
				need = append(need, evidence[i].ID)
				idx = append(idx, i)
			}
		}
		if len(need) > 0 {
			texts := make([]string, len(need))
			for j, evIdx := range idx {
				texts[j] = evidence[evIdx].Claim + " " + evidence[evIdx].Quote
			}
			if vecs, err := p.Embedder.EmbedBatch(ctx, texts); err == nil {
				for j, v := range vecs {
					evIdx := idx[j]
					evidence[evIdx].Embedding = v
					_ = p.Repo.UpdateEvidenceEmbedding(ctx, need[j], v)
				}
			}
		}
	}
	selected := HybridRetrieval(task.Query, evidence, p.ReaderTopK)
	if p.Embedder != nil {
		if qvec, err := p.Embedder.Embed(ctx, task.Query); err == nil {
			selected = VectorHybridRetrieval(task.Query, qvec, evidence, p.ReaderTopK)
		}
	}

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

	answer, err := p.synthesizeWithCandidates(ctx, task.Query, selected, chunks, readerChannel, readerModel)
	if err != nil {
		return fmt.Errorf("final reader: %w", err)
	}
	// Account for final reader tokens/cost.
	readerInputTokens := estimateTokens(task.Query)
	for _, ev := range selected {
		readerInputTokens += estimateTokens(ev.Claim + " " + ev.Quote)
	}
	readerOutputTokens := estimateTokens(answer)
	if aerr := p.Repo.AddTaskUsage(ctx, task.ID, readerInputTokens, readerOutputTokens, estimateCost(readerInputTokens+readerOutputTokens)); aerr != nil {
		return fmt.Errorf("record reader cost: %w", aerr)
	}
	// Refresh task after cost update to re-evaluate max_cost.
	task, err = p.Repo.GetTask(ctx, task.ID, task.TenantID)
	if err != nil {
		return fmt.Errorf("refresh task after reader: %w", err)
	}
	if task.MaxCost != nil && task.EstimatedCost > *task.MaxCost {
		_ = p.Repo.CancelTask(ctx, task.ID, task.TenantID)
		_ = p.Repo.TransitionTask(ctx, task.ID, task.Status, StatusFailed, PhaseQueued)
		return fmt.Errorf("estimated cost %.4f exceeds max_cost %.4f", task.EstimatedCost, *task.MaxCost)
	}

	reducedClaims := make([]ReducedClaim, len(selected))
	for i, ev := range selected {
		reducedClaims[i] = ReducedClaim{
			Claim: ev.Claim, Quote: ev.Quote, ChunkID: ev.ChunkID,
			StartChar: ev.StartChar, EndChar: ev.EndChar, Confidence: ev.Confidence,
		}
	}
	reduced, _ := ReduceClaims(reducedClaims)

	// Phase 4: verify answer citations before storing and marking succeeded.
	verifier := Verifier{MinCitationCoverage: 0.15}
	vResult := verifier.Verify(answer, selected)
	if !vResult.Pass {
		fmt.Printf("verifier: task_id=%s errors=%v coverage=%.2f\n", task.ID, vResult.Errors, vResult.CitationCoverage)
	}

	finalResult := MapToFinalResult(task.Query, answer, reduced)
	finalResult.Verification = &vResult
	ObserveEvidenceCount(len(selected))
	if vResult.Pass == false {
		RecordVerificationFailure()
	}
	if err := p.Repo.StoreResult(ctx, task.ID, FinalReadResultJSON(finalResult)); err != nil {
		return fmt.Errorf("store result: %w", err)
	}
	if err := p.Repo.TransitionTask(ctx, task.ID, StatusSynthesizing, StatusVerifying, PhaseVerifying); err != nil {
		return fmt.Errorf("transition to verifying: %w", err)
	}
	if err := p.Repo.TransitionTask(ctx, task.ID, StatusVerifying, StatusSucceeded, PhaseVerifying); err != nil {
		return fmt.Errorf("transition to succeeded: %w", err)
	}
	RecordTaskSucceeded()
	return nil
}

func (p *FullTaskProcessor) workerID() string {
	if p.WorkerID != "" {
		return p.WorkerID
	}
	return "worker"
}
