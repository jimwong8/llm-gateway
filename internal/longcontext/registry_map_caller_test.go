package longcontext

import (
	"context"
	"testing"

	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/providers"
)

type mapCallerProvider struct{ name string }

func (p mapCallerProvider) Name() string { return p.name }
func (p mapCallerProvider) ChatCompletion(context.Context, providers.ChatCompletionRequest) (providers.ChatCompletionResponse, error) {
	return providers.ChatCompletionResponse{}, nil
}

func TestRegistryMapCallerRequiresDependencies(t *testing.T) {
	c := RegistryMapCaller{}
	if _, err := c.CallMap(context.Background(), "", Chunk{}); err == nil {
		t.Fatal("expected dependency error")
	}
}

func TestRegistryMapCallerRequiresChannel(t *testing.T) {
	p := mapCallerProvider{name: "isolated"}
	r := providers.NewRegistry(config.Config{}, p)
	c := RegistryMapCaller{Registry: r, Model: "map-model"}
	if _, err := c.CallMap(context.Background(), "missing", Chunk{}); err == nil {
		t.Fatal("expected missing channel error")
	}
}
