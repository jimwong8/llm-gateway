package governance

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ApprovalDecision 表示审批动作类型。
type ApprovalDecision string

const (
	ApprovalDecisionApproved   ApprovalDecision = "approved"
	ApprovalDecisionRejected   ApprovalDecision = "rejected"
	ApprovalDecisionOverridden ApprovalDecision = "overridden"
)

func (d ApprovalDecision) Valid() bool {
	switch d {
	case ApprovalDecisionApproved, ApprovalDecisionRejected, ApprovalDecisionOverridden:
		return true
	default:
		return false
	}
}

// ApprovalInput 表示审批决策输入。
type ApprovalInput struct {
	RecommendationID string
	Decision         ApprovalDecision
	FinalModel       string
	ApprovalReason   string
	ApprovedBy       string
	EffectiveScope   EffectiveScope
}

type approvalDataStore interface {
	CreateApprovalDecision(ctx context.Context, input ApprovalInput) (Approval, error)
}

type governanceAuditEmitter interface {
	EmitGovernanceEvent(ctx context.Context, eventType string, actorID string, entityType string, entityID string, payload map[string]any) error
}

// ApprovalService 实现审批/覆盖/拒绝决策逻辑。
type ApprovalService struct {
	store        *Store
	repo         approvalDataStore
	auditEmitter governanceAuditEmitter
}

func NewApprovalService(store *Store) *ApprovalService {
	return &ApprovalService{
		store: store,
		repo:  NewApprovalRepo(store),
	}
}

func NewApprovalServiceWithRepo(repo approvalDataStore) *ApprovalService {
	return &ApprovalService{repo: repo}
}

func (s *ApprovalService) WithAuditEmitter(emitter governanceAuditEmitter) *ApprovalService {
	if s == nil {
		return nil
	}
	s.auditEmitter = emitter
	return s
}

func (s *ApprovalService) Store() *Store {
	if s == nil {
		return nil
	}
	return s.store
}

func (s *ApprovalService) Decide(ctx context.Context, input ApprovalInput) (Approval, error) {
	if s == nil || s.repo == nil {
		return Approval{}, errors.New("approval service is not initialized")
	}

	input.RecommendationID = strings.TrimSpace(input.RecommendationID)
	input.FinalModel = strings.TrimSpace(input.FinalModel)
	input.ApprovalReason = strings.TrimSpace(input.ApprovalReason)
	input.ApprovedBy = strings.TrimSpace(input.ApprovedBy)
	input.EffectiveScope.Scope = strings.TrimSpace(input.EffectiveScope.Scope)
	input.EffectiveScope.ProjectID = strings.TrimSpace(input.EffectiveScope.ProjectID)
	input.EffectiveScope.Environment = strings.TrimSpace(input.EffectiveScope.Environment)

	if input.RecommendationID == "" {
		return Approval{}, fmt.Errorf("recommendation_id is required")
	}
	if !input.Decision.Valid() {
		return Approval{}, fmt.Errorf("invalid decision: %s", input.Decision)
	}
	if input.ApprovedBy == "" {
		return Approval{}, fmt.Errorf("approved_by is required")
	}
	if input.EffectiveScope.Environment == "" {
		return Approval{}, fmt.Errorf("effective_scope.environment is required")
	}
	if input.EffectiveScope.Scope == "" {
		input.EffectiveScope.Scope = "agent"
	}

	if input.Decision == ApprovalDecisionOverridden && input.FinalModel == "" {
		return Approval{}, fmt.Errorf("final_model is required for overridden decision")
	}
	if input.Decision == ApprovalDecisionRejected && input.ApprovalReason == "" {
		return Approval{}, fmt.Errorf("approval_reason is required for rejected decision")
	}

	approval, err := s.repo.CreateApprovalDecision(ctx, input)
	if err != nil {
		return Approval{}, err
	}

	if input.Decision == ApprovalDecisionOverridden {
		approval.FinalModel = input.FinalModel
	}
	if input.Decision == ApprovalDecisionApproved {
		approval.FinalModel = input.FinalModel
	}

	if s.auditEmitter != nil {
		auditSvc := NewGovernanceAuditService(s.auditEmitter)
		emitErr := auditSvc.ApprovalDecided(ctx, input.ApprovedBy, input.RecommendationID, map[string]any{
			"approval_id":     approval.ID,
			"decision":        string(input.Decision),
			"final_model":     input.FinalModel,
			"approval_reason": input.ApprovalReason,
			"effective_scope": input.EffectiveScope,
		})
		if emitErr != nil {
			return Approval{}, emitErr
		}
	}

	return approval, nil
}
