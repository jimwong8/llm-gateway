package longcontext

import (
	"context"
	"testing"
	"time"
)

// expiredChunkRepo is a minimal in-memory ChunkRepository used to verify that
// ClaimChunk re-claims a 'mapping' chunk whose lease has expired.
type expiredChunkRepo struct {
	chunks []*Chunk
	now    time.Time
}

func (r *expiredChunkRepo) add(c *Chunk) { r.chunks = append(r.chunks, c) }

func (r *expiredChunkRepo) ClaimChunk(_ context.Context, taskID, _ string, _ time.Duration) (*Chunk, error) {
	for _, c := range r.chunks {
		if c.TaskID != taskID {
			continue
		}
		if c.Status == "pending" {
			c.Status = "mapping"
			c.MapAttempts++
			return c, nil
		}
		if c.Status == "mapping" && c.MapLeaseUntil.Before(r.now) {
			c.Status = "mapping"
			c.MapAttempts++
			return c, nil
		}
	}
	return nil, nil
}
func (r *expiredChunkRepo) SaveMapResult(_ context.Context, _, _, _, _ string, _ MapResult) error {
	return nil
}
func (r *expiredChunkRepo) ReleaseChunk(_ context.Context, id, _ string) error {
	for _, c := range r.chunks {
		if c.ID == id {
			c.Status = "pending"
		}
	}
	return nil
}

func TestClaimChunkReclaimsExpiredMapping(t *testing.T) {
	repo := &expiredChunkRepo{now: time.Now()}
	repo.add(&Chunk{ID: "c1", TaskID: "t1", Status: "mapping", MapLeaseUntil: time.Now().Add(-time.Minute)})
	c, err := repo.ClaimChunk(context.Background(), "t1", "w1", time.Minute)
	if err != nil {
		t.Fatalf("ClaimChunk err: %v", err)
	}
	if c == nil {
		t.Fatal("expected expired mapping chunk to be reclaimed, got nil")
	}
	if c.MapAttempts != 1 {
		t.Errorf("MapAttempts = %d, want 1", c.MapAttempts)
	}
}

func TestClaimChunkSkipsLiveMapping(t *testing.T) {
	repo := &expiredChunkRepo{now: time.Now()}
	repo.add(&Chunk{ID: "c1", TaskID: "t1", Status: "mapping", MapLeaseUntil: time.Now().Add(time.Hour)})
	c, err := repo.ClaimChunk(context.Background(), "t1", "w1", time.Minute)
	if err != nil {
		t.Fatalf("ClaimChunk err: %v", err)
	}
	if c != nil {
		t.Fatalf("live mapping chunk should not be reclaimed, got %+v", c)
	}
}
