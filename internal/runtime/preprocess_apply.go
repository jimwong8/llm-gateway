package runtime

import (
	"context"
	"fmt"
	"strings"

	"llm-gateway/gateway/internal/preprocess"
)

type preprocessConfigWriter interface {
	Set(cfg preprocess.Config)
	Get() preprocess.Config
}

func BuildPreprocessPayloadDrivenApply(store preprocessConfigWriter, publisher *Publisher, resolver releasedVersionResolver) func(ConfigChangeEvent) error {
	return BuildPreprocessPayloadDrivenApplyWithResolver(store, publisher, resolver)
}

func BuildPreprocessPayloadDrivenApplyWithResolver(store preprocessConfigWriter, publisher *Publisher, resolver releasedVersionResolver) func(ConfigChangeEvent) error {
	return func(event ConfigChangeEvent) error {
		if store == nil {
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
						applied, err := applyPreprocessPayload(store, applyPayload)
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
				applied, err := applyPreprocessPayload(store, applyPayload)
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

func applyPreprocessPayload(store preprocessConfigWriter, applyPayload RuntimeApplyPayload) (bool, error) {
	rawPayload, hasPayload := applyPayload.ModulePayloads["preprocess"]
	if !hasPayload {
		if rawRouterPayload, hasRouterPayload := applyPayload.ModulePayloads["router"]; hasRouterPayload {
			routerPayload, ok := rawRouterPayload.(map[string]any)
			if !ok {
				return false, fmt.Errorf("router runtime apply payload type invalid: %T", rawRouterPayload)
			}
			rawPayload, hasPayload = routerPayload["preprocess"]
			if !hasPayload {
				return false, nil
			}
		} else {
			return false, nil
		}
	}
	payload, ok := rawPayload.(map[string]any)
	if !ok {
		return false, fmt.Errorf("preprocess runtime apply payload type invalid: %T", rawPayload)
	}
	cfg := store.Get()
	if raw, ok := payload["normalize_enabled"]; ok {
		if v, ok := parseBool(raw); ok {
			cfg.NormalizeEnabled = v
		}
	}
	if raw, ok := payload["summary_enabled"]; ok {
		if v, ok := parseBool(raw); ok {
			cfg.SummaryEnabled = v
		}
	}
	if raw, ok := payload["classification_enabled"]; ok {
		if v, ok := parseBool(raw); ok {
			cfg.ClassificationEnabled = v
		}
	}
	if raw, ok := payload["summary_trigger_messages"]; ok {
		if v, ok := parseInt(raw); ok {
			cfg.SummaryTriggerMessages = v
		}
	}
	if raw, ok := payload["summary_max_recent_turns"]; ok {
		if v, ok := parseInt(raw); ok {
			cfg.SummaryMaxRecentTurns = v
		}
	}
	if raw, ok := payload["summary_model"]; ok {
		if v, ok := raw.(string); ok {
			cfg.SummaryModel = strings.TrimSpace(v)
		}
	}
	if raw, ok := payload["summary_provider"]; ok {
		if v, ok := raw.(string); ok {
			cfg.SummaryProvider = strings.TrimSpace(v)
		}
	}
	if raw, ok := payload["classifier_model"]; ok {
		if v, ok := raw.(string); ok {
			cfg.ClassifierModel = strings.TrimSpace(v)
		}
	}
	if raw, ok := payload["classifier_provider"]; ok {
		if v, ok := raw.(string); ok {
			cfg.ClassifierProvider = strings.TrimSpace(v)
		}
	}
	store.Set(cfg)
	return true, nil
}

func parseBool(raw any) (bool, bool) {
	switch v := raw.(type) {
	case bool:
		return v, true
	case string:
		v = strings.TrimSpace(strings.ToLower(v))
		switch v {
		case "true", "1", "yes", "on":
			return true, true
		case "false", "0", "no", "off":
			return false, true
		}
	}
	return false, false
}

func parseInt(raw any) (int, bool) {
	switch v := raw.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), float64(int(v)) == v
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, false
		}
		var parsed int
		if _, err := fmt.Sscanf(trimmed, "%d", &parsed); err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}
