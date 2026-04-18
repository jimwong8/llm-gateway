package runtime

import (
	"context"
	"sort"
	"strings"
	"time"

	"llm-gateway/gateway/internal/controlplane"
)

type multiModuleReleasedVersionLister interface {
	ListVersions(ctx context.Context, module, tenantID, environment, scope, projectID string) []controlplane.ConfigVersion
}

type routerReleasedVersionLister interface {
	ListVersions(ctx context.Context, module, tenantID, environment, scope, projectID string) []controlplane.ConfigVersion
}

// ReplayCurrentReleasedModuleConfig 在启动阶段回放当前已发布模块配置。
// 回放通过 runtime bus 发布 ConfigChangeEvent，确保 manager 状态与补偿记录沿用现有 apply 链路。
func ReplayCurrentReleasedModuleConfig(ctx context.Context, lister multiModuleReleasedVersionLister, bus Bus, module string) error {
	if lister == nil || bus == nil {
		return nil
	}
	module = strings.TrimSpace(module)
	if module == "" {
		return nil
	}

	versions := lister.ListVersions(ctx, module, "", "", "", "")
	released := make([]controlplane.ConfigVersion, 0, len(versions))
	for _, version := range versions {
		if !strings.EqualFold(strings.TrimSpace(version.Module), module) {
			continue
		}
		if version.Source != controlplane.ConfigStatusReleased {
			continue
		}
		released = append(released, version)
	}
	if len(released) == 0 {
		return nil
	}

	sort.Slice(released, func(i, j int) bool {
		return released[i].CreatedAt.After(released[j].CreatedAt)
	})
	current := released[0]
	applyPayload := BuildRuntimeApplyPayload(current)

	changedAt := current.CreatedAt
	if changedAt.IsZero() {
		changedAt = time.Now().UTC()
	}

	return bus.PublishConfigChange(ConfigChangeEvent{
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

// ReplayCurrentReleasedRouterConfig 在启动阶段回放当前已发布 router 配置。
func ReplayCurrentReleasedRouterConfig(ctx context.Context, lister routerReleasedVersionLister, bus Bus) error {
	return ReplayCurrentReleasedModuleConfig(ctx, lister, bus, "router")
}
