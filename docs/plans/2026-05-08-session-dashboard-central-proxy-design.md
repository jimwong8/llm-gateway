# llm-gateway Session Dashboard Central Proxy Design

**Date:** 2026-05-08

## Goal

让 `llm-gateway` 的 Admin UI 在**不修改前端现有调用方式**的前提下，通过 Go 后端代理接入新的 `central-session-server`：`http://10.100.1.13:8443`，从而让现有 Dashboard 页面能够正确读取 `/api/v1/admin/dashboard` 数据。

## Context

当前前端 `web/admin/src/lib/api/session.ts` 通过相对路径请求：

```ts
apiRequest('/api/v1/admin/dashboard')
```

这意味着请求会先进入 `llm-gateway` 自身后端。

但当前 Go 后端中：
- 没有 `/api/v1/admin/dashboard` 路由
- 没有 central-session-server 的 base URL 配置
- 没有到 central-session-server 的代理逻辑

因此，前端 session dashboard 虽然已经完成了类型、页面和组件实现，但实际上没有可用的数据来源。

## Chosen Approach

选择 **Go 后端代理** 方案。

### Why this approach

相比“前端直连”或“外部反向代理”两种方式，Go 后端代理有以下优势：

1. **前端零改动**
   - 继续使用相对路径 `/api/v1/admin/dashboard`
   - 不需要修改 `DashboardPage.tsx`、`session.ts` 或 `http.ts`

2. **避免跨域与浏览器侧鉴权复杂度**
   - 不把 central-session-server 的 token 暴露到浏览器环境
   - 维持现有 `llm-gateway` Admin API 的统一认证模式

3. **错误处理集中在后端**
   - 上游 central-session-server 不可用时，可以在 Go 后端统一映射为 502/504/500
   - 保持前端现有“session dashboard 区块失败但整体 dashboard 仍显示”的退化体验

4. **更符合现有架构**
   - 当前 Admin UI 所有数据都通过同源后端相对路径访问
   - 这个方案延续现有设计，不引入额外基础设施依赖

## Non-Goals

本次设计**不包括**：
- 直接改前端去请求 `10.100.1.13:8443`
- 部署 nginx 或外部 API gateway 来做转发
- 改造 central-session-server 的业务接口结构
- 改造 llm-gateway 的其它 admin 接口

## Proposed Architecture

数据流如下：

```text
Browser Admin UI
  -> GET /api/v1/admin/dashboard
  -> llm-gateway Go backend
  -> proxy to http://10.100.1.13:8443/api/v1/admin/dashboard
  -> return JSON response to browser
```

浏览器仍只与 `llm-gateway` 通信；`llm-gateway` 作为内部代理，负责与 `central-session-server` 通信。

## Required Backend Changes

### 1. Add central-session-server config

**File:** `internal/config/config.go`

新增两个配置项：

- `SessionServerBaseURL string`
- `SessionServerAuthToken string`

对应环境变量建议：

```bash
SESSION_SERVER_BASE_URL=http://10.100.1.13:8443
SESSION_SERVER_AUTH_TOKEN=<central-session-server token>
```

### 2. Add session dashboard proxy route

**File:** `internal/httpserver/server.go`

新增路由：

```go
mux.HandleFunc("/api/v1/admin/dashboard", s.requireAdmin(s.adminSessionDashboardProxy))
```

此路由：
- 只允许已通过 `requireAdmin(...)` 的管理请求访问
- 仅支持 `GET`
- 把请求转发到：
  - `SESSION_SERVER_BASE_URL + "/api/v1/admin/dashboard"`

### 3. Add proxy handler behavior

新增 handler 的职责：

- 校验配置是否存在
  - `SESSION_SERVER_BASE_URL`
  - `SESSION_SERVER_AUTH_TOKEN`
- 构造上游请求
- 注入上游鉴权头：

```http
X-Auth-Token: <SESSION_SERVER_AUTH_TOKEN>
```

- 设置超时（建议 5-10 秒）
- 读取 central-session-server 响应并原样返回 JSON
- 映射错误：
  - 配置缺失 -> `500`
  - 上游超时 -> `504`
  - 上游连接失败 -> `502`
  - 上游返回非 200 -> 保留状态码并透传 body（尽量保留调试信息）

### 4. Reuse existing frontend as-is

以下前端文件保持不变：

- `web/admin/src/lib/api/session.ts`
- `web/admin/src/lib/http.ts`
- `web/admin/src/pages/DashboardPage.tsx`
- `web/admin/src/components/dashboard/DashboardSessionOpsSection.tsx`
- `web/admin/src/types/sessionDashboard.ts`

## Security Design

### Browser-side auth

浏览器到 `llm-gateway`：
- 继续使用现有 admin 鉴权机制（`X-Admin-Key` / `Authorization`）

### Upstream auth

`llm-gateway` 到 `central-session-server`：
- 使用单独配置的 `SESSION_SERVER_AUTH_TOKEN`
- 由 Go 后端服务端注入
- 不暴露给前端浏览器

### Rationale

这样做能保持两层鉴权边界分离：
- 前端只知道 `llm-gateway` 的 admin key
- `central-session-server` token 仅存在于服务端环境变量中

## Error Handling

设计要求：

1. **不能拖垮整个 Dashboard 页面**
   - 若 session dashboard 失败，`DashboardAdminOverviewSection` 仍应继续展示
   - 这与现有前端测试预期一致

2. **后端代理应给出明确的错误语义**
   - `502`：上游 central-session-server 不可达
   - `504`：上游超时
   - `500`：llm-gateway 本地配置错误

3. **日志必须可追踪**
   - 记录上游 URL
   - 记录状态码和失败原因
   - 避免记录敏感 token

## Validation Plan

### Backend validation

- 配置存在时，`GET /api/v1/admin/dashboard` 能返回 central-session-server 的 JSON
- 未通过 admin 鉴权时，返回 `401`
- 配置缺失时，返回 `500`
- 上游不可达时，返回 `502/504`

### Frontend validation

- 现有 Dashboard 页面无需改动即可显示 session dashboard 数据
- 上游失败时，保留现有退化行为：session block 报错，但 dashboard overview 仍显示

### Integration validation

至少覆盖以下场景：
- `SESSION_SERVER_BASE_URL=http://10.100.1.13:8443`
- `SESSION_SERVER_AUTH_TOKEN=<有效 token>`
- central-session-server 正常运行时返回 `200`
- central-session-server token 错误时返回鉴权错误

## Alternatives Considered

### Alternative 1: Frontend direct connect

不采用，原因：
- 需要处理跨域
- central-session-server token 将泄露到浏览器侧设计
- 环境切换复杂度更高

### Alternative 2: External reverse proxy

不采用，原因：
- 依赖仓库外基础设施
- 可维护性较差
- 本次改动目标是让代码内部自解释、可落地

## Files Expected to Change in Implementation

- `internal/config/config.go`
- `internal/httpserver/server.go`
- 可能新增一个小的 proxy/helper 文件（如果实现上为了保持 server.go 简洁）
- 相关 Go 测试文件

## Recommended Next Step

进入 implementation planning，生成精确到文件和验证命令的实现计划。
