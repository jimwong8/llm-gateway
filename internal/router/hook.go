package router

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"
)

type Hook interface {
	Name() string
	Priority() int
	Before(ctx context.Context, req *HookRequest) error
	After(ctx context.Context, req *HookRequest, resp *HookResponse) error
}

type HookRequest struct {
	Request  *http.Request
	UserID   int64
	UserRole string
	Body     []byte
	Metadata map[string]any
}

type HookResponse struct {
	Response     *http.Response
	StatusCode   int
	Body         []byte
	ProviderName string
	Duration     time.Duration
	Error        error
	Metadata     map[string]any
}

type HookPipeline struct {
	hooks []Hook
	mu    sync.RWMutex
}

func NewHookPipeline() *HookPipeline {
	return &HookPipeline{}
}

func (p *HookPipeline) Register(hooks ...Hook) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hooks = append(p.hooks, hooks...)
	sort.Slice(p.hooks, func(i, j int) bool {
		return p.hooks[i].Priority() > p.hooks[j].Priority()
	})
}

func (p *HookPipeline) Before(ctx context.Context, req *HookRequest) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, h := range p.hooks {
		if err := h.Before(ctx, req); err != nil {
			slog.Warn("hook before failed", "hook", h.Name(), "err", err)
			return err
		}
	}
	return nil
}

func (p *HookPipeline) After(ctx context.Context, req *HookRequest, resp *HookResponse) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, h := range p.hooks {
		if err := h.After(ctx, req, resp); err != nil {
			slog.Warn("hook after failed", "hook", h.Name(), "err", err)
		}
	}
	return nil
}

type LoggingHook struct{}

func (h *LoggingHook) Name() string        { return "logging" }
func (h *LoggingHook) Priority() int       { return 100 }
func (h *LoggingHook) Before(ctx context.Context, req *HookRequest) error {
	slog.Debug("request started", "method", req.Request.Method, "path", req.Request.URL.Path, "user_id", req.UserID)
	return nil
}
func (h *LoggingHook) After(ctx context.Context, req *HookRequest, resp *HookResponse) error {
	slog.Info("request completed", "method", req.Request.Method, "path", req.Request.URL.Path, "status", resp.StatusCode, "duration_ms", resp.Duration.Milliseconds(), "provider", resp.ProviderName)
	return nil
}

type TokenCountHook struct {
	tokenCounter func(ctx context.Context, userID int64, tokens int)
}

func NewTokenCountHook(counter func(ctx context.Context, userID int64, tokens int)) *TokenCountHook {
	return &TokenCountHook{tokenCounter: counter}
}
func (h *TokenCountHook) Name() string  { return "token_count" }
func (h *TokenCountHook) Priority() int { return 80 }
func (h *TokenCountHook) Before(ctx context.Context, req *HookRequest) error {
	return nil
}
func (h *TokenCountHook) After(ctx context.Context, req *HookRequest, resp *HookResponse) error {
	if h.tokenCounter != nil && req.UserID > 0 {
		if tokens, ok := resp.Metadata["total_tokens"].(int); ok && tokens > 0 {
			h.tokenCounter(ctx, req.UserID, tokens)
		}
	}
	return nil
}
