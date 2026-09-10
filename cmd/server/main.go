package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"llm-gateway/gateway/internal/admin"
	"llm-gateway/gateway/internal/adminconfig"
	"llm-gateway/gateway/internal/audit"
	"llm-gateway/gateway/internal/auth"
	"llm-gateway/gateway/internal/billing"
	"llm-gateway/gateway/internal/broadcast"
	"llm-gateway/gateway/internal/cache"
	"llm-gateway/gateway/internal/chat"
	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/controlplane"
	"llm-gateway/gateway/internal/governance"
	"llm-gateway/gateway/internal/httpserver"
	"llm-gateway/gateway/internal/limits"
	"llm-gateway/gateway/internal/longcontext"
	"llm-gateway/gateway/internal/memory"
	"llm-gateway/gateway/internal/policy"
	"llm-gateway/gateway/internal/providers"
	"llm-gateway/gateway/internal/quota"
	"llm-gateway/gateway/internal/router"
	"llm-gateway/gateway/internal/runtime"
	"llm-gateway/gateway/internal/semantic"
	"llm-gateway/gateway/internal/tenant"
)

func semanticCachePeriodicSave(cache semantic.L2Cache, stop <-chan struct{}) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if mc, ok := cache.(*semantic.MemoryL2Cache); ok {
				if err := mc.Save(); err != nil {
					fmt.Printf("semantic cache save error: %v\n", err)
				}
			}
		case <-stop:
			if mc, ok := cache.(*semantic.MemoryL2Cache); ok {
				mc.Save()
			}
			return
		}
	}
}

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
	defaultPool := make([]string, 0)
	for _, model := range strings.Split(cfg.DefaultModelPool, ",") {
		model = strings.TrimSpace(model)
		if model != "" {
			modelRouter.RegisterProductionModel(model, "openai", "general", "configured default model pool")
			defaultPool = append(defaultPool, model)
		}
	}
	modelRouter.SetDefaultModelPool(defaultPool)
	// AUTO pool is homogeneous 1M per product design; env may override.
	if len(cfg.ModelContextWindows) == 0 {
		windows := make(map[string]int, len(defaultPool))
		for _, m := range defaultPool {
			windows[m] = 1048576
		}
		modelRouter.SetModelContextWindows(windows)
		slog.Info("default context windows applied to AUTO pool", "count", len(windows))
	} else {
		modelRouter.SetModelContextWindows(cfg.ModelContextWindows)
	}
	// Overlay measured per-model safe windows from the database. For AUTO we
	// use the smallest measured safe window across channels so every key in the
	// pool stays within its own proven capacity.
	if limitDB, err := sql.Open("postgres", cfg.PostgresDSN); err == nil {
		if limitStore, err := limits.NewStore(limitDB); err == nil {
			if profiles, err := limitStore.ListContextProfiles(context.Background()); err == nil {
				measured := map[string]int{}
				rejectedBadWindows := map[string]int{}
				for _, p := range profiles {
					modelKey := strings.ToLower(strings.TrimSpace(p.Model))
					if p.SafeWindow <= 0 {
						continue
					}
					// Sanity guard: reject absurdly small safe windows for long-context models.
					// A bad probe that writes 5333 for a 1M model causes spurious 413s (e.g. glm-5.2).
					if p.ContextWindow >= 900000 && p.SafeWindow < 50000 {
						rejectedBadWindows[modelKey] = p.SafeWindow
						slog.Warn("rejected implausibly small safe context window",
							"model", p.Model,
							"safe_window", p.SafeWindow,
							"context_window", p.ContextWindow,
							"confidence", p.Confidence)
						continue
					}
					if measured[modelKey] == 0 || p.SafeWindow < measured[modelKey] {
						measured[modelKey] = p.SafeWindow
					}
				}
				if len(rejectedBadWindows) > 0 {
					slog.Info("rejected bad context windows on load", "count", len(rejectedBadWindows), "rejected", rejectedBadWindows)
				}
				for model, window := range measured {
					modelRouter.SetModelSafeContextWindows(map[string]int{model: window})
				}
				slog.Info("loaded measured context windows", "count", len(measured), "windows", measured)
			}
			// Restore persisted runtime scores so AUTO routing keeps the learned
			// health/latency ranking across restarts.
			if scores, err := limitStore.ListScores(context.Background()); err == nil {
				best := map[string]limits.Score{}
				for _, s := range scores {
					key := strings.ToLower(strings.TrimSpace(s.Model))
					if prev, ok := best[key]; !ok || s.Updated.After(prev.Updated) {
						best[key] = s
					}
				}
				scoreMap := map[string]struct{ Health, Latency float64 }{}
				for _, s := range best {
					scoreMap[strings.ToLower(strings.TrimSpace(s.Model))] = struct{ Health, Latency float64 }{Health: s.Health, Latency: s.Latency}
				}
				if len(scoreMap) > 0 {
					modelRouter.SetModelScores(scoreMap)
					slog.Info("loaded persisted model scores", "count", len(scoreMap))
				}
			}
			// Keep a persistent connection for score persistence callbacks.
			if scoreDB, err := sql.Open("postgres", cfg.PostgresDSN); err == nil {
				scoreStore, serr := limits.NewStore(scoreDB)
				if serr == nil {
					modelRouter.SetPersistScore(func(model string, health, latency float64) {
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						if err := scoreStore.SaveScore(ctx, "custom", "", model, health, latency); err != nil {
							slog.Warn("persist model score failed", "model", model, "err", err)
						}
					})
				}
			}
		}
		_ = limitDB.Close()
	}
	// Thompson Sampling exploration toggle (灰度用): ADAPTIVE_ROUTING_ENABLED=true 时
	// AUTO 评分混入 Beta 后验采样，实现对新模型的探索与收敛。
	if strings.EqualFold(os.Getenv("ADAPTIVE_ROUTING_ENABLED"), "true") {
		modelRouter.SetThompsonEnabled(true)
		slog.Info("adaptive routing enabled: thompson sampling active in AUTO pool")
	}
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
	var billingService *billing.Service
	if cfg.BillingEnabled {
		if store, err := billing.NewStore(cfg.PostgresDSN); err != nil {
			slog.Warn("billing init failed", "err", err)
		} else {
			billingStore = store
			pricer := billing.NewPricer()
			pricer.SetDefaultProviderPrice("openai", 0.01, 0.03)
			pricer.SetDefaultProviderPrice("anthropic", 0.015, 0.075)
			pricer.SetDefaultProviderPrice("google", 0.0025, 0.0075)
			pricer.SetDefaultProviderPrice("mock", 0.001, 0.002)
			pricer.SetDefaultProviderPrice("mock-code", 0.001, 0.002)
			pricer.SetDefaultProviderPrice("mock-analysis", 0.001, 0.002)
			pricer.SetDefaultProviderPrice("mock-fail", 0.001, 0.002)
			pricer.SetModelPrice("openai", "gpt-4o", 0.01, 0.03)
			pricer.SetModelPrice("openai", "gpt-4o-mini", 0.0015, 0.006)
			pricer.SetModelPrice("openai", "gpt-3.5-turbo", 0.0015, 0.002)
			pricer.SetModelPrice("anthropic", "claude-3-opus", 0.015, 0.075)
			pricer.SetModelPrice("anthropic", "claude-3-sonnet", 0.003, 0.015)
			pricer.SetModelPrice("anthropic", "claude-3-haiku", 0.00025, 0.00125)
			pricer.SetModelPrice("google", "gemini-pro", 0.0025, 0.0075)
			billingService = billing.NewService(store, pricer)
			if err := billingService.LoadPricingFromDB(context.Background()); err != nil {
				slog.Warn("billing load pricing from db failed", "err", err)
			}
		}
	}
	adminStore, err := admin.NewStore(cfg.PostgresDSN)
	if err != nil {
		slog.Warn("admin init failed", "err", err)
	}
	// Register active database channels as isolated providers.
	if adminStore != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		channels, listErr := adminStore.ListChannels(ctx)
		cancel()
		if listErr != nil {
			slog.Warn("failed to list channels for registry", "err", listErr)
		} else {
			routerChannels := make([]router.Channel, 0)
			for _, ch := range channels {
				if strings.ToLower(strings.TrimSpace(ch.Status)) != "active" {
					continue
				}
				provider := providers.NewNamedOpenAIProvider(ch.ID, ch.BaseURL, ch.APIKey, cfg.OpenAITimeoutSec)
				for _, model := range ch.Models {
					registry.RegisterChannelModel(ch.ID, model, provider)
					routerChannels = append(routerChannels, router.Channel{ID: ch.ID, Provider: ch.Provider, Model: model, Task: "general", Enabled: true, Priority: ch.Weight, Weight: float64(ch.Weight), Tags: ch.Tags})
				}
			}
			modelRouter.SetChannels(routerChannels)
			for _, model := range defaultPool {
			registry.CleanupStaleGates()
				channel, ok := registry.PreferredChannelForModel(model)
				if !ok {
					slog.Error("default model has no registered execution channel", "model", model)
				} else {
					slog.Info("registered default model channel", "model", model, "channel", channel)
				}
			}
			slog.Info("registered active channels", "count", len(channels), "router_bindings", len(routerChannels))
		}
	}

	policyStore, err := policy.NewStore(cfg.PostgresDSN)
	if err != nil {
		slog.Warn("policy init failed", "err", err)
	}

	var semanticCache semantic.L2Cache = nil
	var semanticStop chan struct{}
	if cfg.SemanticCacheEnabled {
		if cfg.QdrantURL != "http://127.0.0.1:6333" && strings.TrimSpace(cfg.QdrantAPIKey) != "" {
			semanticCache = semantic.New(cfg.QdrantURL, cfg.QdrantAPIKey, cfg.QdrantCollection, cfg.SemanticVectorSize, cfg.SemanticCacheThreshold)
		} else {
			embedder := providers.NewEmbeddingClient(cfg.EmbeddingServiceURL, "", cfg.EmbeddingServiceModel, cfg.EmbeddingServiceDimensions)
			semanticCache = semantic.NewMemoryL2Cache(cfg.SemanticVectorSize, cfg.SemanticCacheThreshold)
			semanticCache.SetEmbedder(embedder)
			if persistPath := os.Getenv("SEMANTIC_CACHE_PERSIST_PATH"); persistPath != "" {
				semanticCache.SetPersistence(persistPath)
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := semanticCache.EnsureCollection(ctx); err != nil {
			slog.Warn("semantic cache init failed", "err", err)
			semanticCache = nil
		}
		if semanticCache != nil {
			semanticStop = make(chan struct{})
			go semanticCachePeriodicSave(semanticCache, semanticStop)
			// Immediate save to create file
			if mc, ok := semanticCache.(*semantic.MemoryL2Cache); ok {
				go func() { time.Sleep(10 * time.Second); mc.Save() }()
			}
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
	// Runtime config replay may replace router channels; re-apply the
	// database-backed active channel bindings as the final startup source.
	if adminStore != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		channels, listErr := adminStore.ListChannels(ctx)
		cancel()
		if listErr != nil {
			slog.Warn("failed to refresh channels after runtime replay", "err", listErr)
		} else {
			routerChannels := make([]router.Channel, 0)
			for _, ch := range channels {
				if strings.ToLower(strings.TrimSpace(ch.Status)) != "active" {
					continue
				}
				for _, model := range ch.Models {
					routerChannels = append(routerChannels, router.Channel{ID: ch.ID, Provider: ch.Provider, Model: model, Task: "general", Enabled: true, Priority: ch.Weight, Weight: float64(ch.Weight), Tags: ch.Tags})
				}
			}
			modelRouter.SetChannels(routerChannels)
			poolModels := strings.Split(cfg.DefaultModelPool, ",")
			bound := make([]string, 0)
			for _, model := range poolModels {
				for _, ch := range routerChannels {
					if strings.EqualFold(strings.TrimSpace(model), ch.Model) && ch.Enabled {
						bound = append(bound, ch.ID)
						break
					}
				}
			}
			slog.Info("refreshed active channels after runtime replay", "router_bindings", len(routerChannels), "default_pool_bound", len(bound), "default_pool_channels", bound)
		}
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
	var chatStore *chat.PostgresStore
	broadcastStore := broadcast.NewMemoryStore()
	if db, err := sql.Open("postgres", cfg.PostgresDSN); err != nil {
		slog.Warn("auth db init failed", "err", err)
	} else {
		authStore = auth.NewStore(db)
		akUsageStore = auth.NewAPIKeyUsageStore(db)
		akRateLimiter = httpserver.NewAPIKeyRateLimiter(cfg.RedisAddr, cfg.DefaultAPIKeyRPM)
		chatStore, _ = chat.NewStore(cfg.PostgresDSN)
	}

	srv := httpserver.New(cfg, registry, redisCache, modelRouter, auditStore, semanticCache, memoryStore, billingStore, limiter, adminStore, policyStore).
		WithControlPlane(controlPlaneService, controlPlaneAudit, runtimePublisher, runtimeManager).
		WithTenantKeys(tenantKeyStore)
	if billingService != nil {
		srv = srv.WithBillingService(billingService)
	}
	if authStore != nil && akUsageStore != nil {
		srv = srv.WithUserStore(authStore).WithAPIKeyUsageStore(akUsageStore)
		srv = srv.WithOAuthStore(authStore)
	}
	if chatStore != nil {
		srv = srv.WithChatStore(chatStore)
	}
	srv = srv.WithBroadcastAdminHandler(httpserver.NewBroadcastAdminHandler(broadcastStore))
	srv = srv.WithMod1Scorer(billingStore)
	srv = srv.WithAutoScorer(billingStore)
	srv = srv.WithBroadcastUserHandler(httpserver.NewBroadcastUserHandler(broadcastStore))

	adminConfigStore := adminconfig.NewStore()
	srv = srv.WithAdminConfigHandler(httpserver.NewAdminConfigHandler(adminConfigStore))
	if akRateLimiter != nil {
		srv = srv.WithAPIKeyRateLimiter(akRateLimiter, cfg.DefaultAPIKeyRPM)
	}
	if memoryStore != nil {
		srv = srv.WithMemoryAdminHandler(httpserver.NewMemoryAdminHandler(memoryStore))
		if db, err := sql.Open("postgres", cfg.PostgresDSN); err != nil {
			slog.Warn("preset store init failed", "err", err)
		} else {
			srv = srv.WithPresetStore(memory.NewPresetStore(db))
			srv = srv.WithUsageLogStore(httpserver.NewSQLUsageLogStore(db))
		}
	}
	var workerCtx context.Context
	var workerCancel context.CancelFunc
	// Virtual long-context handler and optional background worker pool.
	if longContextDB, err := sql.Open("postgres", cfg.PostgresDSN); err != nil {
		slog.Warn("long context db init failed", "err", err)
	} else if longContextRepo, err := longcontext.NewPostgresRepository(longContextDB); err != nil {
		slog.Warn("long context repository init failed", "err", err)
		_ = longContextDB.Close()
	} else {
		longContextHandler := httpserver.NewLongContextHandler(longContextRepo, cfg.LongContextEnabled, cfg.LongContextMaxInputBytes)
		srv = srv.WithLongContextHandler(longContextHandler)
		// Chat-integrated processor (1Mvir via /v1/chat/completions).
		chatProcessor := &longcontext.FullTaskProcessor{
			Repo:          longContextRepo,
			MapChannel:    cfg.LongContextWorkerChannel,
			MapModel:      cfg.LongContextWorkerModel,
			ReaderChannel: cfg.LongContextWorkerReaderChannel,
			ReaderModel:   cfg.LongContextWorkerReaderModel,
			ReaderTopK:    cfg.LongContextWorkerReaderTopK,
			Registry:      registry,
			MapTimeout:    90 * time.Second,
			SynthTimeout:  90 * time.Second,
		}
		if cfg.LongContextWorkerChannels != "" {
			for _, id := range strings.Split(cfg.LongContextWorkerChannels, ",") {
				if id = strings.TrimSpace(id); id != "" {
					chatProcessor.MapChannels = append(chatProcessor.MapChannels, id)
				}
			}
		}
		if cfg.LongContextWorkerReaderCandidates != "" {
			for _, pair := range strings.Split(cfg.LongContextWorkerReaderCandidates, ",") {
				if pair = strings.TrimSpace(pair); pair == "" {
					continue
				}
				if i := strings.Index(pair, "@"); i > 0 {
					chatProcessor.ReaderCandidates = append(chatProcessor.ReaderCandidates, longcontext.ReaderCandidate{
						Model:   strings.TrimSpace(pair[:i]),
						Channel: strings.TrimSpace(pair[i+1:]),
					})
				}
			}
		}
		if cfg.LongContextEmbeddingURL != "" && cfg.LongContextEmbeddingModel != "" {
			chatProcessor.Embedder = providers.NewEmbeddingClient(
				cfg.LongContextEmbeddingURL, cfg.LongContextEmbeddingKey,
				cfg.LongContextEmbeddingModel, cfg.LongContextEmbeddingDimensions)
		}
		srv = srv.WithLongContextProcessor(chatProcessor)
		pool := longcontext.WorkerPool{
			Claimer: longContextRepo,
			Processor: &longcontext.FullTaskProcessor{
				Repo:          longContextRepo,
				MapChannel:    cfg.LongContextWorkerChannel,
				MapModel:      cfg.LongContextWorkerModel,
				ReaderChannel: cfg.LongContextWorkerReaderChannel,
				ReaderModel:   cfg.LongContextWorkerReaderModel,
				ReaderTopK:    cfg.LongContextWorkerReaderTopK,
				Registry:      registry,
				MapTimeout:    90 * time.Second,
				SynthTimeout:  90 * time.Second,
			},
			Workers:   cfg.LongContextWorkers,
			Lease:     time.Minute,
			IdleDelay: 5 * time.Second,
		}
		if cfg.LongContextWorkers > 0 {
			if cfg.LongContextWorkerChannel == "" || cfg.LongContextWorkerModel == "" {
				slog.Warn("long context workers requested but channel/model not configured; pool will not run")
			} else {
				// Recover stale leases from previous crash/restart.
				if err := longContextRepo.RecoverStaleLeases(context.Background()); err != nil {
					slog.Warn("long context stale lease recovery failed", "err", err)
				} else {
					slog.Info("long context stale leases recovered")
				}
				workerCtx, workerCancel = context.WithCancel(context.Background())
				go func() {
					if err := pool.Run(workerCtx); err != nil && !errors.Is(err, context.Canceled) {
						slog.Error("long context worker pool exited", "err", err)
					}
				}()
				slog.Info("long context worker pool started", "workers", cfg.LongContextWorkers)
			}
		}
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
		if workerCancel != nil {
			workerCancel()
		}
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
