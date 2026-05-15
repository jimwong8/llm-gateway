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

type DomesticProvider struct {
	baseURL      string
	apiKey       string
	defaultModel string
	httpClient   *http.Client
	name         string
}

func NewDomesticProvider(baseURL, apiKey, defaultModel string, timeoutSec int) *DomesticProvider {
	trimmedBaseURL := strings.TrimSpace(baseURL)
	if trimmedBaseURL == "" {
		trimmedBaseURL = "https://api.deepseek.com/v1"
	}
	if timeoutSec <= 0 {
		timeoutSec = 60
	}
	model := strings.TrimSpace(defaultModel)
	if model == "" {
		model = "deepseek-chat"
	}
	return &DomesticProvider{
		baseURL:      strings.TrimRight(trimmedBaseURL, "/"),
		apiKey:       apiKey,
		defaultModel: model,
		name:         "deepseek-domestic",
		httpClient: &http.Client{Timeout: time.Duration(timeoutSec) * time.Second},
	}
}

func (p *DomesticProvider) Name() string {
	return p.name
}

func (p *DomesticProvider) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	request := req
	if strings.TrimSpace(request.Model) == "" {
		request.Model = p.defaultModel
	}

	body, err := json.Marshal(request)
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
		return ChatCompletionResponse{}, fmt.Errorf("domestic provider request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("read provider response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return ChatCompletionResponse{}, fmt.Errorf("domestic provider returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var out ChatCompletionResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("decode provider response: %w", err)
	}
	return out, nil
}
