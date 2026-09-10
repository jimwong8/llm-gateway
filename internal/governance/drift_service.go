package governance

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// DetectDriftInput 描述漂移检测上下文。
type DetectDriftInput struct {
	TenantID    string
	Environment string
	AgentID     string
}

// DriftService 提供最小 drift 检测能力。
type DriftService struct {
	repo driftDataStore
}

type driftDataStore interface {
	LoadActiveModel(ctx context.Context, environment, agentID string) (string, string, error)
	LoadLatestRecommendation(ctx context.Context, environment, agentID string) (string, string, error)
	CreateDriftRecord(ctx context.Context, drift PolicyDrift) (PolicyDrift, error)
	UpdateDriftStatus(ctx context.Context, driftID string, status PolicyDriftStatus, reason string) (PolicyDrift, error)
}

func NewDriftService(store *Store) *DriftService {
	if store == nil {
		return &DriftService{}
	}
	repo := NewDriftRepo(store)
	return &DriftService{repo: repo}
}

func NewDriftServiceWithRepo(repo driftDataStore) *DriftService {
	return &DriftService{repo: repo}
}

func (s *DriftService) DetectModelMismatch(ctx context.Context, input DetectDriftInput) (PolicyDrift, bool, error) {
	if s == nil || s.repo == nil {
		return PolicyDrift{}, false, errors.New("drift service is not initialized")
	}
	input.Environment = strings.TrimSpace(input.Environment)
	input.AgentID = strings.TrimSpace(input.AgentID)
	input.TenantID = strings.TrimSpace(input.TenantID)
	if input.Environment == "" || input.AgentID == "" {
		return PolicyDrift{}, false, errors.New("environment and agent_id are required")
	}

	activePolicyVersion, activeModel, err := s.repo.LoadActiveModel(ctx, input.Environment, input.AgentID)
	if err != nil {
		return PolicyDrift{}, false, err
	}

	_, recommendedModel, err := s.repo.LoadLatestRecommendation(ctx, input.Environment, input.AgentID)
	if err != nil {
		return PolicyDrift{}, false, err
	}
	if activeModel == recommendedModel {
		return PolicyDrift{}, false, nil
	}

	drift, err := s.repo.CreateDriftRecord(ctx, PolicyDrift{
		TenantID:            input.TenantID,
		Environment:         input.Environment,
		AgentID:             input.AgentID,
		Status:              PolicyDriftStatusDetected,
		ActivePolicyVersion: activePolicyVersion,
		CurrentModelID:      activeModel,
		RecommendedModelID:  recommendedModel,
		Reason:              fmt.Sprintf("active model %s differs from latest recommendation %s", activeModel, recommendedModel),
	})
	if err != nil {
		return PolicyDrift{}, false, err
	}

	return drift, true, nil
}

func (s *DriftService) Acknowledge(ctx context.Context, driftID, reason string) (PolicyDrift, error) {
	if s == nil || s.repo == nil {
		return PolicyDrift{}, errors.New("drift service is not initialized")
	}
	return s.repo.UpdateDriftStatus(ctx, strings.TrimSpace(driftID), PolicyDriftStatusAccepted, strings.TrimSpace(reason))
}

func (s *DriftService) Resolve(ctx context.Context, driftID, reason string) (PolicyDrift, error) {
	if s == nil || s.repo == nil {
		return PolicyDrift{}, errors.New("drift service is not initialized")
	}
	return s.repo.UpdateDriftStatus(ctx, strings.TrimSpace(driftID), PolicyDriftStatusResolved, strings.TrimSpace(reason))
}
