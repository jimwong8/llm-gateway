package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// EdgeFnProvider EdgeFn 大模型提供商
type EdgeFnProvider struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	name       string
}

// NewEdgeFnProvider 创建 EdgeFn Provider
func NewEdgeFnProvider(baseURL, apiKey string) *EdgeFnProvider {
	return &EdgeFnProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		name:    "edgefn",
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Name 返回提供商名称
func (p *EdgeFnProvider) Name() string {
	return p.name
}

// ChatCompletion 调用 EdgeFn API
func (p *EdgeFnProvider) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("provider request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("read provider response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return ChatCompletionResponse{}, fmt.Errorf("provider returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var out ChatCompletionResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("decode provider response: %w", err)
	}
	return out, nil
}
