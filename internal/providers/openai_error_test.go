package providers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIProvider_ReturnsHTTPStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte("rate limited"))
	}))
	defer srv.Close()

	p := NewOpenAIProvider(srv.URL, "test-key", 5)
	_, err := p.ChatCompletion(context.Background(), ChatCompletionRequest{
		Model:    "gpt-4o-mini",
		Messages: []ChatMessage{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	var statusErr interface{ HTTPStatusCode() int }
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected error to implement HTTPStatusError interface, got %T", err)
	}
	if statusErr.HTTPStatusCode() != 429 {
		t.Fatalf("expected status 429, got %d", statusErr.HTTPStatusCode())
	}
}
