package preprocess

import (
	"context"
	"encoding/json"
	"strings"

	"llm-gateway/gateway/internal/providers"
)

type Classifier interface {
	Apply(ctx context.Context, req providers.ChatCompletionRequest) (ClassificationMeta, error)
}

type ModelBackedClassifier struct {
	provider providers.Provider
	model    string
	fallback Classifier
}

func NewModelBackedClassifier(provider providers.Provider, model string, fallback Classifier) *ModelBackedClassifier {
	if fallback == nil {
		fallback = NewHeuristicClassifier()
	}
	return &ModelBackedClassifier{
		provider: provider,
		model:    strings.TrimSpace(model),
		fallback: fallback,
	}
}

func (c *ModelBackedClassifier) Apply(ctx context.Context, req providers.ChatCompletionRequest) (ClassificationMeta, error) {
	if c.provider == nil || strings.TrimSpace(c.model) == "" || len(req.Messages) == 0 {
		return c.fallback.Apply(ctx, req)
	}
	latest := strings.TrimSpace(req.Messages[len(req.Messages)-1].Content)
	if latest == "" {
		return c.fallback.Apply(ctx, req)
	}

	prompt := "Classify the user's latest request. Return strict JSON with keys task_hint, complexity, confidence. complexity must be one of simple, medium, complex.\n\nUser request:\n" + latest
	resp, err := c.provider.ChatCompletion(ctx, providers.ChatCompletionRequest{
		Model: c.model,
		Messages: []providers.ChatMessage{{Role: "user", Content: prompt}},
	})
	if err != nil || len(resp.Choices) == 0 {
		return c.fallback.Apply(ctx, req)
	}
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	meta, ok := decodeClassification(content)
	if !ok {
		return c.fallback.Apply(ctx, req)
	}
	meta.Applied = true
	return meta, nil
}

type HeuristicClassifier struct{}

func NewHeuristicClassifier() *HeuristicClassifier {
	return &HeuristicClassifier{}
}

func (c *HeuristicClassifier) Apply(_ context.Context, req providers.ChatCompletionRequest) (ClassificationMeta, error) {
	if len(req.Messages) == 0 {
		return ClassificationMeta{}, nil
	}
	latest := strings.ToLower(strings.TrimSpace(req.Messages[len(req.Messages)-1].Content))
	if latest == "" {
		return ClassificationMeta{}, nil
	}

	meta := ClassificationMeta{Applied: true, TaskHint: "qa", Complexity: "medium", Confidence: 0.55}
	switch {
	case containsAny(latest, "summarize", "summary", "摘要", "总结", "概括"):
		meta.TaskHint = "summarization"
		meta.Complexity = "simple"
		meta.Confidence = 0.85
	case containsAny(latest, "translate", "翻译"):
		meta.TaskHint = "translation"
		meta.Complexity = "simple"
		meta.Confidence = 0.9
	case containsAny(latest, "extract", "提取", "json", "schema"):
		meta.TaskHint = "extraction"
		meta.Complexity = "medium"
		meta.Confidence = 0.72
	case containsAny(latest, "code", "bug", "golang", "python", "typescript", "函数", "代码"):
		meta.TaskHint = "coding"
		meta.Complexity = "medium"
		meta.Confidence = 0.7
	case containsAny(latest, "design", "architecture", "tradeoff", "系统设计", "架构"):
		meta.TaskHint = "design"
		meta.Complexity = "complex"
		meta.Confidence = 0.78
	case containsAny(latest, "why", "analyze", "reason", "推理", "分析"):
		meta.TaskHint = "reasoning"
		meta.Complexity = "complex"
		meta.Confidence = 0.68
	default:
		if len(latest) < 80 {
			meta.Complexity = "simple"
			meta.Confidence = 0.5
		}
	}
	return meta, nil
}

type NoopClassifier struct{}

func NewNoopClassifier() *NoopClassifier {
	return &NoopClassifier{}
}

func (c *NoopClassifier) Apply(_ context.Context, _ providers.ChatCompletionRequest) (ClassificationMeta, error) {
	return ClassificationMeta{}, nil
}

func containsAny(text string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(text, strings.ToLower(candidate)) {
			return true
		}
	}
	return false
}

func decodeClassification(content string) (ClassificationMeta, bool) {
	type payload struct {
		TaskHint   string  `json:"task_hint"`
		Complexity string  `json:"complexity"`
		Confidence float64 `json:"confidence"`
	}
	var parsed payload
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return ClassificationMeta{}, false
	}
	parsed.TaskHint = strings.TrimSpace(strings.ToLower(parsed.TaskHint))
	parsed.Complexity = strings.TrimSpace(strings.ToLower(parsed.Complexity))
	if parsed.TaskHint == "" || (parsed.Complexity != "simple" && parsed.Complexity != "medium" && parsed.Complexity != "complex") {
		return ClassificationMeta{}, false
	}
	if parsed.Confidence < 0 {
		parsed.Confidence = 0
	}
	if parsed.Confidence > 1 {
		parsed.Confidence = 1
	}
	return ClassificationMeta{TaskHint: parsed.TaskHint, Complexity: parsed.Complexity, Confidence: parsed.Confidence}, true
}
