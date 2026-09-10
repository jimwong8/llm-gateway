package controlplane

import "time"

type ReleaseState string

const (
	ReleaseStateDraft    ReleaseState = "draft"
	ReleaseStatePending  ReleaseState = "pending"
	ReleaseStateReleased ReleaseState = "released"
)

type ReleaseRecord struct {
	Module            string       `json:"module"`
	TenantID          string       `json:"tenant_id"`
	Environment       string       `json:"environment"`
	Scope             string       `json:"scope"`
	ProjectID         string       `json:"project_id,omitempty"`
	Version           string       `json:"version"`
	State             ReleaseState `json:"state"`
	SourceEnvironment string       `json:"source_environment,omitempty"`
	SourceVersion     string       `json:"source_version,omitempty"`
	CreatedAt         time.Time    `json:"created_at"`
}

type ReleaseStore struct {
	records []ReleaseRecord
}

func NewReleaseStore() *ReleaseStore {
	return &ReleaseStore{}
}

func (s *ReleaseStore) Add(record ReleaseRecord) {
	s.records = append(s.records, record)
}

func (s *ReleaseStore) SubmitForApproval(record ReleaseRecord) ReleaseRecord {
	record.State = ReleaseStatePending
	s.Add(record)
	return record
}

func (s *ReleaseStore) ApproveRelease(record ReleaseRecord) ReleaseRecord {
	record.State = ReleaseStateReleased
	s.Add(record)
	return record
}

func (s *ReleaseStore) PromoteToEnvironment(record ReleaseRecord, targetEnv string, hooks []ValidationHook) (ReleaseRecord, []GateResult) {
	results := RunValidationHooks(hooks)
	for _, item := range results {
		if !item.Passed {
			return ReleaseRecord{}, results
		}
	}
	promoted := ReleaseRecord{
		Module:            record.Module,
		TenantID:          record.TenantID,
		Environment:       targetEnv,
		Scope:             record.Scope,
		ProjectID:         record.ProjectID,
		Version:           record.Version + "-promoted",
		State:             ReleaseStateReleased,
		SourceEnvironment: record.Environment,
		SourceVersion:     record.Version,
		CreatedAt:         time.Now().UTC(),
	}
	s.Add(promoted)
	return promoted, results
}

func (s *ReleaseStore) RollbackToVersion(module, tenantID, environment, scope, projectID, version string) ReleaseRecord {
	record := ReleaseRecord{
		Module:      module,
		TenantID:    tenantID,
		Environment: environment,
		Scope:       scope,
		ProjectID:   projectID,
		Version:     version,
		State:       ReleaseStateReleased,
		CreatedAt:   time.Now().UTC(),
	}
	s.Add(record)
	return record
}

func (s *ReleaseStore) List() []ReleaseRecord {
	return s.records
}


type PromotionEngine struct {
	Releases      *ReleaseStore
	Compensations *CompensationStore
}

func NewPromotionEngine() *PromotionEngine {
	return &PromotionEngine{Releases: NewReleaseStore(), Compensations: NewCompensationStore()}
}

func (e *PromotionEngine) Promote(record ReleaseRecord, targetEnv string, hooks []ValidationHook) (ReleaseRecord, []GateResult) {
	results := RunValidationHooks(hooks)
	for _, item := range results {
		if !item.Passed {
			e.Compensations.Add(CompensationRecord{
				Module: record.Module,
				TenantID: record.TenantID,
				Environment: targetEnv,
				Version: record.Version,
				FailedStage: FailedStagePromotionValidation,
				ErrorSummary: item.Error,
				SuggestedAction: SuggestedActionFor(FailedStagePromotionValidation),
				CreatedAt: time.Now().UTC(),
			})
			return ReleaseRecord{}, results
		}
	}
	promoted, results := e.Releases.PromoteToEnvironment(record, targetEnv, hooks)
	return promoted, results
}
