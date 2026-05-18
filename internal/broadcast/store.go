package broadcast

import (
	"context"
	"sync"
	"time"
)

type BroadcastType string

const (
	TypeInfo     BroadcastType = "info"
	TypeWarning  BroadcastType = "warning"
	TypeCritical BroadcastType = "critical"
)

type Broadcast struct {
	ID        int64         `json:"id"`
	Title     string        `json:"title"`
	Content   string        `json:"content"`
	Type      BroadcastType `json:"type"`
	StartAt   time.Time     `json:"start_at"`
	EndAt     time.Time     `json:"end_at"`
	CreatedBy string        `json:"created_by"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

type BroadcastInput struct {
	Title   string        `json:"title"`
	Content string        `json:"content"`
	Type    BroadcastType `json:"type"`
	StartAt time.Time     `json:"start_at"`
	EndAt   time.Time     `json:"end_at"`
}

type BroadcastRead struct {
	ID          int64     `json:"id"`
	BroadcastID int64     `json:"broadcast_id"`
	UserID      int64     `json:"user_id"`
	ReadAt      time.Time `json:"read_at"`
}

type Store interface {
	Create(ctx context.Context, input BroadcastInput, createdBy string) (*Broadcast, error)
	List(ctx context.Context) ([]Broadcast, error)
	ListActive(ctx context.Context, now time.Time) ([]Broadcast, error)
	GetByID(ctx context.Context, id int64) (*Broadcast, error)
	Update(ctx context.Context, id int64, input BroadcastInput) (*Broadcast, error)
	Delete(ctx context.Context, id int64) error
	MarkRead(ctx context.Context, broadcastID, userID int64) error
	ListReadByUser(ctx context.Context, userID int64) ([]BroadcastRead, error)
}

type MemoryStore struct {
	mu          sync.RWMutex
	seq         int64
	broadcasts  map[int64]*Broadcast
	reads       []BroadcastRead
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		broadcasts: make(map[int64]*Broadcast),
		reads:      make([]BroadcastRead, 0),
	}
}

func (s *MemoryStore) Create(_ context.Context, input BroadcastInput, createdBy string) (*Broadcast, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	b := &Broadcast{
		ID:        s.seq,
		Title:     input.Title,
		Content:   input.Content,
		Type:      input.Type,
		StartAt:   input.StartAt,
		EndAt:     input.EndAt,
		CreatedBy: createdBy,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	s.broadcasts[b.ID] = b
	return b, nil
}

func (s *MemoryStore) List(_ context.Context) ([]Broadcast, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Broadcast, 0, len(s.broadcasts))
	for _, b := range s.broadcasts {
		result = append(result, *b)
	}
	return result, nil
}

func (s *MemoryStore) ListActive(_ context.Context, now time.Time) ([]Broadcast, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Broadcast
	for _, b := range s.broadcasts {
		if (b.StartAt.IsZero() || !b.StartAt.After(now)) &&
			(b.EndAt.IsZero() || b.EndAt.After(now)) {
			result = append(result, *b)
		}
	}
	return result, nil
}

func (s *MemoryStore) GetByID(_ context.Context, id int64) (*Broadcast, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.broadcasts[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *b
	return &cp, nil
}

func (s *MemoryStore) Update(_ context.Context, id int64, input BroadcastInput) (*Broadcast, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.broadcasts[id]
	if !ok {
		return nil, ErrNotFound
	}
	b.Title = input.Title
	b.Content = input.Content
	b.Type = input.Type
	b.StartAt = input.StartAt
	b.EndAt = input.EndAt
	b.UpdatedAt = time.Now().UTC()
	cp := *b
	return &cp, nil
}

func (s *MemoryStore) Delete(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.broadcasts[id]; !ok {
		return ErrNotFound
	}
	delete(s.broadcasts, id)
	return nil
}

func (s *MemoryStore) MarkRead(_ context.Context, broadcastID, userID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.broadcasts[broadcastID]; !ok {
		return ErrNotFound
	}
	for _, r := range s.reads {
		if r.BroadcastID == broadcastID && r.UserID == userID {
			return nil
		}
	}
	s.reads = append(s.reads, BroadcastRead{
		ID:          int64(len(s.reads) + 1),
		BroadcastID: broadcastID,
		UserID:      userID,
		ReadAt:      time.Now().UTC(),
	})
	return nil
}

func (s *MemoryStore) ListReadByUser(_ context.Context, userID int64) ([]BroadcastRead, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []BroadcastRead
	for _, r := range s.reads {
		if r.UserID == userID {
			result = append(result, r)
		}
	}
	return result, nil
}
