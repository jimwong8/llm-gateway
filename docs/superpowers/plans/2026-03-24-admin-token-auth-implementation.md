# Admin Token Auth Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 [`internal/httpserver/admin_handler.go`](internal/httpserver/admin_handler.go) 增加最小固定 Bearer token 鉴权骨架，统一保护全部 `/admin/*`，并用最小测试与 verify 回归覆盖成功/失败主路径。

**Architecture:** 继续沿用 [`AdminHandler`](internal/httpserver/admin_handler.go:24) 作为唯一入口，不引入新中间件层。通过 [`WithAdminToken()`](internal/httpserver/admin_handler.go:101) 注入固定 token，在 [`ServeHTTP()`](internal/httpserver/admin_handler.go:115) 对全部 `/admin/*` 做前置 Bearer 校验，失败统一返回 `401` 与 `{"error":"unauthorized"}`；测试和 verify 只补最小必要覆盖。

**Tech Stack:** Go, [`net/http`](internal/httpserver/admin_handler.go:7), [`httptest`](internal/httpserver/admin_handler_test.go:8), 现有 [`controlplane`](internal/httpserver/admin_handler.go:12) / [`audit`](internal/httpserver/admin_handler.go:11) / [`runtime`](internal/httpserver/admin_handler.go:13)

---

## Chunk 1: Handler 鉴权骨架

### Task 1: 为 admin handler 先写失败/成功测试

**Files:**
- Modify: [`internal/httpserver/admin_handler_test.go`](internal/httpserver/admin_handler_test.go)
- Test: [`internal/httpserver/admin_handler_test.go`](internal/httpserver/admin_handler_test.go)

- [ ] **Step 1: 写最小鉴权测试骨架**

在 [`internal/httpserver/admin_handler_test.go`](internal/httpserver/admin_handler_test.go) 补齐或收敛到以下最小断言：

```go
const testAdminToken = "admin-secret"

func newAuthenticatedAdminHandler(service *controlplane.Service) *AdminHandler {
    return NewAdminHandler(service).WithAdminToken(testAdminToken)
}

func authorizeAdminRequest(req *http.Request, token string) {
    req.Header.Set("Authorization", "Bearer "+token)
}
```

新增/保留这些测试名与断言方向：

```go
func TestAdminHandlerRejectsRequestsWhenTokenNotConfigured(t *testing.T)
func TestAdminHandlerRejectsRequestsWithoutAuthorizationHeader(t *testing.T)
func TestAdminHandlerRejectsRequestsWithMalformedAuthorizationHeader(t *testing.T)
func TestAdminHandlerRejectsRequestsWithWrongToken(t *testing.T)
func TestAdminHandlerAllowsRequestsWithValidBearerToken(t *testing.T)
```

关键断言：

```go
if rr.Code != http.StatusUnauthorized {
    t.Fatalf("expected status %d, got %d, body=%s", http.StatusUnauthorized, rr.Code, rr.Body.String())
}
```

成功路径断言：

```go
if rr.Code != http.StatusOK {
    t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
}
```

- [ ] **Step 2: 运行定向测试，确认当前基线失败或未完全覆盖**

Run:

```bash
go test ./internal/httpserver -run 'TestAdminHandler(RejectsRequestsWhenTokenNotConfigured|RejectsRequestsWithoutAuthorizationHeader|RejectsRequestsWithMalformedAuthorizationHeader|RejectsRequestsWithWrongToken|AllowsRequestsWithValidBearerToken)$' -count=1
```

Expected：若基线未完全实现，会出现 FAIL；若这些测试已在基线存在，则确认它们仍准确表达本次最小行为。

- [ ] **Step 3: 如测试缺失则补最小辅助函数与断言**

只添加本次鉴权需要的 helper，不扩展新的 fixture 层；请求路径统一使用 admin 只读端点：

```go
req := httptest.NewRequest(http.MethodGet, "/admin/audit-events", nil)
```

成功测试使用：

```go
recorder := audit.NewRecorder()
recorder.RecordRelease("router", "tenant-a", "prod", "cfg_rel_prod", "release-bot", "promote staging to prod")
handler := NewAdminHandler(controlplane.NewService()).WithAuditReader(recorder).WithAdminToken(testAdminToken)
```

- [ ] **Step 4: 重新运行定向测试，确保表达需求且可读**

Run:

```bash
go test ./internal/httpserver -run 'TestAdminHandler(RejectsRequestsWhenTokenNotConfigured|RejectsRequestsWithoutAuthorizationHeader|RejectsRequestsWithMalformedAuthorizationHeader|RejectsRequestsWithWrongToken|AllowsRequestsWithValidBearerToken)$' -count=1
```

Expected：PASS 或只剩实现层失败。

- [ ] **Step 5: 提交这一小步**

```bash
git add internal/httpserver/admin_handler_test.go
git commit -m "test: cover admin bearer token auth"
```

### Task 2: 在 handler 中实现最小 Bearer token 校验

**Files:**
- Modify: [`internal/httpserver/admin_handler.go`](internal/httpserver/admin_handler.go)
- Test: [`internal/httpserver/admin_handler_test.go`](internal/httpserver/admin_handler_test.go)

- [ ] **Step 1: 确认最小实现边界**

只保留这些结构：

```go
type AdminHandler struct {
    mux        *http.ServeMux
    service    *controlplane.Service
    auditor    auditEventReader
    runtime    runtimeEventReader
    adminToken string
}
```

和注入点：

```go
func (h *AdminHandler) WithAdminToken(token string) *AdminHandler {
    h.adminToken = token
    return h
}
```

- [ ] **Step 2: 在 [`ServeHTTP()`](internal/httpserver/admin_handler.go:115) 实现统一拦截**

目标代码形状：

```go
func (h *AdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if strings.HasPrefix(r.URL.Path, "/admin/") && !h.isAuthorizedAdminRequest(r) {
        writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
        return
    }
    h.mux.ServeHTTP(w, r)
}
```

- [ ] **Step 3: 实现最小私有校验函数**

目标代码形状：

```go
func (h *AdminHandler) isAuthorizedAdminRequest(r *http.Request) bool {
    if strings.TrimSpace(h.adminToken) == "" {
        return false
    }

    authorization := strings.TrimSpace(r.Header.Get("Authorization"))
    if !strings.HasPrefix(authorization, "Bearer ") {
        return false
    }

    token := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
    if token == "" {
        return false
    }

    return token == h.adminToken
}
```

约束：

- 不支持其他 scheme
- 不拆分详细错误
- 不做 `403`
- 不影响非 `/admin/*` 路由

- [ ] **Step 4: 运行定向测试验证实现**

Run:

```bash
go test ./internal/httpserver -run 'TestAdminHandler(RejectsRequestsWhenTokenNotConfigured|RejectsRequestsWithoutAuthorizationHeader|RejectsRequestsWithMalformedAuthorizationHeader|RejectsRequestsWithWrongToken|AllowsRequestsWithValidBearerToken)$' -count=1
```

Expected：PASS。

- [ ] **Step 5: 提交这一小步**

```bash
git add internal/httpserver/admin_handler.go internal/httpserver/admin_handler_test.go
git commit -m "feat: add admin bearer token auth"
```

## Chunk 2: Verify 与全量验证

### Task 3: 为 verify 流程补最小 Bearer token 回归

**Files:**
- Modify: [`cmd/verify/inheritance_promotion/main.go`](cmd/verify/inheritance_promotion/main.go)
- Modify: [`internal/httpserver/admin_handler.go`](internal/httpserver/admin_handler.go)（只在需要对照时阅读，不应新增额外逻辑）
- Test: [`cmd/verify/inheritance_promotion/main.go`](cmd/verify/inheritance_promotion/main.go)

- [ ] **Step 1: 在 verify 构造 handler 时注入 token**

保持最小常量：

```go
const adminToken = "admin-secret"
```

handler 构造保持：

```go
handler := httpserver.NewAdminHandler(svc).
    WithAuditReader(recorder).
    WithRuntimeReader(publisher).
    WithAdminToken(adminToken)
```

- [ ] **Step 2: 让 admin 请求 helper 自动带 Bearer token**

在 [`cmd/verify/inheritance_promotion/main.go`](cmd/verify/inheritance_promotion/main.go) 的请求 helper 中统一设置：

```go
req.Header.Set("Authorization", "Bearer "+adminToken)
```

只对 admin API 请求 helper 修改，不额外改业务断言。

- [ ] **Step 3: 保留一个错误 token 返回 401 的最小回归**

保留/补齐这类断言：

```go
expectUnauthorizedStatus(
    handler,
    http.MethodGet,
    "/admin/audit-events",
    nil,
    http.StatusUnauthorized,
    "wrong admin token should return 401",
)
```

如果 helper 当前默认总是写正确 token，则为该断言单独构造错误 header，避免扩散改动。

- [ ] **Step 4: 运行 verify 程序确认端到端主路径**

Run:

```bash
go run ./cmd/verify/inheritance_promotion
```

Expected：程序正常退出，无 `fail(...)` 输出；同时错误 token 检查仍返回 `401`。

- [ ] **Step 5: 提交这一小步**

```bash
git add cmd/verify/inheritance_promotion/main.go
git commit -m "test: cover admin auth in inheritance promotion verify"
```

### Task 4: 运行最小全量验证并整理结果

**Files:**
- Verify only: [`internal/httpserver/admin_handler.go`](internal/httpserver/admin_handler.go)
- Verify only: [`internal/httpserver/admin_handler_test.go`](internal/httpserver/admin_handler_test.go)
- Verify only: [`cmd/verify/inheritance_promotion/main.go`](cmd/verify/inheritance_promotion/main.go)

- [ ] **Step 1: 运行 httpserver 包测试**

Run:

```bash
go test ./internal/httpserver/... -count=1
```

Expected：PASS。

- [ ] **Step 2: 再运行 verify 回归**

Run:

```bash
go run ./cmd/verify/inheritance_promotion
```

Expected：PASS。

- [ ] **Step 3: 做一次差异检查**

Run:

```bash
git diff -- internal/httpserver/admin_handler.go internal/httpserver/admin_handler_test.go cmd/verify/inheritance_promotion/main.go
```

Expected：只出现本计划范围内的最小改动。

- [ ] **Step 4: 汇总最终验证结论**

记录应包含：

- [`/admin/*`](internal/httpserver/admin_handler.go:116) 已统一受保护
- 未配置 token 也会返回 `401`
- 仅接受 `Authorization: Bearer <token>`
- 最小单测与 verify 回归均通过

- [ ] **Step 5: 最终提交**

```bash
git add internal/httpserver/admin_handler.go internal/httpserver/admin_handler_test.go cmd/verify/inheritance_promotion/main.go docs/plans/2026-03-24-admin-token-auth-design.md docs/superpowers/plans/2026-03-24-admin-token-auth-implementation.md
git commit -m "feat: add minimal admin token auth"
```
