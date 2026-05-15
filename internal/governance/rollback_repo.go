package governance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// RollbackRepo 负责执行回滚所需的数据变更。
type RollbackRepo struct {
	db  *sql.DB
	now func() time.Time
}

func NewRollbackRepo(store *Store) *RollbackRepo {
	if store == nil {
		return nil
	}
	return &RollbackRepo{db: store.DB(), now: time.Now}
}

func (r *RollbackRepo) Execute(ctx context.Context, rolloutID string) (Rollout, string, string, error) {
	if r == nil || r.db == nil {
		return Rollout{}, "", "", errors.New("rollback repo is not initialized")
	}
	rolloutID = strings.TrimSpace(rolloutID)
	if rolloutID == "" {
		return Rollout{}, "", "", errors.New("rollout_id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Rollout{}, "", "", err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		rollout                            Rollout
		statusRaw, rolloutMode, triggeredBy string
		triggerReason                      sql.NullString
		createdAt, updatedAt               time.Time
	)
	err = tx.QueryRowContext(ctx, `
SELECT rollout_id, policy_version_id, environment, rollout_mode, rollout_percent, status, trigger_reason, triggered_by, created_at, updated_at
FROM model_rollouts
WHERE rollout_id = $1
FOR UPDATE
`, rolloutID).Scan(&rollout.ID, &rollout.PolicyVersionID, &rollout.TargetEnvironment, &rolloutMode, &rollout.RolloutPercent, &statusRaw, &triggerReason, &triggeredBy, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Rollout{}, "", "", ErrRolloutNotFound
		}
		return Rollout{}, "", "", err
	}
	rollout.Status = RolloutStatus(strings.TrimSpace(statusRaw))
	if rollout.Status.IsTerminal() {
		return Rollout{}, "", "", fmt.Errorf("%w: rollout status is %s", ErrRolloutInvalidTransition, rollout.Status)
	}
	rollout.RolloutMode = strings.TrimSpace(rolloutMode)
	rollout.TriggeredBy = strings.TrimSpace(triggeredBy)
	if triggerReason.Valid {
		rollout.TriggerReason = strings.TrimSpace(triggerReason.String)
	}
	rollout.CreatedAt = createdAt.UTC()
	rollout.StartedAt = rollout.CreatedAt
	rollout.UpdatedAt = updatedAt.UTC()

	currentActiveID, err := selectPolicyVersionIDTx(ctx, tx, rollout.TargetEnvironment, string(PolicyVersionActive))
	if err != nil {
		return Rollout{}, "", "", err
	}
	previousActiveID, err := selectPreviousActivePolicyVersionIDTx(ctx, tx, rollout.TargetEnvironment, rollout.PolicyVersionID)
	if err != nil {
		return Rollout{}, "", "", err
	}
	if previousActiveID == "" {
		return Rollout{}, "", "", fmt.Errorf("%w: no previous active version found for rollback", ErrInvalidVersionTransition)
	}
	if currentActiveID == "" {
		currentActiveID = rollout.PolicyVersionID
	}

	now := r.now().UTC()
	_, err = tx.ExecContext(ctx, `
UPDATE model_policy_versions
SET status = $2,
    approved_at = COALESCE(approved_at, $3)
WHERE policy_version_id = $1
`, previousActiveID, string(PolicyVersionApproved), now)
	if err != nil {
		return Rollout{}, "", "", err
	}
	_, err = tx.ExecContext(ctx, `
UPDATE model_policy_versions
SET status = $2
WHERE policy_version_id = $1
`, currentActiveID, string(PolicyVersionRolledBack))
	if err != nil {
		return Rollout{}, "", "", err
	}
	_, err = tx.ExecContext(ctx, `
UPDATE model_rollouts
SET status = $2,
    updated_at = $3
WHERE rollout_id = $1
`, rollout.ID, string(RolloutStatusRolledBack), now)
	if err != nil {
		return Rollout{}, "", "", err
	}

	if err := tx.Commit(); err != nil {
		return Rollout{}, "", "", err
	}
	rollout.Status = RolloutStatusRolledBack
	rollout.FinishedAt = now
	rollout.UpdatedAt = now
	return rollout, previousActiveID, currentActiveID, nil
}

func selectPolicyVersionIDTx(ctx context.Context, q queryRower, environment, status string) (string, error) {
	var versionID string
	err := q.QueryRowContext(ctx, `
SELECT policy_version_id
FROM model_policy_versions
WHERE environment = $1 AND status = $2
ORDER BY COALESCE(activated_at, created_at) DESC, id DESC
LIMIT 1
`, strings.TrimSpace(environment), strings.TrimSpace(status)).Scan(&versionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(versionID), nil
}

func selectPreviousActivePolicyVersionIDTx(ctx context.Context, q queryRower, environment, excludeVersionID string) (string, error) {
	var versionID string
	err := q.QueryRowContext(ctx, `
SELECT policy_version_id
FROM model_policy_versions
WHERE environment = $1
  AND policy_version_id <> $2
  AND status IN ($3, $4)
ORDER BY COALESCE(activated_at, created_at) DESC, id DESC
LIMIT 1
`, strings.TrimSpace(environment), strings.TrimSpace(excludeVersionID), string(PolicyVersionSuperseded), string(PolicyVersionRolledBack)).Scan(&versionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(versionID), nil
}

type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
