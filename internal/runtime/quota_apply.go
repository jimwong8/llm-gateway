package runtime

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type quotaRuntimeLimiter interface {
	SetRPM(rpm int)
}

func BuildQuotaPayloadDrivenApply(limiter quotaRuntimeLimiter, publisher *Publisher, resolver releasedVersionResolver) func(ConfigChangeEvent) error {
	return BuildQuotaPayloadDrivenApplyWithResolver(limiter, publisher, resolver)
}

// BuildQuotaPayloadDrivenApplyWithResolver 构建 quota runtime apply：
// 1) 优先从 released payload_ref 对应的 controlplane 版本构建 payload 并应用；
// 2) resolver 不可用或失败时回退到 publisher 缓存 payload；
// 3) quota payload 缺失时保持 no-op（兼容静态配置行为）。
func BuildQuotaPayloadDrivenApplyWithResolver(limiter quotaRuntimeLimiter, publisher *Publisher, resolver releasedVersionResolver) func(ConfigChangeEvent) error {
	return func(event ConfigChangeEvent) error {
		if limiter == nil {
			return nil
		}

		payloadRef := strings.TrimSpace(event.PayloadRef)
		if payloadRef == "" {
			return nil
		}

		if resolver != nil {
			if parsed, ok := parseReleasedPayloadRef(payloadRef); ok {
				if version, err := resolver.GetVersion(context.Background(), parsed.Module, parsed.TenantID, parsed.Environment, parsed.Scope, parsed.ProjectID, parsed.VersionID); err == nil {
					if applyPayload, ok := buildRuntimeApplyPayloadFromReleasedVersion(version); ok {
						applied, err := applyQuotaPayload(limiter, applyPayload)
						if err != nil {
							return err
						}
						if applied {
							return nil
						}
					}
				}
			}
		}

		if publisher != nil {
			if applyPayload, ok := publisher.FindApplyPayloadByRef(payloadRef); ok {
				applied, err := applyQuotaPayload(limiter, applyPayload)
				if err != nil {
					return err
				}
				if applied {
					return nil
				}
			}
		}

		return nil
	}
}

func applyQuotaPayload(limiter quotaRuntimeLimiter, applyPayload RuntimeApplyPayload) (bool, error) {
	rawQuotaPayload, hasQuotaPayload := applyPayload.ModulePayloads["quota"]
	if !hasQuotaPayload {
		return false, nil
	}
	quotaPayload, ok := rawQuotaPayload.(map[string]any)
	if !ok {
		return false, fmt.Errorf("quota runtime apply payload type invalid: %T", rawQuotaPayload)
	}
	rawRPM, hasRPM := quotaPayload["rpm"]
	if !hasRPM {
		return false, nil
	}
	rpm, ok := parseQuotaRPM(rawRPM)
	if !ok {
		return false, fmt.Errorf("quota runtime apply rpm is invalid: %v", rawRPM)
	}
	if rpm < 0 {
		return false, fmt.Errorf("quota runtime apply rpm must be >= 0, got %d", rpm)
	}
	limiter.SetRPM(rpm)
	return true, nil
}

func parseQuotaRPM(raw any) (int, bool) {
	switch v := raw.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), float64(int(v)) == v
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}
