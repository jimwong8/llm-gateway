package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// EmbedRequest 嵌入请求
type EmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// EmbedResponse 嵌入响应
type EmbedResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// EmbeddingClient 嵌入服务客户端
type EmbeddingClient struct {
	baseURL    string
	apiKey     string
	model      string
	dimensions int
	client     *http.Client
}

// NewEmbeddingClient 创建嵌入客户端
func NewEmbeddingClient(baseURL, apiKey, model string, dimensions int) *EmbeddingClient {
	if dimensions <= 0 {
		dimensions = 1024 // bge-m3 默认 1024 维
	}
	return &EmbeddingClient{
		baseURL:    baseURL,
		apiKey:     apiKey,
		model:      model,
		dimensions: dimensions,
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

// Embed 生成单个文本的嵌入向量
func (e *EmbeddingClient) Embed(ctx context.Context, text string) ([]float64, error) {
	resp, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(resp) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}
	return resp[0], nil
}

// EmbedBatch 批量生成嵌入向量
func (e *EmbeddingClient) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	body := EmbedRequest{
		Model: e.model,
		Input: texts,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/embeddings", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("embed API error %d: %s", resp.StatusCode, string(data))
	}

	var out EmbedResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("unmarshal embed response: %w", err)
	}

	result := make([][]float64, len(out.Data))
	for _, d := range out.Data {
		result[d.Index] = d.Embedding
	}
	return result, nil
}

// Dimensions 返回嵌入维度
func (e *EmbeddingClient) Dimensions() int {
	return e.dimensions
}
