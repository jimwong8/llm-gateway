package governance

import (
	"context"
	"errors"
	"strings"
)

type rollbackExecutor interface {
	Execute(ctx context.Context, rolloutID string) (Rollout, string, string, error)
}

type rollbackVersionSwitcher interface {
	ActivateVersion(ctx context.Context, versionID string) (PolicyVersion, error)
}

type cacheInvalidator interface {
	InvalidateCache(environment string)
}

// RollbackService 负责执行策略回滚、生成 distribution event，并记录审计接缝。
type RollbackService struct {
	repo                rollbackExecutor
	distributionService *DistributionService
	versions            rollbackVersionSwitcher
	auditEmitter        rolloutAuditEmitter
	invalidator         cacheInvalidator
}

func NewRollbackService(store *Store) *RollbackService {
	if store == nil {
		return &RollbackService{}
	}
	return &RollbackService{
		repo:                NewRollbackRepo(store),
		distributionService: NewDistributionService(store),
		versions:            NewVersionRepo(store),
	}
}

func NewRollbackServiceWithRepo(repo rollbackExecutor, distribution *DistributionRepo, versions rollbackVersionSwitcher) *RollbackService {
	return &RollbackService{repo: repo, distributionService: NewDistributionServiceWithRepo(distribution), versions: versions}
}

func (s *RollbackService) WithAuditEmitter(emitter rolloutAuditEmitter) *RollbackService {
	if s == nil {
		return nil
	}
	s.auditEmitter = emitter
	return s
}

func (s *RollbackService) WithInvalidator(invalidator cacheInvalidator) *RollbackService {
	if s == nil {
		return nil
	}
	s.invalidator = invalidator
	return s
}

func (s *RollbackService) Execute(ctx context.Context, input ExecuteRollbackInput) (ExecuteRollbackResult, error) {
	if s == nil || s.repo == nil || s.distributionService == nil || s.versions == nil {
		return ExecuteRollbackResult{}, errors.New("rollback service is not initialized")
	}
	input.RolloutID = strings.TrimSpace(input.RolloutID)
	input.Actor = strings.TrimSpace(input.Actor)
	input.Reason = strings.TrimSpace(input.Reason)
	if input.RolloutID == "" {
		return ExecuteRollbackResult{}, errors.New("rollout_id is required")
	}
	if input.Actor == "" {
		return ExecuteRollbackResult{}, errors.New("actor is required")
	}

	rollout, restoredID, revertedID, err := s.repo.Execute(ctx, input.RolloutID)
	if err != nil {
		return ExecuteRollbackResult{}, err
	}
	if _, err := s.versions.ActivateVersion(ctx, restoredID); err != nil {
		return ExecuteRollbackResult{}, err
	}
	event, err := s.distributionService.CreateRollbackEvent(
		ctx,
		rollout,
		input.Actor,
		input.Reason,
		restoredID,
		revertedID,
	)
	if err != nil {
		return ExecuteRollbackResult{}, err
	}
	if s.invalidator != nil {
		s.invalidator.InvalidateCache(rollout.TargetEnvironment)
	}
	if s.auditEmitter != nil {
		if err := s.auditEmitter.EmitGovernanceEvent(ctx, "governance.rollout.rollback_executed", input.Actor, "model_rollout", rollout.ID, map[string]any{
			"reason":                     input.Reason,
			"distribution_event_id":      event.ID,
			"restored_policy_version_id": restoredID,
			"reverted_policy_version_id": revertedID,
		}); err != nil {
			return ExecuteRollbackResult{}, err
		}
	}
	return ExecuteRollbackResult{
		Rollout:                 rollout,
		RestoredPolicyVersionID: restoredID,
		RevertedPolicyVersionID: revertedID,
		DistributionEvent:       event,
	}, nil
}
