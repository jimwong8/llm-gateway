package providers

import "context"

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Model           string        `json:"model"`
	Messages        []ChatMessage `json:"messages"`
	RouteMode       string        `json:"route_mode,omitempty"`
	RouteChannel    string        `json:"route_channel,omitempty"`
	RouteAbilities  []string      `json:"route_abilities,omitempty"`
	RoutePolicyKey  string        `json:"route_policy_key,omitempty"`
	PreferredModel  string        `json:"preferred_model,omitempty"`
	CandidateModels []string      `json:"candidate_models,omitempty"`
	TaskHint        string        `json:"task_hint,omitempty"`
	SessionID       string        `json:"session_id,omitempty"`
	UserID          string        `json:"user_id,omitempty"`
	TenantID        string        `json:"tenant_id,omitempty"`
	MaxTokens       int           `json:"max_tokens,omitempty"`
}

type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type Provider interface {
	ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error)
	Name() string
}
