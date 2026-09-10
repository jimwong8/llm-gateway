package longcontext

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type poolClaimer struct{ calls atomic.Int32 }

func (c *poolClaimer) ClaimTask(ctx context.Context, _ string, _ time.Duration) (*Task, error) {
	c.calls.Add(1)
	return nil, nil
}

type poolProcessor struct{ calls atomic.Int32 }

func (p *poolProcessor) ProcessTask(context.Context, *Task) error {
	p.calls.Add(1)
	return nil
}

func TestWorkerPoolDisabledWithZeroWorkers(t *testing.T) {
	c := &poolClaimer{}
	if err := (&WorkerPool{Claimer: c, Processor: &poolProcessor{}, Workers: 0}).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if c.calls.Load() != 0 {
		t.Fatal("claimed with zero workers")
	}
}

func TestWorkerPoolStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (&WorkerPool{Claimer: &poolClaimer{}, Processor: &poolProcessor{}, Workers: 2, IdleDelay: time.Millisecond}).Run(ctx); err != context.Canceled {
		t.Fatalf("err=%v", err)
	}
}
