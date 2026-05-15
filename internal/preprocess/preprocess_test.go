package preprocess

import (
	"context"
	"strings"
	"testing"

	"llm-gateway/gateway/internal/providers"
)

func TestNoopNormalizer(t *testing.T) {
	n := NewNoopNormalizer()
	req := providers.ChatCompletionRequest{
		Model:    "GPT-4O",
		Messages: []providers.ChatMessage{{Role: "User", Content: "Hello"}},
	}

	result, meta, err := n.Apply(context.Background(), req)
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}
	if result.Model != "GPT-4O" {
		t.Errorf("NoopNormalizer should not modify model, got %q", result.Model)
	}
	if meta.Applied {
		t.Error("NoopNormalizer meta.Applied should be false")
	}
}

func TestLowRiskNormalizer(t *testing.T) {
	n := NewLowRiskNormalizer("v1")
	req := providers.ChatCompletionRequest{
		Model:          "  GPT-4O  ",
		TaskHint:       "  CODE  ",
		RouteMode:      "  MANUAL  ",
		RouteChannel:   "  ch-1  ",
		RoutePolicyKey: "  pk-1  ",
		PreferredModel: "  model-1  ",
		TenantID:       "  tenant-1  ",
		UserID:         "  user-1  ",
		SessionID:      "  session-1  ",
		RouteAbilities: []string{"  ability-1  ", "", "  ability-2  "},
		CandidateModels: []string{"  model-1  ", "", "  model-2  "},
		Messages: []providers.ChatMessage{
			{Role: "  User  ", Content: "  Hello   World  "},
		},
	}

	result, meta, err := n.Apply(context.Background(), req)
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	if result.Model != "gpt-4o" {
		t.Errorf("Model = %q, want %q", result.Model, "gpt-4o")
	}
	if result.TaskHint != "code" {
		t.Errorf("TaskHint = %q, want %q", result.TaskHint, "code")
	}
	if result.RouteMode != "manual" {
		t.Errorf("RouteMode = %q, want %q", result.RouteMode, "manual")
	}
	if result.RouteChannel != "ch-1" {
		t.Errorf("RouteChannel = %q, want %q", result.RouteChannel, "ch-1")
	}
	if result.RoutePolicyKey != "pk-1" {
		t.Errorf("RoutePolicyKey = %q, want %q", result.RoutePolicyKey, "pk-1")
	}
	if result.PreferredModel != "model-1" {
		t.Errorf("PreferredModel = %q, want %q", result.PreferredModel, "model-1")
	}
	if result.TenantID != "tenant-1" {
		t.Errorf("TenantID = %q, want %q", result.TenantID, "tenant-1")
	}
	if result.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", result.UserID, "user-1")
	}
	if result.SessionID != "session-1" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "session-1")
	}
	if len(result.RouteAbilities) != 2 {
		t.Errorf("RouteAbilities length = %d, want 2", len(result.RouteAbilities))
	}
	if len(result.CandidateModels) != 2 {
		t.Errorf("CandidateModels length = %d, want 2", len(result.CandidateModels))
	}
	if result.Messages[0].Role != "user" {
		t.Errorf("Message role = %q, want %q", result.Messages[0].Role, "user")
	}
	if result.Messages[0].Content != "Hello World" {
		t.Errorf("Message content = %q, want %q", result.Messages[0].Content, "Hello World")
	}
	if !meta.Applied {
		t.Error("meta.Applied should be true")
	}
	if meta.TemplateVersion != "v1" {
		t.Errorf("TemplateVersion = %q, want %q", meta.TemplateVersion, "v1")
	}
	if meta.CanonicalHash == "" {
		t.Error("CanonicalHash should not be empty")
	}
}

func TestLowRiskNormalizerNoChange(t *testing.T) {
	n := NewLowRiskNormalizer("v1")
	req := providers.ChatCompletionRequest{
		Model:    "gpt-4o",
		TaskHint: "code",
		Messages: []providers.ChatMessage{{Role: "user", Content: "hello"}},
	}

	_, meta, err := n.Apply(context.Background(), req)
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}
	if meta.Applied {
		t.Error("meta.Applied should be false when no changes")
	}
}

func TestLowRiskNormalizerEmptyVersion(t *testing.T) {
	n := NewLowRiskNormalizer("")
	if n.templateVersion != "v1" {
		t.Errorf("templateVersion = %q, want %q", n.templateVersion, "v1")
	}
}

func TestNormalizeField(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  HELLO  ", "hello"},
		{"WORLD", "world"},
		{"", ""},
		{"  ", ""},
		{"MiXeD", "mixed"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeField(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeField(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNormalizeContent(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  Hello   World  ", "Hello World"},
		{"one", "one"},
		{"", ""},
		{"  ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeContent(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeContent(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNormalizeStringSlice(t *testing.T) {
	if result := normalizeStringSlice(nil); result != nil {
		t.Errorf("normalizeStringSlice(nil) = %v, want nil", result)
	}
	if result := normalizeStringSlice([]string{}); result != nil {
		t.Errorf("normalizeStringSlice([]) = %v, want nil", result)
	}
	result := normalizeStringSlice([]string{"  A  ", "", "  B  "})
	if len(result) != 2 {
		t.Errorf("normalizeStringSlice() length = %d, want 2", len(result))
	}
	if result[0] != "a" || result[1] != "b" {
		t.Errorf("normalizeStringSlice() = %v, want [a b]", result)
	}
}

func TestNoopSummarizer(t *testing.T) {
	s := NewNoopSummarizer()
	req := providers.ChatCompletionRequest{
		Messages: []providers.ChatMessage{{Role: "user", Content: "hello"}},
	}

	result, meta, err := s.Apply(context.Background(), req)
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}
	if meta.Applied {
		t.Error("NoopSummarizer meta.Applied should be false")
	}
	if len(result.Messages) != 1 {
		t.Errorf("NoopSummarizer should not modify messages, got %d", len(result.Messages))
	}
}

func TestPlaceholderSummarizerBelowTrigger(t *testing.T) {
	s := NewPlaceholderSummarizer(5, 3)
	req := providers.ChatCompletionRequest{
		Messages: []providers.ChatMessage{{Role: "user", Content: "hello"}},
	}

	_, meta, err := s.Apply(context.Background(), req)
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}
	if meta.Applied {
		t.Error("PlaceholderSummarizer should not apply below trigger")
	}
}

func TestPlaceholderSummarizerDefaultParams(t *testing.T) {
	s := NewPlaceholderSummarizer(0, 0)
	if s.triggerMessages != 20 {
		t.Errorf("triggerMessages = %d, want 20", s.triggerMessages)
	}
	if s.maxRecentTurns != 6 {
		t.Errorf("maxRecentTurns = %d, want 6", s.maxRecentTurns)
	}
}

func TestEstimateMessageTokens(t *testing.T) {
	tests := []struct {
		name     string
		messages []providers.ChatMessage
		expected int
	}{
		{"nil", nil, 0},
		{"empty", []providers.ChatMessage{}, 0},
		{"short", []providers.ChatMessage{{Content: "hello"}}, 5},
		{"two", []providers.ChatMessage{{Content: "hello"}, {Content: "world"}}, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateMessageTokens(tt.messages)
			if got != tt.expected {
				t.Errorf("estimateMessageTokens() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestNoopClassifier(t *testing.T) {
	c := NewNoopClassifier()
	meta, err := c.Apply(context.Background(), providers.ChatCompletionRequest{})
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}
	if meta.Applied {
		t.Error("NoopClassifier meta.Applied should be false")
	}
}

func TestHeuristicClassifier(t *testing.T) {
	c := NewHeuristicClassifier()

	tests := []struct {
		name         string
		content      string
		expectedTask string
	}{
		{"summarization", "please summarize this text", "summarization"},
		{"translation", "translate to chinese", "translation"},
		{"extraction", "extract json data", "extraction"},
		{"coding", "write a golang function", "coding"},
		{"design", "design a system architecture", "design"},
		{"reasoning", "why does this happen", "reasoning"},
		{"qa short", "hello", "qa"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := providers.ChatCompletionRequest{
				Messages: []providers.ChatMessage{{Role: "user", Content: tt.content}},
			}
			meta, err := c.Apply(context.Background(), req)
			if err != nil {
				t.Fatalf("Apply() error: %v", err)
			}
			if meta.TaskHint != tt.expectedTask {
				t.Errorf("TaskHint = %q, want %q", meta.TaskHint, tt.expectedTask)
			}
		})
	}
}

func TestHeuristicClassifierEmptyMessages(t *testing.T) {
	c := NewHeuristicClassifier()
	req := providers.ChatCompletionRequest{Messages: []providers.ChatMessage{}}
	meta, err := c.Apply(context.Background(), req)
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}
	if meta.Applied {
		t.Error("HeuristicClassifier should not apply for empty messages")
	}
}

func TestHeuristicClassifierComplexity(t *testing.T) {
	c := NewHeuristicClassifier()

	tests := []struct {
		name               string
		content            string
		expectedComplexity string
	}{
		{"simple short", "hi", "simple"},
		{"medium code", "write a function that sorts a list of integers and handles edge cases properly with tests", "medium"},
		{"complex design", "design a distributed system", "complex"},
		{"complex reasoning", "analyze the root cause", "complex"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := providers.ChatCompletionRequest{
				Messages: []providers.ChatMessage{{Role: "user", Content: tt.content}},
			}
			meta, err := c.Apply(context.Background(), req)
			if err != nil {
				t.Fatalf("Apply() error: %v", err)
			}
			if meta.Complexity != tt.expectedComplexity {
				t.Errorf("Complexity = %q, want %q", meta.Complexity, tt.expectedComplexity)
			}
		})
	}
}

func TestContainsAny(t *testing.T) {
	if !containsAny("hello world", "hello") {
		t.Error("containsAny should find substring")
	}
	if containsAny("hello world", "xyz") {
		t.Error("containsAny should not find missing substring")
	}
	if containsAny("HELLO WORLD", "hello") {
		t.Error("containsAny should be case-sensitive")
	}
}

func TestDecodeClassification(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		expectOk   bool
		expectTask string
	}{
		{
			name:       "valid",
			content:    `{"task_hint": "code", "complexity": "medium", "confidence": 0.8}`,
			expectOk:   true,
			expectTask: "code",
		},
		{
			name:     "invalid json",
			content:  "not json",
			expectOk: false,
		},
		{
			name:     "empty task_hint",
			content:  `{"task_hint": "", "complexity": "medium", "confidence": 0.8}`,
			expectOk: false,
		},
		{
			name:     "invalid complexity",
			content:  `{"task_hint": "code", "complexity": "invalid", "confidence": 0.8}`,
			expectOk: false,
		},
		{
			name:       "confidence out of range",
			content:    `{"task_hint": "code", "complexity": "medium", "confidence": 1.5}`,
			expectOk:   true,
			expectTask: "code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, ok := decodeClassification(tt.content)
			if ok != tt.expectOk {
				t.Errorf("decodeClassification() ok = %v, want %v", ok, tt.expectOk)
			}
			if ok && meta.TaskHint != tt.expectTask {
				t.Errorf("TaskHint = %q, want %q", meta.TaskHint, tt.expectTask)
			}
		})
	}
}

func TestDecodeClassificationConfidenceClamp(t *testing.T) {
	meta, ok := decodeClassification(`{"task_hint": "code", "complexity": "medium", "confidence": 2.0}`)
	if !ok {
		t.Fatal("decodeClassification() should succeed")
	}
	if meta.Confidence != 1.0 {
		t.Errorf("Confidence = %f, want 1.0", meta.Confidence)
	}

	meta, ok = decodeClassification(`{"task_hint": "code", "complexity": "medium", "confidence": -1.0}`)
	if !ok {
		t.Fatal("decodeClassification() should succeed")
	}
	if meta.Confidence != 0 {
		t.Errorf("Confidence = %f, want 0", meta.Confidence)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.NormalizeEnabled {
		t.Error("DefaultConfig NormalizeEnabled should be false")
	}
	if cfg.SummaryEnabled {
		t.Error("DefaultConfig SummaryEnabled should be false")
	}
	if cfg.ClassificationEnabled {
		t.Error("DefaultConfig ClassificationEnabled should be false")
	}
	if cfg.SummaryTriggerMessages != 20 {
		t.Errorf("SummaryTriggerMessages = %d, want 20", cfg.SummaryTriggerMessages)
	}
	if cfg.SummaryMaxRecentTurns != 6 {
		t.Errorf("SummaryMaxRecentTurns = %d, want 6", cfg.SummaryMaxRecentTurns)
	}
}

func TestConfigStore(t *testing.T) {
	store := NewConfigStore(DefaultConfig())
	cfg := store.Get()
	if cfg.SummaryTriggerMessages != 20 {
		t.Errorf("ConfigStore.Get() SummaryTriggerMessages = %d, want 20", cfg.SummaryTriggerMessages)
	}

	store.Set(Config{SummaryTriggerMessages: 50})
	cfg = store.Get()
	if cfg.SummaryTriggerMessages != 50 {
		t.Errorf("ConfigStore.Set/Get() SummaryTriggerMessages = %d, want 50", cfg.SummaryTriggerMessages)
	}
}

func TestConfigStoreNil(t *testing.T) {
	var store *ConfigStore
	cfg := store.Get()
	if cfg.SummaryTriggerMessages != 20 {
		t.Errorf("nil ConfigStore.Get() should return default, got SummaryTriggerMessages = %d", cfg.SummaryTriggerMessages)
	}
	store.Set(Config{SummaryTriggerMessages: 50})
}

func TestNewDefaultPipelineFromConfig(t *testing.T) {
	cfg := Config{}
	p := NewDefaultPipelineFromConfig(cfg, nil)
	if p == nil {
		t.Fatal("NewDefaultPipelineFromConfig() returned nil")
	}

	cfg = Config{
		NormalizeEnabled:      true,
		SummaryEnabled:        true,
		ClassificationEnabled: true,
		SummaryTriggerMessages: 10,
		SummaryMaxRecentTurns:  4,
		SummaryModel:          "test-model",
		ClassifierModel:       "test-model",
	}
	mock := providers.NewMockProvider("mock", "test-model")
	p = NewDefaultPipelineFromConfig(cfg, mock)
	if p == nil {
		t.Fatal("NewDefaultPipelineFromConfig() returned nil")
	}
}

func TestDefaultPipelineRun(t *testing.T) {
	p := NewNoopPipeline()
	req := providers.ChatCompletionRequest{
		Model:    "gpt-4o",
		Messages: []providers.ChatMessage{{Role: "user", Content: "hello"}},
	}

	result, err := p.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if result.OriginalRequest.Model != "gpt-4o" {
		t.Errorf("OriginalRequest.Model = %q, want %q", result.OriginalRequest.Model, "gpt-4o")
	}
}

func TestDefaultPipelineRunWithNormalizer(t *testing.T) {
	n := NewLowRiskNormalizer("v1")
	p := NewDefaultPipeline(n, nil, nil)

	req := providers.ChatCompletionRequest{
		Model:    "  GPT-4O  ",
		Messages: []providers.ChatMessage{{Role: "User", Content: "  Hello   World  "}},
	}

	result, err := p.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if result.ProcessedRequest.Model != "gpt-4o" {
		t.Errorf("ProcessedRequest.Model = %q, want %q", result.ProcessedRequest.Model, "gpt-4o")
	}
	if !result.Normalize.Applied {
		t.Error("Normalize.Applied should be true")
	}
}

func TestDefaultPipelineRunWithClassifier(t *testing.T) {
	c := NewHeuristicClassifier()
	p := NewDefaultPipeline(nil, nil, c)

	req := providers.ChatCompletionRequest{
		Model:    "gpt-4o",
		Messages: []providers.ChatMessage{{Role: "user", Content: "write a golang function"}},
	}

	result, err := p.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !result.Classification.Applied {
		t.Error("Classification.Applied should be true")
	}
	if result.ProcessedRequest.TaskHint != "coding" {
		t.Errorf("TaskHint = %q, want %q", result.ProcessedRequest.TaskHint, "coding")
	}
}

func TestBuildSummaryPrompt(t *testing.T) {
	messages := []providers.ChatMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}

	prompt := buildSummaryPrompt(messages)
	if !strings.Contains(prompt, "Summarize") {
		t.Error("buildSummaryPrompt() should contain 'Summarize'")
	}
	if !strings.Contains(prompt, "user: hello") {
		t.Error("buildSummaryPrompt() should contain message content")
	}
	if !strings.Contains(prompt, "assistant: hi there") {
		t.Error("buildSummaryPrompt() should contain assistant message")
	}
}

func TestModelBackedSummarizerNilProvider(t *testing.T) {
	s := NewModelBackedSummarizer(nil, "model", 2, 1, nil)
	req := providers.ChatCompletionRequest{
		Messages: make([]providers.ChatMessage, 5),
	}

	_, meta, err := s.Apply(context.Background(), req)
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}
	if !meta.Applied {
		t.Error("Should apply via fallback")
	}
}

func TestModelBackedSummarizerEmptyModel(t *testing.T) {
	mock := providers.NewMockProvider("mock", "model")
	s := NewModelBackedSummarizer(mock, "", 2, 1, nil)
	req := providers.ChatCompletionRequest{
		Messages: make([]providers.ChatMessage, 5),
	}

	_, meta, err := s.Apply(context.Background(), req)
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}
	if !meta.Applied {
		t.Error("Should apply via fallback for empty model")
	}
}

func TestModelBackedClassifierNilProvider(t *testing.T) {
	c := NewModelBackedClassifier(nil, "model", nil)
	req := providers.ChatCompletionRequest{
		Messages: []providers.ChatMessage{{Role: "user", Content: "hello"}},
	}

	meta, err := c.Apply(context.Background(), req)
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}
	if !meta.Applied {
		t.Error("Should apply via heuristic fallback")
	}
}

func TestModelBackedClassifierEmptyModel(t *testing.T) {
	mock := providers.NewMockProvider("mock", "model")
	c := NewModelBackedClassifier(mock, "", nil)
	req := providers.ChatCompletionRequest{
		Messages: []providers.ChatMessage{{Role: "user", Content: "hello"}},
	}

	meta, err := c.Apply(context.Background(), req)
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}
	if !meta.Applied {
		t.Error("Should apply via heuristic fallback for empty model")
	}
}

func TestModelBackedClassifierEmptyMessages(t *testing.T) {
	mock := providers.NewMockProvider("mock", "model")
	c := NewModelBackedClassifier(mock, "model", nil)
	req := providers.ChatCompletionRequest{
		Messages: []providers.ChatMessage{},
	}

	meta, err := c.Apply(context.Background(), req)
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}
	if meta.Applied {
		t.Error("Should not apply for empty messages")
	}
}

func TestEqualStringSlices(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []string
		expected bool
	}{
		{"both nil", nil, nil, true},
		{"one nil", []string{"a"}, nil, false},
		{"equal", []string{"a", "b"}, []string{"a", "b"}, true},
		{"different", []string{"a"}, []string{"b"}, false},
		{"different len", []string{"a"}, []string{"a", "b"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := equalStringSlices(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("equalStringSlices() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestEqualMessages(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []providers.ChatMessage
		expected bool
	}{
		{"both nil", nil, nil, true},
		{"one nil", []providers.ChatMessage{{Role: "u"}}, nil, false},
		{"equal", []providers.ChatMessage{{Role: "u", Content: "hi"}}, []providers.ChatMessage{{Role: "u", Content: "hi"}}, true},
		{"different", []providers.ChatMessage{{Role: "u"}}, []providers.ChatMessage{{Role: "a"}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := equalMessages(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("equalMessages() = %v, want %v", got, tt.expected)
			}
		})
	}
}
