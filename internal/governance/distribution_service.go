package governance

import (
	"context"
	"errors"
	"strings"
)

// DistributionService 负责 rollout 激活/回滚事件构造与持久化。
type DistributionService struct {
	repo *DistributionRepo
}

func NewDistributionService(store *Store) *DistributionService {
	if store == nil {
		return &DistributionService{}
	}
	return &DistributionService{repo: NewDistributionRepo(store)}
}

func NewDistributionServiceWithRepo(repo *DistributionRepo) *DistributionService {
	return &DistributionService{repo: repo}
}

func (s *DistributionService) CreateActivationEvent(ctx context.Context, rollout Rollout) (DistributionEvent, error) {
	if s == nil || s.repo == nil {
		return DistributionEvent{}, errors.New("distribution service is not initialized")
	}
	if strings.TrimSpace(rollout.ID) == "" {
		return DistributionEvent{}, errors.New("rollout.id is required")
	}
	if strings.TrimSpace(rollout.PolicyVersionID) == "" {
		return DistributionEvent{}, errors.New("rollout.policy_version_id is required")
	}
	if strings.TrimSpace(rollout.TargetEnvironment) == "" {
		return DistributionEvent{}, errors.New("rollout.target_environment is required")
	}
	return s.repo.Create(ctx, DistributionEvent{
		PolicyVersionID: rollout.PolicyVersionID,
		RolloutID:       rollout.ID,
		Environment:     rollout.TargetEnvironment,
		EventType:       DistributionEventActivated,
		Payload: map[string]any{
			"rollout_percent": rollout.RolloutPercent,
			"rollout_mode":    strings.TrimSpace(rollout.RolloutMode),
			"triggered_by":    strings.TrimSpace(rollout.TriggeredBy),
		},
	})
}

func (s *DistributionService) CreateRollbackEvent(ctx context.Context, rollout Rollout, actor, reason, restoredPolicyVersionID, revertedPolicyVersionID string) (DistributionEvent, error) {
	if s == nil || s.repo == nil {
		return DistributionEvent{}, errors.New("distribution service is not initialized")
	}
	if strings.TrimSpace(rollout.ID) == "" {
		return DistributionEvent{}, errors.New("rollout.id is required")
	}
	if strings.TrimSpace(rollout.TargetEnvironment) == "" {
		return DistributionEvent{}, errors.New("rollout.target_environment is required")
	}
	return s.repo.Create(ctx, DistributionEvent{
		PolicyVersionID: strings.TrimSpace(restoredPolicyVersionID),
		RolloutID:       rollout.ID,
		Environment:     rollout.TargetEnvironment,
		EventType:       DistributionEventRollback,
		Payload: map[string]any{
			"actor":                      strings.TrimSpace(actor),
			"reason":                     strings.TrimSpace(reason),
			"restored_policy_version_id": strings.TrimSpace(restoredPolicyVersionID),
			"reverted_policy_version_id": strings.TrimSpace(revertedPolicyVersionID),
		},
	})
}
