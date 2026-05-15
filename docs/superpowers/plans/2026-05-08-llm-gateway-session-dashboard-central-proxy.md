# llm-gateway Session Dashboard Central Proxy Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 `llm-gateway` 增加到 `central-session-server` 的后端代理链路，让现有 Admin UI 通过 `/api/v1/admin/dashboard` 读取 `http://10.100.1.13:8443/api/v1/admin/dashboard`，且前端无需改动。

**Architecture:** 在 Go 后端增加两个配置项：`SESSION_SERVER_BASE_URL` 和 `SESSION_SERVER_AUTH_TOKEN`，并新增一个受现有 admin 鉴权保护的代理 handler。该 handler 负责把前端的相对路径请求转发到 `central-session-server`，统一处理超时、上游错误和响应透传。

**Tech Stack:** Go, net/http, llm-gateway internal config/httpserver, React frontend (no changes), curl

---

## File Structure / Responsibilities

- `internal/config/config.go` — 新增 session server 上游配置字段与环境变量加载
- `internal/httpserver/server.go` — 注册 `/api/v1/admin/dashboard` 路由并实现代理逻辑
- `cmd/server_main.go` — 确保新的配置路径在服务启动时可用（若 `cfg` 已整体注入，则仅验证无需改）
- `web/admin/src/lib/api/session.ts` — 保持现有相对路径调用，不修改，仅作为验证参考
- `web/admin/src/pages/DashboardPage.tsx` — 保持现有 query 逻辑，不修改，仅作为验证参考
- `docs/plans/2026-05-08-session-dashboard-central-proxy-design.md` — 设计来源

---

## Chunk 1: 配置层新增 session server 上游参数

### Task 1: 新增 `SESSION_SERVER_BASE_URL` 与 `SESSION_SERVER_AUTH_TOKEN`

**Files:**
- Modify: `internal/config/config.go`
- Test: 如项目已有 config 单元测试则补充；若无，则在 `internal/httpserver` 相关测试中间接覆盖

- [ ] **Step 1: 写一个失败验证（最小可执行）**

如果项目已有配置测试文件，则新增断言：
- `SESSION_SERVER_BASE_URL` 能加载到 Config
- `SESSION_SERVER_AUTH_TOKEN` 能加载到 Config

如果项目没有现成配置测试文件，则先在计划执行时用临时小测试验证：

```go
func TestConfigLoadsSessionServerFields(t *testing.T) {
    t.Setenv("SESSION_SERVER_BASE_URL", "http://10.100.1.13:8443")
    t.Setenv("SESSION_SERVER_AUTH_TOKEN", "token-123")
    cfg := config.Load()
    if cfg.SessionServerBaseURL != "http://10.100.1.13:8443" {
        t.Fatalf("unexpected base url: %s", cfg.SessionServerBaseURL)
    }
    if cfg.SessionServerAuthToken != "token-123" {
        t.Fatalf("unexpected auth token: %s", cfg.SessionServerAuthToken)
    }
}
```

- [ ] **Step 2: 跑测试确认当前失败**

Run:
```bash
go test ./... -run TestConfigLoadsSessionServerFields
```
Expected: 编译失败或字段不存在

- [ ] **Step 3: 在 `internal/config/config.go` 新增字段与加载逻辑**

新增字段：
```go
SessionServerBaseURL string
SessionServerAuthToken string
```

新增加载逻辑（建议做 `strings.TrimRight(..., "/")` 归一化）：
```go
SessionServerBaseURL: strings.TrimRight(getenv("SESSION_SERVER_BASE_URL", ""), "/"),
SessionServerAuthToken: getenv("SESSION_SERVER_AUTH_TOKEN", ""),
```

- [ ] **Step 4: 重新跑测试确认通过**

Run:
```bash
go test ./... -run TestConfigLoadsSessionServerFields
```
Expected: PASS

- [ ] **Step 5: 提交阶段性变更**

```bash
git add internal/config/config.go
git commit -m "feat: add session dashboard upstream config"
```

---

## Chunk 2: 为 `/api/v1/admin/dashboard` 增加后端代理路由

### Task 2: 新增受 admin 保护的 session dashboard 代理 handler

**Files:**
- Modify: `internal/httpserver/server.go`
- Test: `internal/httpserver/server_test.go` 或现有 httpserver 测试文件（若存在）

- [ ] **Step 1: 写失败测试，证明当前没有该路由或行为不正确**

建议新增 httptest 测试，目标覆盖：
1. 未带 admin key -> 401
2. 配置缺失 -> 500
3. 上游成功 -> 200 + 返回 JSON

最小示例：

```go
func TestSessionDashboardProxyRequiresAdmin(t *testing.T) {
    srv := New(testConfig(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
    req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard", nil)
    rr := httptest.NewRecorder()
    srv.Handler().ServeHTTP(rr, req)
    if rr.Code != http.StatusUnauthorized {
        t.Fatalf("expected 401, got %d", rr.Code)
    }
}
```

再写一个带 admin key + mock upstream 的测试：

```go
func TestSessionDashboardProxySuccess(t *testing.T) {
    upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("X-Auth-Token") != "session-token" {
            t.Fatalf("missing upstream auth token")
        }
        w.Header().Set("Content-Type", "application/json")
        _, _ = w.Write([]byte(`{"overall_status":"ok"}`))
    }))
    defer upstream.Close()

    cfg := testConfig()
    cfg.AdminAPIKey = "admin-key"
    cfg.SessionServerBaseURL = upstream.URL
    cfg.SessionServerAuthToken = "session-token"

    srv := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
    req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard", nil)
    req.Header.Set("X-Admin-Key", "admin-key")
    rr := httptest.NewRecorder()

    srv.Handler().ServeHTTP(rr, req)

    if rr.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", rr.Code)
    }
    if !strings.Contains(rr.Body.String(), "overall_status") {
        t.Fatalf("unexpected body: %s", rr.Body.String())
    }
}
```

- [ ] **Step 2: 跑测试确认失败**

Run:
```bash
go test ./internal/httpserver -run SessionDashboardProxy
```
Expected: FAIL（路由/handler 不存在）

- [ ] **Step 3: 在 `server.go` 注册新路由**

在 `Handler()` 中新增：
```go
mux.HandleFunc("/api/v1/admin/dashboard", s.requireAdmin(s.adminSessionDashboardProxy))
```

- [ ] **Step 4: 实现代理 handler（最小实现）**

建议在 `server.go` 中新增方法，或抽到一个小 helper，职责包括：
- 限制只接受 `GET`
- 检查 `s.cfg.SessionServerBaseURL` 和 `s.cfg.SessionServerAuthToken` 非空
- 构造上游 URL：`s.cfg.SessionServerBaseURL + "/api/v1/admin/dashboard"`
- 使用 `http.NewRequestWithContext`
- 注入 `X-Auth-Token`
- 使用带超时的 `http.Client`
- 拷贝 `Content-Type` 与状态码
- 透传 body

最小结构示意：

```go
func (s *Server) adminSessionDashboardProxy(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    if strings.TrimSpace(s.cfg.SessionServerBaseURL) == "" || strings.TrimSpace(s.cfg.SessionServerAuthToken) == "" {
        writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": "session server config missing"}})
        return
    }
    // build upstream request, inject X-Auth-Token, do request, copy response
}
```

- [ ] **Step 5: 增加错误映射测试**

至少再补两类测试：
- 上游连接失败 -> `502 Bad Gateway`
- 上游超时 -> `504 Gateway Timeout`（如果实现了 context deadline / client timeout 映射）

- [ ] **Step 6: 重新跑测试确认通过**

Run:
```bash
go test ./internal/httpserver -run SessionDashboardProxy
```
Expected: PASS

- [ ] **Step 7: 提交阶段性变更**

```bash
git add internal/httpserver/server.go
git commit -m "feat: proxy session dashboard to central server"
```

---

## Chunk 3: 验证启动路径与配置注入

### Task 3: 确认 `cmd/server_main.go` 与 `New(cfg, ...)` 路径无需额外改动，或补齐缺口

**Files:**
- Inspect / Modify: `cmd/server_main.go`
- Inspect: `internal/httpserver/server.go`

- [ ] **Step 1: 检查 `cmd/server_main.go` 是否已经完整把 `cfg` 传给 `httpserver.New(...)`**

Run:
```bash
grep -n "config.Load\|httpserver.New" cmd/server_main.go
```
Expected: `cfg := config.Load()` 且 `httpserver.New(cfg, ...)` 使用同一个配置对象

- [ ] **Step 2: 如果 `cfg` 已整体传入，则记录“无需修改”**

无需代码改动时，在提交说明或实施记录中明确说明：
- `SessionServerBaseURL` / `SessionServerAuthToken` 会自动跟随 `cfg` 生效

- [ ] **Step 3: 如果启动路径丢失新字段，则补最小改动并验证编译**

Run:
```bash
go test ./cmd/... ./internal/httpserver/...
```
Expected: PASS

- [ ] **Step 4: 提交（如有代码改动）**

```bash
git add cmd/server_main.go
git commit -m "chore: wire session dashboard upstream config"
```

---

## Chunk 4: 集成验证与前端零改动验证

### Task 4: 验证前端无需改动且 Dashboard 继续工作

**Files:**
- Verify: `web/admin/src/lib/api/session.ts`
- Verify: `web/admin/src/pages/DashboardPage.tsx`
- Optional test touch: `web/admin/src/pages/DashboardPage.test.tsx`

- [ ] **Step 1: 确认前端 API 保持相对路径不变**

检查：
```ts
fetchSessionDashboard() {
  return apiRequest<SessionAdminDashboard>('/api/v1/admin/dashboard')
}
```
Expected: 无需任何修改

- [ ] **Step 2: 运行 Go 测试与前端测试**

Run:
```bash
go test ./...
cd web/admin && npm test -- --runInBand DashboardPage.test.tsx
```
Expected: 通过；尤其保留“session dashboard 失败时 overview 仍可见”的测试语义

- [ ] **Step 3: 做一次手动集成验证（本地或测试环境）**

设置环境变量：
```bash
export SESSION_SERVER_BASE_URL=http://10.100.1.13:8443
export SESSION_SERVER_AUTH_TOKEN=<central-session-server token>
```

然后启动 `llm-gateway`，执行：
```bash
curl -H 'X-Admin-Key: admin-dev-key' http://127.0.0.1:8080/api/v1/admin/dashboard
```
Expected: 返回 central-session-server 的 dashboard JSON

- [ ] **Step 4: 验证退化行为**

把 `SESSION_SERVER_BASE_URL` 暂时改错后再次请求：
```bash
curl -H 'X-Admin-Key: admin-dev-key' http://127.0.0.1:8080/api/v1/admin/dashboard
```
Expected: 返回 502/504/500；前端保留已有错误展示逻辑

- [ ] **Step 5: 最终提交**

```bash
git add internal/config/config.go internal/httpserver/server.go cmd/server_main.go
# 如果有测试改动，也一并 add
git commit -m "feat: connect admin dashboard to central session server"
```

---

## Notes / Guardrails

1. **不要修改前端相对路径调用。** 设计已确认前端零改动是目标。
2. **不要把 central-session-server token 透给浏览器。** 只能由 Go 后端服务端注入 `X-Auth-Token`。
3. **优先最小改动。** 先只打通 `/api/v1/admin/dashboard`，不要顺手扩展到更多 central-session-server 路由。
4. **错误映射要明确。** 上游不可达/超时必须可区分，便于后续运维。
5. **日志不要打印敏感 token。** 可记录上游 URL、状态码、耗时，但不要记录 `SESSION_SERVER_AUTH_TOKEN`。

---

Plan complete and saved to `docs/superpowers/plans/2026-05-08-llm-gateway-session-dashboard-central-proxy.md`. Ready to execute?
