package semantic

import (
	"context"
	"math"
	"testing"

	"llm-gateway/gateway/internal/providers"
)

func TestMemoryL2CacheUpsertAndSearch(t *testing.T) {
	cache := NewMemoryL2Cache(16, 0.8)
	ctx := context.Background()
	request := providers.ChatCompletionRequest{
		TenantID:  "tenant-a",
		UserID:    "user-a",
		SessionID: "session-a",
		Model:     "model-a",
		TaskHint:  "chat",
		Messages: []providers.ChatMessage{
			{Role: "user", Content: "Hello   world"},
		},
	}
	response := providers.ChatCompletionResponse{Model: "model-a"}
	response.Choices = []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	}{{Index: 0, FinishReason: "stop"}}
	response.Choices[0].Message.Role = "assistant"
	response.Choices[0].Message.Content = "cached"

	if err := cache.Upsert(ctx, request, response); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	hit, err := cache.Search(ctx, request)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if hit == nil {
		t.Fatal("Search() hit = nil, want cache hit")
	}
	if hit.Response.Choices[0].Message.Content != "cached" {
		t.Fatalf("hit response content = %q, want %q", hit.Response.Choices[0].Message.Content, "cached")
	}
	if hit.TenantID != "tenant-a" || hit.UserID != "user-a" || hit.SessionID != "session-a" {
		t.Fatalf("hit scope = %+v, want tenant/user/session preserved", hit)
	}
}

func TestMemoryL2CacheSearchRespectsFiltersAndUpdatesExistingPoint(t *testing.T) {
	cache := NewMemoryL2Cache(16, 0.8)
	ctx := context.Background()
	request := providers.ChatCompletionRequest{
		TenantID:  "tenant-a",
		UserID:    "user-a",
		SessionID: "session-a",
		Model:     "model-a",
		Messages: []providers.ChatMessage{{Role: "user", Content: "hello world"}},
	}
	first := providers.ChatCompletionResponse{Model: "v1"}
	second := providers.ChatCompletionResponse{Model: "v2"}

	if err := cache.Upsert(ctx, request, first); err != nil {
		t.Fatalf("first Upsert() error = %v", err)
	}
	if len(cache.points) != 1 {
		t.Fatalf("len(cache.points) = %d, want 1", len(cache.points))
	}
	if err := cache.Upsert(ctx, request, second); err != nil {
		t.Fatalf("second Upsert() error = %v", err)
	}
	if len(cache.points) != 1 {
		t.Fatalf("len(cache.points) after update = %d, want 1", len(cache.points))
	}

	hit, err := cache.Search(ctx, request)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if hit == nil || hit.Response.Model != "v2" {
		t.Fatalf("Search() hit = %+v, want updated response model v2", hit)
	}

	miss, err := cache.Search(ctx, providers.ChatCompletionRequest{
		TenantID:  "tenant-b",
		UserID:    "user-a",
		SessionID: "session-a",
		Model:     "model-a",
		Messages: []providers.ChatMessage{{Role: "user", Content: "hello world"}},
	})
	if err != nil {
		t.Fatalf("Search() with different tenant error = %v", err)
	}
	if miss != nil {
		t.Fatalf("Search() with different tenant = %+v, want nil", miss)
	}
}

func TestSemanticHelperFunctions(t *testing.T) {
	if err := NewMemoryL2Cache(0, 0).EnsureCollection(context.Background()); err != nil {
		t.Fatalf("EnsureCollection() error = %v, want nil", err)
	}

	prompt := flattenPrompt(providers.ChatCompletionRequest{
		TaskHint:  " Summarize ",
		TenantID:  "Tenant-A",
		UserID:    "User-A",
		SessionID: "Session-A",
		Messages: []providers.ChatMessage{
			{Role: "User", Content: "  Hello   WORLD  "},
			{Role: "Assistant", Content: "  Hi there "},
		},
	})
	wantPrompt := "task:summarize | tenant:tenant-a | user:user-a | session:session-a | user:hello world | assistant:hi there"
	if prompt != wantPrompt {
		t.Fatalf("flattenPrompt() = %q, want %q", prompt, wantPrompt)
	}

	vec := embed("hello world", 8)
	if len(vec) != 8 {
		t.Fatalf("len(embed()) = %d, want 8", len(vec))
	}
	var norm float64
	for _, value := range vec {
		norm += value * value
	}
	if math.Abs(norm-1) > 1e-9 {
		t.Fatalf("embed() norm = %v, want 1", norm)
	}
	zeroVec := embed("   ", 8)
	for i, value := range zeroVec {
		if value != 0 {
			t.Fatalf("zeroVec[%d] = %v, want 0", i, value)
		}
	}

	if got := cosineSimilarity([]float64{1, 0}, []float64{1, 0}); math.Abs(got-1) > 1e-9 {
		t.Fatalf("cosineSimilarity(same) = %v, want 1", got)
	}
	if got := cosineSimilarity([]float64{1, 0}, []float64{0, 1}); math.Abs(got) > 1e-9 {
		t.Fatalf("cosineSimilarity(orthogonal) = %v, want 0", got)
	}
	if got := cosineSimilarity([]float64{1}, []float64{1, 2}); got != 0 {
		t.Fatalf("cosineSimilarity(mismatched) = %v, want 0", got)
	}

	filter := buildFilter(providers.ChatCompletionRequest{TenantID: "tenant-a", UserID: "user-a", SessionID: "session-a"})
	must, ok := filter["must"].([]map[string]any)
	if !ok {
		t.Fatalf("buildFilter() must type = %T, want []map[string]any", filter["must"])
	}
	if len(must) != 3 {
		t.Fatalf("len(buildFilter().must) = %d, want 3", len(must))
	}
	if got := normalize("  Hello   WORLD "); got != "hello world" {
		t.Fatalf("normalize() = %q, want %q", got, "hello world")
	}
	if got := toString("abc"); got != "abc" {
		t.Fatalf("toString(string) = %q, want %q", got, "abc")
	}
	if got := toString(123); got != "" {
		t.Fatalf("toString(non-string) = %q, want empty", got)
	}
	if pointID("same") != pointID("same") {
		t.Fatal("pointID(same) should be deterministic")
	}
}
