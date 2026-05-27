package httpserver

import (
	"context"
	"net/http"
	"strings"

	"llm-gateway/gateway/internal/auth"
)

// tenantContextKey 用于在 context 中存储 tenant_id 的键类型。
type tenantContextKey struct{}

// WithTenantID 将 tenant_id 注入 context。
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantContextKey{}, tenantID)
}

// TenantIDFromContext 从 context 中提取 tenant_id。
// 若不存在则返回空字符串。
func TenantIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(tenantContextKey{}).(string)
	return v
}

// tenantIDResolver 解析请求对应的 tenant_id。
type tenantIDResolver interface {
	// DefaultTenant 返回一个默认 tenant_id（当 JWT claims 中无 tenant_id 时降级使用）。
	// 若无法确定则返回空字符串。
	DefaultTenant(ctx context.Context) (string, error)
}

// resolveTenantID 按优先级解析 tenant_id：
// 1. JWT claims 中的 TenantID
// 2. X-Tenant-Id 请求头
// 3. tenant store 中的默认 tenant（若实现了 tenantIDResolver 接口）
func resolveTenantID(ctx context.Context, claims *auth.Claims, r *http.Request, tenantStore interface{ DefaultTenant(context.Context) (string, error) }) string {
	// 1. JWT claims
	if claims != nil && claims.TenantID != "" {
		return claims.TenantID
	}
	// 2. X-Tenant-Id header
	hdr := strings.TrimSpace(r.Header.Get("X-Tenant-Id"))
	if hdr != "" {
		return hdr
	}
	// 3. 从 tenant store 获取默认 tenant
	if tenantStore != nil {
		tid, err := tenantStore.DefaultTenant(ctx)
		if err == nil && tid != "" {
			return tid
		}
	}
	return ""
}

// tenantMiddleware 租户隔离中间件。
// 应在 requireUser 之后执行（确保 JWT 已解析），
// 将 tenant_id 注入 context 供下游 handler 使用。
func (s *Server) tenantMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := getUserClaims(r.Context())
		tenantID := resolveTenantID(r.Context(), claims, r, s.tenantKeys)
		if tenantID != "" {
			r = r.WithContext(WithTenantID(r.Context(), tenantID))
		}
		next(w, r)
	}
}
