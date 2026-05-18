package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"llm-gateway/gateway/internal/quota"
)

// QuotaManager 定义配额管理器的最小接口。
type QuotaManager interface {
	CheckQuota(tenantID string, tokens int64) (allowed bool, remaining int64)
	GetQuotaStatus(tenantID string) *quota.QuotaStatus
	SetQuota(tenantID string, rpmLimit int, tpdLimit int64)
	GetQuota(tenantID string) (*quota.TenantQuota, bool)
}

// WithQuotaManager 注入配额管理器。
func (s *Server) WithQuotaManager(mgr QuotaManager) *Server {
	s.quotaManager = mgr
	return s
}

// mountQuotaRoutes 注册租户配额相关路由。
func (s *Server) mountQuotaRoutes(mux *http.ServeMux) {
	if s.quotaManager == nil {
		return
	}
	mux.HandleFunc("/api/tenant/quota", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.quotaGetStatus(w, r)
		case http.MethodPut:
			s.quotaPutConfig(w, r)
		default:
			methodNotAllowed(w, r)
		}
	})
}

// quotaGetStatus GET /api/tenant/quota
// 查看当前租户的配额状态（需认证）。
func (s *Server) quotaGetStatus(w http.ResponseWriter, r *http.Request) {
	tenantID := resolveTenantIDForQuota(r)
	if tenantID == "" {
		badRequest(w, "unable to resolve tenant_id from request")
		return
	}

	status := s.quotaManager.GetQuotaStatus(tenantID)
	writeJSON(w, http.StatusOK, status)
}

// quotaPutConfig PUT /api/tenant/quota
// 管理员设置租户配额。
// 请求体: {"tenant_id": "xxx", "rpm_limit": 60, "tpd_limit": 100000}
func (s *Server) quotaPutConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TenantID string `json:"tenant_id"`
		RPMLimit int    `json:"rpm_limit"`
		TPDLimit int64  `json:"tpd_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	if strings.TrimSpace(body.TenantID) == "" {
		badRequest(w, "tenant_id is required")
		return
	}

	s.quotaManager.SetQuota(body.TenantID, body.RPMLimit, body.TPDLimit)

	status := s.quotaManager.GetQuotaStatus(body.TenantID)
	writeJSON(w, http.StatusOK, status)
}

// resolveTenantIDForQuota 从请求中解析 tenant_id。
// 优先从 JWT claims 获取，其次从 X-Tenant-Id 请求头获取。
func resolveTenantIDForQuota(r *http.Request) string {
	claims := getUserClaims(r.Context())
	if claims != nil && claims.TenantID != "" {
		return claims.TenantID
	}
	return strings.TrimSpace(r.Header.Get("X-Tenant-Id"))
}
