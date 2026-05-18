package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"llm-gateway/gateway/internal/admin"
	"llm-gateway/gateway/internal/audit"
	"llm-gateway/gateway/internal/billing"
	"llm-gateway/gateway/internal/cache"
	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/controlplane"
	"llm-gateway/gateway/internal/governance"
	"llm-gateway/gateway/internal/health"
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
	cfg := config.Load()

	openaiProvider := providers.NewOpenAIProvider(cfg.OpenAIBaseURL, cfg.OpenAIAPIKey, cfg.OpenAITimeoutSec)
	defaultMock := providers.NewMockProvider(cfg.DefaultProvider, cfg.DefaultModel)
	codeMock := providers.NewMockProvider("mock-code", "deepseek-coder")
	analysisMock := providers.NewMockProvider("mock-analysis", "claude-sonnet")
	failMock := providers.NewMockProvider("mock-fail", "fail-code")

	var fallback providers.Provider = defaultMock
	if !cfg.MockMode && strings.TrimSpace(cfg.OpenAIAPIKey) != "" {
		fallback = openaiProvider
	}

	registry := providers.NewRegistry(cfg, fallback, defaultMock, codeMock, analysisMock, failMock, openaiProvider)
	redisCache := cache.NewRedis(cfg.RedisAddr, time.Duration(cfg.L1CacheTTLSeconds)*time.Second)
	modelRouter := router.New(cfg.DefaultProvider, cfg.DefaultModel)
	if err := modelRouter.BootstrapFromFile(cfg.RouterBootstrapPath); err != nil {
		log.Printf("router bootstrap skipped due to error: %v", err)
	}
	limiter := quota.New(cfg.RedisAddr, cfg.TenantRPM)

	var auditStore *audit.Store
	if cfg.AuditLogEnabled {
		if store, err := audit.NewStore(cfg.PostgresDSN); err != nil {
			log.Printf("audit init failed: %v", err)
		} else {
			auditStore = store
		}
	}
	var billingStore *billing.Store
	var billingService *billing.Service
	if cfg.BillingEnabled {
		if store, err := billing.NewStore(cfg.PostgresDSN); err != nil {
			log.Printf("billing init failed: %v", err)
		} else {
			billingStore = store
			pricer := billing.NewPricer()
			pricer.SetDefaultProviderPrice("openai", 0.01, 0.03)
			pricer.SetDefaultProviderPrice("anthropic", 0.015, 0.075)
			pricer.SetDefaultProviderPrice("google", 0.0025, 0.0075)
			pricer.SetDefaultProviderPrice("mock", 0.001, 0.002)
			billingService = billing.NewService(store, pricer)
			billingService.LoadPricingFromDB(context.Background())
		}
	}
	adminStore, err := admin.NewStore(cfg.PostgresDSN)
	if err != nil {
		log.Printf("admin init failed: %v", err)
	}
	policyStore, err := policy.NewStore(cfg.PostgresDSN)
	if err != nil {
		log.Printf("policy init failed: %v", err)
	}

	var semanticCache *semantic.Cache
	if cfg.SemanticCacheEnabled {
		semanticCache = semantic.New(cfg.QdrantURL, cfg.QdrantAPIKey, cfg.QdrantCollection, cfg.SemanticVectorSize, cfg.SemanticCacheThreshold)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := semanticCache.EnsureCollection(ctx); err != nil {
			log.Printf("semantic cache init failed: %v", err)
			semanticCache = nil
		}
	}

	var memoryStore *memory.Store
	if cfg.MemoryEnabled {
		if store, err := memory.NewStore(cfg.PostgresDSN, redisCache); err != nil {
			log.Printf("memory init failed: %v", err)
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
		log.Printf("router startup replay skipped due to error: %v", err)
	}
	if err := runtime.ReplayCurrentReleasedModuleConfig(context.Background(), controlPlaneService, runtimeBus, "quota"); err != nil {
		log.Printf("quota startup replay skipped due to error: %v", err)
	}
	if err := runtime.ReplayCurrentReleasedModuleConfig(context.Background(), controlPlaneService, runtimeBus, "policy"); err != nil {
		log.Printf("policy startup replay skipped due to error: %v", err)
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
			log.Printf("governance init failed: %v", err)
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

	srv := httpserver.New(cfg, registry, redisCache, modelRouter, auditStore, semanticCache, memoryStore, billingStore, limiter, adminStore, policyStore).
		WithControlPlane(controlPlaneService, controlPlaneAudit, runtimePublisher, runtimeManager)
	if billingService != nil {
		srv = srv.WithBillingService(billingService)
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
	// 创建并启动 HealthChecker
	pingers := make(map[string]health.Pinger)
	if auditStore != nil {
		pingers["audit"] = auditStore
	}
	if redisCache != nil {
		pingers["redis"] = redisCache
	}
	if memoryStore != nil {
		pingers["memory"] = memoryStore
	}
	if billingStore != nil {
		pingers["billing"] = billingStore
	}

	checkerCfg := health.CheckerConfig{
		Interval:           30 * time.Second,
		FailureThreshold:   3,
		MemoryThresholdPct: 85.0,
		GoroutineThreshold: 10000,
	}
	healthChecker := health.NewHealthChecker(cfg, registry, pingers, checkerCfg)
	healthChecker.Start()
	srv = srv.WithHealthChecker(healthChecker)

	log.Printf("starting %s on %s mock_mode=%v redis=%s audit=%v semantic=%v memory=%v billing=%v governance=%v", cfg.AppName, cfg.Addr(), cfg.MockMode, cfg.RedisAddr, auditStore != nil, semanticCache != nil, memoryStore != nil, billingStore != nil, governanceStore != nil)

	httpServer := &http.Server{
		Addr:    cfg.Addr(),
		Handler: srv.Handler(),
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
		sig := <-sigChan
		log.Printf("received signal %v, initiating graceful shutdown", sig)

		healthChecker.Stop()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown error: %v", err)
		}
	}()

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server stopped: %v", err)
	}
	log.Printf("server exited gracefully")
}
