package controlplane

import "time"

type ConfigVersion struct {
	Module            string            `json:"module"`
	TenantID          string            `json:"tenant_id"`
	Environment       string            `json:"environment"`
	Scope             string            `json:"scope"`
	ProjectID         string            `json:"project_id,omitempty"`
	Version           string            `json:"version"`
	Config            map[string]string `json:"config,omitempty"`
	SourceEnvironment string            `json:"source_environment,omitempty"`
	SourceVersion     string            `json:"source_version,omitempty"`
	Actor             string            `json:"actor,omitempty"`
	Source            string            `json:"source,omitempty"`
	Summary           string            `json:"summary,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
}

type ActivePointer struct {
	Module      string `json:"module"`
	TenantID    string `json:"tenant_id"`
	Environment string `json:"environment"`
	Scope       string `json:"scope"`
	ProjectID   string `json:"project_id,omitempty"`
	Version     string `json:"version"`
}

type ConfigDiff struct {
	FromVersion string         `json:"from_version"`
	ToVersion   string         `json:"to_version"`
	Changes     map[string]any `json:"changes"`
}

type VersionStore struct {
	versions []ConfigVersion
	active   map[string]ActivePointer
}

func NewVersionStore() *VersionStore {
	return &VersionStore{active: map[string]ActivePointer{}}
}

func (s *VersionStore) AddVersion(v ConfigVersion) {
	s.versions = append(s.versions, v)
}

func pointerKey(module, tenantID, environment, scope, projectID string) string {
	return module + "|" + tenantID + "|" + environment + "|" + scope + "|" + projectID
}

func (s *VersionStore) SetActive(pointer ActivePointer) {
	s.active[pointerKey(pointer.Module, pointer.TenantID, pointer.Environment, pointer.Scope, pointer.ProjectID)] = pointer
}

func (s *VersionStore) GetActive(module, tenantID, environment, scope, projectID string) (ActivePointer, bool) {
	v, ok := s.active[pointerKey(module, tenantID, environment, scope, projectID)]
	return v, ok
}

func (s *VersionStore) ListVersions() []ConfigVersion {
	return s.versions
}

func DiffVersions(from, to ConfigVersion) ConfigDiff {
	changes := map[string]any{}
	if from.Summary != to.Summary {
		changes["summary"] = map[string]string{"from": from.Summary, "to": to.Summary}
	}
	if from.SourceEnvironment != to.SourceEnvironment {
		changes["source_environment"] = map[string]string{"from": from.SourceEnvironment, "to": to.SourceEnvironment}
	}
	if from.SourceVersion != to.SourceVersion {
		changes["source_version"] = map[string]string{"from": from.SourceVersion, "to": to.SourceVersion}
	}
	return ConfigDiff{FromVersion: from.Version, ToVersion: to.Version, Changes: changes}
}

func (s *VersionStore) RollbackSet(pointers []ActivePointer) {
	for _, p := range pointers {
		s.SetActive(p)
	}
}
