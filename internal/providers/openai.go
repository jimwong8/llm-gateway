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

type OpenAIProvider struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	name       string
}

func NewOpenAIProvider(baseURL, apiKey string, timeoutSec int) *OpenAIProvider {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if timeoutSec <= 0 {
		timeoutSec = 120
	}
	return &OpenAIProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		name:    "openai",
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
	}
}

func (p *OpenAIProvider) Name() string {
	return p.name
}

func (p *OpenAIProvider) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
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
		hdrs := map[string]string{}
		for k, v := range resp.Header {
			if len(v) > 0 {
				hdrs[k] = v[0]
			}
		}
		return ChatCompletionResponse{}, newUpstreamHTTPErrorWithHeaders(resp.StatusCode, strings.TrimSpace(string(raw)), hdrs)
	}

	var out ChatCompletionResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("decode provider response: %w", err)
	}
	return out, nil
}

// ChatCompletionStream opens an SSE stream against the upstream OpenAI-compatible
// endpoint and returns the raw response body. The gateway relays it to the caller.
func (p *OpenAIProvider) ChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (io.ReadCloser, error) {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if strings.TrimSpace(p.apiKey) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("provider request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		hdrs := map[string]string{}
		for k, v := range resp.Header {
			if len(v) > 0 {
				hdrs[k] = v[0]
			}
		}
		return nil, newUpstreamHTTPErrorWithHeaders(resp.StatusCode, strings.TrimSpace(string(raw)), hdrs)
	}

	return resp.Body, nil
}
