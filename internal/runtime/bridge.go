package runtime

import (
	"errors"
	"fmt"
	"strings"
)

type ModuleRuntimeApplier func(ConfigChangeEvent) error

// BuildModuleRuntimeApplyDispatcher 构建模块导向 runtime apply 分发器。
// - 仅分发到已注册模块
// - unknown/empty module 直接 no-op（保持兼容与安全）
func BuildModuleRuntimeApplyDispatcher(moduleAppliers map[string]ModuleRuntimeApplier) func(ConfigChangeEvent) error {
	if len(moduleAppliers) == 0 {
		return func(ConfigChangeEvent) error { return nil }
	}

	normalized := make(map[string]ModuleRuntimeApplier, len(moduleAppliers))
	for module, applier := range moduleAppliers {
		name := strings.ToLower(strings.TrimSpace(module))
		if name == "" || applier == nil {
			continue
		}
		normalized[name] = applier
	}

	if len(normalized) == 0 {
		return func(ConfigChangeEvent) error { return nil }
	}

	return func(event ConfigChangeEvent) error {
		moduleName := strings.ToLower(strings.TrimSpace(event.Module))
		if moduleName == "" {
			return nil
		}
		applier, ok := normalized[moduleName]
		if !ok {
			return nil
		}
		return applier(event)
	}
}

// SubscribeManagerApplyBridge 将 runtime bus 事件桥接到 manager 的 HandleConfigChange。
// 当 reload 为空时，默认使用 no-op reload，保持兼容并避免触发错误补偿记录。
func SubscribeManagerApplyBridge(bus Bus, manager *Manager, reload func(ConfigChangeEvent) error) {
	if bus == nil || manager == nil {
		return
	}
	bridgeReload := reload
	if bridgeReload == nil {
		bridgeReload = func(ConfigChangeEvent) error { return nil }
	}
	bus.SubscribeConfigChange(func(event ConfigChangeEvent) {
		manager.HandleConfigChange(event, func() error {
			return bridgeReload(event)
		})
	})
}

// BuildRouterReloadApply 构建 router-only 的 runtime apply 函数。
// - 非 router 模块直接跳过（兼容未来多模块事件）
// - router 事件必须携带 released:// payload ref 才允许进入实际 apply
// - 当 applier 为空时返回 no-op（保持兼容）
func BuildRouterReloadApply(applier func(ConfigChangeEvent) error) func(ConfigChangeEvent) error {
	if applier == nil {
		return func(ConfigChangeEvent) error { return nil }
	}
	routerOnlyApplier := func(event ConfigChangeEvent) error {
		if !strings.EqualFold(strings.TrimSpace(event.Module), "router") {
			return nil
		}
		payloadRef := strings.TrimSpace(event.PayloadRef)
		if payloadRef == "" {
			return errors.New("router runtime apply payload_ref is empty")
		}
		if !strings.HasPrefix(payloadRef, "released://") {
			return fmt.Errorf("router runtime apply payload_ref must start with released://, got %q", payloadRef)
		}
		if !strings.HasPrefix(payloadRef, "released://router/") {
			return fmt.Errorf("router runtime apply payload_ref must target router module, got %q", payloadRef)
		}
		return applier(event)
	}

	return BuildModuleRuntimeApplyDispatcher(map[string]ModuleRuntimeApplier{
		"router": routerOnlyApplier,
	})
}

// BuildQuotaReloadApply 构建 quota-only 的 runtime apply 函数。
// - 非 quota 模块直接跳过（兼容未来多模块事件）
// - quota 事件必须携带 released:// payload ref 才允许进入实际 apply
// - 当 applier 为空时返回 no-op（保持兼容）
func BuildQuotaReloadApply(applier func(ConfigChangeEvent) error) func(ConfigChangeEvent) error {
	if applier == nil {
		return func(ConfigChangeEvent) error { return nil }
	}
	quotaOnlyApplier := func(event ConfigChangeEvent) error {
		if !strings.EqualFold(strings.TrimSpace(event.Module), "quota") {
			return nil
		}
		payloadRef := strings.TrimSpace(event.PayloadRef)
		if payloadRef == "" {
			return errors.New("quota runtime apply payload_ref is empty")
		}
		if !strings.HasPrefix(payloadRef, "released://") {
			return fmt.Errorf("quota runtime apply payload_ref must start with released://, got %q", payloadRef)
		}
		if !strings.HasPrefix(payloadRef, "released://quota/") {
			return fmt.Errorf("quota runtime apply payload_ref must target quota module, got %q", payloadRef)
		}
		return applier(event)
	}

	return BuildModuleRuntimeApplyDispatcher(map[string]ModuleRuntimeApplier{
		"quota": quotaOnlyApplier,
	})
}

// BuildPolicyReloadApply 构建 policy-only 的 runtime apply 函数。
// - 非 policy 模块直接跳过（兼容未来多模块事件）
// - policy 事件必须携带 released:// payload ref 才允许进入实际 apply
// - 当 applier 为空时返回 no-op（保持兼容）
func BuildPolicyReloadApply(applier func(ConfigChangeEvent) error) func(ConfigChangeEvent) error {
	if applier == nil {
		return func(ConfigChangeEvent) error { return nil }
	}
	policyOnlyApplier := func(event ConfigChangeEvent) error {
		if !strings.EqualFold(strings.TrimSpace(event.Module), "policy") {
			return nil
		}
		payloadRef := strings.TrimSpace(event.PayloadRef)
		if payloadRef == "" {
			return errors.New("policy runtime apply payload_ref is empty")
		}
		if !strings.HasPrefix(payloadRef, "released://") {
			return fmt.Errorf("policy runtime apply payload_ref must start with released://, got %q", payloadRef)
		}
		if !strings.HasPrefix(payloadRef, "released://policy/") {
			return fmt.Errorf("policy runtime apply payload_ref must target policy module, got %q", payloadRef)
		}
		return applier(event)
	}

	return BuildModuleRuntimeApplyDispatcher(map[string]ModuleRuntimeApplier{
		"policy": policyOnlyApplier,
	})
}

