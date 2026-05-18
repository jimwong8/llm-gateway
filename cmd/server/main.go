package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"strings"
	"time"

	"llm-gateway/gateway/internal/admin"
	"llm-gateway/gateway/internal/audit"
	"llm-gateway/gateway/internal/auth"
	"llm-gateway/gateway/internal/billing"
	"llm-gateway/gateway/internal/cache"
	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/controlplane"
	"llm-gateway/gateway/internal/governance"
	"llm-gateway/gateway/internal/httpserver"
	"llm-gateway/gateway/internal/memory"
	"llm-gateway/gateway/internal/policy"
	"llm-gateway/gateway/internal/providers"
	"llm-gateway/gateway/internal/quota"
	"llm-gateway/gateway/internal/router"
	"llm-gateway/gateway/internal/runtime"
	"llm-gateway/gateway/internal/semantic"
	"llm-gateway/gateway/internal/tenant"
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
	if cfg.SemanticCacheEnabled {
		if cfg.QdrantURL != "http://127.0.0.1:6333" && strings.TrimSpace(cfg.QdrantAPIKey) != "" {
			semanticCache = semantic.New(cfg.QdrantURL, cfg.QdrantAPIKey, cfg.QdrantCollection, cfg.SemanticVectorSize, cfg.SemanticCacheThreshold)
		} else {
			semanticCache = semantic.NewMemoryL2Cache(cfg.SemanticVectorSize, cfg.SemanticCacheThreshold)
		}
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

	var tenantKeyStore *tenant.Store
	if store, err := tenant.NewStore(cfg.PostgresDSN, cfg.AdminAPIKey); err != nil {
		slog.Warn("tenant key store init failed", "err", err)
	} else {
		tenantKeyStore = store
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

	var governanceStore *governance.Store
	var governanceRecommendationService *governance.RecommendationService
	var governanceApprovalService *governance.ApprovalService
	var governanceVersionService *governance.VersionService
	var governanceRolloutService *governance.RolloutService
	var governanceRolloutDashboardService *governance.RolloutDashboardService
	var governanceRollbackService *governance.RollbackService
	var governanceRollbackRecordRepo *governance.RollbackRecordRepo
	var governanceEvaluationService *governance.EvaluationService
	var governanceDriftService *governance.DriftService
	var governanceRuntimeResolver *governance.RuntimeResolver
	var governanceQueryDB *sql.DB
	if cfg.ModelGovernanceEnabled {
		if store, err := governance.NewStore(cfg.PostgresDSN); err != nil {
			slog.Warn("governance init failed", "err", err)
		} else {
			governanceStore = store
			governanceQueryDB = store.DB()
			governanceAuditRepo := governance.NewGovernanceAuditRepo(store)
			governanceAuditSvc := governance.NewGovernanceAuditService(governanceAuditRepo)
			governanceRecommendationService = governance.NewRecommendationService(governance.NewRecommendationRepo(store))
			governanceApprovalService = governance.NewApprovalService(store).WithAuditEmitter(governanceAuditRepo)
			governanceVersionService = governance.NewVersionService(store)
			governanceRuntimeResolver = governance.NewRuntimeResolver(store)
			governanceRolloutService = governance.NewRolloutService(store).WithAuditEmitter(governanceAuditRepo).WithInvalidator(governanceRuntimeResolver)
			governanceRolloutDashboardService = governance.NewRolloutDashboardService(store)
			governanceRollbackService = governance.NewRollbackService(store).WithAuditEmitter(governanceAuditRepo).WithInvalidator(governanceRuntimeResolver)
			governanceRollbackRecordRepo = governance.NewRollbackRecordRepo(store)
			governanceEvaluationService = governance.NewEvaluationService(store)
			governanceDriftService = governance.NewDriftService(store)
			_ = governanceAuditSvc
		}
	}

	var akRateLimiter *httpserver.APIKeyRateLimiter
	var akUsageStore *auth.APIKeyUsageStore
	var authStore *auth.Store
	if db, err := sql.Open("postgres", cfg.PostgresDSN); err != nil {
		slog.Warn("auth db init failed", "err", err)
	} else {
		authStore = auth.NewStore(db)
		akUsageStore = auth.NewAPIKeyUsageStore(db)
		akRateLimiter = httpserver.NewAPIKeyRateLimiter(cfg.RedisAddr, cfg.DefaultAPIKeyRPM)
	}

	srv := httpserver.New(cfg, registry, redisCache, modelRouter, auditStore, semanticCache, memoryStore, billingStore, limiter, adminStore, policyStore).
		WithControlPlane(controlPlaneService, controlPlaneAudit, runtimePublisher, runtimeManager).
		WithTenantKeys(tenantKeyStore)
	if authStore != nil && akUsageStore != nil {
		srv = srv.WithUserStore(authStore).WithAPIKeyUsageStore(akUsageStore)
	}
	if akRateLimiter != nil {
		srv = srv.WithAPIKeyRateLimiter(akRateLimiter, cfg.DefaultAPIKeyRPM)
	}
	if memoryStore != nil {
		srv = srv.WithMemoryAdminHandler(httpserver.NewMemoryAdminHandler(memoryStore))
	}
	if cfg.ModelGovernanceEnabled && governanceStore != nil {
		modelGovernanceHandler := httpserver.NewModelGovernanceHandler().
			WithRecommendationService(governanceRecommendationService).
			WithApprovalService(governanceApprovalService).
			WithVersionService(governanceVersionService).
			WithRolloutService(governanceRolloutService).
			WithRolloutDashboardService(governanceRolloutDashboardService).
			WithRollbackService(governanceRollbackService).
			WithRollbackRecordStore(governanceRollbackRecordRepo).
			WithEvaluationService(governanceEvaluationService).
			WithDriftService(governanceDriftService).
			WithQueryer(governanceQueryDB)
		modelRuntimeHandler := httpserver.NewModelRuntimeHandler().
			WithResolver(governanceRuntimeResolver).
			WithQueryer(governanceQueryDB)
		srv = srv.WithModelGovernanceHandler(modelGovernanceHandler).
			WithModelRuntimeHandler(modelRuntimeHandler)
	}

	slog.Info("starting", "app", cfg.AppName, "addr", cfg.Addr(), "mock", cfg.MockMode, "redis", cfg.RedisAddr,
		"audit", auditStore != nil, "semantic", semanticCache != nil, "memory", memoryStore != nil,
		"billing", billingStore != nil, "governance", governanceStore != nil)

	if auditStore != nil && cfg.AuditRetentionDays > 0 {
		go func() {
			ticker := time.NewTicker(24 * time.Hour)
			defer ticker.Stop()
			for range ticker.C {
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				affected, err := auditStore.DeleteOldEvents(ctx, cfg.AuditRetentionDays)
				cancel()
				if err != nil {
					slog.Warn("audit cleanup failed", "err", err)
				} else if affected > 0 {
					slog.Info("audit cleanup completed", "deleted", affected, "retention_days", cfg.AuditRetentionDays)
				}
			}
		}()
	}

	httpServer := &http.Server{
		Addr:    cfg.Addr(),
		Handler: srv.Handler(),
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
		sig := <-sigChan
		slog.Info("received signal, initiating graceful shutdown", "signal", sig)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Warn("graceful shutdown error", "err", err)
		}
	}()

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
	slog.Info("server exited gracefully")
}
