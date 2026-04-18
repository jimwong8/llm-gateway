package semantic

import (
	"context"

	"llm-gateway/gateway/internal/providers"
)

// L2Cache 定义了语义缓存接口，支持模糊命中
type L2Cache interface {
	Search(ctx context.Context, req providers.ChatCompletionRequest) (*SearchHit, error)
	Upsert(ctx context.Context, req providers.ChatCompletionRequest, resp providers.ChatCompletionResponse) error
	EnsureCollection(ctx context.Context) error
}

// 保证已有的 Qdrant 实现隐式实现了 L2Cache
var _ L2Cache = (*Cache)(nil)
