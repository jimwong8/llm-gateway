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

var (
	ErrRolloutNotFound          = errors.New("rollout not found")
	ErrRolloutInvalidTransition = errors.New("invalid rollout transition")
)

// RolloutRepo 负责 rollout 生命周期与回滚所需数据访问。
type RolloutRepo struct {
	db  *sql.DB
	now func() time.Time
	seq atomic.Int64
}

func NewRolloutRepo(store *Store) *RolloutRepo {
	if store == nil {
		return nil
	}
	return &RolloutRepo{db: store.DB(), now: time.Now}
}

func (r *RolloutRepo) Create(ctx context.Context, input StartRolloutInput) (Rollout, error) {
	if r == nil || r.db == nil {
		return Rollout{}, errors.New("rollout repo is not initialized")
	}

	version, err := r.loadVersion(ctx, strings.TrimSpace(input.PolicyVersionID))
	if err != nil {
		return Rollout{}, err
	}
	if version.Status != PolicyVersionApproved {
		return Rollout{}, fmt.Errorf("%w: policy version must be approved before rollout", ErrInvalidVersionTransition)
	}

	rolloutMode := strings.TrimSpace(input.RolloutMode)
	if rolloutMode == "" {
		rolloutMode = "progressive"
	}
	rolloutPercent := input.RolloutPercent
	if rolloutPercent <= 0 {
		rolloutPercent = 1
	}
	if rolloutPercent > 100 {
		rolloutPercent = 100
	}
	triggeredBy := strings.TrimSpace(input.TriggeredBy)
	if triggeredBy == "" {
		triggeredBy = "system"
	}
	triggerReason := strings.TrimSpace(input.TriggerReason)
	rolloutID := r.nextID("rollout")
	now := r.now().UTC()

	_, err = r.db.ExecContext(ctx, `
INSERT INTO model_rollouts (
    rollout_id,
    policy_version_id,
    environment,
    rollout_mode,
    rollout_percent,
    status,
    trigger_reason,
    triggered_by,
    created_at,
    updated_at
) VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8,$9,$9)
`, rolloutID, version.ID, version.Environment, rolloutMode, rolloutPercent, string(RolloutStatusRunning), triggerReason, triggeredBy, now)
	if err != nil {
		return Rollout{}, err
	}

	return Rollout{
		ID:                rolloutID,
		PolicyVersionID:   version.ID,
		Status:            RolloutStatusRunning,
		TargetEnvironment: version.Environment,
		RolloutMode:       rolloutMode,
		RolloutPercent:    rolloutPercent,
		TriggeredBy:       triggeredBy,
		TriggerReason:     triggerReason,
		StartedAt:         now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

func (r *RolloutRepo) Promote(ctx context.Context, input PromoteRolloutInput) (Rollout, error) {
	if r == nil || r.db == nil {
		return Rollout{}, errors.New("rollout repo is not initialized")
	}
	rollout, err := r.Get(ctx, strings.TrimSpace(input.RolloutID))
	if err != nil {
		return Rollout{}, err
	}
	if rollout.Status != RolloutStatusRunning && rollout.Status != RolloutStatusPlanned {
		return Rollout{}, fmt.Errorf("%w: rollout status is %s", ErrRolloutInvalidTransition, rollout.Status)
	}

	percent := input.RolloutPercent
	if percent < rollout.RolloutPercent {
		percent = rollout.RolloutPercent
	}
	if percent > 100 {
		percent = 100
	}
	status := RolloutStatusRunning
	finishedAt := sql.NullTime{}
	if percent >= 100 {
		status = RolloutStatusPromoted
		finishedAt = sql.NullTime{Time: r.now().UTC(), Valid: true}
	}
	updatedAt := r.now().UTC()

	_, err = r.db.ExecContext(ctx, `
UPDATE model_rollouts
SET rollout_percent = $2,
    status = $3,
    updated_at = $4
WHERE rollout_id = $1
`, rollout.ID, percent, string(status), updatedAt)
	if err != nil {
		return Rollout{}, err
	}

	rollout.RolloutPercent = percent
	rollout.Status = status
	rollout.GuardSummary = strings.TrimSpace(input.GuardSummary)
	rollout.UpdatedAt = updatedAt
	if finishedAt.Valid {
		rollout.FinishedAt = finishedAt.Time.UTC()
	}
	return rollout, nil
}

func (r *RolloutRepo) MarkRolledBack(ctx context.Context, rolloutID string) (Rollout, error) {
	if r == nil || r.db == nil {
		return Rollout{}, errors.New("rollout repo is not initialized")
	}
	rollout, err := r.Get(ctx, strings.TrimSpace(rolloutID))
	if err != nil {
		return Rollout{}, err
	}
	if rollout.Status.IsTerminal() {
		return Rollout{}, fmt.Errorf("%w: rollout status is %s", ErrRolloutInvalidTransition, rollout.Status)
	}
	updatedAt := r.now().UTC()
	_, err = r.db.ExecContext(ctx, `
UPDATE model_rollouts
SET status = $2,
    updated_at = $3
WHERE rollout_id = $1
`, rollout.ID, string(RolloutStatusRolledBack), updatedAt)
	if err != nil {
		return Rollout{}, err
	}
	rollout.Status = RolloutStatusRolledBack
	rollout.FinishedAt = updatedAt
	rollout.UpdatedAt = updatedAt
	return rollout, nil
}

func (r *RolloutRepo) Get(ctx context.Context, rolloutID string) (Rollout, error) {
	if r == nil || r.db == nil {
		return Rollout{}, errors.New("rollout repo is not initialized")
	}
	var (
		rollout                            Rollout
		statusRaw, rolloutMode, triggeredBy string
		triggerReason                      sql.NullString
		createdAt, updatedAt               time.Time
	)
	err := r.db.QueryRowContext(ctx, `
SELECT rollout_id, policy_version_id, environment, rollout_mode, rollout_percent, status, trigger_reason, triggered_by, created_at, updated_at
FROM model_rollouts
WHERE rollout_id = $1
`, strings.TrimSpace(rolloutID)).Scan(
		&rollout.ID,
		&rollout.PolicyVersionID,
		&rollout.TargetEnvironment,
		&rolloutMode,
		&rollout.RolloutPercent,
		&statusRaw,
		&triggerReason,
		&triggeredBy,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Rollout{}, ErrRolloutNotFound
		}
		return Rollout{}, err
	}
	rollout.Status = RolloutStatus(strings.TrimSpace(statusRaw))
	rollout.RolloutMode = strings.TrimSpace(rolloutMode)
	rollout.TriggeredBy = strings.TrimSpace(triggeredBy)
	rollout.CreatedAt = createdAt.UTC()
	rollout.UpdatedAt = updatedAt.UTC()
	rollout.StartedAt = rollout.CreatedAt
	if triggerReason.Valid {
		rollout.TriggerReason = strings.TrimSpace(triggerReason.String)
	}
	if rollout.Status == RolloutStatusPromoted || rollout.Status == RolloutStatusRolledBack || rollout.Status == RolloutStatusFinalized || rollout.Status == RolloutStatusHalted {
		rollout.FinishedAt = rollout.UpdatedAt
	}
	return rollout, nil
}

func (r *RolloutRepo) loadVersion(ctx context.Context, versionID string) (PolicyVersion, error) {
	var (
		environment string
		statusRaw   string
		policyRaw   []byte
	)
	err := r.db.QueryRowContext(ctx, `
SELECT environment, status, policy_json
FROM model_policy_versions
WHERE policy_version_id = $1
`, versionID).Scan(&environment, &statusRaw, &policyRaw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PolicyVersion{}, ErrPolicyVersionNotFound
		}
		return PolicyVersion{}, err
	}
	var policy RuntimePolicy
	if len(policyRaw) > 0 {
		if err := json.Unmarshal(policyRaw, &policy); err != nil {
			return PolicyVersion{}, err
		}
	}
	return PolicyVersion{
		ID:          versionID,
		Environment: strings.TrimSpace(environment),
		Status:      PolicyVersionStatus(strings.TrimSpace(statusRaw)),
		Policy:      policy,
		Version:     policy.Version,
	}, nil
}

func (r *RolloutRepo) nextID(prefix string) string {
	n := r.seq.Add(1)
	return fmt.Sprintf("%s_%d_%d", prefix, r.now().UTC().UnixNano(), n)
}
