package quota

import (
	"sync"
	"time"
)

// TenantQuota 表示单个租户的配额配置。
type TenantQuota struct {
	TenantID  string `json:"tenant_id"`
	RPMLimit  int    `json:"rpm_limit"`  // 每分钟请求数限制（0 表示不限制）
	TPDLimit  int64  `json:"tpd_limit"`  // 每日 token 数限制（0 表示不限制）
	UpdatedAt time.Time `json:"updated_at"`
}

// QuotaStatus 表示租户的实时配额状态。
type QuotaStatus struct {
	TenantID  string `json:"tenant_id"`
	RPMLimit  int    `json:"rpm_limit"`
	RPMUsed   int64  `json:"rpm_used"`
	TPDLimit  int64  `json:"tpd_limit"`
	TPDUsed   int64  `json:"tpd_used"`
}

// quotaUsage 记录内存中的用量计数（分钟级 RPM 和天级 TPD）。
// mu 保护所有字段的并发访问。
type quotaUsage struct {
	mu          sync.Mutex
	rpmCount    int64
	rpmMinute   string // YYYYMMDDHHmm
	tpdCount    int64
	tpdDay      string // YYYYMMDD
	lastUpdated time.Time
}

// TenantQuotaManager 管理多租户配额配置与用量追踪。
// 使用 sync.Map 存储配额配置，不引入外部依赖。
type TenantQuotaManager struct {
	mu       sync.RWMutex
	quotas   sync.Map // map[string]*TenantQuota  keyed by tenantID
	usage    sync.Map // map[string]*quotaUsage  keyed by tenantID
}

// NewTenantQuotaManager 创建一个新的 TenantQuotaManager 实例。
func NewTenantQuotaManager() *TenantQuotaManager {
	return &TenantQuotaManager{}
}

// SetQuota 设置或更新租户的配额配置。
func (m *TenantQuotaManager) SetQuota(tenantID string, rpmLimit int, tpdLimit int64) {
	if tenantID == "" {
		return
	}
	if rpmLimit < 0 {
		rpmLimit = 0
	}
	if tpdLimit < 0 {
		tpdLimit = 0
	}
	m.quotas.Store(tenantID, &TenantQuota{
		TenantID:  tenantID,
		RPMLimit:  rpmLimit,
		TPDLimit:  tpdLimit,
		UpdatedAt: time.Now().UTC(),
	})
}

// GetQuota 获取租户的配额配置。若不存在则返回 nil, false。
func (m *TenantQuotaManager) GetQuota(tenantID string) (*TenantQuota, bool) {
	v, ok := m.quotas.Load(tenantID)
	if !ok {
		return nil, false
	}
	return v.(*TenantQuota), true
}

// CheckQuota 检查租户是否允许消耗指定数量的 tokens。
// 返回 allowed（是否允许）、remaining（剩余可用 token 数，仅 TPD 维度）、err。
// RPM 维度：若配置了 RPM 限制，则按分钟窗口计数。
// TPD 维度：若配置了 TPD 限制，则按天窗口累计 tokens。
func (m *TenantQuotaManager) CheckQuota(tenantID string, tokens int64) (allowed bool, remaining int64) {
	if tenantID == "" {
		return true, 0
	}

	q, ok := m.quotas.Load(tenantID)
	if !ok {
		// 未配置配额，默认放行
		return true, 0
	}
	quota := q.(*TenantQuota)
	now := time.Now().UTC()

	// 获取或创建用量记录
	usage := m.getOrCreateUsage(tenantID)

	// 对单个租户的用量记录加锁（细粒度锁，不同租户不互相阻塞）
	usage.mu.Lock()
	defer usage.mu.Unlock()

	// --- RPM 检查 ---
	if quota.RPMLimit > 0 {
		currentMinute := now.Format("200601021504")
		if usage.rpmMinute != currentMinute {
			// 新分钟窗口，重置计数
			usage.rpmMinute = currentMinute
			usage.rpmCount = 0
		}
		usage.rpmCount++
		if int(usage.rpmCount) > quota.RPMLimit {
			return false, 0
		}
	}

	// --- TPD 检查 ---
	if quota.TPDLimit > 0 {
		currentDay := now.Format("20060102")
		if usage.tpdDay != currentDay {
			// 新天窗口，重置计数
			usage.tpdDay = currentDay
			usage.tpdCount = 0
		}
		usage.tpdCount += tokens
		usage.lastUpdated = now
		if usage.tpdCount > quota.TPDLimit {
			return false, 0
		}
		remaining = quota.TPDLimit - usage.tpdCount
		if remaining < 0 {
			remaining = 0
		}
	}

	return true, remaining
}

// GetQuotaStatus 返回租户的实时配额状态（含当前用量）。
func (m *TenantQuotaManager) GetQuotaStatus(tenantID string) *QuotaStatus {
	status := &QuotaStatus{TenantID: tenantID}

	q, ok := m.quotas.Load(tenantID)
	if !ok {
		return status
	}
	quota := q.(*TenantQuota)
	status.RPMLimit = quota.RPMLimit
	status.TPDLimit = quota.TPDLimit

	now := time.Now().UTC()

	v, ok := m.usage.Load(tenantID)
	if !ok {
		return status
	}
	usage := v.(*quotaUsage)

	usage.mu.Lock()
	defer usage.mu.Unlock()

	// RPM 用量（当前分钟）
	currentMinute := now.Format("200601021504")
	if usage.rpmMinute == currentMinute {
		status.RPMUsed = usage.rpmCount
	}

	// TPD 用量（当前天）
	currentDay := now.Format("20060102")
	if usage.tpdDay == currentDay {
		status.TPDUsed = usage.tpdCount
	}

	return status
}

// getOrCreateUsage 获取或创建租户的用量记录（线程安全）。
func (m *TenantQuotaManager) getOrCreateUsage(tenantID string) *quotaUsage {
	v, ok := m.usage.Load(tenantID)
	if ok {
		return v.(*quotaUsage)
	}
	newUsage := &quotaUsage{}
	actual, loaded := m.usage.LoadOrStore(tenantID, newUsage)
	if loaded {
		return actual.(*quotaUsage)
	}
	return newUsage
}
