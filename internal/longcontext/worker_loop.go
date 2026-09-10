package longcontext

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type TaskClaimer interface {
	ClaimTask(context.Context, string, time.Duration) (*Task, error)
}
type TaskProcessor interface {
	ProcessTask(context.Context, *Task) error
}
type WorkerLoop struct {
	Claimer   TaskClaimer
	Processor TaskProcessor
	Workers   int
	Lease     time.Duration
	IdleDelay time.Duration
}

func waitWorkerRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (l WorkerLoop) Run(ctx context.Context) error {
	if l.Claimer == nil || l.Processor == nil {
		return fmt.Errorf("worker loop dependencies are required")
	}
	n := l.Workers
	if n <= 0 {
		return nil
	}
	lease := l.Lease
	if lease <= 0 {
		lease = time.Minute
	}
	idle := l.IdleDelay
	if idle <= 0 {
		idle = time.Second
	}
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			worker := fmt.Sprintf("map-worker-%d", i+1)
			for {
				if ctx.Err() != nil {
					return
				}
				task, err := l.Claimer.ClaimTask(ctx, worker, lease)
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
				if err := l.Processor.ProcessTask(ctx, task); err != nil {
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
