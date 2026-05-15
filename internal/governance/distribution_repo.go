package governance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

// DistributionRepo 负责 policy distribution event 持久化。
type DistributionRepo struct {
	db  *sql.DB
	now func() time.Time
	seq atomic.Int64
}

func NewDistributionRepo(store *Store) *DistributionRepo {
	if store == nil {
		return nil
	}
	return &DistributionRepo{db: store.DB(), now: time.Now}
}

func (r *DistributionRepo) Create(ctx context.Context, event DistributionEvent) (DistributionEvent, error) {
	if r == nil || r.db == nil {
		return DistributionEvent{}, errors.New("distribution repo is not initialized")
	}
	event.PolicyVersionID = strings.TrimSpace(event.PolicyVersionID)
	event.RolloutID = strings.TrimSpace(event.RolloutID)
	event.Environment = strings.TrimSpace(event.Environment)
	event.EventType = DistributionEventType(strings.TrimSpace(string(event.EventType)))
	if event.Environment == "" {
		return DistributionEvent{}, errors.New("environment is required")
	}
	if event.EventType == "" {
		return DistributionEvent{}, errors.New("event_type is required")
	}
	if strings.TrimSpace(event.ID) == "" {
		event.ID = r.nextID("distribution")
	}
	if event.Payload == nil {
		event.Payload = map[string]any{}
	}
	payloadJSON, err := json.Marshal(event.Payload)
	if err != nil {
		return DistributionEvent{}, err
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = r.now().UTC()
	} else {
		event.CreatedAt = event.CreatedAt.UTC()
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO model_distribution_events (
    event_id,
    policy_version_id,
    rollout_id,
    environment,
    event_type,
    payload,
    created_at
) VALUES ($1,NULLIF($2,''),NULLIF($3,''),$4,$5,$6::jsonb,$7)
`, event.ID, event.PolicyVersionID, event.RolloutID, event.Environment, string(event.EventType), string(payloadJSON), event.CreatedAt)
	if err != nil {
		return DistributionEvent{}, err
	}
	return event, nil
}

func (r *DistributionRepo) nextID(prefix string) string {
	n := r.seq.Add(1)
	return fmt.Sprintf("%s_%d_%d", prefix, r.now().UTC().UnixNano(), n)
}
