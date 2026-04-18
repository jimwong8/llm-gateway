package controlplane

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"llm-gateway/gateway/internal/audit"
)

const (
	ConfigStatusDraft    = "draft"
	ConfigStatusReleased = "released"

	SourceTypeInheritance = "inheritance"
)

var ErrReleasedConfigNotFound = errors.New("released config not found")
var ErrVersionNotFound = errors.New("config version not found")

type ConfigSource struct {
	Type              string
	SourceEnvironment string
	SourceVersionID   string
	GeneratedAt       time.Time
}

type CreateVersionInput struct {
	Module      string
	TenantID    string
	Environment string
	Scope       string
	ProjectID   string
	Version     string
	Status      string
	Actor       string
	Source      string
	Summary     string
	Config      map[string]string
}

type CreateInheritanceDraftInput struct {
	Module            string
	TenantID          string
	Scope             string
	ProjectID         string
	SourceEnvironment string
	TargetEnvironment string
	Reason            string
	Actor             string
}

type releasePublisher interface {
	PublishIfReleased(version ConfigVersion) bool
}

type RollbackReleasedInput struct {
	Module      string
	TenantID    string
	Environment string
	Scope       string
	ProjectID   string
	VersionID   string
	Actor       string
	Reason      string
}

type Service struct {
	now       func() time.Time
	seq       int
	versions  []ConfigVersion
	auditor   *audit.Recorder
	publisher releasePublisher
}

func NewService() *Service {
	return &Service{now: time.Now}
}

func (s *Service) WithAuditRecorder(recorder *audit.Recorder) *Service {
	s.auditor = recorder
	return s
}

func (s *Service) WithReleasePublisher(publisher releasePublisher) *Service {
	s.publisher = publisher
	return s
}

func (s *Service) CreateVersion(_ context.Context, input CreateVersionInput) (ConfigVersion, error) {
	s.seq++
	now := s.now().UTC()
	versionID := input.Version
	if versionID == "" {
		versionID = fmt.Sprintf("cfg_%03d", s.seq)
	}
	version := ConfigVersion{
		Module:            input.Module,
		TenantID:          input.TenantID,
		Environment:       input.Environment,
		Scope:             input.Scope,
		ProjectID:         input.ProjectID,
		Version:           versionID,
		Config:            cloneConfig(input.Config),
		SourceEnvironment: "",
		SourceVersion:     "",
		Actor:             input.Actor,
		Source:            input.Source,
		Summary:           input.Summary,
		CreatedAt:         now,
	}
	if input.Status == ConfigStatusReleased {
		version.Source = ConfigStatusReleased
	}
	s.versions = append(s.versions, version)
	return version, nil
}

func (s *Service) CreateInheritanceDraft(_ context.Context, input CreateInheritanceDraftInput) (ConfigVersion, error) {
	source, ok := s.findCurrentReleased(input.Module, input.TenantID, input.SourceEnvironment, input.Scope, input.ProjectID)
	if !ok {
		return ConfigVersion{}, fmt.Errorf("source environment %q: %w", input.SourceEnvironment, ErrReleasedConfigNotFound)
	}

	s.seq++
	now := s.now().UTC()
	draft := ConfigVersion{
		Module:            source.Module,
		TenantID:          input.TenantID,
		Environment:       input.TargetEnvironment,
		Scope:             source.Scope,
		ProjectID:         source.ProjectID,
		Version:           fmt.Sprintf("cfg_%03d", s.seq),
		SourceEnvironment: source.Environment,
		SourceVersion:     source.Version,
		Actor:             input.Actor,
		Source:            SourceTypeInheritance,
		Summary:           input.Reason,
		CreatedAt:         now,
	}
	s.versions = append(s.versions, draft)
	return draft, nil
}

func (s *Service) ReleaseDraft(_ context.Context, module, tenantID, environment, scope, projectID, versionID, actor, reason string) (ConfigVersion, error) {
	for idx, version := range s.versions {
		if version.Module != module || version.TenantID != tenantID || version.Environment != environment || version.Scope != scope {
			continue
		}
		if projectID != "" && version.ProjectID != projectID {
			continue
		}
		if version.Version != versionID {
			continue
		}
		if version.Source == ConfigStatusReleased {
			return version, nil
		}

		released := version
		released.Actor = actor
		released.Summary = reason
		released.Source = ConfigStatusReleased
		released.CreatedAt = s.now().UTC()
		s.versions[idx] = released
		s.recordReleaseSideEffects(released, actor, reason)
		return released, nil
	}
	return ConfigVersion{}, ErrVersionNotFound
}

func (s *Service) PromoteReleased(_ context.Context, module, tenantID, sourceEnvironment, targetEnvironment, scope, projectID, actor, reason string) (ConfigVersion, error) {
	source, ok := s.findCurrentReleased(module, tenantID, sourceEnvironment, scope, projectID)
	if !ok {
		return ConfigVersion{}, fmt.Errorf("source environment %q: %w", sourceEnvironment, ErrReleasedConfigNotFound)
	}

	s.seq++
	promoted := ConfigVersion{
		Module:            source.Module,
		TenantID:          tenantID,
		Environment:       targetEnvironment,
		Scope:             source.Scope,
		ProjectID:         source.ProjectID,
		Version:           fmt.Sprintf("cfg_%03d", s.seq),
		SourceEnvironment: source.Environment,
		SourceVersion:     source.Version,
		Actor:             actor,
		Source:            ConfigStatusReleased,
		Summary:           reason,
		CreatedAt:         s.now().UTC(),
	}
	s.versions = append(s.versions, promoted)
	s.recordReleaseSideEffects(promoted, actor, reason)
	return promoted, nil
}

func (s *Service) RollbackReleased(ctx context.Context, input RollbackReleasedInput) (ConfigVersion, error) {
	target, err := s.GetVersion(ctx, input.Module, input.TenantID, input.Environment, input.Scope, input.ProjectID, input.VersionID)
	if err != nil {
		return ConfigVersion{}, err
	}

	s.seq++
	rolledBack := ConfigVersion{
		Module:            target.Module,
		TenantID:          target.TenantID,
		Environment:       target.Environment,
		Scope:             target.Scope,
		ProjectID:         target.ProjectID,
		Version:           fmt.Sprintf("cfg_%03d", s.seq),
		Config:            cloneConfig(target.Config),
		SourceEnvironment: target.Environment,
		SourceVersion:     target.Version,
		Actor:             input.Actor,
		Source:            ConfigStatusReleased,
		Summary:           input.Reason,
		CreatedAt:         s.now().UTC(),
	}
	s.versions = append(s.versions, rolledBack)
	s.recordReleaseSideEffects(rolledBack, input.Actor, input.Reason)
	return rolledBack, nil
}

func (s *Service) CurrentReleased(_ context.Context, module, tenantID, environment, scope, projectID string) (ConfigVersion, error) {
	version, ok := s.findCurrentReleased(module, tenantID, environment, scope, projectID)
	if !ok {
		return ConfigVersion{}, ErrReleasedConfigNotFound
	}
	return version, nil
}

func (s *Service) GetVersion(_ context.Context, module, tenantID, environment, scope, projectID, versionID string) (ConfigVersion, error) {
	for _, version := range s.versions {
		if version.Module != module || version.TenantID != tenantID || version.Environment != environment || version.Scope != scope {
			continue
		}
		if projectID != "" && version.ProjectID != projectID {
			continue
		}
		if version.Version != versionID {
			continue
		}
		return version, nil
	}
	return ConfigVersion{}, ErrVersionNotFound
}

func (s *Service) ListVersions(_ context.Context, module, tenantID, environment, scope, projectID string) []ConfigVersion {
	versions := make([]ConfigVersion, 0)
	for _, version := range s.versions {
		if module != "" && version.Module != module {
			continue
		}
		if tenantID != "" && version.TenantID != tenantID {
			continue
		}
		if environment != "" && version.Environment != environment {
			continue
		}
		if scope != "" && version.Scope != scope {
			continue
		}
		if projectID != "" && version.ProjectID != projectID {
			continue
		}
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].CreatedAt.After(versions[j].CreatedAt)
	})
	return versions
}

func (s *Service) ResolveConfig(_ context.Context, module, tenantID, environment, scope, projectID string, projectOverride, tenantOverride, tenantTemplate, tierDefault map[string]string) (map[string]string, error) {
	resolved := cloneConfig(tierDefault)
	mergeConfig(resolved, tenantTemplate)
	mergeConfig(resolved, tenantOverride)
	mergeConfig(resolved, projectOverride)

	_, err := s.CurrentReleased(context.Background(), module, tenantID, environment, scope, projectID)
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

func (s *Service) findCurrentReleased(module, tenantID, environment, scope, projectID string) (ConfigVersion, bool) {
	var matches []ConfigVersion
	for _, version := range s.versions {
		if version.Module != module || version.TenantID != tenantID || version.Environment != environment || version.Scope != scope {
			continue
		}
		if projectID != "" && version.ProjectID != projectID {
			continue
		}
		if version.Source != ConfigStatusReleased {
			continue
		}
		matches = append(matches, version)
	}
	if len(matches) == 0 {
		return ConfigVersion{}, false
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].CreatedAt.After(matches[j].CreatedAt)
	})
	return matches[0], true
}

func (s *Service) recordReleaseSideEffects(version ConfigVersion, actor, reason string) {
	if s.auditor != nil {
		s.auditor.RecordRelease(version.Module, version.TenantID, version.Environment, version.Version, actor, reason)
	}
	if s.publisher != nil {
		s.publisher.PublishIfReleased(version)
	}
}

func cloneConfig(input map[string]string) map[string]string {
	if input == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func mergeConfig(dst, src map[string]string) {
	for k, v := range src {
		dst[k] = v
	}
}
