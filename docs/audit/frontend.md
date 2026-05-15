# Admin 前端联调审计

审计范围：`/tmp/opencode/llm-gateway/web/admin`

结论摘要：

- 16 个受保护侧边栏页面里，除 `Playground` 走公开 `/v1/chat/completions` 外，其余都通过前端相对路径直接请求真实后端路由；没有发现 `src/pages` 直接引用 fixtures/mock 数据文件。
- `LoginPage` 不在侧边栏，但负责把 Admin Token 写入 `sessionStorage`。`ProtectedRoute` 仅检查本地 token 是否存在，不做服务端会话校验。
- HTTP client 没有配置显式 `baseURL`，依赖浏览器当前 origin，同源访问 `/admin/*` 和 `/v1/*`；路由 basename 是 `/admin/ui`。
- token 通过 `Authorization: Bearer <token>` 附加；前端不会发 `X-Admin-Key`。后端 `requireAdmin` 同时接受 Bearer 或 `X-Admin-Key`。
- 没有 refresh token / silent refresh 逻辑；收到 `401/403` 时前端会清掉 token，但当前页不会自动跳登录，通常需依赖后续路由守卫或用户刷新。
- 页面与 `server.go` 的主要已接线后端路由基本匹配，但存在两类缺口：
  - 后端有路由、前端无页面：`/admin/assets*`、`/admin/control-plane/compensations*`、`/admin/releases/rollback`、`/admin/releases/replay`、`/admin/governance/evaluations*`、`/admin/governance/runtime/resolve`、`/admin/governance/runtime-decisions`、`/admin/governance/distribution-events`、`/admin/observability/cache`、`/v1/models`、`/v1/runtime/resolve`。
  - 前端调用了“路由前缀存在但具体子路由是否实现取决于 handler”的接口：`/admin/governance/policy-versions/:id/diff|approve|activate`、`/admin/memory/candidate-facts/:factKey/:action`、`/admin/memory/candidate-facts/actions/:action`。`server.go` 只通过 `.../` 前缀挂载，需到对应 handler 确认具体子路径支持。
- 测试使用 Vitest + jsdom + Testing Library；项目安装了 `msw` 但当前测试未使用，主要通过 `vi.stubGlobal('fetch', ...)` 做 fetch mock。无 coverage 配置，也没有真实后端 contract 测试。

## 页面/API 矩阵

| 页面 | 侧边栏路径 | 使用 lib/hooks | 后端接口 | 测试覆盖 |
|---|---|---|---|---|
| DashboardPage | `/dashboard` | `lib/http.ts` | `GET /admin/health`; `GET /admin/observability/summary` | 有 `/tmp/opencode/llm-gateway/web/admin/src/pages/DashboardPage.test.tsx`：断言首页 heading、健康信息、缓存命中率和错误率展示 |
| ConfigCenterPage | `/config-center` | `hooks/useConfigVersions.ts`; `lib/http.ts`; `components/config/CreateDraftForm.tsx` | `GET /admin/config-versions`; `POST /admin/inheritance-drafts` | 有 `ConfigCenterPage.test.tsx`：断言加载版本列表、打开 detail drawer、提交 filter 后请求包含 query 参数 |
| ReleasesPage | `/releases` | `lib/http.ts`; `components/releases/ReleaseDraftPanel.tsx`; `components/releases/PromotionPanel.tsx` | `POST /admin/releases`; `POST /admin/promotions` | 有 `ReleasesPage.test.tsx`：断言两个工作台表单存在；发布成功后展示最近结果 |
| AuditRuntimePage | `/audit-runtime` | `hooks/useAdminEvents.ts`; `lib/http.ts` | `GET /admin/audit-events`; `GET /admin/runtime-events` | 有 `AuditRuntimePage.test.tsx`：断言 audit/runtime tab 切换；summary 模式会带 `summary=true` |
| SystemPage | `/system` | `lib/http.ts` | `GET /admin/health`; `GET /admin/usage`; `GET /admin/audit` | 有 `SystemPage.test.tsx`：点击加载后展示 health、usage、audit 汇总卡片 |
| PlaygroundPage | `/playground` | `lib/playground.ts` | `POST /v1/chat/completions` | 有 `PlaygroundPage.test.tsx`：断言发送 chat completion、展示响应元信息、可新增 message 行 |
| ObservabilityPage | `/observability` | `lib/http.ts` | `GET /admin/observability/summary`; `GET /admin/observability/providers`; `GET /admin/observability/hotspots` | 有 `ObservabilityPage.test.tsx`：断言 summary/provider/hotspot 渲染；filter 会进入请求 URL |
| QuotaPage | `/quota` | `lib/http.ts` | `GET /admin/observability/quota`; `GET /admin/observability/quota/trends` | 有 `QuotaPage.test.tsx`：断言 quota summary/trends 渲染；filter 会附带 `window_minutes` |
| PoliciesPage | `/policies` | `lib/http.ts` | `GET /admin/policies/models` | 有 `PoliciesPage.test.tsx`：断言模型列表渲染 |
| MemoryGovernancePage | `/memory-governance` | `lib/memory.ts` | `GET /admin/memory/candidate-facts`; `GET /admin/memory/project-facts`; `POST /admin/memory/candidate-facts/:factKey/:action`; `POST /admin/memory/candidate-facts/actions/:action` | 有 `MemoryGovernancePage.test.tsx`：覆盖列表、排序、详情、筛选、单条 action、批量 confirm/reject/promote、分页、本地搜索、校验与错误聚合 |
| RecommendationCenterPage | `/recommendations` | `lib/recommendations.ts`; `lib/http.ts` | `GET /admin/governance/recommendations?limit=50`; `POST /admin/governance/approvals` | 有 `RecommendationCenterPage.test.tsx`：断言推荐列表/摘要、打开审批弹窗、提交 approval、深链到 approvals |
| ApprovalsPage | `/approvals` | `lib/recommendations.ts`; `lib/http.ts` | `GET /admin/governance/recommendations?limit=50`; `POST /admin/governance/approvals` | 有 `ApprovalsPage.test.tsx`：覆盖 approve/override/reject 三类动作、表单校验、后端报错、query 参数预填 |
| PolicyVersionsPage | `/policy-versions` | `lib/policyVersions.ts` | `GET /admin/governance/policy-versions?limit=50`; `GET /admin/governance/policy-versions/:id/diff`; `POST /admin/governance/policy-versions/:id/approve`; `POST /admin/governance/policy-versions/:id/activate` | 有 `PolicyVersionsPage.test.tsx`：覆盖版本列表、diff、approve/activate、diff 不可用时的降级提示、环境深链与 rollout link |
| RolloutsPage | `/rollouts` | `lib/rollouts.ts` | `GET /admin/governance/dashboard/rollouts?limit=50`; `POST /admin/governance/rollbacks` | 有 `RolloutsPage.test.tsx`：覆盖 rollout 指标展示、回滚弹窗与提交、通过 query 参数高亮目标行 |
| RuntimeObserverPage | `/runtime-observer` | `lib/runtimeObserver.ts` | `GET /admin/governance/runtime-observer?environment=...&limit=20` | 有 `RuntimeObserverPage.test.tsx`：断言 active policy、cache、recent facts 渲染；环境筛选进入请求 URL |
| DriftDashboardPage | `/drifts` | `lib/drifts.ts` | `GET /admin/governance/drifts?limit=50` | 有 `DriftDashboardPage.test.tsx`：断言 drift 行/摘要；空列表显示 empty state |
| LoginPage | 非侧边栏，`/login` | `lib/auth.ts` | 无后端请求，仅本地写 token | 有 `LoginPage.test.tsx`：断言提交后将 token 写入 `sessionStorage` 并导航到 dashboard；空 token 报错 |

## HTTP client、base URL、token wiring

### `src/lib/http.ts`

- 文件：`/tmp/opencode/llm-gateway/web/admin/src/lib/http.ts`
- 没有 `baseURL`、`import.meta.env`、`VITE_*` 或 axios 实例。
- 所有请求都直接 `fetch(input, ...)`，且各页面/库传入的是相对路径，如 `/admin/...` 或 `/v1/...`。
- 这意味着前端默认假设：
  - 管理控制台从同一 origin 提供；
  - `/admin/ui` 仅用于前端路由 basename；
  - API 路径仍直接打到同源 `/admin/*`、`/v1/*`。

### token 如何附加

- `apiRequest()` 默认 `options.auth = 'admin'`。
- 会从 `getToken()` 读取 token，并设置 `Authorization: Bearer <token>`。
- `jsonRequest()` 只是 `apiRequest()` 的 JSON POST 包装，也沿用相同认证方式。
- `PlaygroundPage` 使用 `lib/playground.ts`，直接 `fetch('/v1/chat/completions')`，不会附加 admin token。

### 与后端的对应关系

- 后端 `requireAdmin()` 读取：
  - 首选 `X-Admin-Key`
  - 其次 `Authorization: Bearer ...`
- 所以前端 Bearer 模式是兼容的。
- 控制面 proxy 路由还会把传入的 `X-Admin-Key` 转成 Bearer，但前端当前并未使用该头。

## ProtectedRoute 与 auth flow

### 关键文件

- `/tmp/opencode/llm-gateway/web/admin/src/components/auth/ProtectedRoute.tsx`
- `/tmp/opencode/llm-gateway/web/admin/src/lib/auth.ts`
- `/tmp/opencode/llm-gateway/web/admin/src/pages/LoginPage.tsx`
- `/tmp/opencode/llm-gateway/web/admin/src/router.tsx`

### 存储位置

- token key：`llm_gateway_admin_token`
- 存储介质：`window.sessionStorage`
- 不是 `localStorage`
- 关闭 tab/session 后 token 会丢失

### 登录流程

1. 用户打开受保护路由。
2. `ProtectedRoute` 调用 `hasToken()`；本质是检查 `sessionStorage` 中是否存在非空 token。
3. 没 token 时 `Navigate` 到 `/login`，并把来源路由放到 `location.state.from`。
4. `LoginPage` 仅做本地非空校验，通过后 `setToken()` 写入 `sessionStorage`。
5. 登录后直接 `navigate(nextPath)` 回原页或 `/dashboard`。

### 刷新/续期逻辑

- 没有 refresh token 机制。
- 没有过期时间管理。
- 没有自动续期或后台 re-auth。
- 收到 `401/403` 时，`apiRequest()` 会执行 `clearToken()`；但不会主动 `navigate('/login')`。
- 因此当前 auth UX 存在缺口：接口失效后，页面大多只会报错，用户通常要再次触发路由守卫或手动刷新/跳转才会回登录页。

## 看起来偏 stub/TODO 的页面

以下页面虽然接了真实接口，但“功能完成度”仍偏薄，适合作为 TODO 候选：

1. `PoliciesPage`
   - 仅展示 `/admin/policies/models` 返回的 allowed models 列表。
   - 页面描述也明确写着“为后续策略编辑功能保留最小可用入口”。
   - 没有编辑、保存、版本化、RBAC 操作。

2. `SystemPage`
   - 需要用户点击按钮后才并发拉取三个简单接口。
   - 只展示 4 张 summary 卡片，没有 usage/audit 明细表，也没有自动刷新。
   - 更像“健康检查入口”，不是完整系统运维页。

3. `PlaygroundPage`
   - 直连真实 `/v1/chat/completions`，不是 mock；但它不走 admin auth，也没有 provider/model 列表、模板、流式响应、错误分类等增强功能。
   - 属于真实接线但功能仍轻量。

4. `DriftDashboardPage`
   - 只是列表+摘要，没有 drill-down、接受/解决 drift 的动作入口。
   - 后端还有 `/admin/governance/evaluations*`，前端未接，说明 drift 治理链条不完整。

5. `RuntimeObserverPage`
   - 真实接线，但只读展示；没有与 `/admin/governance/runtime-decisions`、`/admin/governance/distribution-events` 明细页打通。

6. `ReleasesPage`
   - 已接 `release/promote` 两个写接口，但未接后端现有 `releases/rollback`、`releases/replay`。
   - 因而“发布工作台”仍不完整。

## 后端路由与前端交叉校验

### 前端已命中、且在 `server.go` 可见的路由

- `/admin/health`
- `/admin/usage`
- `/admin/audit`
- `/admin/observability/summary`
- `/admin/observability/providers`
- `/admin/observability/hotspots`
- `/admin/observability/quota`
- `/admin/observability/quota/trends`
- `/admin/policies/models`
- `/admin/inheritance-drafts`
- `/admin/releases`
- `/admin/promotions`
- `/admin/audit-events`
- `/admin/runtime-events`
- `/admin/config-versions`
- `/admin/governance/recommendations`
- `/admin/governance/approvals`
- `/admin/governance/policy-versions`（含子路径）
- `/admin/governance/dashboard/rollouts`
- `/admin/governance/rollbacks`
- `/admin/governance/drifts`
- `/admin/governance/runtime-observer`
- `/admin/memory/candidate-facts`（含子路径）
- `/admin/memory/project-facts`
- `/v1/chat/completions`

### 后端有、前端没有任何页面入口的路由

#### 资产/资源相关
- `/admin/assets`
- `/admin/assets/stats`
- `/admin/assets/reuse-audits`
- `/admin/assets/versions`
- `/admin/assets/rollback`

#### 控制面补偿/重放
- `/admin/control-plane/compensations`
- `/admin/control-plane/compensations/replay`
- `/admin/releases/rollback`
- `/admin/releases/replay`

#### 治理链路剩余接口
- `/admin/governance/evaluations`
- `/admin/governance/evaluations/`
- `/admin/governance/rollouts`
- `/admin/governance/rollouts/`
- `/admin/governance/runtime/resolve`
- `/admin/governance/runtime-decisions`
- `/admin/governance/distribution-events`

#### 观测/公共接口
- `/admin/observability/cache`
- `/v1/models`
- `/v1/runtime/resolve`

备注：`/admin/governance/rollouts` 虽无页面直接调用，但 `RolloutsPage` 使用的是 dashboard 聚合接口 `/admin/governance/dashboard/rollouts`，因此并非完全缺席，只是缺少“原始 rollout CRUD/详情”页。

### 前端调用但需进一步确认 handler 细粒度支持的接口

`server.go` 通过前缀挂载，以下路径前端明确在调用，但是否支持要看各子 handler：

- `/admin/governance/policy-versions/:id/diff`
- `/admin/governance/policy-versions/:id/approve`
- `/admin/governance/policy-versions/:id/activate`
- `/admin/memory/candidate-facts/:factKey/:action`
- `/admin/memory/candidate-facts/actions/:action`

从现有前端测试命名与页面文案看，团队预期这些接口存在；但若要做严格契约审计，还需继续检查对应 Go handler 文件。

### 未发现“前端命中完全不存在的 server.go 路由前缀”

- 也就是说，没有发现前端去请求一个 `server.go` 完全没挂载的顶层路径前缀。
- 主要风险是“挂了前缀，但子路径动作未实现/未开启”。

## 每页补充说明

### DashboardPage
- 文件：`/tmp/opencode/llm-gateway/web/admin/src/pages/DashboardPage.tsx`
- 真实查询页，使用 TanStack Query 拉 `health + observability summary`。
- 适合作为已接线页面样板。

### ConfigCenterPage
- 文件：`/tmp/opencode/llm-gateway/web/admin/src/pages/ConfigCenterPage.tsx`
- 真实接线，且写操作在组件层完成：创建 draft、列表查询、详情抽屉。
- 尚未看到 delete/rollback/replay 能力。

### ReleasesPage
- 文件：`/tmp/opencode/llm-gateway/web/admin/src/pages/ReleasesPage.tsx`
- 页面本身不发 query，请求由子组件触发。
- 已接线 release/promote，但未覆盖 rollback/replay，是明显功能缺口。

### AuditRuntimePage
- 文件：`/tmp/opencode/llm-gateway/web/admin/src/pages/AuditRuntimePage.tsx`
- 真实接线；支持 tab 和 summary query 参数。
- 更偏“事件读页”，无导出/分页/详情下钻。

### SystemPage
- 文件：`/tmp/opencode/llm-gateway/web/admin/src/pages/SystemPage.tsx`
- 真实接线但信息面很浅，建议补 usage/audit 表格和自动加载。

### PlaygroundPage
- 文件：`/tmp/opencode/llm-gateway/web/admin/src/pages/PlaygroundPage.tsx`
- 真正直连运行时 `/v1/chat/completions`。
- 与 admin token 体系脱钩，意味着它测试的是网关公开推理接口，不是 admin API。

### ObservabilityPage
- 文件：`/tmp/opencode/llm-gateway/web/admin/src/pages/ObservabilityPage.tsx`
- 真实接线，筛选项会拼 query string。
- 未接 `/admin/observability/cache`，所以观测维度并不完整。

### QuotaPage
- 文件：`/tmp/opencode/llm-gateway/web/admin/src/pages/QuotaPage.tsx`
- 真实接线；租户和窗口筛选已落到请求。

### PoliciesPage
- 文件：`/tmp/opencode/llm-gateway/web/admin/src/pages/PoliciesPage.tsx`
- 最明显的“轻量占位页”之一：只有读模型列表。

### MemoryGovernancePage
- 文件：`/tmp/opencode/llm-gateway/web/admin/src/pages/MemoryGovernancePage.tsx`
- 这是当前最完整的治理页之一；测试也最深入。
- 如果后端子动作接口真实存在，则接线成熟度最高。

### RecommendationCenterPage
- 文件：`/tmp/opencode/llm-gateway/web/admin/src/pages/RecommendationCenterPage.tsx`
- 真实接线，带行内审批弹窗。
- 与 `ApprovalsPage` 有功能重叠。

### ApprovalsPage
- 文件：`/tmp/opencode/llm-gateway/web/admin/src/pages/ApprovalsPage.tsx`
- 真实接线，承担完整 approve/override/reject 表单。
- 会先拉 recommendations，再提交 approvals。

### PolicyVersionsPage
- 文件：`/tmp/opencode/llm-gateway/web/admin/src/pages/PolicyVersionsPage.tsx`
- 真实接线，带 diff/approve/activate 生命周期操作。
- 页面文案已承认 diff API 可能“尚未就绪”，这是显式 wiring 风险点。

### RolloutsPage
- 文件：`/tmp/opencode/llm-gateway/web/admin/src/pages/RolloutsPage.tsx`
- 用 dashboard 聚合接口而不是原始 rollouts 列表接口。
- 具备 rollback 写操作，但没有 rollout 创建/推进能力。

### RuntimeObserverPage
- 文件：`/tmp/opencode/llm-gateway/web/admin/src/pages/RuntimeObserverPage.tsx`
- 真实接线，读 runtime observer 聚合视图。
- 未拆成 runtime-decisions/distribution-events 独立页面。

### DriftDashboardPage
- 文件：`/tmp/opencode/llm-gateway/web/admin/src/pages/DriftDashboardPage.tsx`
- 真实接线但只读，缺后续治理动作。

### LoginPage
- 文件：`/tmp/opencode/llm-gateway/web/admin/src/pages/LoginPage.tsx`
- 无后端登录 API；这就是“纯前端 token 注入页”。
- 若团队预期真正认证流程，这是当前最大 auth 产品缺口。

## 测试策略与 coverage gap

### 当前测试栈

- 配置文件：`/tmp/opencode/llm-gateway/web/admin/vitest.config.ts`
- `environment: 'jsdom'`
- `globals: true`
- `setupFiles: ['./src/test/setup.ts']`
- `setup.ts` 只引入 `@testing-library/jest-dom`

### mock 策略

- 当前页面/库测试主要全部使用 `vi.stubGlobal('fetch', fetchMock)`。
- 已安装 `msw`（见 `package-lock.json`），但没有看到 `setupServer`、`msw/node`、`handlers`、`mockServiceWorker` 的使用。
- 也就是说，测试是“手工 fetch mock”，不是统一 API mock 层。

### 优点

- 每个页面几乎都有 `.test.tsx`，表层存在感不错。
- 关键路由守卫、HTTP header 注入、主要管理页 happy path 基本都覆盖到了。
- `MemoryGovernancePage` 测试非常深入，说明复杂交互已被约束。

### 明显缺口

1. 无真实 contract 校验
   - 没有用 MSW 共享后端 schema/handler。
   - 也没有从 Go 路由导出的 OpenAPI/契约校验。

2. 无 coverage 配置/门槛
   - `vitest.config.ts` 没有 `coverage` 段。
   - 无法从配置直接判断阈值或覆盖率报告。

3. 对 auth 失效后的重定向缺口未测
   - 只测了 `401` 时 token 被清掉，没测 UI 如何回登录页。

4. 对不存在/未就绪接口的降级只覆盖了一部分
   - 目前明确只看到 `PolicyVersionsPage` 测了 diff API unavailable。
   - `Memory`、`Rollouts`、`RuntimeObserver` 等页面没有系统性异常矩阵测试。

5. 未覆盖 basename `/admin/ui` 的浏览器部署语义
   - router 有 `basename: '/admin/ui'`，但页面测试多数是组件级，不验证真实部署路径回退问题。

6. 无端到端测试
   - 没看到 Playwright/Cypress。
   - 所以无法验证同源部署、静态资源路径、真实 token 输入、后端联调流程。

7. 测试代码本身有小异味
   - `router.test.tsx` 中有重复的 “allows authenticated users to access runtime observer page” case。
   - `ReleasesPage.test.tsx` 末尾才补 `within` import，虽可能被编译器接受，但结构不规范。

## 建议优先级

### P0：先确认/补齐真正影响联调的 wiring

1. 确认以下子路由在后端 handler 中实际可用：
   - `/admin/governance/policy-versions/:id/diff|approve|activate`
   - `/admin/memory/candidate-facts/:factKey/:action`
   - `/admin/memory/candidate-facts/actions/:action`
2. 为 `401/403` 增加统一跳转登录逻辑，而不只是清 token。
3. 决定 `Playground` 是否也需要附加 admin token，或明确它就是公开 runtime 调试入口。

### P1：补前端缺失页面/动作

1. `ReleasesPage` 增加 rollback / replay。
2. `ObservabilityPage` 增加 `/admin/observability/cache` 视图。
3. 增加控制面补偿页：`/admin/control-plane/compensations` 与 replay。
4. 为治理链路补充 evaluations / runtime-decisions / distribution-events 页面。

### P2：提高测试可信度

1. 统一改为 MSW handler 层 mock，减少每测手写 fetch stub。
2. 打开 Vitest coverage 并设置阈值。
3. 新增一条最小 E2E：登录 -> dashboard -> config center -> release -> runtime observer。
4. 增加 basename `/admin/ui` 与 401 失效重登场景测试。
