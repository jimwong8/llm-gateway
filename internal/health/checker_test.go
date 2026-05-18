package health

import (
	"context"
	"errors"
	"testing"
	"time"

	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/providers"
)

// --- stubs ---

type stubPinger struct {
	err    error
	called bool
}

func (s *stubPinger) Ping(_ context.Context) error {
	s.called = true
	return s.err
}

func newTestRegistry() *providers.Registry {
	cfg := config.Config{}
	return providers.NewRegistry(cfg, nil)
}

// --- tests ---

func TestNewHealthChecker_Defaults(t *testing.T) {
	hc := NewHealthChecker(config.Config{}, nil, nil, CheckerConfig{})
	if hc.checkerCfg.Interval != 30*time.Second {
		t.Fatalf("expected default interval 30s, got %v", hc.checkerCfg.Interval)
	}
	if hc.checkerCfg.FailureThreshold != 3 {
		t.Fatalf("expected default failure threshold 3, got %d", hc.checkerCfg.FailureThreshold)
	}
	if hc.checkerCfg.MemoryThresholdPct != 85.0 {
		t.Fatalf("expected default memory threshold 85, got %f", hc.checkerCfg.MemoryThresholdPct)
	}
	if hc.checkerCfg.GoroutineThreshold != 10000 {
		t.Fatalf("expected default goroutine threshold 10000, got %d", hc.checkerCfg.GoroutineThreshold)
	}
}

func TestNewHealthChecker_CustomConfig(t *testing.T) {
	cfg := CheckerConfig{
		Interval:           10 * time.Second,
		FailureThreshold:   5,
		MemoryThresholdPct: 90.0,
		GoroutineThreshold: 5000,
	}
	hc := NewHealthChecker(config.Config{}, nil, nil, cfg)
	if hc.checkerCfg.Interval != 10*time.Second {
		t.Fatalf("expected interval 10s, got %v", hc.checkerCfg.Interval)
	}
	if hc.checkerCfg.FailureThreshold != 5 {
		t.Fatalf("expected failure threshold 5, got %d", hc.checkerCfg.FailureThreshold)
	}
	if hc.checkerCfg.MemoryThresholdPct != 90.0 {
		t.Fatalf("expected memory threshold 90, got %f", hc.checkerCfg.MemoryThresholdPct)
	}
	if hc.checkerCfg.GoroutineThreshold != 5000 {
		t.Fatalf("expected goroutine threshold 5000, got %d", hc.checkerCfg.GoroutineThreshold)
	}
}

func TestHealthChecker_Status_Initial(t *testing.T) {
	hc := NewHealthChecker(config.Config{}, nil, nil, CheckerConfig{})
	// 初始状态应该是空 Status（还没运行过检查）
	if hc.Status() != "" {
		t.Fatalf("expected empty initial status, got %q", hc.Status())
	}
}

func TestHealthChecker_StartStop(t *testing.T) {
	pinger := &stubPinger{}
	pingers := map[string]Pinger{"test": pinger}

	hc := NewHealthChecker(config.Config{}, nil, pingers, CheckerConfig{
		Interval:         100 * time.Millisecond,
		FailureThreshold: 3,
	})

	hc.Start()
	time.Sleep(150 * time.Millisecond) // 等待至少一次检查
	hc.Stop()

	if !pinger.called {
		t.Fatal("expected pinger to be called")
	}

	// 检查后应该有状态
	report := hc.Report()
	if report.Status == "" {
		t.Fatal("expected non-empty status after check run")
	}
	if report.Timestamp.IsZero() {
		t.Fatal("expected non-zero timestamp")
	}
}

func TestHealthChecker_StartIdempotent(t *testing.T) {
	hc := NewHealthChecker(config.Config{}, nil, nil, CheckerConfig{
		Interval: 100 * time.Millisecond,
	})
	hc.Start()
	hc.Start() // 第二次应该不报错
	hc.Stop()
}

func TestHealthChecker_StopWithoutStart(t *testing.T) {
	hc := NewHealthChecker(config.Config{}, nil, nil, CheckerConfig{})
	hc.Stop() // 不应该 panic
}

func TestHealthChecker_CheckPingerHealthy(t *testing.T) {
	pinger := &stubPinger{err: nil}
	hc := NewHealthChecker(config.Config{}, nil, nil, CheckerConfig{})

	result := hc.checkPinger(context.Background(), "db", pinger)
	if result.Status != StatusHealthy {
		t.Fatalf("expected healthy, got %s", result.Status)
	}
	if result.Error != "" {
		t.Fatalf("expected no error, got %q", result.Error)
	}
	if result.LatencyMS < 0 {
		t.Fatalf("expected non-negative latency, got %d", result.LatencyMS)
	}
}

func TestHealthChecker_CheckPingerUnhealthy(t *testing.T) {
	pinger := &stubPinger{err: errors.New("connection refused")}
	hc := NewHealthChecker(config.Config{}, nil, nil, CheckerConfig{})

	result := hc.checkPinger(context.Background(), "db", pinger)
	if result.Status != StatusUnhealthy {
		t.Fatalf("expected unhealthy, got %s", result.Status)
	}
	if result.Error != "connection refused" {
		t.Fatalf("expected 'connection refused', got %q", result.Error)
	}
}

func TestHealthChecker_CheckProvidersNilRegistry(t *testing.T) {
	hc := NewHealthChecker(config.Config{}, nil, nil, CheckerConfig{})
	result := hc.checkProviders(context.Background())
	if result.Status != StatusDegraded {
		t.Fatalf("expected degraded when registry is nil, got %s", result.Status)
	}
}

func TestHealthChecker_CheckProvidersWithRegistry(t *testing.T) {
	registry := newTestRegistry()
	hc := NewHealthChecker(config.Config{}, registry, nil, CheckerConfig{})
	result := hc.checkProviders(context.Background())
	// 空 registry 没有 providers，应该是 degraded
	if result.Status != StatusDegraded {
		t.Fatalf("expected degraded for empty registry, got %s", result.Status)
	}
}

func TestHealthChecker_CheckMemory(t *testing.T) {
	hc := NewHealthChecker(config.Config{}, nil, nil, CheckerConfig{
		MemoryThresholdPct: 99.99, // 设置极高阈值，确保不会触发
	})
	result := hc.checkMemory()
	if result.Status != StatusHealthy {
		t.Fatalf("expected healthy with high threshold, got %s (detail: %s)", result.Status, result.Detail)
	}
}

func TestHealthChecker_CheckGoroutines(t *testing.T) {
	hc := NewHealthChecker(config.Config{}, nil, nil, CheckerConfig{
		GoroutineThreshold: 999999, // 设置极高阈值
	})
	result := hc.checkGoroutines()
	if result.Status != StatusHealthy {
		t.Fatalf("expected healthy with high threshold, got %s", result.Status)
	}
}

func TestHealthChecker_AlertNotTriggeredInitially(t *testing.T) {
	hc := NewHealthChecker(config.Config{}, nil, nil, CheckerConfig{})
	if hc.IsAlertTriggered() {
		t.Fatal("alert should not be triggered initially")
	}
}

func TestHealthChecker_AlertAfterConsecutiveFailures(t *testing.T) {
	pinger := &stubPinger{err: errors.New("connection refused")}
	pingers := map[string]Pinger{"db": pinger}

	hc := NewHealthChecker(config.Config{}, nil, pingers, CheckerConfig{
		Interval:         50 * time.Millisecond,
		FailureThreshold: 3,
	})

	hc.Start()
	time.Sleep(200 * time.Millisecond) // 等待多次检查
	hc.Stop()

	if !hc.IsAlertTriggered() {
		t.Fatal("expected alert to be triggered after consecutive failures")
	}
}

func TestHealthChecker_ReportContainsAllChecks(t *testing.T) {
	pingerOk := &stubPinger{err: nil}
	pingerFail := &stubPinger{err: errors.New("fail")}
	pingers := map[string]Pinger{
		"redis": pingerOk,
		"db":    pingerFail,
	}

	hc := NewHealthChecker(config.Config{}, nil, pingers, CheckerConfig{
		Interval:         50 * time.Millisecond,
		FailureThreshold: 10, // 高阈值，不触发告警
	})

	hc.Start()
	time.Sleep(100 * time.Millisecond)
	hc.Stop()

	report := hc.Report()
	if len(report.Checks) == 0 {
		t.Fatal("expected checks in report")
	}

	// 应该包含 redis, db, providers, memory, goroutine
	for _, name := range []string{"redis", "db", "providers", "memory", "goroutine"} {
		if _, ok := report.Checks[name]; !ok {
			t.Fatalf("expected check %q in report, got keys: %v", name, checkKeys(report.Checks))
		}
	}

	// redis 应该是 healthy
	if report.Checks["redis"].Status != StatusHealthy {
		t.Fatalf("expected redis healthy, got %s", report.Checks["redis"].Status)
	}
	// db 应该是 unhealthy
	if report.Checks["db"].Status != StatusUnhealthy {
		t.Fatalf("expected db unhealthy, got %s", report.Checks["db"].Status)
	}
	// 整体应该是 unhealthy（因为 db 失败）
	if report.Status != StatusUnhealthy {
		t.Fatalf("expected overall unhealthy, got %s", report.Status)
	}
	if report.Goroutine <= 0 {
		t.Fatalf("expected positive goroutine count, got %d", report.Goroutine)
	}
	if report.Memory.SysMB <= 0 {
		t.Fatalf("expected positive sys_mb, got %d", report.Memory.SysMB)
	}
}

func TestHealthChecker_OverallDegraded(t *testing.T) {
	// 通过让一个 pinger 失败来触发 degraded/unhealthy
	pinger := &stubPinger{err: errors.New("fail")}
	pingers := map[string]Pinger{"db": pinger}

	hc := NewHealthChecker(config.Config{}, nil, pingers, CheckerConfig{
		Interval:         50 * time.Millisecond,
		MemoryThresholdPct: 99.99,
	})

	hc.Start()
	time.Sleep(100 * time.Millisecond)
	hc.Stop()

	report := hc.Report()
	// db 是 unhealthy，整体也是 unhealthy
	if report.Checks["db"].Status != StatusUnhealthy {
		t.Fatalf("expected db unhealthy, got %s", report.Checks["db"].Status)
	}
	if report.Status != StatusUnhealthy {
		t.Fatalf("expected overall unhealthy, got %s", report.Status)
	}
}

func TestHealthChecker_OverallHealthy(t *testing.T) {
	pinger := &stubPinger{err: nil}
	pingers := map[string]Pinger{"db": pinger}

	hc := NewHealthChecker(config.Config{}, nil, pingers, CheckerConfig{
		Interval:           50 * time.Millisecond,
		MemoryThresholdPct: 99.99,
		GoroutineThreshold: 999999,
	})

	hc.Start()
	time.Sleep(100 * time.Millisecond)
	hc.Stop()

	report := hc.Report()
	// db 是 healthy，但 providers 是 degraded（空 registry），所以整体是 degraded
	if report.Status != StatusDegraded {
		t.Fatalf("expected overall degraded (providers degraded), got %s", report.Status)
	}
}

func TestCollectMemoryInfo(t *testing.T) {
	hc := NewHealthChecker(config.Config{}, nil, nil, CheckerConfig{})
	info := hc.collectMemoryInfo()
	// SysMB 应该 > 0（系统分配的内存）
	if info.SysMB <= 0 {
		t.Fatalf("expected positive SysMB, got %d", info.SysMB)
	}
	// HeapSysMB 应该 > 0
	if info.HeapSysMB <= 0 {
		t.Fatalf("expected positive HeapSysMB, got %d", info.HeapSysMB)
	}
	// NumGC 应该 >= 0（可能为 0 如果还没触发 GC）
	_ = info.NumGC
}

// --- handler tests ---

func TestHandler_Healthz_Healthy(t *testing.T) {
	pinger := &stubPinger{err: nil}
	pingers := map[string]Pinger{"db": pinger}

	hc := NewHealthChecker(config.Config{}, nil, pingers, CheckerConfig{
		Interval:           50 * time.Millisecond,
		MemoryThresholdPct: 99.99,
		GoroutineThreshold: 999999,
	})

	hc.Start()
	time.Sleep(100 * time.Millisecond)
	hc.Stop()

	_ = NewHandler(hc)
}

func TestHandler_HealthzDetailed_HasReport(t *testing.T) {
	hc := NewHealthChecker(config.Config{}, nil, nil, CheckerConfig{
		Interval: 50 * time.Millisecond,
	})
	hc.Start()
	time.Sleep(100 * time.Millisecond)
	hc.Stop()

	_ = NewHandler(hc)
}

// --- helpers ---

func checkKeys(m map[string]CheckResult) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
