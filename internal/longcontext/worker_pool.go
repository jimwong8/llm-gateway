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
				if err := p.Processor.ProcessTask(ctx, task); err != nil {
					// Errors are persisted on the task by the processor; just
					// back off briefly before claiming the next task.
					if !waitWorkerRetry(ctx, 500*time.Millisecond) {
						return
					}
				}
			}
		}(i)
	}
	wg.Wait()
	return ctx.Err()
}
