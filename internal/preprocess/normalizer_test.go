package preprocess

import (
	"context"
	"testing"

	"llm-gateway/gateway/internal/providers"
)

func TestLowRiskNormalizerApply(t *testing.T) {
	n := NewLowRiskNormalizer("v1")
	req := providers.ChatCompletionRequest{
		Model:          " GPT-4O-MINI ",
		TaskHint:       " Summarization ",
		RouteMode:      " AUTO ",
		RouteChannel:   " PRIMARY ",
		RoutePolicyKey: " Default ",
		PreferredModel: " GPT-4O ",
		TenantID:       " Tenant-A ",
		UserID:         " User-A ",
		SessionID:      " Session-A ",
		RouteAbilities: []string{" Code ", " Analysis "},
		CandidateModels: []string{" GPT-4O ", " Claude-Sonnet "},
		Messages: []providers.ChatMessage{
			{Role: " USER ", Content: "hello   world\n\nagain"},
			{Role: " assistant ", Content: "  hi there  "},
		},
	}

	normalized, meta, err := n.Apply(context.Background(), req)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if !meta.Applied {
		t.Fatalf("expected normalization to be applied")
	}
	if meta.TemplateVersion != "v1" {
		t.Fatalf("unexpected template version: %s", meta.TemplateVersion)
	}
	if meta.CanonicalHash == "" {
		t.Fatalf("expected canonical hash")
	}
	if normalized.Model != "gpt-4o-mini" {
		t.Fatalf("unexpected model: %s", normalized.Model)
	}
	if normalized.TaskHint != "summarization" {
		t.Fatalf("unexpected task hint: %s", normalized.TaskHint)
	}
	if normalized.RouteMode != "auto" {
		t.Fatalf("unexpected route mode: %s", normalized.RouteMode)
	}
	if normalized.RouteChannel != "primary" {
		t.Fatalf("unexpected route channel: %s", normalized.RouteChannel)
	}
	if normalized.Messages[0].Role != "user" {
		t.Fatalf("unexpected normalized role: %s", normalized.Messages[0].Role)
	}
	if normalized.Messages[0].Content != "hello world again" {
		t.Fatalf("unexpected normalized content: %q", normalized.Messages[0].Content)
	}
	if len(normalized.RouteAbilities) != 2 || normalized.RouteAbilities[0] != "code" || normalized.RouteAbilities[1] != "analysis" {
		t.Fatalf("unexpected route abilities: %#v", normalized.RouteAbilities)
	}
}

func TestDefaultPipelineRunWithNoops(t *testing.T) {
	p := NewNoopPipeline()
	req := providers.ChatCompletionRequest{
		Model: "gpt-4o-mini",
		Messages: []providers.ChatMessage{
			{Role: "user", Content: "hello"},
		},
	}

	result, err := p.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.OriginalRequest.Model != req.Model || result.ProcessedRequest.Model != req.Model {
		t.Fatalf("expected noop pipeline to preserve request model")
	}
	if len(result.OriginalRequest.Messages) != 1 || len(result.ProcessedRequest.Messages) != 1 {
		t.Fatalf("expected noop pipeline to preserve messages")
	}
	if result.OriginalRequest.Messages[0] != req.Messages[0] || result.ProcessedRequest.Messages[0] != req.Messages[0] {
		t.Fatalf("expected noop pipeline to preserve message content")
	}
	if result.Normalize.Applied || result.Summary.Applied || result.Classification.Applied {
		t.Fatalf("expected noop metadata to remain empty")
	}
}

func TestNewDefaultPipelineFromConfigEnablesRequestedStages(t *testing.T) {
	cfg := DefaultConfig()
	cfg.NormalizeEnabled = true
	cfg.ClassificationEnabled = true
	cfg.ClassifierModel = "fail-model"
	provider := providers.NewMockProvider("local", "fail-model")
	pipeline := NewDefaultPipelineFromConfig(cfg, provider)
	result, err := pipeline.Run(context.Background(), providers.ChatCompletionRequest{
		Model: " GPT-4O-MINI ",
		Messages: []providers.ChatMessage{{Role: " USER ", Content: "Please   summarize   this document"}},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !result.Normalize.Applied {
		t.Fatalf("expected normalize stage to apply")
	}
	if !result.Classification.Applied {
		t.Fatalf("expected classifier stage to apply")
	}
}

func TestPlaceholderSummarizerAppliesOnLongConversation(t *testing.T) {
	s := NewPlaceholderSummarizer(4, 2)
	req := providers.ChatCompletionRequest{
		Messages: []providers.ChatMessage{
			{Role: "user", Content: "one"},
			{Role: "assistant", Content: "two"},
			{Role: "user", Content: "three"},
			{Role: "assistant", Content: "four"},
			{Role: "user", Content: "five"},
		},
	}

	_, meta, err := s.Apply(context.Background(), req)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if !meta.Applied {
		t.Fatalf("expected summary placeholder to apply")
	}
	if meta.OriginalTokenEstimate == 0 || meta.ReducedTokenEstimate == 0 {
		t.Fatalf("expected token estimates to be populated")
	}
	if meta.CompressionRatio <= 0 || meta.CompressionRatio >= 1 {
		t.Fatalf("unexpected compression ratio: %f", meta.CompressionRatio)
	}
}

func TestModelBackedSummarizerFallsBackOnProviderFailure(t *testing.T) {
	provider := providers.NewMockProvider("local", "fail-model")
	s := NewModelBackedSummarizer(provider, "fail-model", 2, 1, NewPlaceholderSummarizer(2, 1))
	req := providers.ChatCompletionRequest{Messages: []providers.ChatMessage{
		{Role: "user", Content: "one"},
		{Role: "assistant", Content: "two"},
		{Role: "user", Content: "three"},
	}}
	_, meta, err := s.Apply(context.Background(), req)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if !meta.Applied || meta.SummaryText != "placeholder-summary" {
		t.Fatalf("expected placeholder fallback summary, got %#v", meta)
	}
}

func TestHeuristicClassifierAssignsExpectedTaskHint(t *testing.T) {
	c := NewHeuristicClassifier()
	meta, err := c.Apply(context.Background(), providers.ChatCompletionRequest{
		Messages: []providers.ChatMessage{{Role: "user", Content: "Please summarize this document"}},
	})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if !meta.Applied {
		t.Fatalf("expected classifier to apply")
	}
	if meta.TaskHint != "summarization" {
		t.Fatalf("unexpected task hint: %s", meta.TaskHint)
	}
	if meta.Complexity != "simple" {
		t.Fatalf("unexpected complexity: %s", meta.Complexity)
	}
}

func TestModelBackedClassifierFallsBackToHeuristic(t *testing.T) {
	provider := providers.NewMockProvider("local", "fail-model")
	c := NewModelBackedClassifier(provider, "fail-model", NewHeuristicClassifier())
	meta, err := c.Apply(context.Background(), providers.ChatCompletionRequest{
		Messages: []providers.ChatMessage{{Role: "user", Content: "Please summarize this document"}},
	})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if !meta.Applied || meta.TaskHint != "summarization" {
		t.Fatalf("expected heuristic fallback classification, got %#v", meta)
	}
}
