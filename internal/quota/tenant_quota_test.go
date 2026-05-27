package quota

import (
	"sync"
	"testing"
)

func TestNewTenantQuotaManager(t *testing.T) {
	m := NewTenantQuotaManager()
	if m == nil {
		t.Fatal("NewTenantQuotaManager 返回 nil")
	}
}

func TestSetQuotaAndGetQuota(t *testing.T) {
	m := NewTenantQuotaManager()

	// 设置配额
	m.SetQuota("tenant-1", 60, 100000)

	// 读取配额
	q, ok := m.GetQuota("tenant-1")
	if !ok {
		t.Fatal("期望找到 tenant-1 的配额配置")
	}
	if q.RPMLimit != 60 {
		t.Errorf("RPMLimit = %d, want 60", q.RPMLimit)
	}
	if q.TPDLimit != 100000 {
		t.Errorf("TPDLimit = %d, want 100000", q.TPDLimit)
	}
	if q.TenantID != "tenant-1" {
		t.Errorf("TenantID = %q, want tenant-1", q.TenantID)
	}

	// 不存在的租户
	_, ok = m.GetQuota("nonexistent")
	if ok {
		t.Error("不存在的租户应返回 false")
	}
}

func TestSetQuota_EmptyTenantID(t *testing.T) {
	m := NewTenantQuotaManager()
	m.SetQuota("", 60, 100000)

	_, ok := m.GetQuota("")
	if ok {
		t.Error("空 tenantID 不应被存储")
	}
}

func TestSetQuota_NegativeValues(t *testing.T) {
	m := NewTenantQuotaManager()
	m.SetQuota("tenant-neg", -10, -100)

	q, ok := m.GetQuota("tenant-neg")
	if !ok {
		t.Fatal("应找到配额配置")
	}
	if q.RPMLimit != 0 {
		t.Errorf("负 RPM 应被归零, got %d", q.RPMLimit)
	}
	if q.TPDLimit != 0 {
		t.Errorf("负 TPD 应被归零, got %d", q.TPDLimit)
	}
}

func TestCheckQuota_NoConfig(t *testing.T) {
	m := NewTenantQuotaManager()

	// 未配置配额的租户应放行
	allowed, remaining := m.CheckQuota("unknown-tenant", 100)
	if !allowed {
		t.Error("未配置配额的租户应默认放行")
	}
	if remaining != 0 {
		t.Errorf("未配置配额时 remaining 应为 0, got %d", remaining)
	}
}

func TestCheckQuota_EmptyTenantID(t *testing.T) {
	m := NewTenantQuotaManager()

	allowed, _ := m.CheckQuota("", 100)
	if !allowed {
		t.Error("空 tenantID 应放行")
	}
}

func TestCheckQuota_RPMLimit(t *testing.T) {
	m := NewTenantQuotaManager()
	m.SetQuota("tenant-rpm", 5, 0) // RPM=5, TPD 不限制

	// 前 5 次应通过
	for i := 0; i < 5; i++ {
		allowed, _ := m.CheckQuota("tenant-rpm", 1)
		if !allowed {
			t.Errorf("第 %d 次请求应被允许", i+1)
		}
	}

	// 第 6 次应被拒绝
	allowed, _ := m.CheckQuota("tenant-rpm", 1)
	if allowed {
		t.Error("第 6 次请求应被 RPM 限制拒绝")
	}
}

func TestCheckQuota_TPDLimit(t *testing.T) {
	m := NewTenantQuotaManager()
	m.SetQuota("tenant-tpd", 0, 1000) // RPM 不限制, TPD=1000

	// 消耗 900 tokens
	allowed, remaining := m.CheckQuota("tenant-tpd", 900)
	if !allowed {
		t.Error("900 tokens 应被允许")
	}
	if remaining != 100 {
		t.Errorf("remaining = %d, want 100", remaining)
	}

	// 再消耗 100 tokens，刚好用完
	allowed, remaining = m.CheckQuota("tenant-tpd", 100)
	if !allowed {
		t.Error("再消耗 100 tokens 应被允许")
	}
	if remaining != 0 {
		t.Errorf("remaining = %d, want 0", remaining)
	}

	// 超出限制
	allowed, _ = m.CheckQuota("tenant-tpd", 1)
	if allowed {
		t.Error("超出 TPD 限制应被拒绝")
	}
}

func TestCheckQuota_BothLimits(t *testing.T) {
	m := NewTenantQuotaManager()
	m.SetQuota("tenant-both", 10, 5000)

	// 正常请求
	allowed, remaining := m.CheckQuota("tenant-both", 100)
	if !allowed {
		t.Error("首次请求应被允许")
	}
	if remaining != 4900 {
		t.Errorf("remaining = %d, want 4900", remaining)
	}
}

func TestCheckQuota_ZeroMeansUnlimited(t *testing.T) {
	m := NewTenantQuotaManager()
	m.SetQuota("tenant-unlimited", 0, 0) // 全 0 = 不限制

	// 大量请求应全部通过
	for i := 0; i < 1000; i++ {
		allowed, _ := m.CheckQuota("tenant-unlimited", 1000)
		if !allowed {
			t.Errorf("第 %d 次请求应被允许（无限制）", i+1)
			break
		}
	}
}

func TestGetQuotaStatus_NoConfig(t *testing.T) {
	m := NewTenantQuotaManager()

	status := m.GetQuotaStatus("unknown")
	if status.TenantID != "unknown" {
		t.Errorf("TenantID = %q, want unknown", status.TenantID)
	}
	if status.RPMLimit != 0 {
		t.Error("未配置时 RPMLimit 应为 0")
	}
	if status.TPDLimit != 0 {
		t.Error("未配置时 TPDLimit 应为 0")
	}
}

func TestGetQuotaStatus_WithUsage(t *testing.T) {
	m := NewTenantQuotaManager()
	m.SetQuota("tenant-status", 100, 50000)

	// 发起一些请求
	for i := 0; i < 3; i++ {
		m.CheckQuota("tenant-status", 1000)
	}

	status := m.GetQuotaStatus("tenant-status")
	if status.RPMLimit != 100 {
		t.Errorf("RPMLimit = %d, want 100", status.RPMLimit)
	}
	if status.RPMUsed != 3 {
		t.Errorf("RPMUsed = %d, want 3", status.RPMUsed)
	}
	if status.TPDLimit != 50000 {
		t.Errorf("TPDLimit = %d, want 50000", status.TPDLimit)
	}
	if status.TPDUsed != 3000 {
		t.Errorf("TPDUsed = %d, want 3000", status.TPDUsed)
	}
}

func TestConcurrentCheckQuota(t *testing.T) {
	m := NewTenantQuotaManager()
	// RPM 不限制, TPD 足够大：所有 5000 次请求都应通过
	m.SetQuota("tenant-concurrent", 0, 10000000)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				m.CheckQuota("tenant-concurrent", 10)
			}
		}()
	}
	wg.Wait()

	// 验证最终状态：100 goroutines × 50 calls × 10 tokens = 50000
	status := m.GetQuotaStatus("tenant-concurrent")
	if status.RPMUsed != 0 {
		t.Errorf("RPMUsed = %d, want 0 (RPM limit is 0 = unlimited, no RPM tracking)", status.RPMUsed)
	}
	if status.TPDUsed != 50000 {
		t.Errorf("TPDUsed = %d, want 50000", status.TPDUsed)
	}
}

func TestSetQuota_Update(t *testing.T) {
	m := NewTenantQuotaManager()
	m.SetQuota("tenant-update", 10, 1000)

	// 更新配额
	m.SetQuota("tenant-update", 20, 5000)

	q, ok := m.GetQuota("tenant-update")
	if !ok {
		t.Fatal("应找到配额")
	}
	if q.RPMLimit != 20 {
		t.Errorf("RPMLimit = %d, want 20", q.RPMLimit)
	}
	if q.TPDLimit != 5000 {
		t.Errorf("TPDLimit = %d, want 5000", q.TPDLimit)
	}
}
