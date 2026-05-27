# LLM Gateway Comprehensive Optimization Master Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 llm-gateway 从“企业级管理型 LLM 网关”升级为兼具高性能路由、用户自助 SaaS、流式聊天、商业化计费、长期记忆与治理闭环的一体化 AI Gateway 平台。

**Architecture:** 在现有 Go 1.22+ 单体服务、PostgreSQL、Redis、Qdrant、React 18 Admin Console 基础上渐进演进，优先补齐数据面可靠性与用户面 P0 能力，不重写现有治理/审计/缓存架构。新增能力按 bounded context 拆分为 routing、identity、chat、billing、memory、frontend-platform、observability 七条可独立验证的工作流。

**Tech Stack:** Go 1.22+、PostgreSQL、Redis、Qdrant、React 18/TypeScript/Vite、TanStack Query、Vitest、Go table-driven tests、Puppeteer/Playwright E2E、OpenAI-compatible HTTP/SSE API。

---

## 0. 背景与输入来源

本计划综合以下研究与现有代码状态：

- `BIFROST_PORTKEY_COMPARISON.md`
  - Bifrost：泛型重试、Key 池轮转、流式首块错误检测、对象池、高性能路由。
  - Portkey：条件路由、Hook 管道、Guardrails、企业级可观测。
- `COAI_COMPARISON.md`
  - CoAI：用户体系、WebSocket/SSE 聊天、订阅/兑换码/邀请码、用户级 API Key、Mask、文件解析、广播通知。
- `NEW_API_COMPARISON.md`
  - new-api：完整用户端 SaaS 功能、钱包/充值、用户日志、OAuth/2FA/Passkey、现代前端栈。
- `SESSION_MEMORY_RESEARCH.md`
  - 10.100.1.13：L1 Atom / L2 Scenario / L3 Persona 记忆金字塔、Hybrid Search、KG、上下文预算。
- 当前代码结构：
  - 后端：`internal/router`、`internal/providers`、`internal/cache`、`internal/memory`、`internal/governance`、`internal/billing`、`internal/audit`、`internal/httpserver`、`internal/admin`。
  - 前端：`web/admin/src/pages`、`web/admin/src/components`、`web/admin/src/lib/api`、`web/admin/src/types`。
  - 已有基础：OpenAI-compatible API、Provider adapters、policy/governance、quota、billing repository、audit、L1 cache、semantic/memory/asset 概念、admin console。

## 1. 总体判断

llm-gateway 不应该照搬 new-api/CoAI 做一次前后端重写。当前项目已经具备较强的企业级控制面、治理面和多层缓存/记忆方向基础，其短板主要是：

1. **用户侧 SaaS 能力不足**：目前偏管理员控制台，缺少注册登录、用户 API Key、钱包、订阅、用户调用日志、聊天工作台。
2. **数据面可靠性不足**：已有 provider/router 基础，但缺少 Bifrost 风格的统一重试、Key 池轮转、流式首块错误检测、条件路由和 Hook 管道。
3. **商业化闭环不完整**：已有 billing repository 迹象，但缺少完整 plan/subscription/wallet/redemption/payment ledger。
4. **前端工程化仍偏原生**：已有 React/Vite/TS/TanStack Query/Vitest，但缺少统一表单、校验、toast、状态管理、主题、权限路由、用户端布局。
5. **长期记忆尚未产品化**：已有 `internal/memory` 与 Memory Governance 页面，但还没有 10.100.1.13 级别的 Atom/Scenario/Persona/Hybrid Search/Context Budget 产品闭环。

因此本计划采用：

- **先数据面稳定，再用户面增长，再商业化，再智能记忆，再前端现代化**。
- 所有阶段都必须能独立上线、独立回滚、独立验收。
- 保持 OpenAI-compatible API 兼容性；任何新增能力不得破坏 `/v1/chat/completions`、`/v1/models`、`/v1/embeddings` 的现有调用路径。

## 2. 目标分层

### 2.1 P0：必须先做

P0 是平台可被真实用户使用、可被稳定收费、可被安全运营的最低闭环。

- Routing Reliability v2
  - Provider Key 池
  - 泛型/统一重试执行器
  - Rate-limit 感知轮转
  - 网络错误重试
  - 流式首块错误检测
  - 条件 fallback 与 retry audit
- Identity & User API Key
  - 用户表、密码登录、JWT session
  - 用户级 API Key
  - Admin/User 角色隔离
  - 用户调用日志与 usage 归属
- Streaming Chat MVP
  - 用户聊天页面
  - SSE 流式输出
  - 多模型选择
  - 会话历史
- Billing MVP
  - 钱包余额
  - 模型倍率/价格
  - 每次调用扣费 ledger
  - 余额不足阻断
- Verification Harness
  - OpenAI contract tests
  - Routing unit/integration tests
  - Admin/User E2E smoke
  - Load smoke baseline

### 2.2 P1：规模化运营

- 订阅套餐、兑换码、邀请码
- OAuth GitHub、邮箱验证、找回密码
- 用户 dashboard、调用日志搜索、成本趋势
- 条件路由 DSL、Hook 管道、Guardrails MVP
- Memory Pyramid MVP：Atom/Scenario/Persona
- 前端引入 react-hook-form、zod、sonner、统一 UI primitives
- 国际化基础：中文/英文

### 2.3 P2：高级差异化

- WebSocket 双通道聊天
- 文件解析、搜索增强、Mask/Prompt 预设
- Hybrid BM25 + Vector + RRF 记忆搜索
- Context Budget Engine
- 高级支付渠道、发票/对账、组织/团队
- React 19 / Tailwind / shadcn / Zustand 迁移评估
- Performance object pooling、allocation profiling、stream backpressure

### 2.4 P3：企业级与生态

- Passkey/WebAuthn、2FA、OIDC/SAML
- 多区域 provider health 与就近路由
- 企业审计导出、合规保留策略增强
- Plugin marketplace / provider adapter SDK
- Desktop/PWA/移动端体验
- 自动化模型质量评估、漂移告警闭环

## 3. 工作流边界与文件结构

### 3.1 Routing Reliability v2

**责任：** 请求选择、重试、fallback、key 轮转、provider error 分类、stream first chunk 检测。

预计文件：

- Modify: `internal/router/router.go`
- Modify: `internal/router/policy.go`
- Create: `internal/router/retry.go`
- Create: `internal/router/retry_test.go`
- Create: `internal/router/key_pool.go`
- Create: `internal/router/key_pool_test.go`
- Create: `internal/router/error_classification.go`
- Create: `internal/router/error_classification_test.go`
- Modify: `internal/providers/provider.go`
- Modify: `internal/providers/registry.go`
- Test: `internal/router/router_test.go`
- Test: `internal/providers/provider_test.go`

接口原则：

- Provider 适配器只负责调用上游和返回标准错误。
- Router 负责候选 provider/key 选择与 retry/fallback 决策。
- Retry executor 不能依赖 HTTP handler，便于单元测试。
- 流式请求必须在首块错误检测后再将 stream 暴露给 client。

### 3.2 Identity & User API Key

**责任：** 用户身份、JWT、用户级 API Key、角色权限、tenant/user 归属。

预计文件：

- Create: `internal/identity/user.go`
- Create: `internal/identity/password.go`
- Create: `internal/identity/jwt.go`
- Create: `internal/identity/api_key.go`
- Create: `internal/identity/postgres.go`
- Create: `internal/identity/*_test.go`
- Modify: `internal/httpserver/server.go`
- Create: `internal/httpserver/auth_handler.go`
- Create: `internal/httpserver/user_key_handler.go`
- Modify: `internal/tenant/keys.go`
- Create: `migrations/identity_*.sql` or existing migration location if present
- Frontend create: `web/admin/src/pages/user/LoginPage.tsx`
- Frontend create: `web/admin/src/pages/user/SignupPage.tsx`
- Frontend create: `web/admin/src/pages/user/ApiKeysPage.tsx`
- Frontend create: `web/admin/src/lib/api/identity.ts`
- Frontend create: `web/admin/src/types/identity.ts`

接口原则：

- 保留现有管理员 fixed bearer token 作为 break-glass 入口。
- 新增用户 JWT 不替代 admin token，先并存。
- 用户 API Key 使用 OpenAI 风格 bearer key，映射到 user_id、tenant_id、plan_id。
- API Key 明文只在创建时返回；数据库只存 hash。

### 3.3 Streaming Chat MVP

**责任：** 用户聊天工作台、SSE 流式响应、会话历史、多模型选择。

预计文件：

- Modify: `internal/httpserver/server.go`
- Create: `internal/httpserver/chat_handler.go`
- Create: `internal/chat/session.go`
- Create: `internal/chat/postgres.go`
- Create: `internal/chat/stream.go`
- Create: `internal/chat/*_test.go`
- Modify: `internal/providers/provider.go` if stream interface incomplete
- Frontend create: `web/admin/src/pages/user/ChatPage.tsx`
- Frontend create: `web/admin/src/components/chat/MessageList.tsx`
- Frontend create: `web/admin/src/components/chat/Composer.tsx`
- Frontend create: `web/admin/src/hooks/useSseChat.ts`
- Frontend create: `web/admin/src/lib/api/chat.ts`
- Frontend create: `web/admin/src/types/chat.ts`

接口原则：

- Chat API 内部复用 `/v1/chat/completions` 的数据面路径，不另写 provider 调用逻辑。
- SSE 遵循 OpenAI stream chunk 习惯，便于兼容 SDK。
- 会话历史入库与扣费 ledger 解耦；流结束后写 usage summary。

### 3.4 Billing MVP

**责任：** 钱包、价格、倍率、扣费、余额不足阻断、ledger。

预计文件：

- Modify: `internal/billing/postgres.go`
- Create: `internal/billing/wallet.go`
- Create: `internal/billing/pricing.go`
- Create: `internal/billing/ledger.go`
- Create: `internal/billing/service.go`
- Create: `internal/billing/*_test.go`
- Modify: `internal/httpserver/chat_completions...` current handler file
- Create: `internal/httpserver/billing_handler.go`
- Frontend create: `web/admin/src/pages/user/BillingPage.tsx`
- Frontend create: `web/admin/src/pages/admin/PricingPage.tsx`
- Frontend create: `web/admin/src/lib/api/billing.ts`
- Frontend create: `web/admin/src/types/billing.ts`

接口原则：

- 先做内部余额与手动充值，不先接 Stripe。
- 所有扣费必须 ledger 化：可重放、可审计、可补偿。
- 估算费用与实际费用都要记录，实际以 provider usage 为准，缺失时用 tokenizer/估算规则。

### 3.5 Memory Productization

**责任：** 将现有 memory/governance 能力产品化为 Atom/Scenario/Persona 与上下文预算。

预计文件：

- Modify: `internal/memory/postgres.go`
- Create: `internal/memory/atom.go`
- Create: `internal/memory/scenario.go`
- Create: `internal/memory/persona.go`
- Create: `internal/memory/context_budget.go`
- Create: `internal/memory/hybrid_search.go`
- Create: `internal/memory/*_test.go`
- Modify: `internal/httpserver/memory_admin_handler.go`
- Create: `internal/httpserver/memory_user_handler.go`
- Frontend modify: `web/admin/src/pages/MemoryGovernancePage.tsx`
- Frontend create: `web/admin/src/pages/user/MemoryPage.tsx`

接口原则：

- 先做 PostgreSQL-only keyword search + recency/rank；Qdrant/hybrid 放到 P2。
- 记忆写入必须带 confidence、source_message_ids、sensitivity、superseded_by。
- 任何自动记忆进入候选状态，用户/管理员确认后进入长期记忆。

### 3.6 Frontend Platform Modernization

**责任：** 减少页面级重复代码，提高用户端与管理端一致性。

预计文件：

- Modify: `web/admin/package.json` if exists under web/admin
- Modify: `web/admin/src/router.tsx`
- Modify: `web/admin/src/components/layout/AppShell.tsx`
- Create: `web/admin/src/components/ui/Button.tsx`
- Create: `web/admin/src/components/ui/Input.tsx`
- Create: `web/admin/src/components/ui/Dialog.tsx`
- Create: `web/admin/src/components/ui/ToastProvider.tsx`
- Create: `web/admin/src/lib/forms.ts`
- Create: `web/admin/src/lib/permissions.ts`

原则：

- 不在 P0 强制 Tailwind/shadcn 全量迁移。
- 先统一 primitives、toast、form validation、权限路由。
- React 19/Rsbuild/Tailwind v4 作为 P2 迁移评估，不阻塞 P0 业务。

## 4. 分阶段路线图

## Chunk 1: P0 Stabilize Data Plane & User Entry

### Phase 0.1: Baseline Freeze and Safety Net

**目标：** 在动核心路径前建立可重复验证基线。

- [ ] **Step 1: 记录当前测试基线**

Run:

```bash
go test ./...
```

Expected: 记录通过/失败列表。若已有非本计划导致的失败，写入 `docs/plans/p0-baseline-known-failures.md`。

- [ ] **Step 2: 记录前端测试基线**

Run:

```bash
cd web/admin && npm test -- --runInBand
```

Expected: 记录 Vitest 当前通过/失败情况。

- [ ] **Step 3: 增加 OpenAI contract smoke**

Create: `cmd/verify/openai_contract.go` or follow existing verify pattern.

覆盖：

- `GET /v1/models`
- `POST /v1/chat/completions` non-stream
- `POST /v1/chat/completions` stream
- auth missing -> 401
- invalid model -> documented 4xx

- [ ] **Step 4: 建立 E2E smoke 脚本**

Create: `docs/testing/admin-user-e2e-smoke.md`

包含：

- Admin login
- Provider list/edit/health/discovery
- User login once available
- User API key creation once available
- Chat once available

- [ ] **Step 5: Commit**

```bash
git add docs/testing docs/plans cmd/verify
git commit -m "test: add gateway baseline verification plan"
```

### Phase 0.2: Routing Reliability v2

**目标：** 实现 Bifrost 风格可靠请求执行，不改变外部 API。

- [ ] **Step 1: 写 KeyPool 单元测试**

Test file: `internal/router/key_pool_test.go`

Cases:

- unused key is selected first
- rate-limited key is excluded during current retry window
- all keys exhausted resets used set
- disabled/unhealthy key is skipped
- weighted random respects zero weight exclusion

- [ ] **Step 2: 实现 KeyPool**

Create: `internal/router/key_pool.go`

Minimal API:

```go
type ProviderKey struct {
    ID string
    Provider string
    SecretRef string
    Weight int
    Enabled bool
}

type KeyPool interface {
    Next(ctx context.Context, provider string, used map[string]bool) (ProviderKey, error)
}
```

- [ ] **Step 3: 写错误分类测试**

Test file: `internal/router/error_classification_test.go`

Cases:

- HTTP 429 -> rate_limit
- HTTP 408/502/503/504 -> retryable_network_or_upstream
- context canceled -> non_retryable_client_cancel
- invalid api key -> non_retryable_auth
- stream first chunk provider error -> retryable when classified 429/5xx

- [ ] **Step 4: 实现 error classification**

Create: `internal/router/error_classification.go`

- [ ] **Step 5: 写 RetryExecutor 测试**

Test file: `internal/router/retry_test.go`

Cases:

- success first attempt returns once
- 429 rotates key
- network error retries same provider/key until max retry
- all keys exhausted resets cycle
- non-retryable auth error stops immediately
- stream first chunk error triggers retry before client receives chunks

- [ ] **Step 6: 实现 RetryExecutor**

Create: `internal/router/retry.go`

Target shape:

```go
type ExecuteFunc[T any] func(ctx context.Context, key ProviderKey) (T, error)

type RetryExecutor struct {
    MaxRetries int
    KeyPool KeyPool
    Classifier ErrorClassifier
    Recorder RetryRecorder
}

func (e *RetryExecutor) Execute[T any](ctx context.Context, provider string, fn ExecuteFunc[T]) (T, RetryResult, error)
```

Note: 如果 Go 当前版本/项目风格不允许 method type parameter，改为 package-level generic function。

- [ ] **Step 7: 接入现有 router**

Modify: `internal/router/router.go`

要求：

- 默认行为保持兼容。
- 未配置 key pool 时使用现有 provider credentials。
- retry audit 写入 structured log；P1 再入库。

- [ ] **Step 8: 跑测试**

Run:

```bash
go test ./internal/router ./internal/providers ./internal/httpserver
```

Expected: PASS。

- [ ] **Step 9: 手动验证 OpenAI contract**

Run verify command from Phase 0.1。

- [ ] **Step 10: Commit**

```bash
git add internal/router internal/providers internal/httpserver
git commit -m "feat: add retry-aware routing executor"
```

### Phase 0.3: Identity & User API Key MVP

**目标：** 用户能注册/登录、创建自己的 API Key，并调用 OpenAI-compatible API。

- [ ] **Step 1: 写用户 repository 测试**

Create: `internal/identity/postgres_test.go`

Cases:

- create user with unique email
- duplicate email rejected
- password hash not equal plaintext
- verify password success/failure
- create API key returns plaintext once and stores hash
- list keys never returns plaintext
- revoke key prevents auth

- [ ] **Step 2: 实现 identity domain**

Create files under `internal/identity`。

- [ ] **Step 3: 写 auth handler 测试**

Create: `internal/httpserver/auth_handler_test.go`

Cases:

- signup creates user
- login returns JWT
- invalid password returns 401
- JWT middleware attaches user context
- admin token still works for admin routes

- [ ] **Step 4: 实现 auth handlers**

Create: `internal/httpserver/auth_handler.go`
Modify: `internal/httpserver/server.go`

Routes proposal:

- `POST /api/auth/signup`
- `POST /api/auth/login`
- `GET /api/auth/me`
- `POST /api/user/api-keys`
- `GET /api/user/api-keys`
- `DELETE /api/user/api-keys/{id}`

- [ ] **Step 5: 接入 API key middleware**

Modify existing OpenAI-compatible auth path so bearer user API key maps to user/tenant context。

- [ ] **Step 6: 前端 API client**

Create:

- `web/admin/src/lib/api/identity.ts`
- `web/admin/src/types/identity.ts`

- [ ] **Step 7: 前端页面**

Create:

- `web/admin/src/pages/user/LoginPage.tsx`
- `web/admin/src/pages/user/SignupPage.tsx`
- `web/admin/src/pages/user/ApiKeysPage.tsx`

Modify:

- `web/admin/src/router.tsx`
- `web/admin/src/components/layout/Sidebar.tsx`

- [ ] **Step 8: 前端测试**

Create tests:

- login success stores JWT
- signup validation
- API key create shows plaintext once
- revoked key disappears or shows revoked status

- [ ] **Step 9: 跑测试**

```bash
go test ./internal/identity ./internal/httpserver
cd web/admin && npm test -- --runInBand
```

- [ ] **Step 10: Commit**

```bash
git add internal/identity internal/httpserver web/admin/src
git commit -m "feat: add user identity and api keys"
```

### Phase 0.4: Streaming Chat MVP

**目标：** 用户端可进行真实 SSE 流式聊天。

- [ ] **Step 1: 写 chat session repository 测试**

Create: `internal/chat/postgres_test.go`

Cases:

- create session
- append user/assistant messages
- list recent sessions by user
- fetch messages by session and user
- user cannot read other user's session

- [ ] **Step 2: 实现 chat repository**

Create: `internal/chat/session.go`, `internal/chat/postgres.go`。

- [ ] **Step 3: 写 SSE handler 测试**

Create: `internal/httpserver/chat_handler_test.go`

Cases:

- non-authenticated rejected
- stream response has `text/event-stream`
- chunks are forwarded in order
- final usage summary recorded
- provider error returns SSE error event

- [ ] **Step 4: 实现 chat handler**

Create: `internal/httpserver/chat_handler.go`

Routes:

- `POST /api/chat/sessions`
- `GET /api/chat/sessions`
- `GET /api/chat/sessions/{id}/messages`
- `POST /api/chat/sessions/{id}/messages:stream`

- [ ] **Step 5: 前端 hook 测试**

Create: `web/admin/src/hooks/useSseChat.test.tsx`

Cases:

- appends user message optimistically
- handles streamed chunks
- handles SSE error event
- abort stops stream

- [ ] **Step 6: 实现前端聊天页面**

Create:

- `web/admin/src/pages/user/ChatPage.tsx`
- `web/admin/src/components/chat/MessageList.tsx`
- `web/admin/src/components/chat/Composer.tsx`
- `web/admin/src/hooks/useSseChat.ts`
- `web/admin/src/lib/api/chat.ts`

- [ ] **Step 7: E2E smoke**

Use browser:

- Login
- Open Chat
- Select model
- Send “hello”
- Observe streaming text
- Refresh and verify history

- [ ] **Step 8: Commit**

```bash
git add internal/chat internal/httpserver web/admin/src
git commit -m "feat: add streaming chat MVP"
```

### Phase 0.5: Billing MVP

**目标：** 每次用户调用都能归属、计价、扣费、审计。

- [ ] **Step 1: 写 pricing tests**

Create: `internal/billing/pricing_test.go`

Cases:

- input/output token rates calculate cost
- model override beats provider default
- unknown model uses fallback price or rejects depending config

- [ ] **Step 2: 写 wallet ledger tests**

Create: `internal/billing/ledger_test.go`

Cases:

- credit increases balance
- debit decreases balance
- insufficient balance rejects
- idempotency key prevents double charge
- refund compensates original ledger item

- [ ] **Step 3: 实现 billing service**

Create/modify `internal/billing/*`。

- [ ] **Step 4: 接入 chat completions path**

Modify existing chat completions handler:

- Before request: check balance if user API key.
- After response: record usage and debit.
- On stream: reserve estimate before stream, settle actual after final chunk; if actual missing, use estimate.

- [ ] **Step 5: 前端用户账单页**

Create:

- `web/admin/src/pages/user/BillingPage.tsx`
- `web/admin/src/lib/api/billing.ts`
- `web/admin/src/types/billing.ts`

- [ ] **Step 6: Admin pricing page**

Create:

- `web/admin/src/pages/admin/PricingPage.tsx`

- [ ] **Step 7: 跑测试**

```bash
go test ./internal/billing ./internal/httpserver
cd web/admin && npm test -- --runInBand
```

- [ ] **Step 8: Commit**

```bash
git add internal/billing internal/httpserver web/admin/src
git commit -m "feat: add wallet billing MVP"
```

## Chunk 2: P1 Product & Operations Expansion

### Phase 1.1: Subscription, Redemption, Invitation

**目标：** 从手工钱包升级到可运营套餐。

Tasks:

- [ ] Add tables/domain for `plans`, `subscriptions`, `redemption_codes`, `invitation_codes`.
- [ ] Tests for plan activation, expiration, quota reset, redemption idempotency.
- [ ] Admin pages for plan CRUD and redemption batch generation.
- [ ] User pages for subscription status and code redemption.
- [ ] Billing service integrates plan quota before wallet debit.
- [ ] E2E: admin creates code -> user redeems -> quota increases -> ledger records.

Acceptance:

- 用户可通过兑换码获得余额或套餐。
- 过期套餐不再提供权益。
- 所有权益变化都有 ledger/audit。

### Phase 1.2: User Dashboard and Logs

**目标：** 用户能看到自己的调用量、成本、错误和延迟。

Tasks:

- [ ] Extend audit/usage schema with user_id/api_key_id/session_id.
- [ ] Add user usage query API with pagination and filters.
- [ ] Add dashboard cards: balance, requests, tokens, cost, error rate.
- [ ] Add log table: time/model/provider/status/tokens/cost/request_id.
- [ ] Tests for user isolation.

Acceptance:

- 用户不能看到其他用户日志。
- Admin 可按用户/模型/provider 聚合。

### Phase 1.3: Conditional Routing and Hook Pipeline

**目标：** 引入 Portkey 风格企业路由能力。

Tasks:

- [ ] Define route condition schema: model, tenant, user group, budget, latency, health, metadata.
- [ ] Implement condition evaluator with table-driven tests.
- [ ] Add hook interfaces: before_route, before_provider, after_provider, after_response, on_error.
- [ ] Guardrail MVP: reject by prompt regex/sensitivity flag.
- [ ] Admin UI for route rule preview and dry-run.

Acceptance:

- 管理员能创建条件路由规则。
- 每次命中规则有 runtime decision trace。
- Hook 失败策略明确：fail-open/fail-closed 可配置。

### Phase 1.4: Memory Pyramid MVP

**目标：** 将记忆从后台治理能力变成用户可见能力。

Tasks:

- [ ] Add Atom model with kind/confidence/source/sensitivity/dedup.
- [ ] Add Scenario model referencing atoms.
- [ ] Add Persona profile facts.
- [ ] Add candidate -> confirm -> promote lifecycle.
- [ ] Chat pipeline writes memory candidates after conversation.
- [ ] User Memory page supports view/confirm/delete/export.

Acceptance:

- Chat 后能生成候选记忆。
- 用户确认后，后续 chat context 可召回该记忆。
- 敏感记忆默认不自动注入。

### Phase 1.5: Frontend UX Foundation

**目标：** 统一用户端体验，减少 alert/useState 式页面膨胀。

Tasks:

- [ ] Add Sonner or existing-compatible toast provider.
- [ ] Add react-hook-form + zod for new forms.
- [ ] Add UI primitives: Button/Input/Dialog/Table/Pagination.
- [ ] Add permission-aware route guards.
- [ ] Add i18n skeleton zh-CN/en-US.
- [ ] Refactor only newly touched pages first; do not mass rewrite old admin pages.

Acceptance:

- 新页面不使用 `alert()`。
- 表单都有 schema validation。
- User/Admin 导航清晰分离。

## Chunk 3: P2 Advanced Intelligence & Performance

### Phase 2.1: Hybrid Memory Search and Context Budget

Tasks:

- [ ] Add keyword scorer.
- [ ] Add vector scorer via existing Qdrant/semantic path.
- [ ] Add RRF rank fusion.
- [ ] Add context budget state machine: tiny/small/normal/large.
- [ ] Add injection planner: recent messages + persona + scenarios + atoms + assets.
- [ ] Tests for deterministic budget allocation.

Acceptance:

- 给定 token budget，context planner 输出稳定且可解释。
- Memory injection 有 trace，用户/admin 可查看。

### Phase 2.2: File, Search, Mask Features

Tasks:

- [ ] Mask/preset CRUD and chat apply.
- [ ] File upload metadata and text extraction abstraction.
- [ ] Optional SearXNG web search connector.
- [ ] Chat supports attachments/search context display.

Acceptance:

- 文件和搜索都作为 context sources 进入 trace。
- 超预算内容被摘要或截断，不能无限塞入 prompt。

### Phase 2.3: WebSocket and Realtime Operations

Tasks:

- [ ] Add WebSocket gateway for chat/control notifications.
- [ ] Broadcast system announcement.
- [ ] Admin realtime provider health updates.
- [ ] User realtime balance/log update.

Acceptance:

- SSE 仍保留；WebSocket 是增强不是替代。
- 断线重连不重复扣费/不重复写消息。

### Phase 2.4: Performance Engineering

Tasks:

- [ ] Add benchmark tests for router selection/retry path.
- [ ] Add pprof runbook.
- [ ] Evaluate sync.Pool only for measured hot allocations.
- [ ] Stream backpressure tests.
- [ ] Load test baseline: 100/500/1000 concurrent requests.

Acceptance:

- 任何对象池引入都必须有 benchmark 证明。
- P95/P99 latency 和 allocation/op 有记录。

## Chunk 4: P3 Enterprise and Ecosystem

### Phase 3.1: Enterprise Authentication

Tasks:

- [ ] OAuth GitHub first.
- [ ] 2FA TOTP.
- [ ] Passkey/WebAuthn.
- [ ] OIDC/SAML evaluation.

### Phase 3.2: Enterprise Governance

Tasks:

- [ ] Compliance retention policy per tenant.
- [ ] Audit export scheduling.
- [ ] Approval workflow for route/billing/model changes.
- [ ] Drift alert -> recommendation -> approval -> rollout loop.

### Phase 3.3: Provider Adapter SDK

Tasks:

- [ ] Stabilize provider interface.
- [ ] Add adapter conformance tests.
- [ ] Add docs for new provider.
- [ ] Add marketplace-like registry metadata.

## 5. Cross-Cutting Testing Strategy

### 5.1 Go Unit Tests

Required for every domain service:

```bash
go test ./internal/<module>
```

Patterns:

- Table-driven tests.
- No network in unit tests.
- Fake provider/fake clock/fake ledger for deterministic behavior.
- Race-prone code must pass `go test -race` for target package.

### 5.2 Integration Tests

Required for:

- Auth middleware + HTTP routes.
- Billing debit on completion.
- Routing fallback chain.
- Memory recall injection.

Command pattern:

```bash
go test ./internal/httpserver ./internal/router ./internal/billing ./internal/memory
```

### 5.3 Contract Tests

OpenAI-compatible contract must include:

- model list shape
- chat completion non-stream shape
- chat completion stream chunks
- error object shape
- auth error behavior
- unsupported model behavior

These tests protect external SDK compatibility.

### 5.4 Frontend Tests

Required for every new page/hook:

```bash
cd web/admin && npm test -- --runInBand
```

Required cases:

- loading state
- empty state
- success state
- validation error
- server error
- permission denied if route is protected

### 5.5 Browser E2E

Use Playwright if profile is available; Puppeteer is acceptable fallback.

P0 E2E smoke:

1. Admin login.
2. Provider health/discovery/edit.
3. User signup/login.
4. User creates API key.
5. User sends non-stream OpenAI request with API key.
6. User opens chat and receives stream.
7. Billing balance decreases.
8. User sees usage log.

### 5.6 Load and Reliability

Minimum before P0 release:

- 100 concurrent non-stream requests.
- 100 concurrent stream requests.
- Provider 429 injection verifies key rotation.
- Provider 503 injection verifies retry/fallback.
- Client disconnect during stream verifies no goroutine leak and no double charge.

## 6. Rollout and Rollback Strategy

### 6.1 Feature Flags

Add flags for:

- `routing_retry_v2`
- `user_identity_enabled`
- `user_api_keys_enabled`
- `chat_sse_enabled`
- `billing_enforcement_enabled`
- `memory_injection_enabled`

Flags should be readable from config and visible in admin runtime observer.

### 6.2 Database Migration Safety

Rules:

- Additive migrations first.
- Backfill separately from schema change.
- No destructive migration until at least one release after code no longer reads old field.
- API keys/passwords are hash-only; never log plaintext.

### 6.3 Rollback

For every phase:

- Disable feature flag.
- Keep old admin token path.
- Keep existing provider credential path.
- Billing enforcement can switch to observe-only mode.
- Memory injection can switch to read-only/no-inject mode.

## 7. Observability Requirements

Every request should eventually carry:

- request_id
- user_id or tenant_id
- api_key_id hash prefix only
- model requested
- provider selected
- route policy id/version
- retry attempts
- fallback chain
- cache hit layer
- prompt/completion tokens
- estimated/actual cost
- ledger id if billed
- memory context ids injected

Admin UI should surface:

- routing decision trace
- billing ledger trace
- memory injection trace
- provider health over time
- retry/fallback rates

## 8. Security Requirements

- Passwords: Argon2id or bcrypt with cost documented.
- API keys: generate high entropy, store hash only, show plaintext once.
- JWT: short access token + refresh token only if needed; P0 can start with access token and explicit expiration.
- Admin token: keep break-glass, but warn if default/weak.
- Logs: never log provider API key, user API key plaintext, password, JWT.
- CORS: user-facing origins explicit.
- Rate limit: login/signup/API key creation endpoints.
- Billing: idempotency keys on charge/refund.

## 9. Acceptance Gates

### P0 Release Gate

Must pass:

- `go test ./...` or documented unrelated known failures.
- `cd web/admin && npm test -- --runInBand`.
- OpenAI contract smoke.
- Browser E2E P0 smoke.
- Billing double-charge test.
- Routing 429/503 injection test.
- Security log scan: no plaintext keys/tokens in logs.

User-visible capabilities:

- Admin can still manage providers and policies.
- User can sign up/login.
- User can create API key.
- User can call OpenAI-compatible API.
- User can chat with SSE streaming.
- User can see balance and usage.
- Calls are billed or observed according to flag.

### P1 Release Gate

Must pass P0 plus:

- Subscription/redemption E2E.
- Conditional route dry-run and live-run tests.
- Memory candidate lifecycle tests.
- i18n smoke for zh/en.

### P2 Release Gate

Must pass P1 plus:

- Hybrid memory search quality smoke.
- Context budget deterministic tests.
- WebSocket reconnect tests.
- Load benchmarks with before/after comparison.

## 10. Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Scope too large | Delivery stalls | Execute P0 only first; each phase independently releasable |
| Billing bugs double-charge users | Trust loss | Ledger idempotency, observe-only mode, refund compensation tests |
| Retry causes duplicate upstream calls | Cost increase | Retry only before response body is committed; stream first chunk gate; idempotency trace |
| Stream errors after partial output | UX inconsistency | SSE error event, final status, no retry after client-visible content except explicit policy |
| Identity breaks admin access | Lockout | Preserve fixed admin token break-glass |
| Frontend rewrite churn | Regression | Do not mass migrate UI stack in P0 |
| Memory injection leaks sensitive info | Privacy incident | sensitivity_level, explicit confirmation, injection trace, default no auto-inject for sensitive |
| Provider adapter interface churn | Many regressions | Adapter conformance tests before adding providers |

## 11. Recommended Execution Order

1. Phase 0.1 Baseline Freeze.
2. Phase 0.2 Routing Reliability v2.
3. Phase 0.3 Identity & User API Key.
4. Phase 0.4 Streaming Chat MVP.
5. Phase 0.5 Billing MVP in observe-only, then enforce mode.
6. Phase 1.2 User Dashboard and Logs.
7. Phase 1.1 Subscription/Redemption.
8. Phase 1.3 Conditional Routing/Hook Pipeline.
9. Phase 1.4 Memory Pyramid MVP.
10. Phase 1.5 Frontend UX Foundation.
11. P2/P3 as product priorities require.

## 12. Immediate Next Plan Documents

This master plan is intentionally broad. Before implementation, split P0 into executable detailed plans:

1. `docs/plans/2026-05-18-routing-reliability-v2-implementation.md`
2. `docs/plans/2026-05-18-identity-user-api-keys-implementation.md`
3. `docs/plans/2026-05-18-streaming-chat-mvp-implementation.md`
4. `docs/plans/2026-05-18-billing-mvp-implementation.md`
5. `docs/plans/2026-05-18-p0-verification-harness.md`

Each sub-plan should follow strict TDD steps with exact code snippets and commit points.

## 13. Definition of Done for This Master Plan

- [x] Synthesizes Bifrost/Portkey/CoAI/new-api/session-memory research.
- [x] Respects current llm-gateway architecture and avoids rewrite-first strategy.
- [x] Defines P0/P1/P2/P3 priorities.
- [x] Defines file-level boundaries for major subsystems.
- [x] Defines TDD-first execution tasks for P0.
- [x] Defines verification, rollout, rollback, observability, security, and acceptance gates.

Plan complete and saved to `docs/plans/2026-05-18-llm-gateway-comprehensive-optimization-master.md`. Ready to execute by first expanding the P0 Routing Reliability v2 sub-plan.
