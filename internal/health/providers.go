package health

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/providers"
)

type ProviderStatus struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Enabled      bool   `json:"enabled"`
	Status       string `json:"status"`
	FailureCount int    `json:"failure_count,omitempty"`
	LastError    string `json:"last_error,omitempty"`
	LatencyMS    int64  `json:"latency_ms,omitempty"`
	CheckedAt    string `json:"checked_at,omitempty"`
	OpenedUntil  string `json:"opened_until,omitempty"`
	Detail       string `json:"detail,omitempty"`
}

func CheckProviders(cfg config.Config, registry *providers.Registry) []ProviderStatus {
	if registry == nil {
		return []ProviderStatus{{Name: "registry", Type: "internal", Enabled: false, Status: "error", Detail: "provider registry unavailable"}}
	}
	probeOpenAI(cfg, registry)
	snapshots := registry.HealthStatuses()
	out := make([]ProviderStatus, 0, len(snapshots))
	for _, item := range snapshots {
		out = append(out, ProviderStatus{
			Name:         item.Name,
			Type:         item.Type,
			Enabled:      item.Enabled,
			Status:       item.Status,
			FailureCount: item.FailureCount,
			LastError:    item.LastError,
			LatencyMS:    item.LatencyMS,
			CheckedAt:    formatTime(item.CheckedAt),
			OpenedUntil:  formatTime(item.OpenedUntil),
			Detail:       describe(item),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(out[i].Name))
		right := strings.ToLower(strings.TrimSpace(out[j].Name))
		if left != right {
			return left < right
		}
		return strings.ToLower(strings.TrimSpace(out[i].Type)) < strings.ToLower(strings.TrimSpace(out[j].Type))
	})
	return out
}

func SummarizeProviders(statuses []ProviderStatus) map[string]int {
	summary := map[string]int{
		"total":    len(statuses),
		"ok":       0,
		"error":    0,
		"open":     0,
		"disabled": 0,
		"unknown":  0,
	}
	for _, item := range statuses {
		key := strings.ToLower(strings.TrimSpace(item.Status))
		switch key {
		case "ok", "error", "open", "disabled", "unknown":
			summary[key]++
		default:
			summary["unknown"]++
		}
	}
	return summary
}

func probeOpenAI(cfg config.Config, registry *providers.Registry) {
	if cfg.MockMode || strings.TrimSpace(cfg.OpenAIAPIKey) == "" {
		registry.RecordProbe("openai", false, "disabled", "mock mode enabled or api key missing", 0)
		return
	}
	timeout := cfg.ProviderHealthTimeoutSec
	if timeout <= 0 {
		timeout = 5
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	started := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.OpenAIBaseURL+"/models", nil)
	if err != nil {
		registry.RecordProbe("openai", true, "error", err.Error(), time.Since(started))
		return
	}
	req.Header.Set("Authorization", "Bearer "+cfg.OpenAIAPIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		registry.RecordProbe("openai", true, "error", err.Error(), time.Since(started))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		registry.RecordProbe("openai", true, "error", resp.Status, time.Since(started))
		return
	}
	registry.RecordProbe("openai", true, "ok", "models endpoint reachable", time.Since(started))
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func describe(item providers.ProviderHealth) string {
	switch item.Status {
	case "disabled":
		if item.LastError != "" {
			return item.LastError
		}
		return "provider disabled"
	case "open":
		return "circuit open after consecutive failures"
	case "error":
		if item.LastError != "" {
			return item.LastError
		}
		return "provider request failed"
	case "ok":
		return "provider healthy"
	default:
		return "provider status unknown"
	}
}
