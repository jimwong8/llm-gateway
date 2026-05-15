package governance

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type rolloutVersionActivator interface {
	ActivateVersion(ctx context.Context, versionID string) (PolicyVersion, error)
	GetVersion(ctx context.Context, versionID string) (PolicyVersion, error)
}

type rolloutAuditEmitter interface {
	EmitGovernanceEvent(ctx context.Context, eventType string, actorID string, entityType string, entityID string, payload map[string]any) error
}

type rolloutCacheInvalidator interface {
	InvalidateCache(environment string)
}

// RolloutService 负责 rollout 启动、推进与激活分发事件。
type RolloutService struct {
	repo                *RolloutRepo
	distributionService *DistributionService
	versions            rolloutVersionActivator
	auditEmitter        rolloutAuditEmitter
	invalidator         rolloutCacheInvalidator
}

func NewRolloutService(store *Store) *RolloutService {
	if store == nil {
		return &RolloutService{}
	}
	return &RolloutService{
		repo:                NewRolloutRepo(store),
		distributionService: NewDistributionService(store),
		versions:            NewVersionRepo(store),
	}
}

func NewRolloutServiceWithRepo(repo *RolloutRepo, distribution *DistributionRepo, versions rolloutVersionActivator) *RolloutService {
	return &RolloutService{
		repo:                repo,
		distributionService: NewDistributionServiceWithRepo(distribution),
		versions:            versions,
	}
}

func (s *RolloutService) WithAuditEmitter(emitter rolloutAuditEmitter) *RolloutService {
	if s == nil {
		return nil
	}
	s.auditEmitter = emitter
	return s
}

func (s *RolloutService) WithInvalidator(invalidator rolloutCacheInvalidator) *RolloutService {
	if s == nil {
		return nil
	}
	s.invalidator = invalidator
	return s
}

func (s *RolloutService) Start(ctx context.Context, input StartRolloutInput) (Rollout, DistributionEvent, error) {
	if s == nil || s.repo == nil || s.distributionService == nil || s.versions == nil {
		return Rollout{}, DistributionEvent{}, errors.New("rollout service is not initialized")
	}
	input.PolicyVersionID = strings.TrimSpace(input.PolicyVersionID)
	input.TriggeredBy = strings.TrimSpace(input.TriggeredBy)
	input.TriggerReason = strings.TrimSpace(input.TriggerReason)
	input.RolloutMode = strings.TrimSpace(input.RolloutMode)
	if input.PolicyVersionID == "" {
		return Rollout{}, DistributionEvent{}, errors.New("policy_version_id is required")
	}
	if input.TriggeredBy == "" {
		return Rollout{}, DistributionEvent{}, errors.New("triggered_by is required")
	}

	version, err := s.versions.GetVersion(ctx, input.PolicyVersionID)
	if err != nil {
		return Rollout{}, DistributionEvent{}, err
	}
	if version.Status != PolicyVersionApproved {
		return Rollout{}, DistributionEvent{}, fmt.Errorf("%w: policy version must be approved before rollout", ErrInvalidVersionTransition)
	}

	rollout, err := s.repo.Create(ctx, input)
	if err != nil {
		return Rollout{}, DistributionEvent{}, err
	}
	if _, err := s.versions.ActivateVersion(ctx, input.PolicyVersionID); err != nil {
		return Rollout{}, DistributionEvent{}, err
	}

	event, err := s.distributionService.CreateActivationEvent(ctx, rollout)
	if err != nil {
		return Rollout{}, DistributionEvent{}, err
	}

	if s.invalidator != nil {
		s.invalidator.InvalidateCache(rollout.TargetEnvironment)
	}

	if s.auditEmitter != nil {
		if err := s.auditEmitter.EmitGovernanceEvent(ctx, "governance.rollout.started", input.TriggeredBy, "model_rollout", rollout.ID, map[string]any{
			"policy_version_id": rollout.PolicyVersionID,
			"environment":       rollout.TargetEnvironment,
			"distribution_event": event.ID,
		}); err != nil {
			return Rollout{}, DistributionEvent{}, err
		}
	}
	return rollout, event, nil
}

func (s *RolloutService) Promote(ctx context.Context, input PromoteRolloutInput) (Rollout, error) {
	if s == nil || s.repo == nil {
		return Rollout{}, errors.New("rollout service is not initialized")
	}
	input.RolloutID = strings.TrimSpace(input.RolloutID)
	input.GuardSummary = strings.TrimSpace(input.GuardSummary)
	if input.RolloutID == "" {
		return Rollout{}, errors.New("rollout_id is required")
	}
	return s.repo.Promote(ctx, input)
}
