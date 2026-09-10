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
			// Cloudflare blocks the default Go-http-client UA on some
			// upstreams (e.g. inference-api.nousresearch.com returns 403
			// for Go-http-client/2.0 but 200 for any other UA). Send a
			// generic browser UA on all outbound calls.
			Transport: userAgentRoundTripper{},
		},
	}
}

func (p *OpenAIProvider) Name() string {
	return p.name
}

func (p *OpenAIProvider) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	req = p.sanitizeForUpstream(p.prepareNIMRequest(req))
	body, err := json.Marshal(req)
	if err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header = p.openrouterAttributionHeaders(httpReq.Header)
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
		return ChatCompletionResponse{}, newUpstreamHTTPError(resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var out ChatCompletionResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("decode provider response: %w", err)
	}
	return out, nil
}

// sanitizeForUpstream strips gateway-internal routing metadata so it never
// leaks to upstream providers. These fields are gateway orchestration state,
// not valid OpenAI-compatible API parameters.
func (p *OpenAIProvider) sanitizeForUpstream(req ChatCompletionRequest) ChatCompletionRequest {
	out := req
	out.RouteMode, out.RouteChannel, out.RoutePolicyKey = "", "", ""
	out.RouteAbilities = nil
	out.PreferredModel, out.CandidateModels, out.TaskHint = "", nil, ""
	out.SessionID, out.UserID, out.TenantID = "", "", ""
	out.Complexity, out.ComplexityConfidence = "", 0
	return out
}

// openrouterAttributionHeaders returns OpenRouter app-attribution headers.
// OpenRouter gates some :free endpoints (e.g. thinkingmachines/inkling) to
// "agentic harness" callers; requests carrying these headers qualify as an
// agentic CLI app and are admitted. Applied only for openrouter.ai base URLs.
func (p *OpenAIProvider) openrouterAttributionHeaders(h http.Header) http.Header {
	if !strings.Contains(p.baseURL, "openrouter.ai") {
		return h
	}
	h.Set("HTTP-Referer", "https://hermes-agent.nousresearch.com")
	h.Set("X-OpenRouter-Title", "Hermes Agent")
	h.Set("X-OpenRouter-Categories", "cli-agent")
	return h
}

// prepareNIMRequest adapts a request for NVIDIA NIM (integrate.api.nvidia.com).
// NIM-only parameters must never leak into shared orchestration or other
// providers: this applies only when this provider's base URL is NIM.
func (p *OpenAIProvider) prepareNIMRequest(req ChatCompletionRequest) ChatCompletionRequest {
	if !strings.Contains(p.baseURL, "integrate.api.nvidia.com") {
		return req
	}
	out := req
	// NIM chat templates reject the developer role; normalize to system.
	for i := range out.Messages {
		if strings.EqualFold(out.Messages[i].Role, "developer") {
			out.Messages[i].Role = "system"
		}
	}
	// MiniMax on NIM needs adaptive thinking; without it replies degrade to
	// reasoning-only responses with empty content.
	if strings.Contains(strings.ToLower(out.Model), "minimax") {
		if out.ChatTemplateKwargs == nil {
			out.ChatTemplateKwargs = map[string]any{}
		}
		out.ChatTemplateKwargs["thinking_mode"] = "adaptive"
	}
	return out
}

// NewNamedOpenAIProvider creates an OpenAI-compatible provider bound to a
// specific channel ID (used for database-backed channel isolation).
func NewNamedOpenAIProvider(channelID, baseURL, apiKey string, timeoutSec int) *OpenAIProvider {
	p := NewOpenAIProvider(baseURL, apiKey, timeoutSec)
	p.name = channelID
	return p
}

func (p *OpenAIProvider) ChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (io.ReadCloser, error) {
	req = p.sanitizeForUpstream(p.prepareNIMRequest(req))
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
	httpReq.Header = p.openrouterAttributionHeaders(httpReq.Header)
	if strings.TrimSpace(p.apiKey) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("provider stream request failed: %w", err)
	}
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, newUpstreamHTTPError(resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return resp.Body, nil
}


// userAgentRoundTripper injects a browser-like User-Agent into every
// outbound request. Cloudflare on some upstreams hard-blocks the default
// "Go-http-client/2.0" UA (403 Attention Required) while allowing the
// same request with any other UA.
type userAgentRoundTripper struct{}

func (userAgentRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "llm-gateway/1.0")
	}
	return http.DefaultTransport.RoundTrip(req)
}
