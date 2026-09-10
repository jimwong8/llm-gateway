package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"llm-gateway/gateway/internal/controlplane"
)

type routerBootstrapApplier interface {
	BootstrapFromJSON(raw []byte) error
	BootstrapFromFile(path string) error
}

type releasedVersionResolver interface {
	GetVersion(ctx context.Context, module, tenantID, environment, scope, projectID, versionID string) (controlplane.ConfigVersion, error)
}

type releasedPayloadRef struct {
	Module      string
	TenantID    string
	Environment string
	Scope       string
	ProjectID   string
	VersionID   string
}

// BuildRouterPayloadDrivenApply 构建 router runtime apply：
// 1) 优先按 released payload_ref 回放发布时 payload；
// 2) payload 不可用时回退到启动 bootstrap 文件（兼容旧行为）。
func BuildRouterPayloadDrivenApply(applier routerBootstrapApplier, publisher *Publisher, bootstrapPath string) func(ConfigChangeEvent) error {
	return BuildRouterPayloadDrivenApplyWithResolver(applier, publisher, nil, bootstrapPath)
}

func BuildRouterPayloadDrivenApplyWithResolver(applier routerBootstrapApplier, publisher *Publisher, resolver releasedVersionResolver, bootstrapPath string) func(ConfigChangeEvent) error {
	return func(event ConfigChangeEvent) error {
		if applier == nil {
			return nil
		}

		payloadRef := strings.TrimSpace(event.PayloadRef)
		if resolver != nil && payloadRef != "" {
			if parsed, ok := parseReleasedPayloadRef(payloadRef); ok {
				if version, err := resolver.GetVersion(context.Background(), parsed.Module, parsed.TenantID, parsed.Environment, parsed.Scope, parsed.ProjectID, parsed.VersionID); err == nil {
					if applyPayload, ok := buildRuntimeApplyPayloadFromReleasedVersion(version); ok {
						if err := applyRouterPayload(applier, applyPayload); err != nil {
							return err
						}
						return nil
					}
				}
			}
		}

		if publisher != nil && payloadRef != "" {
			if applyPayload, ok := publisher.FindApplyPayloadByRef(payloadRef); ok {
				if err := applyRouterPayload(applier, applyPayload); err != nil {
					return err
				}
				if _, hasRouterPayload := applyPayload.ModulePayloads["router"]; hasRouterPayload {
					return nil
				}
			}
		}

		return applier.BootstrapFromFile(bootstrapPath)
	}
}

func parseReleasedPayloadRef(payloadRef string) (releasedPayloadRef, bool) {
	payloadRef = strings.TrimSpace(payloadRef)
	if !strings.HasPrefix(payloadRef, "released://") {
		return releasedPayloadRef{}, false
	}
	parts := strings.Split(strings.TrimPrefix(payloadRef, "released://"), "/")
	if len(parts) != 6 {
		return releasedPayloadRef{}, false
	}
	if strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" || strings.TrimSpace(parts[3]) == "" || strings.TrimSpace(parts[5]) == "" {
		return releasedPayloadRef{}, false
	}
	return releasedPayloadRef{
		Module:      strings.TrimSpace(parts[0]),
		TenantID:    strings.TrimSpace(parts[1]),
		Environment: strings.TrimSpace(parts[2]),
		Scope:       strings.TrimSpace(parts[3]),
		ProjectID:   strings.TrimSpace(parts[4]),
		VersionID:   strings.TrimSpace(parts[5]),
	}, true
}

func buildRuntimeApplyPayloadFromReleasedVersion(version controlplane.ConfigVersion) (RuntimeApplyPayload, bool) {
	if version.Source != controlplane.ConfigStatusReleased {
		return RuntimeApplyPayload{}, false
	}
	return BuildRuntimeApplyPayload(version), true
}

func applyRouterPayload(applier routerBootstrapApplier, applyPayload RuntimeApplyPayload) error {
	rawRouterPayload, hasRouterPayload := applyPayload.ModulePayloads["router"]
	if !hasRouterPayload {
		return nil
	}
	routerPayload, ok := rawRouterPayload.(map[string]any)
	if !ok {
		return fmt.Errorf("router runtime apply payload type invalid: %T", rawRouterPayload)
	}
	raw, err := json.Marshal(routerPayload)
	if err != nil {
		return fmt.Errorf("marshal router runtime payload: %w", err)
	}
	if err := applier.BootstrapFromJSON(raw); err != nil {
		return err
	}
	return nil
}
