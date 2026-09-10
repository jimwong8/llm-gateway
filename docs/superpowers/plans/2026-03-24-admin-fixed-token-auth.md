# 管理接口最小固定 Token 鉴权 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 [`/admin/*`](internal/httpserver/admin_handler.go:99) 管理接口增加最小固定 token 鉴权骨架，使用 `Authorization: Bearer <token>` 保护管理面，并在未配置 token 时默认拒绝访问。

**Architecture:** 方案在 [`AdminHandler`](internal/httpserver/admin_handler.go:23) 内部增加 `adminToken` 字段与 [`WithAdminToken()`](internal/httpserver/admin_handler.go:80) 样式注入点，在 [`ServeHTTP()`](internal/httpserver/admin_handler.go:108) 前统一拦截全部 `/admin/*` 请求。鉴权逻辑仅停留在 HTTP 层，不改控制面，不引入配置系统，失败统一返回 `401 Unauthorized`。

**Tech Stack:** Go, net/http, Bearer token header parsing, admin HTTP handlers, httptest, Go tests, 现有 verify 脚本

---

## 文件结构与职责

- [`internal/httpserver/admin_handler.go`](internal/httpserver/admin_handler.go)  
  管理接口主入口。需要新增固定 token 注入字段、Bearer token 校验逻辑、统一未授权响应。
- [`internal/httpserver/admin_handler_test.go`](internal/httpserver/admin_handler_test.go)  
  管理接口 HTTP 测试。需要补鉴权成功/失败路径测试，并确认现有管理接口在授权通过时仍正常工作。
- [`cmd/verify/inheritance_promotion/main.go`](cmd/verify/inheritance_promotion/main.go)  
  API 化回归脚本。需要在开启鉴权后，为请求统一附带 Bearer token，并补一组最小未授权回归。
- [`docs/plans/2026-03-24-admin-fixed-token-auth-design.md`](docs/plans/2026-03-24-admin-fixed-token-auth-design.md)  
  已确认的鉴权设计依据，实施时所有边界应与该文档保持一致。

## Chunk 1: HTTP 骨架与最小鉴权语义

### Task 1: 先写失败测试，锁定最小 Bearer token 语义

**Files:**
- Modify: [`internal/httpserver/admin_handler_test.go`](internal/httpserver/admin_handler_test.go)
- Test: [`internal/httpserver/`](internal/httpserver)

- [ ] **Step 1: 写失败测试，验证未配置 token 时全部 `/admin/*` 默认返回 `401`**

优先选一个代表性接口，例如 `POST /admin/inheritance-drafts` 或 `GET /admin/audit-events`。

示例断言：

```go
func TestAdminHandlerRejectsRequestsWhenTokenNotConfigured(t *testing.T) {
    handler := NewAdminHandler(controlplane.NewService())
    req := httptest.NewRequest(http.MethodGet, "/admin/audit-events", nil)
    rr := httptest.NewRecorder()

    handler.ServeHTTP(rr, req)

    if rr.Code != http.StatusUnauthorized {
        t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
    }
}
```

- [ ] **Step 2: 写失败测试，验证缺失 `Authorization` Header 返回 `401`**

要求：
- 已注入 token
- 请求不带 Header
- 返回 `401`

- [ ] **Step 3: 写失败测试，验证 Header 不是 `Bearer <token>` 格式时返回 `401`**

覆盖至少一个场景：
- `Basic abc`
- `Bearer`
- 空字符串

- [ ] **Step 4: 写失败测试，验证 token 不匹配时返回 `401`**

- [ ] **Step 5: 写失败测试，验证 token 正确时原有接口行为不变**

要求：
- 例如 `POST /admin/inheritance-drafts` 成功返回 `201`
- 或 `GET /admin/runtime-events` 成功返回 `200`

- [ ] **Step 6: 运行测试确认失败**

Run: `go test ./internal/httpserver/...`

Expected: FAIL，错误应集中在缺少 token 注入点或缺少鉴权逻辑。

- [ ] **Step 7: 提交**

```bash
git add internal/httpserver/admin_handler_test.go
git commit -m "test: define fixed token admin auth behavior"
```

### Task 2: 为 [`AdminHandler`](internal/httpserver/admin_handler.go:23) 增加 token 注入点

**Files:**
- Modify: [`internal/httpserver/admin_handler.go`](internal/httpserver/admin_handler.go)
- Test: [`internal/httpserver/admin_handler_test.go`](internal/httpserver/admin_handler_test.go)

- [ ] **Step 1: 在 [`AdminHandler`](internal/httpserver/admin_handler.go:23) 上新增 `adminToken` 字段**

要求：
- 保持现有字段布局清晰
- 不影响已有 audit/runtime reader 注入能力

- [ ] **Step 2: 增加 [`WithAdminToken()`](internal/httpserver/admin_handler.go:80) 风格的 setter**

示例最小实现：

```go
func (h *AdminHandler) WithAdminToken(token string) *AdminHandler {
    h.adminToken = token
    return h
}
```

- [ ] **Step 3: 让测试从“缺少注入点”推进到“缺少鉴权检查”**

- [ ] **Step 4: 运行测试**

Run: `go test ./internal/httpserver/...`

Expected: FAIL，但失败点应收敛到鉴权逻辑未实现。

- [ ] **Step 5: 提交**

```bash
git add internal/httpserver/admin_handler.go internal/httpserver/admin_handler_test.go
git commit -m "feat: add admin token injection point"
```

## Chunk 2: 统一拦截与错误响应

### Task 3: 在 [`ServeHTTP()`](internal/httpserver/admin_handler.go:108) 前统一拦截全部 `/admin/*`

**Files:**
- Modify: [`internal/httpserver/admin_handler.go`](internal/httpserver/admin_handler.go)
- Test: [`internal/httpserver/admin_handler_test.go`](internal/httpserver/admin_handler_test.go)

- [ ] **Step 1: 新增最小 Bearer token 解析函数**

建议新增一个小函数，例如：
- `authenticateAdminRequest()`
- 或 `validateAdminAuthorizationHeader()`

要求：
- 输入：`*http.Request`
- 输出：是否通过 / 错误原因

- [ ] **Step 2: 在 [`ServeHTTP()`](internal/httpserver/admin_handler.go:108) 中，对 `/admin/` 前缀做统一鉴权检查**

推荐顺序：
1. 路径是否以 `/admin/` 开头
2. `adminToken` 是否已配置
3. Header 是否存在且为 `Bearer <token>`
4. token 是否匹配
5. 通过后才进入现有 `mux`

- [ ] **Step 3: 未授权时统一返回 `401` + 简单错误体**

建议错误体：

```json
{
  "error": "unauthorized"
}
```

- [ ] **Step 4: 运行测试**

Run: `go test ./internal/httpserver/...`

Expected: PASS，鉴权相关测试与既有接口测试都通过。

- [ ] **Step 5: 提交**

```bash
git add internal/httpserver/admin_handler.go internal/httpserver/admin_handler_test.go
git commit -m "feat: enforce fixed token auth for admin routes"
```

### Task 4: 补足格式错误与边界场景测试

**Files:**
- Modify: [`internal/httpserver/admin_handler_test.go`](internal/httpserver/admin_handler_test.go)

- [ ] **Step 1: 为 `Authorization` 格式错误补多个子场景**

建议表驱动覆盖：
- `Authorization: Basic abc`
- `Authorization: Bearer`
- `Authorization: Bearer `
- `Authorization: Token abc`

- [ ] **Step 2: 为未配置 token 场景锁定默认拒绝行为**

确认：
- `WithAdminToken()` 未调用时
- 任意管理接口都应 `401`

- [ ] **Step 3: 为查询接口补授权通过用例**

至少覆盖：
- `GET /admin/audit-events`
- `GET /admin/runtime-events`

- [ ] **Step 4: 运行测试**

Run: `go test ./internal/httpserver/...`

Expected: PASS，授权与未授权行为全部通过。

- [ ] **Step 5: 提交**

```bash
git add internal/httpserver/admin_handler_test.go
git commit -m "test: cover admin auth failure cases"
```

## Chunk 3: 回归脚本与文档同步

### Task 5: 更新 API 回归脚本，使其适配鉴权

**Files:**
- Modify: [`cmd/verify/inheritance_promotion/main.go`](cmd/verify/inheritance_promotion/main.go)

- [ ] **Step 1: 在回归脚本中为 [`AdminHandler`](internal/httpserver/admin_handler.go:23) 注入固定 token**

示例：

```go
handler := httpserver.NewAdminHandler(svc).
    WithAuditReader(recorder).
    WithRuntimeReader(publisher).
    WithAdminToken("admin-secret")
```

- [ ] **Step 2: 为所有 API 请求统一附带 `Authorization: Bearer <token>`**

需要覆盖：
- `postVersion()`
- `getVersion()`
- `getAuditEvents()`
- `getRuntimeEvents()`
- 失败路径 helper

- [ ] **Step 3: 增加一个最小未授权回归校验**

建议：
- 用错误 token 请求 `GET /admin/audit-events`
- 断言返回 `401`

- [ ] **Step 4: 运行 verify**

Run: `go run ./cmd/verify/inheritance_promotion`

Expected: PASS，并明确显示授权通过路径正常、未授权路径被拒绝。

- [ ] **Step 5: 提交**

```bash
git add cmd/verify/inheritance_promotion/main.go
git commit -m "test: update verify flow for admin token auth"
```

### Task 6: 文档回填鉴权边界

**Files:**
- Modify: [`docs/plans/2026-03-24-admin-fixed-token-auth-design.md`](docs/plans/2026-03-24-admin-fixed-token-auth-design.md)
- Modify: [`docs/superpowers/plans/2026-03-24-cross-environment-inheritance-and-promotion-composition.md`](docs/superpowers/plans/2026-03-24-cross-environment-inheritance-and-promotion-composition.md)

- [ ] **Step 1: 在设计文档中补“已实现后状态”小节**

写明：
- 使用 `Authorization: Bearer <token>`
- 未配置 token 默认拒绝
- `/admin/*` 全部受保护

- [ ] **Step 2: 在实施进展文档中增加鉴权切片回填**

至少写明：
- 已完成的文件
- 已完成的测试
- verify 已适配鉴权

- [ ] **Step 3: 补一个最小请求示例**

例如：

```http
GET /admin/audit-events HTTP/1.1
Authorization: Bearer admin-secret
```

- [ ] **Step 4: 提交**

```bash
git add docs
git commit -m "docs: document fixed token auth for admin routes"
```

## 最终回归

### Task 7: 做一轮完整回归并收口

**Files:**
- Test: [`internal/httpserver/`](internal/httpserver)
- Test: [`cmd/verify/inheritance_promotion/main.go`](cmd/verify/inheritance_promotion/main.go)

- [ ] **Step 1: 运行 HTTP 层测试**

Run: `go test ./internal/httpserver/...`

Expected: PASS

- [ ] **Step 2: 运行核心相关测试**

Run: `go test ./internal/controlplane/... ./internal/httpserver/... ./internal/audit/... ./internal/runtime/...`

Expected: PASS

- [ ] **Step 3: 运行 API 回归脚本**

Run: `go run ./cmd/verify/inheritance_promotion`

Expected: PASS，并包含授权通过与未授权拒绝的验证输出。

- [ ] **Step 4: 收口提交**

```bash
git add .
git commit -m "feat: add fixed token auth for admin routes"
```

## 实施注意事项

- 不要改 [`internal/controlplane/`](internal/controlplane) 层的业务接口，这一轮只在 HTTP 层加最小安全边界。
- 不要引入配置系统、环境变量、JWT、RBAC 或多 token 支持。
- 鉴权应优先放在 [`ServeHTTP()`](internal/httpserver/admin_handler.go:108) 总入口，避免散落到每个 handler。
- 只保护 `/admin/*`，不要影响非管理面路由。
- 失败返回统一保持简单，优先复用现有 [`errorResponse`](internal/httpserver/admin_handler.go:76) 结构。

## 完成定义

实现完成后，应满足：

- 所有 [`/admin/*`](internal/httpserver/admin_handler.go:99) 请求都需要 `Authorization: Bearer <token>`
- 未配置 token 时全部管理接口默认 `401`
- Header 缺失、格式错误、token 不匹配时统一 `401`
- token 正确时现有 draft / release / promotion / query 能力保持可用
- HTTP 测试与 verify 全部通过

Plan complete and saved to [`docs/superpowers/plans/2026-03-24-admin-fixed-token-auth.md`](docs/superpowers/plans/2026-03-24-admin-fixed-token-auth.md). Ready to execute?