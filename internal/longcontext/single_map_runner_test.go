package longcontext

import (
	"context"
	"testing"
	"time"
)

type fakeClaimer struct {
	chunk    *Chunk
	released bool
}

func (f *fakeClaimer) ClaimChunk(context.Context, string, string, time.Duration) (*Chunk, error) {
	return f.chunk, nil
}
func (f *fakeClaimer) ReleaseChunk(context.Context, string, string) error {
	f.released = true
	return nil
}

type fakeCaller struct {
	called string
	err    error
}

func (f *fakeCaller) CallMap(_ context.Context, cid string, _ Chunk) (string, error) {
	f.called = cid
	return "", f.err
}
func TestSingleMapRunnerUsesHealthyChannel(t *testing.T) {
	c := &fakeClaimer{chunk: &Chunk{ID: "c"}}
	p := &fakeCaller{}
	r := &SingleMapRunner{Claimer: c, Selector: NewChannelSelector([]string{"key-01"}, time.Minute), Caller: p, WorkerID: "w"}
	if err := r.RunOnce(context.Background(), "task"); err != nil {
		t.Fatal(err)
	}
	if p.called != "key-01" {
		t.Fatalf("called=%q", p.called)
	}
	if c.released {
		t.Fatal("released successful chunk")
	}
}
func TestSingleMapRunnerReleasesOnProviderFailure(t *testing.T) {
	c := &fakeClaimer{chunk: &Chunk{ID: "c"}}
	p := &fakeCaller{err: context.DeadlineExceeded}
	r := &SingleMapRunner{Claimer: c, Selector: NewChannelSelector([]string{"key-01"}, time.Minute), Caller: p, WorkerID: "w"}
	if err := r.RunOnce(context.Background(), "task"); err == nil {
		t.Fatal("expected error")
	}
	if !c.released {
		t.Fatal("did not release failed chunk")
	}
}
