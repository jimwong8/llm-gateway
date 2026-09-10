package runtime

import (
	"sort"
	"sync"
	"time"

	"llm-gateway/gateway/internal/controlplane"
)

type ReloadStatus struct {
	Name                 string    `json:"name"`
	LastSeenEventAt      time.Time `json:"last_seen_event_at"`
	LastSeenEventVersion string    `json:"last_seen_event_version"`
	LastReloadAt         time.Time `json:"last_reload_at"`
	LastReloadStatus     string    `json:"last_reload_status"`
	LastReloadError      string    `json:"last_reload_error"`
}

type Manager struct {
	mu            sync.RWMutex
	statuses      map[string]ReloadStatus
	compensations *controlplane.CompensationStore
}

func NewManager() *Manager {
	return &Manager{statuses: map[string]ReloadStatus{}, compensations: controlplane.NewCompensationStore()}
}

func (m *Manager) SetStatus(name, status, err string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.statuses[name]
	s.Name = name
	s.LastReloadAt = time.Now().UTC()
	s.LastReloadStatus = status
	s.LastReloadError = err
	m.statuses[name] = s
}

func (m *Manager) MarkEventSeen(name, version string, at time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.statuses[name]
	s.Name = name
	s.LastSeenEventAt = at.UTC()
	s.LastSeenEventVersion = version
	m.statuses[name] = s
}

func (m *Manager) GetStatus(name string) ReloadStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.statuses[name]
}

func (m *Manager) AllStatuses() []ReloadStatus {
	m.mu.RLock()
	out := make([]ReloadStatus, 0, len(m.statuses))
	for _, status := range m.statuses {
		out = append(out, status)
	}
	m.mu.RUnlock()
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		if !out[i].LastSeenEventAt.Equal(out[j].LastSeenEventAt) {
			return out[i].LastSeenEventAt.After(out[j].LastSeenEventAt)
		}
		return out[i].LastReloadAt.After(out[j].LastReloadAt)
	})
	return out
}

func (m *Manager) CompensationRecords() []controlplane.CompensationRecord {
	if m.compensations == nil {
		return nil
	}
	return m.compensations.List()
}

func (m *Manager) HandleConfigChange(event ConfigChangeEvent, reload func() error) {
	m.MarkEventSeen(event.Module, event.Version, event.ChangedAt)
	if reload == nil {
		m.SetStatus(event.Module, "skipped", "reload func is nil")
		if m.compensations != nil {
			m.compensations.Add(controlplane.CompensationRecord{Module: event.Module, TenantID: event.TenantID, Environment: event.Environment, Version: event.Version, FailedStage: controlplane.FailedStageReload, ErrorSummary: "reload func is nil", SuggestedAction: controlplane.SuggestedActionFor(controlplane.FailedStageReload), CreatedAt: time.Now().UTC()})
		}
		return
	}
	if err := reload(); err != nil {
		m.SetStatus(event.Module, "error", err.Error())
		if m.compensations != nil {
			m.compensations.Add(controlplane.CompensationRecord{Module: event.Module, TenantID: event.TenantID, Environment: event.Environment, Version: event.Version, FailedStage: controlplane.FailedStageReload, ErrorSummary: err.Error(), SuggestedAction: controlplane.SuggestedActionFor(controlplane.FailedStageReload), CreatedAt: time.Now().UTC()})
		}
		return
	}
	m.SetStatus(event.Module, "ok", "")
}
