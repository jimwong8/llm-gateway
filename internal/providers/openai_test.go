package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIProvider_DoesNotLeakInternalRoutingFields(t *testing.T) {
	var received ChatCompletionRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"x","object":"chat.completion","model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer srv.Close()
	p := NewNamedOpenAIProvider("test", srv.URL, "key", 10)
	_, err := p.ChatCompletion(context.Background(), ChatCompletionRequest{
		Model: "m", Messages: []ChatMessage{{Role: "user", Content: "hi"}},
		RouteMode: "auto", RouteChannel: "ch", TaskHint: "general", SessionID: "secret-session",
		UserID: "secret-user", TenantID: "secret-tenant", PreferredModel: "m", CandidateModels: []string{"m"},
	})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if received.SessionID != "" || received.UserID != "" || received.TenantID != "" || received.RouteMode != "" || received.TaskHint != "" {
		t.Fatalf("internal fields leaked upstream: %+v", received)
	}
}

func TestNIMRequestUsesModelSpecificTemplateKwargs(t *testing.T) {
	var received ChatCompletionRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"x","object":"chat.completion","model":"minimaxai/minimax-m3","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer srv.Close()
	p := NewNamedOpenAIProvider("nvidia", "https://integrate.api.nvidia.com/v1", "key", 10)
	// The test server cannot replace the NIM URL, so exercise the pure adapter.
	prepared := p.prepareNIMRequest(ChatCompletionRequest{Model: "minimaxai/minimax-m3", Messages: []ChatMessage{{Role: "developer", Content: "rules"}}})
	if prepared.Messages[0].Role != "system" {
		t.Fatalf("developer role was not normalized: %+v", prepared.Messages)
	}
	if prepared.ChatTemplateKwargs["thinking_mode"] != "adaptive" {
		t.Fatalf("unexpected MiniMax kwargs: %+v", prepared.ChatTemplateKwargs)
	}
}

func TestNonNIMSameNamedModelDoesNotGetNIMKwargs(t *testing.T) {
	p := NewNamedOpenAIProvider("bai", "https://api.b.ai/v1", "key", 10)
	prepared := p.prepareNIMRequest(ChatCompletionRequest{Model: "deepseek-v4-flash", Messages: []ChatMessage{{Role: "developer", Content: "rules"}}})
	if prepared.Messages[0].Role != "developer" {
		t.Fatalf("non-NIM developer role changed: %+v", prepared.Messages)
	}
	if prepared.ChatTemplateKwargs != nil {
		t.Fatalf("non-NIM request got NIM kwargs: %+v", prepared.ChatTemplateKwargs)
	}
}
