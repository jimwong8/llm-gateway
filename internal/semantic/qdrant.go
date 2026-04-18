package semantic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"

	"llm-gateway/gateway/internal/providers"
)

type Cache struct {
	baseURL    string
	apiKey     string
	collection string
	vectorSize int
	threshold  float64
	client     *http.Client
}

type SearchHit struct {
	Score     float64
	Response  providers.ChatCompletionResponse
	Prompt    string
	Model     string
	TenantID  string
	UserID    string
	SessionID string
}

func New(baseURL, apiKey, collection string, vectorSize int, threshold float64) *Cache {
	if vectorSize <= 0 {
		vectorSize = 64
	}
	if threshold <= 0 {
		threshold = 0.85
	}
	return &Cache{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, collection: collection, vectorSize: vectorSize, threshold: threshold, client: &http.Client{Timeout: 10 * time.Second}}
}

func (c *Cache) EnsureCollection(ctx context.Context) error {
	body := map[string]any{"vectors": map[string]any{"size": c.vectorSize, "distance": "Cosine"}}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/collections/"+c.collection, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(c.apiKey) != "" {
		req.Header.Set("api-key", c.apiKey)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		text := strings.TrimSpace(string(data))
		if strings.Contains(strings.ToLower(text), "already exists") {
			return nil
		}
		return fmt.Errorf("ensure collection failed: %s", text)
	}
	return nil
}

func (c *Cache) Search(ctx context.Context, reqPayload providers.ChatCompletionRequest) (*SearchHit, error) {
	prompt := flattenPrompt(reqPayload)
	vector := embed(prompt, c.vectorSize)
	body := map[string]any{"vector": vector, "limit": 1, "with_payload": true, "filter": buildFilter(reqPayload)}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/collections/"+c.collection+"/points/search", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(c.apiKey) != "" {
		req.Header.Set("api-key", c.apiKey)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("semantic search failed: %s", strings.TrimSpace(string(data)))
	}

	var out struct {
		Result []struct {
			Score   float64        `json:"score"`
			Payload map[string]any `json:"payload"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if len(out.Result) == 0 || out.Result[0].Score < c.threshold {
		return nil, nil
	}

	payload := out.Result[0].Payload
	respJSON, _ := json.Marshal(payload["response"])
	var completion providers.ChatCompletionResponse
	_ = json.Unmarshal(respJSON, &completion)
	return &SearchHit{Score: out.Result[0].Score, Response: completion, Prompt: toString(payload["prompt"]), Model: toString(payload["model"]), TenantID: toString(payload["tenant_id"]), UserID: toString(payload["user_id"]), SessionID: toString(payload["session_id"])}, nil
}

func (c *Cache) Upsert(ctx context.Context, reqPayload providers.ChatCompletionRequest, respPayload providers.ChatCompletionResponse) error {
	prompt := flattenPrompt(reqPayload)
	vector := embed(prompt, c.vectorSize)
	id := pointID(reqPayload.TenantID + "|" + reqPayload.UserID + "|" + reqPayload.SessionID + "|" + prompt + "|" + reqPayload.Model)
	body := map[string]any{"points": []map[string]any{{"id": id, "vector": vector, "payload": map[string]any{"tenant_id": reqPayload.TenantID, "user_id": reqPayload.UserID, "session_id": reqPayload.SessionID, "prompt": prompt, "model": reqPayload.Model, "response": respPayload, "created_at": time.Now().UTC().Format(time.RFC3339)}}}}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/collections/"+c.collection+"/points", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(c.apiKey) != "" {
		req.Header.Set("api-key", c.apiKey)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("semantic upsert failed: %s", strings.TrimSpace(string(data)))
	}
	return nil
}

func buildFilter(req providers.ChatCompletionRequest) map[string]any {
	must := []map[string]any{}
	must = append(must, matchField("tenant_id", req.TenantID))
	must = append(must, matchField("user_id", req.UserID))
	if strings.TrimSpace(req.SessionID) != "" {
		must = append(must, matchField("session_id", req.SessionID))
	}
	return map[string]any{"must": must}
}

func matchField(key, value string) map[string]any {
	return map[string]any{"key": key, "match": map[string]any{"value": value}}
}

func flattenPrompt(req providers.ChatCompletionRequest) string {
	parts := make([]string, 0, len(req.Messages)+4)
	if req.TaskHint != "" {
		parts = append(parts, "task:"+strings.ToLower(strings.TrimSpace(req.TaskHint)))
	}
	if req.TenantID != "" {
		parts = append(parts, "tenant:"+strings.ToLower(strings.TrimSpace(req.TenantID)))
	}
	if req.UserID != "" {
		parts = append(parts, "user:"+strings.ToLower(strings.TrimSpace(req.UserID)))
	}
	if req.SessionID != "" {
		parts = append(parts, "session:"+strings.ToLower(strings.TrimSpace(req.SessionID)))
	}
	for _, msg := range req.Messages {
		parts = append(parts, strings.ToLower(strings.TrimSpace(msg.Role))+":"+normalize(msg.Content))
	}
	return strings.Join(parts, " | ")
}

var wordRe = regexp.MustCompile(`[\p{L}\p{N}_]+`)

func embed(text string, size int) []float64 {
	vec := make([]float64, size)
	tokens := wordRe.FindAllString(strings.ToLower(text), -1)
	if len(tokens) == 0 {
		return vec
	}
	for _, token := range tokens {
		h := fnv.New64a()
		_, _ = h.Write([]byte(token))
		idx := int(h.Sum64() % uint64(size))
		vec[idx] += 1
		if len(token) > 3 {
			prefix := token[:3]
			h2 := fnv.New64a()
			_, _ = h2.Write([]byte(prefix))
			idx2 := int(h2.Sum64() % uint64(size))
			vec[idx2] += 0.5
		}
	}
	var norm float64
	for _, v := range vec {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		return vec
	}
	for i := range vec {
		vec[i] = vec[i] / norm
	}
	return vec
}

func normalize(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}
func pointID(s string) uint64 { h := fnv.New64a(); _, _ = h.Write([]byte(s)); return h.Sum64() }
func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
