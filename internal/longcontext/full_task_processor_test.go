package longcontext

import (
	"context"
	"testing"
)

func TestFullTaskProcessorRequiresDependencies(t *testing.T) {
	p := FullTaskProcessor{}
	if err := p.ProcessTask(context.Background(), &Task{ID: "t1", Status: StatusQueued}); err == nil {
		t.Fatal("expected dependency error")
	}
}

func TestFullTaskProcessorRejectsNilTask(t *testing.T) {
	p := FullTaskProcessor{Repo: &PostgresRepository{}}
	if err := p.ProcessTask(context.Background(), nil); err == nil {
		t.Fatal("expected task required error")
	}
}
