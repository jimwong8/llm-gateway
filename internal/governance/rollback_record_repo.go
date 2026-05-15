package governance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

var (
	ErrRollbackNotFound = errors.New("rollback not found")
)

// RollbackRecordRepo 负责治理回滚记录持久化。
type RollbackRecordRepo struct {
	db  *sql.DB
	now func() time.Time
	seq atomic.Int64
}

func NewRollbackRecordRepo(store *Store) *RollbackRecordRepo {
	if store == nil {
		return nil
	}
	return &RollbackRecordRepo{db: store.DB(), now: time.Now}
}

func (r *RollbackRecordRepo) Create(ctx context.Context, input ExecuteRollbackInput, result ExecuteRollbackResult) (RollbackRecord, error) {
	if r == nil || r.db == nil {
		return RollbackRecord{}, errors.New("rollback record repo is not initialized")
	}
	rolloutID := strings.TrimSpace(result.Rollout.ID)
	if rolloutID == "" {
		rolloutID = strings.TrimSpace(input.RolloutID)
	}
	if rolloutID == "" {
		return RollbackRecord{}, errors.New("rollout_id is required")
	}
	restoredID := strings.TrimSpace(result.RestoredPolicyVersionID)
	revertedID := strings.TrimSpace(result.RevertedPolicyVersionID)
	if restoredID == "" || revertedID == "" {
		return RollbackRecord{}, errors.New("restored/reverted policy version id is required")
	}
	actor := strings.TrimSpace(input.Actor)
	if actor == "" {
		actor = "system"
	}
	reason := strings.TrimSpace(input.Reason)
	environment := strings.TrimSpace(result.Rollout.TargetEnvironment)
	now := r.now().UTC()
	rollbackID := r.nextID("rollback")

	_, err := r.db.ExecContext(ctx, `
INSERT INTO model_rollbacks (
    rollback_id,
    rollout_id,
    environment,
    actor,
    reason,
    restored_policy_version_id,
    reverted_policy_version_id,
    created_at
) VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8)
`, rollbackID, rolloutID, environment, actor, reason, restoredID, revertedID, now)
	if err != nil {
		return RollbackRecord{}, err
	}

	return RollbackRecord{
		ID:                      rollbackID,
		RolloutID:               rolloutID,
		Environment:             environment,
		Actor:                   actor,
		Reason:                  reason,
		RestoredPolicyVersionID: restoredID,
		RevertedPolicyVersionID: revertedID,
		CreatedAt:               now,
	}, nil
}

func (r *RollbackRecordRepo) List(ctx context.Context, limit int) ([]RollbackRecord, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("rollback record repo is not initialized")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT rollback_id,
       rollout_id,
       environment,
       actor,
       COALESCE(reason, ''),
       restored_policy_version_id,
       reverted_policy_version_id,
       created_at
FROM model_rollbacks
ORDER BY created_at DESC
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]RollbackRecord, 0, limit)
	for rows.Next() {
		var item RollbackRecord
		if err := rows.Scan(
			&item.ID,
			&item.RolloutID,
			&item.Environment,
			&item.Actor,
			&item.Reason,
			&item.RestoredPolicyVersionID,
			&item.RevertedPolicyVersionID,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.ID = strings.TrimSpace(item.ID)
		item.RolloutID = strings.TrimSpace(item.RolloutID)
		item.Environment = strings.TrimSpace(item.Environment)
		item.Actor = strings.TrimSpace(item.Actor)
		item.Reason = strings.TrimSpace(item.Reason)
		item.RestoredPolicyVersionID = strings.TrimSpace(item.RestoredPolicyVersionID)
		item.RevertedPolicyVersionID = strings.TrimSpace(item.RevertedPolicyVersionID)
		item.CreatedAt = item.CreatedAt.UTC()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *RollbackRecordRepo) Get(ctx context.Context, rollbackID string) (RollbackRecord, error) {
	if r == nil || r.db == nil {
		return RollbackRecord{}, errors.New("rollback record repo is not initialized")
	}
	rollbackID = strings.TrimSpace(rollbackID)
	if rollbackID == "" {
		return RollbackRecord{}, errors.New("rollback_id is required")
	}

	var item RollbackRecord
	err := r.db.QueryRowContext(ctx, `
SELECT rollback_id,
       rollout_id,
       environment,
       actor,
       COALESCE(reason, ''),
       restored_policy_version_id,
       reverted_policy_version_id,
       created_at
FROM model_rollbacks
WHERE rollback_id = $1
`, rollbackID).Scan(
		&item.ID,
		&item.RolloutID,
		&item.Environment,
		&item.Actor,
		&item.Reason,
		&item.RestoredPolicyVersionID,
		&item.RevertedPolicyVersionID,
		&item.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RollbackRecord{}, ErrRollbackNotFound
		}
		return RollbackRecord{}, err
	}

	item.ID = strings.TrimSpace(item.ID)
	item.RolloutID = strings.TrimSpace(item.RolloutID)
	item.Environment = strings.TrimSpace(item.Environment)
	item.Actor = strings.TrimSpace(item.Actor)
	item.Reason = strings.TrimSpace(item.Reason)
	item.RestoredPolicyVersionID = strings.TrimSpace(item.RestoredPolicyVersionID)
	item.RevertedPolicyVersionID = strings.TrimSpace(item.RevertedPolicyVersionID)
	item.CreatedAt = item.CreatedAt.UTC()
	return item, nil
}

func (r *RollbackRecordRepo) nextID(prefix string) string {
	n := r.seq.Add(1)
	return fmt.Sprintf("%s_%d_%d", prefix, r.now().UTC().UnixNano(), n)
}
