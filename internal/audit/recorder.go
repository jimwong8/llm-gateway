package audit

import "time"

const (
	ControlPlaneEventTypeRelease          = "release"
	ControlPlaneEventTypeInheritanceDraft = "inheritance-draft"
)

type ControlPlaneEvent struct {
	Type              string    `json:"type"`
	Module            string    `json:"module"`
	TenantID          string    `json:"tenant_id"`
	Environment       string    `json:"environment"`
	SourceEnvironment string    `json:"source_environment,omitempty"`
	SourceVersionID   string    `json:"source_version_id,omitempty"`
	VersionID         string    `json:"version_id"`
	Actor             string    `json:"actor,omitempty"`
	Reason            string    `json:"reason,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

type Recorder struct {
	now    func() time.Time
	events []ControlPlaneEvent
}

func NewRecorder() *Recorder {
	return &Recorder{now: time.Now}
}

func (r *Recorder) RecordRelease(module, tenantID, environment, versionID, actor, reason string) ControlPlaneEvent {
	event := ControlPlaneEvent{
		Type:        ControlPlaneEventTypeRelease,
		Module:      module,
		TenantID:    tenantID,
		Environment: environment,
		VersionID:   versionID,
		Actor:       actor,
		Reason:      reason,
		CreatedAt:   r.now().UTC(),
	}
	r.events = append(r.events, event)
	return event
}

func (r *Recorder) RecordInheritanceDraft(module, tenantID, environment, sourceEnvironment, sourceVersionID, versionID, actor, reason string) ControlPlaneEvent {
	event := ControlPlaneEvent{
		Type:              ControlPlaneEventTypeInheritanceDraft,
		Module:            module,
		TenantID:          tenantID,
		Environment:       environment,
		SourceEnvironment: sourceEnvironment,
		SourceVersionID:   sourceVersionID,
		VersionID:         versionID,
		Actor:             actor,
		Reason:            reason,
		CreatedAt:         r.now().UTC(),
	}
	r.events = append(r.events, event)
	return event
}

func (r *Recorder) Events() []ControlPlaneEvent {
	out := make([]ControlPlaneEvent, len(r.events))
	copy(out, r.events)
	return out
}
