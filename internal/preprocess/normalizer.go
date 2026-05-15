package preprocess

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"llm-gateway/gateway/internal/providers"
)

type Normalizer interface {
	Apply(ctx context.Context, req providers.ChatCompletionRequest) (providers.ChatCompletionRequest, NormalizeMeta, error)
}

type LowRiskNormalizer struct {
	templateVersion string
}

func NewLowRiskNormalizer(templateVersion string) *LowRiskNormalizer {
	version := strings.TrimSpace(templateVersion)
	if version == "" {
		version = "v1"
	}
	return &LowRiskNormalizer{templateVersion: version}
}

func (n *LowRiskNormalizer) Apply(_ context.Context, req providers.ChatCompletionRequest) (providers.ChatCompletionRequest, NormalizeMeta, error) {
	normalized := req
	normalized.Model = normalizeField(req.Model)
	normalized.TaskHint = normalizeField(req.TaskHint)
	normalized.RouteMode = normalizeField(req.RouteMode)
	normalized.RouteChannel = normalizeField(req.RouteChannel)
	normalized.RoutePolicyKey = normalizeField(req.RoutePolicyKey)
	normalized.PreferredModel = normalizeField(req.PreferredModel)
	normalized.TenantID = normalizeField(req.TenantID)
	normalized.UserID = normalizeField(req.UserID)
	normalized.SessionID = normalizeField(req.SessionID)

	normalized.RouteAbilities = normalizeStringSlice(req.RouteAbilities)
	normalized.CandidateModels = normalizeStringSlice(req.CandidateModels)
	normalized.Messages = normalizeMessages(req.Messages)

	hash, err := canonicalHash(normalized, n.templateVersion)
	if err != nil {
		return providers.ChatCompletionRequest{}, NormalizeMeta{}, err
	}

	applied := requestChanged(req, normalized)
	return normalized, NormalizeMeta{
		Applied:         applied,
		CanonicalHash:   hash,
		TemplateVersion: n.templateVersion,
	}, nil
}

type NoopNormalizer struct{}

func NewNoopNormalizer() *NoopNormalizer {
	return &NoopNormalizer{}
}

func (n *NoopNormalizer) Apply(_ context.Context, req providers.ChatCompletionRequest) (providers.ChatCompletionRequest, NormalizeMeta, error) {
	return req, NormalizeMeta{}, nil
}

func normalizeMessages(messages []providers.ChatMessage) []providers.ChatMessage {
	out := make([]providers.ChatMessage, 0, len(messages))
	for _, msg := range messages {
		out = append(out, providers.ChatMessage{
			Role:    normalizeField(msg.Role),
			Content: normalizeContent(msg.Content),
		})
	}
	return out
}

func normalizeContent(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func normalizeField(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func normalizeStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, item := range values {
		normalized := normalizeField(item)
		if normalized == "" {
			continue
		}
		out = append(out, normalized)
	}
	return out
}

func canonicalHash(req providers.ChatCompletionRequest, version string) (string, error) {
	payload := struct {
		TemplateVersion string                  `json:"template_version"`
		Model           string                  `json:"model"`
		RouteMode       string                  `json:"route_mode,omitempty"`
		RouteChannel    string                  `json:"route_channel,omitempty"`
		RoutePolicyKey  string                  `json:"route_policy_key,omitempty"`
		PreferredModel  string                  `json:"preferred_model,omitempty"`
		CandidateModels []string                `json:"candidate_models,omitempty"`
		RouteAbilities  []string                `json:"route_abilities,omitempty"`
		TaskHint        string                  `json:"task_hint,omitempty"`
		TenantID        string                  `json:"tenant_id,omitempty"`
		UserID          string                  `json:"user_id,omitempty"`
		SessionID       string                  `json:"session_id,omitempty"`
		Messages        []providers.ChatMessage `json:"messages"`
	}{
		TemplateVersion: version,
		Model:           req.Model,
		RouteMode:       req.RouteMode,
		RouteChannel:    req.RouteChannel,
		RoutePolicyKey:  req.RoutePolicyKey,
		PreferredModel:  req.PreferredModel,
		CandidateModels: req.CandidateModels,
		RouteAbilities:  req.RouteAbilities,
		TaskHint:        req.TaskHint,
		TenantID:        req.TenantID,
		UserID:          req.UserID,
		SessionID:       req.SessionID,
		Messages:        req.Messages,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func requestChanged(original, normalized providers.ChatCompletionRequest) bool {
	return original.Model != normalized.Model ||
		original.TaskHint != normalized.TaskHint ||
		original.RouteMode != normalized.RouteMode ||
		original.RouteChannel != normalized.RouteChannel ||
		original.RoutePolicyKey != normalized.RoutePolicyKey ||
		original.PreferredModel != normalized.PreferredModel ||
		original.TenantID != normalized.TenantID ||
		original.UserID != normalized.UserID ||
		original.SessionID != normalized.SessionID ||
		!equalStringSlices(original.RouteAbilities, normalized.RouteAbilities) ||
		!equalStringSlices(original.CandidateModels, normalized.CandidateModels) ||
		!equalMessages(original.Messages, normalized.Messages)
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalMessages(a, b []providers.ChatMessage) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
