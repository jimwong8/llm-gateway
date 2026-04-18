# 2026-03-24 Admin Token Auth Design

## 背景

目标是在 [`internal/httpserver/admin_handler.go`](internal/httpserver/admin_handler.go) 上，从当前稳定基线小步加入最小固定 token 鉴权骨架，范围严格限制为：

- 仅支持 `Authorization: Bearer <token>`
- 通过 [`WithAdminToken()`](internal/httpserver/admin_handler.go:101) 注入固定 token
- 保护全部 `/admin/*`
- 统一返回 `401 Unauthorized`
- 补最小测试与最小 verify 回归

用户已确认默认行为采用最严格策略：未配置 token 时，也拒绝全部 `/admin/*`。

## 目标

1. 在 [`AdminHandler`](internal/httpserver/admin_handler.go:24) 内实现单点前置鉴权。
2. 不改动各个 admin 业务 handler 的内部逻辑。
3. 所有失败场景统一返回相同的未授权响应，不暴露具体失败原因。
4. 以最小测试面覆盖成功与失败主路径。

## 非目标

以下内容明确不在本次范围内：

- 多用户、多 token、角色模型
- 配置中心或环境变量装配策略扩展
- 通用 HTTP 中间件抽象
- 非 `/admin/*` 路由的认证或授权
- `403 Forbidden`、错误码细分、审计增强

## 方案选择

### 方案 A：在 [`AdminHandler`](internal/httpserver/admin_handler.go:24) 内单点前置校验（推荐）

在 [`WithAdminToken()`](internal/httpserver/admin_handler.go:101) 注入固定 token，并在 [`ServeHTTP()`](internal/httpserver/admin_handler.go:115) 对全部 `/admin/*` 请求执行 Bearer token 校验。校验通过后，再转发给内部 mux。

优点：

- 改动最小，严格符合本次目标
- 不影响已有 handler 业务代码
- 风险低，便于从稳定基线推进

缺点：

- 鉴权逻辑耦合在 [`AdminHandler`](internal/httpserver/admin_handler.go:24) 内
- 后续若升级为通用认证机制，仍需重构

### 方案 B：按路由包装 admin 专用鉴权层

在 [`registerRoutes()`](internal/httpserver/admin_handler.go:106) 为每个 admin 路由包一层鉴权包装器。

优点：结构更清晰。

缺点：对当前最小目标来说改动偏大，收益有限。

### 方案 C：在外层 server/router 统一拦截

把 `/admin/*` 鉴权上提到更外层路由装配处。

优点：长期架构更统一。

缺点：超出本次严格限制范围，不适合当前小步修改。

## 最终设计

### 1. 结构

保持 [`AdminHandler`](internal/httpserver/admin_handler.go:24) 作为唯一改动入口。

- 使用已有字段 [`adminToken`](internal/httpserver/admin_handler.go:29) 保存固定 token
- 使用 [`WithAdminToken()`](internal/httpserver/admin_handler.go:101) 作为注入点
- 不改变 [`registerRoutes()`](internal/httpserver/admin_handler.go:106) 的 admin 路由集合

### 2. 请求数据流

1. 请求进入 [`ServeHTTP()`](internal/httpserver/admin_handler.go:115)
2. 若路径前缀匹配 `/admin/`，先进入鉴权逻辑
3. 从 `Authorization` 请求头读取值
4. 严格要求格式为 `Bearer <token>`
5. 将请求 token 与 [`adminToken`](internal/httpserver/admin_handler.go:29) 做固定值比对
6. 校验成功后，才调用 [`h.mux.ServeHTTP()`](internal/httpserver/admin_handler.go:120)
7. 非 `/admin/*` 请求不受影响

### 3. 失败处理

以下场景统一视为未授权：

- 未配置 token
- 缺少 `Authorization` 头
- scheme 不是 `Bearer`
- `Bearer` 后 token 为空
- token 不匹配

所有失败场景统一返回：

- HTTP 状态码：`401 Unauthorized`
- 响应体：`{"error":"unauthorized"}`

这样可以避免向调用方泄露失败原因，保持最小骨架的一致性。

### 4. 测试策略

在 [`internal/httpserver/admin_handler_test.go`](internal/httpserver/admin_handler_test.go) 增加或保持最小测试面：

- 未配置 token 时访问 `/admin/*` 返回 `401`
- 未带 `Authorization` 头返回 `401`
- 错误或畸形 `Authorization` 头返回 `401`
- 错误 token 返回 `401`
- 正确 Bearer token 可以访问 admin 路由

测试重点是鉴权前置分支，不扩展到更多与本次目标无关的业务细节。

### 5. Verify 回归

在 [`cmd/verify/inheritance_promotion/main.go`](cmd/verify/inheritance_promotion/main.go) 做最小适配：

- 构造 [`AdminHandler`](internal/httpserver/admin_handler.go:24) 时注入固定 token
- 所有 admin API helper 自动附带 Bearer token
- 保留一个错误 token 返回 `401` 的断言

这样可以确认 admin API 的端到端路径已被统一保护。

## 风险与回滚

### 风险

- 若忘记为 verify 或测试请求补 Bearer token，现有 admin 请求会统一失败为 `401`
- 若某些调用方依赖“未配置 token 时放行”的旧行为，会被本次最严格策略打断

### 回滚方式

若需要快速回退，只需移除 [`ServeHTTP()`](internal/httpserver/admin_handler.go:115) 的前置鉴权分支，以及 [`WithAdminToken()`](internal/httpserver/admin_handler.go:101) 相关测试适配。

## 实施边界

本次只允许修改：

- [`internal/httpserver/admin_handler.go`](internal/httpserver/admin_handler.go)
- [`internal/httpserver/admin_handler_test.go`](internal/httpserver/admin_handler_test.go)
- [`cmd/verify/inheritance_promotion/main.go`](cmd/verify/inheritance_promotion/main.go)

不额外引入新包，不扩展配置系统，不重构 router 层。
