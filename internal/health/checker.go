package health

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/providers"
)

// Status 表示整体健康状态
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// CheckResult 单个检查项的结果
type CheckResult struct {
	Name      string        `json:"name"`
	Status    Status        `json:"status"`
	LatencyMS int64         `json:"latency_ms"`
	Error     string        `json:"error,omitempty"`
	CheckedAt time.Time     `json:"checked_at"`
	Detail    string        `json:"detail,omitempty"`
}

// Report 健康检查报告
type Report struct {
	Status    Status                  `json:"status"`
	Version   string                  `json:"version"`
	Uptime    string                  `json:"uptime"`
	Checks    map[string]CheckResult  `json:"checks"`
	Memory    MemoryInfo              `json:"memory"`
	Goroutine int                     `json:"goroutine_count"`
	Timestamp time.Time               `json:"timestamp"`
}

// MemoryInfo 内存使用信息
type MemoryInfo struct {
	AllocMB      uint64  `json:"alloc_mb"`
	TotalAllocMB uint64  `json:"total_alloc_mb"`
	SysMB        uint64  `json:"sys_mb"`
	NumGC        uint32  `json:"num_gc"`
	HeapAllocMB  uint64  `json:"heap_alloc_mb"`
	HeapSysMB    uint64  `json:"heap_sys_mb"`
	UsagePercent float64 `json:"usage_percent"`
}

// Pinger 可 Ping 的组件接口
type Pinger interface {
	Ping(ctx context.Context) error
}

// CheckerConfig HealthChecker 配置
type CheckerConfig struct {
	Interval           time.Duration // 检查间隔，默认 30s
	FailureThreshold   int           // 连续失败 N 次触发告警，默认 3
	MemoryThresholdPct float64       // 内存使用率告警阈值，默认 85%
	GoroutineThreshold int           // Goroutine 数量告警阈值，默认 10000
}

// HealthChecker 定期巡检的健康检查器
type HealthChecker struct {
	cfg        config.Config
	registry   *providers.Registry
	pingers    map[string]Pinger
	checkerCfg CheckerConfig

	mu             sync.RWMutex
	latestReport   Report
	running        bool
	stopCh         chan struct{}
	wg             sync.WaitGroup
	failureCounts  map[string]int
	alertTriggered bool
	startTime      time.Time
}

// NewHealthChecker 创建 HealthChecker
func NewHealthChecker(cfg config.Config, registry *providers.Registry, pingers map[string]Pinger, checkerCfg CheckerConfig) *HealthChecker {
	if checkerCfg.Interval <= 0 {
		checkerCfg.Interval = 30 * time.Second
	}
	if checkerCfg.FailureThreshold <= 0 {
		checkerCfg.FailureThreshold = 3
	}
	if checkerCfg.MemoryThresholdPct <= 0 {
		checkerCfg.MemoryThresholdPct = 85.0
	}
	if checkerCfg.GoroutineThreshold <= 0 {
		checkerCfg.GoroutineThreshold = 10000
	}

	return &HealthChecker{
		cfg:           cfg,
		registry:      registry,
		pingers:       pingers,
		checkerCfg:    checkerCfg,
		stopCh:        make(chan struct{}),
		failureCounts: make(map[string]int),
		startTime:     time.Now(),
	}
}

// Start 启动定期巡检
func (hc *HealthChecker) Start() {
	hc.mu.Lock()
	if hc.running {
		hc.mu.Unlock()
		return
	}
	hc.running = true
	hc.mu.Unlock()

	hc.wg.Add(1)
	go func() {
		defer hc.wg.Done()
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("health checker panic", "err", rec)
			}
		}()
		slog.Info("health checker started", "interval", hc.checkerCfg.Interval, "failure_threshold", hc.checkerCfg.FailureThreshold)
		ticker := time.NewTicker(hc.checkerCfg.Interval)
		defer ticker.Stop()

		// 启动立即执行一次
		hc.runChecks()

		for {
			select {
			case <-ticker.C:
				hc.runChecks()
			case <-hc.stopCh:
				slog.Info("health checker stopped")
				return
			}
		}
	}()
}

// Stop 停止定期巡检
func (hc *HealthChecker) Stop() {
	hc.mu.Lock()
	if !hc.running {
		hc.mu.Unlock()
		return
	}
	hc.running = false
	close(hc.stopCh)
	hc.mu.Unlock()

	hc.wg.Wait()
}

// Status 返回当前健康状态摘要
func (hc *HealthChecker) Status() Status {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.latestReport.Status
}

// Report 返回详细健康报告
func (hc *HealthChecker) Report() Report {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.latestReport
}

// IsAlertTriggered 返回是否已触发告警
func (hc *HealthChecker) IsAlertTriggered() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.alertTriggered
}

func (hc *HealthChecker) runChecks() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	checks := make(map[string]CheckResult)
	overallStatus := StatusHealthy

	// 1. 检查各 Pinger 组件（DB、Redis 等）
	for name, pinger := range hc.pingers {
		result := hc.checkPinger(ctx, name, pinger)
		checks[name] = result
		hc.trackFailure(name, result.Status)
		if result.Status == StatusUnhealthy {
			overallStatus = StatusUnhealthy
		} else if result.Status == StatusDegraded && overallStatus == StatusHealthy {
			overallStatus = StatusDegraded
		}
	}

	// 2. 检查 Provider 可用性
	providerResult := hc.checkProviders(ctx)
	checks["providers"] = providerResult
	hc.trackFailure("providers", providerResult.Status)
	if providerResult.Status == StatusUnhealthy {
		overallStatus = StatusUnhealthy
	} else if providerResult.Status == StatusDegraded && overallStatus == StatusHealthy {
		overallStatus = StatusDegraded
	}

	// 3. 检查内存使用率
	memResult := hc.checkMemory()
	checks["memory"] = memResult
	hc.trackFailure("memory", memResult.Status)
	if memResult.Status == StatusUnhealthy {
		overallStatus = StatusUnhealthy
	} else if memResult.Status == StatusDegraded && overallStatus == StatusHealthy {
		overallStatus = StatusDegraded
	}

	// 4. 检查 Goroutine 数量
	goroutineResult := hc.checkGoroutines()
	checks["goroutine"] = goroutineResult
	hc.trackFailure("goroutine", goroutineResult.Status)
	if goroutineResult.Status == StatusUnhealthy {
		overallStatus = StatusUnhealthy
	} else if goroutineResult.Status == StatusDegraded && overallStatus == StatusHealthy {
		overallStatus = StatusDegraded
	}

	// 检查是否触发告警
	hc.checkAlert()

	report := Report{
		Status:    overallStatus,
		Uptime:    time.Since(hc.startTime).Round(time.Second).String(),
		Checks:    checks,
		Memory:    hc.collectMemoryInfo(),
		Goroutine: runtime.NumGoroutine(),
		Timestamp: time.Now().UTC(),
	}

	hc.mu.Lock()
	hc.latestReport = report
	hc.mu.Unlock()

	if overallStatus != StatusHealthy {
		slog.Warn("health check completed", "status", overallStatus, "checks", len(checks))
	}
}

func (hc *HealthChecker) checkPinger(ctx context.Context, name string, pinger Pinger) CheckResult {
	started := time.Now()
	err := pinger.Ping(ctx)
	latency := time.Since(started).Milliseconds()

	if err != nil {
		return CheckResult{
			Name:      name,
			Status:    StatusUnhealthy,
			LatencyMS: latency,
			Error:     err.Error(),
			CheckedAt: time.Now().UTC(),
			Detail:    fmt.Sprintf("%s ping failed", name),
		}
	}
	return CheckResult{
		Name:      name,
		Status:    StatusHealthy,
		LatencyMS: latency,
		CheckedAt: time.Now().UTC(),
		Detail:    fmt.Sprintf("%s reachable", name),
	}
}

func (hc *HealthChecker) checkProviders(ctx context.Context) CheckResult {
	if hc.registry == nil {
		return CheckResult{
			Name:      "providers",
			Status:    StatusDegraded,
			CheckedAt: time.Now().UTC(),
			Detail:    "provider registry unavailable",
		}
	}

	started := time.Now()
	statuses := hc.registry.HealthStatuses()
	latency := time.Since(started).Milliseconds()

	if len(statuses) == 0 {
		return CheckResult{
			Name:      "providers",
			Status:    StatusDegraded,
			LatencyMS: latency,
			CheckedAt: time.Now().UTC(),
			Detail:    "no providers registered",
		}
	}

	healthyCount := 0
	unhealthyCount := 0
	for _, s := range statuses {
		switch s.Status {
		case "ok":
			healthyCount++
		case "error", "open":
			unhealthyCount++
		}
	}

	total := len(statuses)
	if unhealthyCount == total && total > 0 {
		return CheckResult{
			Name:      "providers",
			Status:    StatusUnhealthy,
			LatencyMS: latency,
			CheckedAt: time.Now().UTC(),
			Detail:    fmt.Sprintf("all %d providers unhealthy", total),
		}
	}
	if unhealthyCount > 0 {
		return CheckResult{
			Name:      "providers",
			Status:    StatusDegraded,
			LatencyMS: latency,
			CheckedAt: time.Now().UTC(),
			Detail:    fmt.Sprintf("%d/%d providers unhealthy", unhealthyCount, total),
		}
	}
	return CheckResult{
		Name:      "providers",
		Status:    StatusHealthy,
		LatencyMS: latency,
		CheckedAt: time.Now().UTC(),
		Detail:    fmt.Sprintf("all %d providers healthy", healthyCount),
	}
}

func (hc *HealthChecker) checkMemory() CheckResult {
	info := hc.collectMemoryInfo()
	status := StatusHealthy
	if info.UsagePercent >= hc.checkerCfg.MemoryThresholdPct {
		status = StatusUnhealthy
	} else if info.UsagePercent >= hc.checkerCfg.MemoryThresholdPct*0.8 {
		status = StatusDegraded
	}
	detail := fmt.Sprintf("memory usage %.1f%%", info.UsagePercent)
	return CheckResult{
		Name:      "memory",
		Status:    status,
		CheckedAt: time.Now().UTC(),
		Detail:    detail,
	}
}

func (hc *HealthChecker) checkGoroutines() CheckResult {
	count := runtime.NumGoroutine()
	status := StatusHealthy
	if count >= hc.checkerCfg.GoroutineThreshold {
		status = StatusUnhealthy
	} else if count >= int(float64(hc.checkerCfg.GoroutineThreshold)*0.8) {
		status = StatusDegraded
	}
	detail := fmt.Sprintf("goroutine count %d", count)
	return CheckResult{
		Name:      "goroutine",
		Status:    status,
		CheckedAt: time.Now().UTC(),
		Detail:    detail,
	}
}

func (hc *HealthChecker) collectMemoryInfo() MemoryInfo {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	allocMB := mem.Alloc / 1024 / 1024
	totalAllocMB := mem.TotalAlloc / 1024 / 1024
	sysMB := mem.Sys / 1024 / 1024
	heapAllocMB := mem.HeapAlloc / 1024 / 1024
	heapSysMB := mem.HeapSys / 1024 / 1024

	var usagePercent float64
	if sysMB > 0 {
		usagePercent = float64(allocMB) / float64(sysMB) * 100
	}

	return MemoryInfo{
		AllocMB:      allocMB,
		TotalAllocMB: totalAllocMB,
		SysMB:        sysMB,
		NumGC:        mem.NumGC,
		HeapAllocMB:  heapAllocMB,
		HeapSysMB:    heapSysMB,
		UsagePercent: usagePercent,
	}
}

func (hc *HealthChecker) trackFailure(name string, status Status) {
	if status == StatusUnhealthy {
		hc.failureCounts[name]++
	} else {
		hc.failureCounts[name] = 0
	}
}

func (hc *HealthChecker) checkAlert() {
	triggered := false
	for name, count := range hc.failureCounts {
		if count >= hc.checkerCfg.FailureThreshold {
			if !hc.alertTriggered {
				slog.Error("health alert triggered", "component", name, "consecutive_failures", count, "threshold", hc.checkerCfg.FailureThreshold)
			}
			triggered = true
			break
		}
	}
	hc.alertTriggered = triggered
}
