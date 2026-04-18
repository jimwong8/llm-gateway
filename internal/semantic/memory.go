package semantic

import (
	"context"
	"math"
	"strings"
	"sync"
	"time"

	"llm-gateway/gateway/internal/providers"
)

// MemoryL2Cache 提供了一个用于本地测试或无 Qdrant 依赖时的简单内存 L2 缓存
// 其内部向量检索采用极其暴力的全量遍历算余弦相似度，不适用于生产环境的大量数据。
type MemoryL2Cache struct {
	vectorSize int
	threshold  float64
	points     []memoryPoint
	mu         sync.RWMutex
}

type memoryPoint struct {
	id        uint64
	vector    []float64
	tenantID  string
	userID    string
	sessionID string
	prompt    string
	model     string
	response  providers.ChatCompletionResponse
	createdAt time.Time
}

func NewMemoryL2Cache(vectorSize int, threshold float64) *MemoryL2Cache {
	if vectorSize <= 0 {
		vectorSize = 64
	}
	if threshold <= 0 {
		threshold = 0.85
	}
	return &MemoryL2Cache{
		vectorSize: vectorSize,
		threshold:  threshold,
	}
}

func (c *MemoryL2Cache) EnsureCollection(ctx context.Context) error {
	// 内存版无需建表
	return nil
}

func (c *MemoryL2Cache) Search(ctx context.Context, reqPayload providers.ChatCompletionRequest) (*SearchHit, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	prompt := flattenPrompt(reqPayload)
	vector := embed(prompt, c.vectorSize)

	var bestHit *SearchHit
	var bestScore float64 = -1.0

	for _, p := range c.points {
		// 1. Filter match (must)
		if reqPayload.TenantID != "" && p.tenantID != reqPayload.TenantID {
			continue
		}
		if reqPayload.UserID != "" && p.userID != reqPayload.UserID {
			continue
		}
		if reqPayload.SessionID != "" && p.sessionID != reqPayload.SessionID {
			continue
		}

		// 2. Score
		score := cosineSimilarity(vector, p.vector)
		if score >= c.threshold && score > bestScore {
			bestScore = score
			bestHit = &SearchHit{
				Score:     score,
				Response:  p.response,
				Prompt:    p.prompt,
				Model:     p.model,
				TenantID:  p.tenantID,
				UserID:    p.userID,
				SessionID: p.sessionID,
			}
		}
	}

	return bestHit, nil
}

func (c *MemoryL2Cache) Upsert(ctx context.Context, reqPayload providers.ChatCompletionRequest, respPayload providers.ChatCompletionResponse) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	prompt := flattenPrompt(reqPayload)
	vector := embed(prompt, c.vectorSize)
	id := pointID(reqPayload.TenantID + "|" + reqPayload.UserID + "|" + reqPayload.SessionID + "|" + prompt + "|" + reqPayload.Model)

	// Check if exists to update, else append
	found := false
	for i, p := range c.points {
		if p.id == id {
			c.points[i].response = respPayload
			found = true
			break
		}
	}

	if !found {
		c.points = append(c.points, memoryPoint{
			id:        id,
			vector:    vector,
			tenantID:  strings.TrimSpace(reqPayload.TenantID),
			userID:    strings.TrimSpace(reqPayload.UserID),
			sessionID: strings.TrimSpace(reqPayload.SessionID),
			prompt:    prompt,
			model:     strings.TrimSpace(reqPayload.Model),
			response:  respPayload,
			createdAt: time.Now().UTC(),
		})
	}
	return nil
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0.0
	}
	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0.0
	}
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// 保证 MemoryL2Cache 实现了 L2Cache
var _ L2Cache = (*MemoryL2Cache)(nil)
