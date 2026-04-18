package runtime

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"llm-gateway/gateway/internal/controlplane"
)

type Event struct {
	Version controlplane.ConfigVersion
	Apply   RuntimeApplyPayload
}

type RuntimeApplyPayload struct {
	Module         string         `json:"module"`
	Scope          string         `json:"scope"`
	TenantID       string         `json:"tenant_id"`
	Environment    string         `json:"environment,omitempty"`
	ProjectID      string         `json:"project_id,omitempty"`
	Version        string         `json:"version"`
	PayloadRef     string         `json:"payload_ref"`
	ModulePayloads map[string]any `json:"module_payloads,omitempty"`
}

type Publisher struct {
	events []Event
	bus    Bus
}

func NewPublisher() *Publisher {
	return &Publisher{}
}

func (p *Publisher) WithBus(bus Bus) *Publisher {
	p.bus = bus
	return p
}

func (p *Publisher) PublishIfReleased(version controlplane.ConfigVersion) bool {
	if version.Source != controlplane.ConfigStatusReleased {
		return false
	}
	applyPayload := BuildRuntimeApplyPayload(version)
	p.events = append(p.events, Event{Version: version, Apply: applyPayload})
	if p.bus != nil {
		changedAt := version.CreatedAt
		if changedAt.IsZero() {
			changedAt = time.Now().UTC()
		}
		_ = p.bus.PublishConfigChange(ConfigChangeEvent{
			Module:      applyPayload.Module,
			Scope:       applyPayload.Scope,
			TenantID:    applyPayload.TenantID,
			Environment: applyPayload.Environment,
			ProjectID:   applyPayload.ProjectID,
			Version:     applyPayload.Version,
			ChangedAt:   changedAt,
			PayloadRef:  applyPayload.PayloadRef,
		})
	}
	return true
}

func BuildRuntimeApplyPayload(version controlplane.ConfigVersion) RuntimeApplyPayload {
	modulePayloads := map[string]any{}
	if routerPayload, ok := buildRouterModulePayload(version); ok {
		modulePayloads["router"] = routerPayload
	}
	if quotaPayload, ok := buildQuotaModulePayload(version); ok {
		modulePayloads["quota"] = quotaPayload
	}
	// policy payload 由 buildPolicyModulePayload 统一构建，支持 allowed_models 与 provider policies live apply 透传。
	if policyPayload, ok := buildPolicyModulePayload(version); ok {
		modulePayloads["policy"] = policyPayload
	}
	return RuntimeApplyPayload{
		Module:         version.Module,
		Scope:          version.Scope,
		TenantID:       version.TenantID,
		Environment:    version.Environment,
		ProjectID:      version.ProjectID,
		Version:        version.Version,
		PayloadRef:     buildReleasedPayloadRef(version),
		ModulePayloads: modulePayloads,
	}
}

func buildRouterModulePayload(version controlplane.ConfigVersion) (map[string]any, bool) {
	if !strings.EqualFold(strings.TrimSpace(version.Module), "router") {
		return nil, false
	}
	if len(version.Config) == 0 {
		return nil, false
	}

	channels, hasChannels := parseRouterConfigList(version.Config, "channels")
	abilities, hasAbilities := parseRouterConfigList(version.Config, "abilities")
	policy, hasPolicy := parseRouterConfigObject(version.Config, "policy")
	if !hasChannels && !hasAbilities && !hasPolicy {
		return nil, false
	}

	payload := map[string]any{}
	if hasChannels {
		payload["channels"] = channels
	}
	if hasAbilities {
		payload["abilities"] = abilities
	}
	if hasPolicy {
		payload["policy"] = policy
	}
	return payload, true
}

func parseRouterConfigList(config map[string]string, key string) ([]map[string]any, bool) {
	raw := strings.TrimSpace(config[key])
	if raw == "" {
		return nil, false
	}
	var out []map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, false
	}
	return out, true
}

func parseRouterConfigObject(config map[string]string, key string) (map[string]any, bool) {
	raw := strings.TrimSpace(config[key])
	if raw == "" {
		return nil, false
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, false
	}
	return out, true
}

func buildQuotaModulePayload(version controlplane.ConfigVersion) (map[string]any, bool) {
	if !strings.EqualFold(strings.TrimSpace(version.Module), "quota") {
		return nil, false
	}
	if len(version.Config) == 0 {
		return nil, false
	}
	rawRPM := strings.TrimSpace(version.Config["rpm"])
	if rawRPM == "" {
		return nil, false
	}
	var rpm int
	if _, err := fmt.Sscanf(rawRPM, "%d", &rpm); err != nil {
		return nil, false
	}
	if rpm < 0 {
		return nil, false
	}
	return map[string]any{"rpm": rpm}, true
}

func buildReleasedPayloadRef(version controlplane.ConfigVersion) string {
	module := strings.TrimSpace(version.Module)
	tenantID := strings.TrimSpace(version.TenantID)
	environment := strings.TrimSpace(version.Environment)
	scope := strings.TrimSpace(version.Scope)
	projectID := strings.TrimSpace(version.ProjectID)
	versionID := strings.TrimSpace(version.Version)
	return fmt.Sprintf("released://%s/%s/%s/%s/%s/%s", module, tenantID, environment, scope, projectID, versionID)
}

func (p *Publisher) Events() []Event {
	out := make([]Event, len(p.events))
	copy(out, p.events)
	return out
}

func (p *Publisher) FindApplyPayloadByRef(payloadRef string) (RuntimeApplyPayload, bool) {
	payloadRef = strings.TrimSpace(payloadRef)
	if payloadRef == "" {
		return RuntimeApplyPayload{}, false
	}
	for idx := len(p.events) - 1; idx >= 0; idx-- {
		if p.events[idx].Apply.PayloadRef == payloadRef {
			return p.events[idx].Apply, true
		}
	}
	return RuntimeApplyPayload{}, false
}
