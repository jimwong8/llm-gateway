package longcontext

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// WorkerPool runs N background workers that claim and process long-context
// tasks. The pool is a no-op when Workers == 0.
type WorkerPool struct {
	Claimer   TaskClaimer
	Processor TaskProcessor
	Workers   int
	Lease     time.Duration
	IdleDelay time.Duration
}

func (p *WorkerPool) Run(ctx context.Context) error {
	if p.Claimer == nil || p.Processor == nil {
		return fmt.Errorf("worker pool dependencies are required")
	}
	n := p.Workers
	if n <= 0 {
		return nil
	}
	SetWorkerPoolSize(n)
	lease := p.Lease
	if lease <= 0 {
		lease = time.Minute
	}
	idle := p.IdleDelay
	if idle <= 0 {
		idle = time.Second
	}
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			worker := fmt.Sprintf("longcontext-worker-%d", i+1)
			// Create a per-worker processor copy with matching WorkerID
			// so that RenewLease inside ProcessTask uses the same identity
			// that ClaimTask used to acquire the task.
			var workerProc TaskProcessor = p.Processor
			if fp, ok := p.Processor.(*FullTaskProcessor); ok {
				copy := *fp
				copy.WorkerID = worker
				copy.Lease = lease
				workerProc = &copy
			}
			for {
				if ctx.Err() != nil {
					return
				}
				task, err := p.Claimer.ClaimTask(ctx, worker, lease)
				if err != nil {
					if !waitWorkerRetry(ctx, 500*time.Millisecond) {
						return
					}
					continue
				}
				if task == nil {
					timer := time.NewTimer(idle)
					select {
					case <-ctx.Done():
						timer.Stop()
						return
					case <-timer.C:
						continue
					}
				}
				SetActiveWorkers(1)
				if err := workerProc.ProcessTask(ctx, task); err != nil {
					RecordTaskFailed("map_chunk")
					if failed, ok := p.Claimer.(interface {
						FailTask(context.Context, string, string, string) error
					}); ok {
						_ = failed.FailTask(ctx, task.ID, task.TenantID, err.Error())
					}
					if !waitWorkerRetry(ctx, 500*time.Millisecond) {
						SetActiveWorkers(0)
						return
					}
				}
				SetActiveWorkers(0)
			}
		}(i)
	}
	wg.Wait()
	return ctx.Err()
}
