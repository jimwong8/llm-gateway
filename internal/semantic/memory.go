package semantic

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"llm-gateway/gateway/internal/providers"
)

// MemoryL2Cache provides an in-memory L2 semantic cache with disk persistence.
type MemoryL2Cache struct {
	vectorSize  int
	threshold   float64
	points      []memoryPoint
	mu          sync.RWMutex
	embedder    EmbeddingClient
	persistPath string
	maxPoints   int
	ttl         time.Duration
}

type memoryPoint struct {
	ID        uint64                           `json:"id"`
	Vector    []float64                        `json:"vector"`
	TenantID  string                           `json:"tenant_id"`
	Prompt    string                           `json:"prompt"`
	Model     string                           `json:"model"`
	Response  providers.ChatCompletionResponse `json:"response"`
	CreatedAt time.Time                        `json:"created_at"`
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
		maxPoints:  10000,
		ttl:        24 * time.Hour,
	}
}

// SetPersistence configures disk persistence for restart recovery.
func (c *MemoryL2Cache) SetPersistence(path string) {
	c.persistPath = path
}

// SetEmbedder sets the real embedding client.
func (c *MemoryL2Cache) SetEmbedder(embedder EmbeddingClient) {
	c.embedder = embedder
	if embedder != nil {
		c.vectorSize = embedder.Dimensions()
	}
}

// EnsureCollection loads persisted data if available.
func (c *MemoryL2Cache) EnsureCollection(ctx context.Context) error {
	if c.persistPath != "" {
		if err := c.Load(); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("load cache: %w", err)
		}
	}
	return nil
}

// Save persists cache to disk atomically.
func (c *MemoryL2Cache) Save() error {
	if c.persistPath == "" {
		return nil
	}
	c.mu.RLock()
	data, err := json.Marshal(c.points)
	c.mu.RUnlock()
	if err != nil {
		return err
	}
	tmpPath := c.persistPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, c.persistPath)
}

// Load restores cache from disk.
func (c *MemoryL2Cache) Load() error {
	if c.persistPath == "" {
		return nil
	}
	data, err := os.ReadFile(c.persistPath)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return json.Unmarshal(data, &c.points)
}

func (c *MemoryL2Cache) Search(ctx context.Context, reqPayload providers.ChatCompletionRequest) (*SearchHit, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	prompt := flattenPrompt(reqPayload)
	var vector []float64
	if c.embedder != nil {
		var err error
		vector, err = c.embedder.Embed(ctx, prompt)
		if err != nil {
			vector = embed(prompt, c.vectorSize)
		}
	} else {
		vector = embed(prompt, c.vectorSize)
	}

	var bestHit *SearchHit
	var bestScore float64 = -1.0

	for _, p := range c.points {
		if reqPayload.TenantID != "" && p.TenantID != reqPayload.TenantID {
			continue
		}
		score := cosineSimilarity(vector, p.Vector)
		if score >= c.threshold && score > bestScore {
			bestScore = score
			bestHit = &SearchHit{
				Score:    score,
				Response: p.Response,
				Prompt:   p.Prompt,
				Model:    p.Model,
				TenantID: p.TenantID,
			}
		}
	}

	return bestHit, nil
}

func (c *MemoryL2Cache) Upsert(ctx context.Context, reqPayload providers.ChatCompletionRequest, respPayload providers.ChatCompletionResponse) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.points)%100 == 0 {
		c.evictExpired()
	}

	prompt := flattenPrompt(reqPayload)
	var vector []float64
	if c.embedder != nil {
		var err error
		vector, err = c.embedder.Embed(ctx, prompt)
		if err != nil {
			vector = embed(prompt, c.vectorSize)
		}
	} else {
		vector = embed(prompt, c.vectorSize)
	}
	id := pointID(reqPayload.TenantID + "|" + prompt + "|" + reqPayload.Model)

	for i, p := range c.points {
		if p.ID == id {
			c.points[i].Response = respPayload
			return nil
		}
	}

	c.points = append(c.points, memoryPoint{
		ID:        id,
		Vector:    vector,
		TenantID:  strings.TrimSpace(reqPayload.TenantID),
		Prompt:    prompt,
		Model:     strings.TrimSpace(reqPayload.Model),
		Response:  respPayload,
		CreatedAt: time.Now().UTC(),
	})

	c.evictOldest()
	return nil
}

func (c *MemoryL2Cache) evictExpired() {
	if c.ttl <= 0 {
		return
	}
	cutoff := time.Now().UTC().Add(-c.ttl)
	filtered := c.points[:0]
	for _, p := range c.points {
		if p.CreatedAt.After(cutoff) {
			filtered = append(filtered, p)
		}
	}
	c.points = filtered
}

func (c *MemoryL2Cache) evictOldest() {
	if c.maxPoints <= 0 || len(c.points) <= c.maxPoints {
		return
	}
	c.points = c.points[len(c.points)-c.maxPoints:]
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

var _ L2Cache = (*MemoryL2Cache)(nil)
