package main

import (
	"context"
	"log/slog"
	"os"
	"net/http"
	"strings"
	"time"

	"llm-gateway/gateway/internal/admin"
	"llm-gateway/gateway/internal/audit"
	"llm-gateway/gateway/internal/billing"
	"llm-gateway/gateway/internal/cache"
	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/controlplane"
	"llm-gateway/gateway/internal/httpserver"
	"llm-gateway/gateway/internal/memory"
	"llm-gateway/gateway/internal/policy"
	"llm-gateway/gateway/internal/providers"
	"llm-gateway/gateway/internal/quota"
	"llm-gateway/gateway/internal/router"
	"llm-gateway/gateway/internal/runtime"
	"llm-gateway/gateway/internal/semantic"
)

func main() {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	cfg := config.Load()

	openaiProvider := providers.NewOpenAIProvider(cfg.OpenAIBaseURL, cfg.OpenAIAPIKey, cfg.OpenAITimeoutSec)
	xstxProvider := providers.NewXSTXProvider(cfg.XSTXBaseURL, cfg.XSTXAPIKey, cfg.XSTXTimeoutSec)
	anthropicProvider := providers.NewAnthropicProvider(cfg.AnthropicBaseURL, cfg.AnthropicAPIKey, cfg.AnthropicTimeoutSec)
	defaultMock := providers.NewMockProvider(cfg.DefaultProvider, cfg.DefaultModel)
	codeMock := providers.NewMockProvider("mock-code", "deepseek-coder")
	analysisMock := providers.NewMockProvider("mock-analysis", "claude-sonnet")
	failMock := providers.NewMockProvider("mock-fail", "fail-code")

	var fallback providers.Provider = defaultMock
	if !cfg.MockMode && strings.TrimSpace(cfg.OpenAIAPIKey) != "" {
		fallback = openaiProvider
	}

	registry := providers.NewRegistry(cfg, fallback, defaultMock, codeMock, analysisMock, failMock, openaiProvider, xstxProvider, anthropicProvider)
	redisCache := cache.NewRedis(cfg.RedisAddr, time.Duration(cfg.L1CacheTTLSeconds)*time.Second)
	modelRouter := router.New(cfg.DefaultProvider, cfg.DefaultModel)
	if err := modelRouter.BootstrapFromFile(cfg.RouterBootstrapPath); err != nil {
		slog.Warn("router bootstrap skipped", "err", err)
	}
	limiter := quota.New(cfg.RedisAddr, cfg.TenantRPM)

	var auditStore *audit.Store
	if cfg.AuditLogEnabled {
		if store, err := audit.NewStore(cfg.PostgresDSN); err != nil {
			slog.Warn("audit init failed", "err", err)
		} else {
			auditStore = store
		}
	}
	var billingStore *billing.Store
	if cfg.BillingEnabled {
		if store, err := billing.NewStore(cfg.PostgresDSN); err != nil {
			slog.Warn("billing init failed", "err", err)
		} else {
			billingStore = store
		}
	}
	adminStore, err := admin.NewStore(cfg.PostgresDSN)
	if err != nil {
		slog.Warn("admin init failed", "err", err)
	}
	policyStore, err := policy.NewStore(cfg.PostgresDSN)
	if err != nil {
		slog.Warn("policy init failed", "err", err)
	}

	var semanticCache semantic.L2Cache = nil
	if true {
		semanticCache = semantic.NewMemoryL2Cache(cfg.SemanticVectorSize, cfg.SemanticCacheThreshold)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := semanticCache.EnsureCollection(ctx); err != nil {
			slog.Warn("semantic cache init failed", "err", err)
			semanticCache = nil
		}
	}

	var memoryStore *memory.Store = nil
	if true {
		if store, err := memory.NewStore(cfg.PostgresDSN, redisCache); err != nil {
			slog.Warn("memory init failed", "err", err)
		} else {
			memoryStore = store
		}
	}

	controlPlaneAudit := audit.NewRecorder()
	runtimeBus := runtime.NewInProcessBus()
	runtimePublisher := runtime.NewPublisher()
	runtimePublisher.WithBus(runtimeBus)
	runtimeManager := runtime.NewManager()
	controlPlaneService := controlplane.NewService().WithAuditRecorder(controlPlaneAudit).WithReleasePublisher(runtimePublisher)
	runtime.SubscribeManagerApplyBridge(runtimeBus, runtimeManager, runtime.BuildModuleRuntimeApplyDispatcher(map[string]runtime.ModuleRuntimeApplier{
		"router": runtime.BuildRouterReloadApply(
			runtime.BuildRouterPayloadDrivenApplyWithResolver(modelRouter, runtimePublisher, controlPlaneService, cfg.RouterBootstrapPath),
		),
		"quota": runtime.BuildQuotaReloadApply(
			runtime.BuildQuotaPayloadDrivenApplyWithResolver(limiter, runtimePublisher, controlPlaneService),
		),
		"policy": runtime.BuildPolicyReloadApply(
			runtime.BuildPolicyPayloadDrivenApplyWithResolver(policyStore, runtimePublisher, controlPlaneService),
		),
	}))
	if err := runtime.ReplayCurrentReleasedRouterConfig(context.Background(), controlPlaneService, runtimeBus); err != nil {
		slog.Warn("router startup replay skipped", "err", err)
	}
	if err := runtime.ReplayCurrentReleasedModuleConfig(context.Background(), controlPlaneService, runtimeBus, "quota"); err != nil {
		slog.Warn("quota startup replay skipped", "err", err)
	}
	if err := runtime.ReplayCurrentReleasedModuleConfig(context.Background(), controlPlaneService, runtimeBus, "policy"); err != nil {
		slog.Warn("policy startup replay skipped", "err", err)
	}

	srv := httpserver.New(cfg, registry, redisCache, modelRouter, auditStore, semanticCache, memoryStore, billingStore, limiter, adminStore, policyStore).
		WithControlPlane(controlPlaneService, controlPlaneAudit, runtimePublisher, runtimeManager)
	slog.Info("starting", "app", cfg.AppName, "addr", cfg.Addr(), "mock", cfg.MockMode, "redis", cfg.RedisAddr, "audit", auditStore != nil, "semantic", semanticCache != nil, "memory", memoryStore != nil, "billing", billingStore != nil)
	if err := http.ListenAndServe(cfg.Addr(), srv.Handler()); err != nil {
		slog.Error("server stopped: %v", err)
	}
}
