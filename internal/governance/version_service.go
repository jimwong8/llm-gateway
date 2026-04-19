package governance

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// VersionService 负责策略版本创建/审批/激活生命周期。
// 说明：此服务只负责 precondition 与状态流转，不负责 rollout。
type VersionService struct {
	repo versionDataStore
}

type versionDataStore interface {
	CreateDraftFromApproval(ctx context.Context, approvalID, createdBy string) (PolicyVersion, error)
	ApproveVersion(ctx context.Context, versionID, approvedBy string) (PolicyVersion, error)
	ActivateVersion(ctx context.Context, versionID string) (PolicyVersion, error)
	GetVersion(ctx context.Context, versionID string) (PolicyVersion, error)
	ActiveVersionCountByEnvironment(ctx context.Context, environment string) (int, error)
}

func NewVersionService(store *Store) *VersionService {
	if store == nil {
		return &VersionService{}
	}
	return &VersionService{repo: NewVersionRepo(store)}
}

func NewVersionServiceWithRepo(repo versionDataStore) *VersionService {
	return &VersionService{repo: repo}
}

func (s *VersionService) CreateFromApproval(ctx context.Context, approvalID, createdBy string) (PolicyVersion, error) {
	if s == nil || s.repo == nil {
		return PolicyVersion{}, errors.New("version service is not initialized")
	}
	approvalID = strings.TrimSpace(approvalID)
	if approvalID == "" {
		return PolicyVersion{}, errors.New("approval_id is required")
	}
	return s.repo.CreateDraftFromApproval(ctx, approvalID, strings.TrimSpace(createdBy))
}

func (s *VersionService) Approve(ctx context.Context, versionID, approvedBy string) (PolicyVersion, error) {
	if s == nil || s.repo == nil {
		return PolicyVersion{}, errors.New("version service is not initialized")
	}
	versionID = strings.TrimSpace(versionID)
	approvedBy = strings.TrimSpace(approvedBy)
	if versionID == "" {
		return PolicyVersion{}, errors.New("policy_version_id is required")
	}
	if approvedBy == "" {
		return PolicyVersion{}, errors.New("approved_by is required")
	}
	return s.repo.ApproveVersion(ctx, versionID, approvedBy)
}

func (s *VersionService) Activate(ctx context.Context, versionID string) (PolicyVersion, error) {
	if s == nil || s.repo == nil {
		return PolicyVersion{}, errors.New("version service is not initialized")
	}
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return PolicyVersion{}, errors.New("policy_version_id is required")
	}

	current, err := s.repo.GetVersion(ctx, versionID)
	if err != nil {
		return PolicyVersion{}, err
	}
	if current.Status != PolicyVersionApproved {
		return PolicyVersion{}, fmt.Errorf("%w: version must be approved before activation", ErrInvalidVersionTransition)
	}

	activeCnt, err := s.repo.ActiveVersionCountByEnvironment(ctx, current.Environment)
	if err != nil {
		return PolicyVersion{}, err
	}
	if activeCnt > 1 {
		return PolicyVersion{}, fmt.Errorf("%w: environment %s has %d active versions", ErrInvalidVersionTransition, current.Environment, activeCnt)
	}

	activated, err := s.repo.ActivateVersion(ctx, versionID)
	if err != nil {
		return PolicyVersion{}, err
	}

	activeCnt, err = s.repo.ActiveVersionCountByEnvironment(ctx, activated.Environment)
	if err != nil {
		return PolicyVersion{}, err
	}
	if activeCnt != 1 {
		return PolicyVersion{}, fmt.Errorf("%w: expected exactly one active version in %s, got %d", ErrInvalidVersionTransition, activated.Environment, activeCnt)
	}

	return activated, nil
}

func (s *VersionService) Get(ctx context.Context, versionID string) (PolicyVersion, error) {
	if s == nil || s.repo == nil {
		return PolicyVersion{}, errors.New("version service is not initialized")
	}
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return PolicyVersion{}, errors.New("policy_version_id is required")
	}
	return s.repo.GetVersion(ctx, versionID)
}
