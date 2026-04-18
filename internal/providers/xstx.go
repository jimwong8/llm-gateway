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

type XSTXProvider struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	name      string
}

func NewXSTXProvider(baseURL, apiKey string, timeoutSec int) *XSTXProvider {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.xstx.info/v1"
	}
	if timeoutSec <= 0 {
		timeoutSec = 120
	}
	return &XSTXProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		name:   "xstx",
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
	}
}

func (p *XSTXProvider) Name() string {
	return p.name
}

func (p *XSTXProvider) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(p.apiKey) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

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