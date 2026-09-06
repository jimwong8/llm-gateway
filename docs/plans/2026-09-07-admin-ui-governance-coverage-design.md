# Admin UI 治理覆盖与导航重构设计

日期: 2026-09-07
状态: 设计定稿（用户已确认"要"）
范围: web/admin (React 18 + Vite + TanStack Query)，只动前端 + 少量 lib，不改后端

## 1. 背景与修正

对后端 105 条 distinct 路由与前端 114 条调用做尾斜杠/前缀/id 归一化比对后，修正此前两个误判：

- `policy-versions`：**已覆盖**（PolicyVersionsPage.tsx + lib/policyVersions.ts，含 approve/activate/diff）。
- `/admin/assets/stats`：**已覆盖**（lib/api/assets.ts:60，AssetsPage 存在）。
- 后端 `/admin/channels/`、`/admin/assets/`、`/admin/tenant-keys/`、`/admin/config-versions/` 等尾斜杠前缀路由正确承接 `{id}` 子路径，早期"phantom"均为误报。

### 真实缺口（28 条原始 miss 分类后）

A 类 — 运维/协议端点，不需要 UI（9 条，忽略）:
`/`, `/healthz`, `/healthz/detailed`, `/metrics`, `/debug/vars`, `/v1/models`, `/v1/runtime/resolve`, `/v1/long-context/tasks`, `/api/openapi.json`

B 类 — 正则误报（引号/插值截断），复核后确认已覆盖（4 条）:
`/api/billing/ledger`（lib/api/billing.ts:43 userFetch 模板串）、`/api/config/versions`、`/api/config/rollback`（ConfigCenterPage/lib/api/config.ts 命中过）、`/admin/runtime-events` 类 control-plane 只读流

C 类 — 用户侧低优先（2 条）:
`/api/auth/resend-verification`、`/api/auth/verify-email`

D 类 — **真实 admin UI 缺口（本设计目标，约 11 条）**:

| 后端路由 | 现状 | 目标 |
|---|---|---|
| /admin/governance/evaluations | 无调用无页面 | 新页 EvaluationsPage |
| /admin/control-plane/compensations | 无调用无页面 | 新页 CompensationsPage |
| /admin/control-plane/compensations/replay | 同上 | 页内 replay 操作 |
| /admin/observability/cache | 无调用 | ObservabilityPage 加缓存卡片 |
| /admin/observability/prefix-families | 无调用 | ObservabilityPage 加前缀族卡片 |
| /admin/long-context/health | 无调用 | 新页 LongContextPage |
| /admin/long-context/tasks/stuck | 无调用 | 同页 stuck 任务面板 |
| /admin/long-context/run | 无调用 | 同页手动触发（危险操作需确认） |
| /admin/governance/runtime-decisions | 无直接调用 | RuntimeObserverPage facts 已含同类数据 → P2 仅在 observer 页补直达 tab，不新建页 |
| /admin/governance/distribution-events | 同上 | 同上 |
| /api/abtest/experiments | 无调用 | P2：AbTestPanel（放入 ConfigCenter） |
| /api/files/parse | 无调用 | P2：playground 附件解析入口 |

## 2. 导航重构

现状：Sidebar.tsx 平铺约 20+ 项，治理/资产/控制面入口混杂。

目标结构（Sidebar.tsx 引入 `NAV_GROUPS` 分组数组，AppShell 渲染分组标题）：

1. **总览**: Dashboard、Observability
2. **治理中心**（icon 🛡）: 策略版本、审批、Rollouts、Drifts、建议、Evaluations(新)
3. **发布与资产**（icon 📦）: Releases、Assets、Promotions、ConfigCenter
4. **运营中心**（icon 🛠）: 补偿(新)、Runtime Observer、Long Context(新)、渠道、租户
5. **用户与计费**: Users、Billing、Playground、Session Dashboard

分组为纯展示层（可折叠 section header），路由 path 不变，避免破坏现有深链与测试。

## 3. 页面设计

- 统一沿用 AppShell + useQuery + useTranslation 既有模式（参照 PolicyVersionsPage）。
- EvaluationsPage: 列表 + 状态 badge + 详情抽屉（复用 config-drawer 样式类）。
- CompensationsPage: 列表 + replay 按钮（confirm 二次确认，错误用 actionError 模式）。
- LongContextPage: health 概览卡 + stuck 任务表 + run 手动触发（danger 样式）。
- ObservabilityPage 增量: 两个只读卡片，不重构现有布局。
- 所有新 lib 放 `src/lib/governanceEvals.ts`、`src/lib/compensations.ts`、`src/lib/longContext.ts`，类型放 `src/types/`。

## 4. 非目标

- 不改后端路由、不加鉴权、不做 i18n key 全量补齐（新增 key 随页面补）。
- 不动 adminui/index.html 嵌入版。
- runtime-decisions / distribution-events 不建独立页（数据已在 observer 聚合）。
