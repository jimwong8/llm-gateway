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

type AnthropicProvider struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	name       string
}

func NewAnthropicProvider(baseURL, apiKey string, timeoutSec int) *AnthropicProvider {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	if timeoutSec <= 0 {
		timeoutSec = 120
	}
	return &AnthropicProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		name:    "anthropic",
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
	}
}

func (p *AnthropicProvider) Name() string { return p.name }

// anthropicMessage is the request body for Anthropic Messages API.
type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	Messages  []anthropicMessage `json:"messages"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicResponse struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type"`
	Role       string                  `json:"role"`
	Content    []anthropicContentBlock `json:"content"`
	Model      string                  `json:"model"`
	StopReason string                  `json:"stop_reason"`
	Usage      anthropicUsage          `json:"usage"`
}

func (p *AnthropicProvider) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	// Convert OpenAI request format to Anthropic format
	anthropicMsgs := make([]anthropicMessage, 0, len(req.Messages))
	var systemPrompt string

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			systemPrompt = msg.Content
			continue
		}
		role := msg.Role
		if role == "" {
			role = "user"
		}
		anthropicMsgs = append(anthropicMsgs, anthropicMessage{Role: role, Content: msg.Content})
	}
	if len(anthropicMsgs) == 0 {
		anthropicMsgs = append(anthropicMsgs, anthropicMessage{Role: "user", Content: "Hello"})
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	body := anthropicRequest{
		Model:     req.Model,
		Messages:  anthropicMsgs,
		MaxTokens: maxTokens,
		System:    systemPrompt,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("anthropic marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(payload))
	if err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("anthropic new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("anthropic read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return ChatCompletionResponse{}, fmt.Errorf("anthropic status=%d body=%s", resp.StatusCode, string(raw))
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(raw, &anthropicResp); err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("anthropic unmarshal: %w", err)
	}

	// Convert Anthropic response to OpenAI-compatible format
	content := ""
	for _, block := range anthropicResp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	finishReason := "stop"
	if anthropicResp.StopReason == "end_turn" || anthropicResp.StopReason == "stop_sequence" {
		finishReason = "stop"
	} else if anthropicResp.StopReason == "max_tokens" {
		finishReason = "length"
	}

	var choice struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	}
	choice.Index = 0
	choice.Message.Role = "assistant"
	choice.Message.Content = content
	choice.FinishReason = finishReason

	return ChatCompletionResponse{
		ID:      anthropicResp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   anthropicResp.Model,
		Choices: []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{choice},
		Usage: struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		}{
			PromptTokens:     anthropicResp.Usage.InputTokens,
			CompletionTokens: anthropicResp.Usage.OutputTokens,
			TotalTokens:      anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		},
	}, nil
}
