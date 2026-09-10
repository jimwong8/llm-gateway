package preprocess

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"llm-gateway/gateway/internal/providers"
)

// SmartClassifier 智能分类器
// 使用在线大模型进行任务分类，替代启发式关键词匹配
type SmartClassifier struct {
	provider providers.Provider
	// 分类缓存
	cache *ClassifierCache
}

// ClassifierCache 分类缓存
type ClassifierCache struct {
	entries map[string]*ClassificationEntry
	maxSize int
}

// ClassificationEntry 分类缓存项
type ClassificationEntry struct {
	TaskHint   string
	Complexity string
	Confidence float64
	Tokens     int
	Model      string
	Timestamp  int64
}

// NewSmartClassifier 创建智能分类器
func NewSmartClassifier(provider providers.Provider) *SmartClassifier {
	return &SmartClassifier{
		provider: provider,
		cache: &ClassifierCache{
			entries: make(map[string]*ClassificationEntry),
			maxSize: 10000,
		},
	}
}

// Classify 智能分类
func (sc *SmartClassifier) Classify(ctx context.Context, req providers.ChatCompletionRequest) (ClassificationMeta, error) {
	// 1. 检查缓存
	cacheKey := buildCacheKey(req)
	if cached := sc.cache.Get(cacheKey); cached != nil {
		return ClassificationMeta{
			Applied:    true,
			TaskHint:   cached.TaskHint,
			Complexity: cached.Complexity,
			Confidence: cached.Confidence,
		}, nil
	}
	
	// 2. 调用大模型分类
	result, err := sc.classifyWithLLM(ctx, req)
	if err != nil {
		// 降级到启发式分类
		return sc.fallbackClassify(req), nil
	}
	
	// 3. 缓存结果
	sc.cache.Set(cacheKey, result)
	
	return ClassificationMeta{
		Applied:    true,
		TaskHint:   result.TaskHint,
		Complexity: result.Complexity,
		Confidence: result.Confidence,
	}, nil
}

// classifyWithLLM 使用大模型分类
func (sc *SmartClassifier) classifyWithLLM(ctx context.Context, req providers.ChatCompletionRequest) (*ClassificationEntry, error) {
	// 构建分类 prompt
	latestMsg := ""
	if len(req.Messages) > 0 {
		latestMsg = req.Messages[len(req.Messages)-1].Content
	}
	
	prompt := fmt.Sprintf(`Classify the following user request. Return JSON with fields:
- task_type: one of [code, analysis, qa, writing, reasoning, extraction, general]
- complexity: one of [simple, medium, complex]
- estimated_tokens: integer estimate of response length
- recommended_model: one of [cheap, medium, premium]

User request: %s

Respond with ONLY valid JSON, no explanation.`, latestMsg)
	
	resp, err := sc.provider.ChatCompletion(ctx, providers.ChatCompletionRequest{
		Model: "DeepSeek-V3.2-EXP",
		Messages: []providers.ChatMessage{
			{Role: "system", Content: "You are a JSON-only classification assistant."},
			{Role: "user", Content: prompt},
		},
		MaxTokens: 100,
	})
	if err != nil {
		return nil, err
	}
	
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty classification response")
	}
	
	// 解析 JSON 响应
	var result struct {
		TaskType        string `json:"task_type"`
		Complexity      string `json:"complexity"`
		EstimatedTokens int    `json:"estimated_tokens"`
		RecommendedModel string `json:"recommended_model"`
	}
	
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse classification response: %w", err)
	}
	
	return &ClassificationEntry{
		TaskHint:   normalizeTaskType(result.TaskType),
		Complexity: normalizeComplexity(result.Complexity),
		Confidence: 0.9,
		Tokens:     result.EstimatedTokens,
		Model:      result.RecommendedModel,
		Timestamp:  time.Now().Unix(),
	}, nil
}

// fallbackClassify 降级到启发式分类
func (sc *SmartClassifier) fallbackClassify(req providers.ChatCompletionRequest) ClassificationMeta {
	task := heuristicClassifyTask(req)
	complexity := sc.estimateComplexity(req)
	
	return ClassificationMeta{
		Applied:    true,
		TaskHint:   task,
		Complexity: complexity,
		Confidence: 0.5,
	}
}

// heuristicClassifyTask 启发式任务分类（降级方案）
func heuristicClassifyTask(req providers.ChatCompletionRequest) string {
	combined := make([]string, 0, len(req.Messages))
	for _, msg := range req.Messages {
		combined = append(combined, strings.ToLower(msg.Content))
	}
	text := strings.Join(combined, " ")
	switch {
	case strings.Contains(text, "code"), strings.Contains(text, "golang"),
		strings.Contains(text, "python"), strings.Contains(text, "function"):
		return "code"
	case strings.Contains(text, "analyze"), strings.Contains(text, "分析"):
		return "analysis"
	default:
		return "general"
	}
}

// estimateComplexity 估计复杂度
func (sc *SmartClassifier) estimateComplexity(req providers.ChatCompletionRequest) string {
	totalLen := 0
	for _, msg := range req.Messages {
		totalLen += len(msg.Content)
	}
	
	switch {
	case totalLen < 100:
		return "simple"
	case totalLen < 500:
		return "medium"
	default:
		return "complex"
	}
}

// 缓存方法
func (c *ClassifierCache) Get(key string) *ClassificationEntry {
	return c.entries[key]
}

func (c *ClassifierCache) Set(key string, entry *ClassificationEntry) {
	if len(c.entries) >= c.maxSize {
		// 简单淘汰：删除最早的 10%
		c.evictOldest(10)
	}
	c.entries[key] = entry
}

func (c *ClassifierCache) evictOldest(percent int) {
	// 简单实现：随机删除一些条目
	count := len(c.entries) * percent / 100
	for k := range c.entries {
		if count <= 0 {
			break
		}
		delete(c.entries, k)
		count--
	}
}

func buildCacheKey(req providers.ChatCompletionRequest) string {
	// 基于消息内容构建缓存 key
	var parts []string
	for _, msg := range req.Messages {
		parts = append(parts, msg.Role+":"+msg.Content)
	}
	return strings.Join(parts, "|")
}

func normalizeTaskType(task string) string {
	task = strings.ToLower(strings.TrimSpace(task))
	switch task {
	case "code", "coding", "programming":
		return "code"
	case "analysis", "analyze", "analyzing":
		return "analysis"
	case "qa", "question", "answer":
		return "qa"
	case "writing", "write", "compose":
		return "writing"
	case "reasoning", "reason", "logic":
		return "reasoning"
	case "extraction", "extract", "parse":
		return "extraction"
	default:
		return "general"
	}
}

func normalizeComplexity(c string) string {
	c = strings.ToLower(strings.TrimSpace(c))
	switch c {
	case "simple", "easy", "basic":
		return "simple"
	case "medium", "moderate", "intermediate":
		return "medium"
	case "complex", "hard", "advanced", "difficult":
		return "complex"
	default:
		return "medium"
	}
}


