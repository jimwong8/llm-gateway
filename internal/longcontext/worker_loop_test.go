package longcontext

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type loopClaimer struct{ calls atomic.Int32 }

func (c *loopClaimer) ClaimTask(ctx context.Context, _ string, _ time.Duration) (*Task, error) {
	c.calls.Add(1)
	return nil, nil
}

type loopProcessor struct{}

func (loopProcessor) ProcessTask(context.Context, *Task) error { return nil }
func TestWorkerLoopDisabledWithZeroWorkers(t *testing.T) {
	c := &loopClaimer{}
	if err := (WorkerLoop{Claimer: c, Processor: loopProcessor{}, Workers: 0}).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if c.calls.Load() != 0 {
		t.Fatal("claimed with zero workers")
	}
}
func TestWorkerLoopStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (WorkerLoop{Claimer: &loopClaimer{}, Processor: loopProcessor{}, Workers: 2, IdleDelay: time.Millisecond}).Run(ctx); err != context.Canceled {
		t.Fatalf("err=%v", err)
	}
}
