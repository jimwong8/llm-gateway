package controlplane

import "time"

type CompensationRecord struct {
	Module          string    `json:"module"`
	TenantID        string    `json:"tenant_id"`
	Environment     string    `json:"environment"`
	Version         string    `json:"version"`
	FailedStage     string    `json:"failed_stage"`
	ErrorSummary    string    `json:"error_summary"`
	SuggestedAction string    `json:"suggested_action"`
	CreatedAt       time.Time `json:"created_at"`
}

const (
	FailedStagePromotionGate       = "promotion_gate_failed"
	FailedStagePromotionValidation = "promotion_validation_failed"
	FailedStageReleaseWrite        = "release_write_failed"
	FailedStageReload              = "reload_failed"
	FailedStageConfigSync          = "config_sync_failed"
)

type CompensationStore struct {
	records []CompensationRecord
}

func NewCompensationStore() *CompensationStore {
	return &CompensationStore{}
}

func (s *CompensationStore) Add(record CompensationRecord) {
	s.records = append(s.records, record)
}

func (s *CompensationStore) List() []CompensationRecord {
	return s.records
}

func SuggestedActionFor(stage string) string {
	switch stage {
	case FailedStagePromotionGate:
		return "review promotion gate conditions and retry manually"
	case FailedStagePromotionValidation:
		return "inspect validation hook output and retry after fix"
	case FailedStageReleaseWrite:
		return "inspect target release write path and retry"
	case FailedStageReload:
		return "inspect reload error and retry reload manually"
	case FailedStageConfigSync:
		return "inspect message propagation and re-send config event"
	default:
		return "inspect failure details and recover manually"
	}
}
