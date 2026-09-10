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
	ErrApprovalRecommendationNotFound   = errors.New("approval recommendation not found")
	ErrApprovalRecommendationNotPending = errors.New("recommendation is not pending")
)

// ApprovalRepo 负责审批决策落库与推荐状态更新。
type ApprovalRepo struct {
	db  *sql.DB
	now func() time.Time
	seq atomic.Int64
}

func NewApprovalRepo(store *Store) *ApprovalRepo {
	if store == nil {
		return nil
	}
	return &ApprovalRepo{
		db:  store.DB(),
		now: time.Now,
	}
}

func (r *ApprovalRepo) CreateApprovalDecision(ctx context.Context, input ApprovalInput) (Approval, error) {
	if r == nil || r.db == nil {
		return Approval{}, errors.New("approval repo is not initialized")
	}

	recommendationID := strings.TrimSpace(input.RecommendationID)
	if recommendationID == "" {
		return Approval{}, errors.New("recommendation_id is required")
	}

	decision := strings.TrimSpace(string(input.Decision))
	if decision == "" {
		return Approval{}, errors.New("decision is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Approval{}, err
	}
	defer tx.Rollback()

	var currentStatus string
	err = tx.QueryRowContext(ctx, `
SELECT status
FROM model_recommendations
WHERE recommendation_id = $1
FOR UPDATE
`, recommendationID).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Approval{}, ErrApprovalRecommendationNotFound
		}
		return Approval{}, err
	}

	if strings.TrimSpace(currentStatus) != string(RecommendationStatusPending) {
		return Approval{}, ErrApprovalRecommendationNotPending
	}

	nextRecommendationStatus := RecommendationStatusApproved
	if input.Decision == ApprovalDecisionRejected {
		nextRecommendationStatus = RecommendationStatusRejected
	}

	approvalID := r.nextID("approval")
	createdAt := r.now().UTC()
	scopeJSON, err := json.Marshal(input.EffectiveScope)
	if err != nil {
		return Approval{}, err
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO model_approvals (
    approval_id,
    recommendation_id,
    decision,
    final_model,
    approval_reason,
    approved_by,
    effective_scope,
    created_at
) VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),$6,$7::jsonb,$8)
`,
		approvalID,
		recommendationID,
		decision,
		strings.TrimSpace(input.FinalModel),
		strings.TrimSpace(input.ApprovalReason),
		strings.TrimSpace(input.ApprovedBy),
		string(scopeJSON),
		createdAt,
	)
	if err != nil {
		return Approval{}, err
	}

	_, err = tx.ExecContext(ctx, `
UPDATE model_recommendations
SET status = $2,
    updated_at = $3
WHERE recommendation_id = $1
`, recommendationID, string(nextRecommendationStatus), createdAt)
	if err != nil {
		return Approval{}, err
	}

	if err := tx.Commit(); err != nil {
		return Approval{}, err
	}

	return Approval{
		ID:               approvalID,
		RecommendationID: recommendationID,
		Status:           mapDecisionToApprovalStatus(input.Decision),
		Reason:           strings.TrimSpace(input.ApprovalReason),
		Actor:            strings.TrimSpace(input.ApprovedBy),
		Scope:            input.EffectiveScope,
		ApprovedAt:       createdAt,
		CreatedAt:        createdAt,
	}, nil
}

func mapDecisionToApprovalStatus(decision ApprovalDecision) ApprovalStatus {
	switch decision {
	case ApprovalDecisionApproved:
		return ApprovalStatusApproved
	case ApprovalDecisionRejected:
		return ApprovalStatusRejected
	case ApprovalDecisionOverridden:
		return ApprovalStatusOverridden
	default:
		return ApprovalStatusPending
	}
}

func (r *ApprovalRepo) nextID(prefix string) string {
	n := r.seq.Add(1)
	return fmt.Sprintf("%s_%d_%d", prefix, r.now().UTC().UnixNano(), n)
}
