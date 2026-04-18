# LLM Gateway Admin Console and Playground Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 LLM Gateway 构建一个可随 [`cmd/server/main.go`](cmd/server/main.go:1) 同进程发布的完整响应式 Web 管理控制台与在线请求测试台。

**Architecture:** 采用同仓单体前端方案，在 `web/admin/` 下实现基于 React + TypeScript + Vite 的 SPA，构建产物输出为静态文件并由 Go 服务通过 [`internal/httpserver/adminui/`](internal/httpserver/adminui) 与 [`internal/httpserver/server.go`](internal/httpserver/server.go:47) 统一托管。前端优先复用现有 [`internal/httpserver/admin_handler.go`](internal/httpserver/admin_handler.go:1) 与数据面接口，仅在 Dashboard 聚合与 Playground 元信息展示确有必要时补少量只读接口。

**Tech Stack:** Go, React, TypeScript, Vite, Tailwind CSS, TanStack Query, React Router, Vitest, Playwright

---

## 文件结构规划

### 前端工程
- Create: `web/admin/package.json`
- Create: `web/admin/tsconfig.json`
- Create: `web/admin/vite.config.ts`
- Create: `web/admin/index.html`
- Create: `web/admin/src/main.tsx`
- Create: `web/admin/src/App.tsx`
- Create: `web/admin/src/styles/index.css`
- Create: `web/admin/src/router.tsx`
- Create: `web/admin/src/lib/http.ts`
- Create: `web/admin/src/lib/query.ts`
- Create: `web/admin/src/lib/auth.ts`
- Create: `web/admin/src/lib/playground.ts`
- Create: `web/admin/src/types/admin.ts`
- Create: `web/admin/src/types/runtime.ts`
- Create: `web/admin/src/types/playground.ts`
- Create: `web/admin/src/components/layout/AppShell.tsx`
- Create: `web/admin/src/components/layout/Sidebar.tsx`
- Create: `web/admin/src/components/layout/Topbar.tsx`
- Create: `web/admin/src/components/common/PageHeader.tsx`
- Create: `web/admin/src/components/common/EmptyState.tsx`
- Create: `web/admin/src/components/common/ErrorPanel.tsx`
- Create: `web/admin/src/components/common/StatCard.tsx`
- Create: `web/admin/src/components/common/JsonViewer.tsx`
- Create: `web/admin/src/components/forms/*`
- Create: `web/admin/src/pages/LoginPage.tsx`
- Create: `web/admin/src/pages/DashboardPage.tsx`
- Create: `web/admin/src/pages/ConfigCenterPage.tsx`
- Create: `web/admin/src/pages/ReleasesPage.tsx`
- Create: `web/admin/src/pages/AuditRuntimePage.tsx`
- Create: `web/admin/src/pages/PlaygroundPage.tsx`
- Create: `web/admin/src/pages/ObservabilityPage.tsx`
- Create: `web/admin/src/pages/QuotaPage.tsx`
- Create: `web/admin/src/pages/PoliciesPage.tsx`
- Create: `web/admin/src/pages/SystemPage.tsx`
- Create: `web/admin/src/hooks/*`
- Create: `web/admin/src/mocks/*`

### 前端测试
- Create: `web/admin/src/pages/*.test.tsx`
- Create: `web/admin/src/components/*.test.tsx`
- Create: `web/admin/vitest.config.ts`
- Create: `web/admin/playwright.config.ts`
- Create: `web/admin/tests/smoke.spec.ts`

### Go 静态资源集成
- Modify: `internal/httpserver/admin_ui_handler.go`
- Modify: `internal/httpserver/admin_ui_handler_test.go`
- Modify: `internal/httpserver/server.go`
- Modify: `cmd/server/main.go`
- Create: `internal/httpserver/adminui/embed.go`
- Create: `internal/httpserver/adminui/handler.go`

### 可能新增的后端只读聚合接口
- Modify: `internal/httpserver/admin_handler.go`
- Modify: `internal/httpserver/admin_handler_test.go`
- Create: `internal/httpserver/dashboard_handler.go`
- Create: `internal/httpserver/dashboard_handler_test.go`
- Create: `internal/httpserver/playground_metadata_handler.go`
- Create: `internal/httpserver/playground_metadata_handler_test.go`

### 文档与脚本
- Modify: `docs/plans/2026-03-25-llm-gateway-project-report.md`
- Create: `docs/plans/2026-03-25-llm-gateway-admin-console-usage.md`
- Create: `cmd/verify/admin_ui/`

---

## Chunk 1: 基础框架与静态资源集成

### Task 1: 初始化前端工程骨架

**Files:**
- Create: `web/admin/package.json`
- Create: `web/admin/tsconfig.json`
- Create: `web/admin/vite.config.ts`
- Create: `web/admin/index.html`
- Create: `web/admin/src/main.tsx`
- Create: `web/admin/src/App.tsx`
- Create: `web/admin/src/styles/index.css`
- Create: `web/admin/src/router.tsx`

- [ ] **Step 1: 写一个最小前端工程存在性测试清单**

记录需要满足的最小事实：
- `npm run build` 可生成 `dist/`
- 路由能渲染登录页和主壳
- CSS 能正确加载

- [ ] **Step 2: 创建前端基础工程文件**

要求：
- 使用 TypeScript
- 使用 Vite
- 使用 React
- 预置 `build`, `dev`, `test` 脚本

- [ ] **Step 3: 实现最小应用骨架**

要求：
- [`web/admin/src/main.tsx`](web/admin/src/main.tsx) 挂载 React 应用
- [`web/admin/src/App.tsx`](web/admin/src/App.tsx) 只负责 QueryProvider / RouterProvider 等全局包装
- [`web/admin/src/router.tsx`](web/admin/src/router.tsx) 提供最小路由树

- [ ] **Step 4: 运行前端构建**

Run: `cd web/admin && npm install && npm run build`
Expected: 生成 `dist/` 且无 TypeScript 错误

- [ ] **Step 5: Commit**

```bash
git add web/admin
git commit -m "feat: bootstrap admin console frontend"
```

### Task 2: 实现 AppShell 与响应式布局骨架

**Files:**
- Create: `web/admin/src/components/layout/AppShell.tsx`
- Create: `web/admin/src/components/layout/Sidebar.tsx`
- Create: `web/admin/src/components/layout/Topbar.tsx`
- Create: `web/admin/src/components/common/PageHeader.tsx`
- Test: `web/admin/src/components/layout/AppShell.test.tsx`

- [ ] **Step 1: 写失败测试，验证桌面与移动布局都能渲染核心区域**

测试点：
- 左侧导航存在
- 顶部栏存在
- 主内容区域存在
- 窄屏时导航可以折叠为抽屉触发器

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web/admin && npm run test -- AppShell.test.tsx`
Expected: FAIL with component not found

- [ ] **Step 3: 实现最小 AppShell**

要求：
- 桌面端左侧固定导航
- 平板/手机端折叠
- 页面容器具备统一边距与滚动策略

- [ ] **Step 4: 再跑测试确认通过**

Run: `cd web/admin && npm run test -- AppShell.test.tsx`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/admin/src/components
git commit -m "feat: add responsive admin shell"
```

### Task 3: 将前端 dist 接入 Go 服务

**Files:**
- Modify: `internal/httpserver/admin_ui_handler.go`
- Modify: `internal/httpserver/admin_ui_handler_test.go`
- Create: `internal/httpserver/adminui/embed.go`
- Create: `internal/httpserver/adminui/handler.go`
- Modify: `internal/httpserver/server.go`

- [ ] **Step 1: 写失败测试，验证 `/admin-ui` 或根入口可返回前端入口页**

测试点：
- 静态入口 HTML 返回 200
- JS/CSS 资源返回 200
- 未命中的前端路由回退到 `index.html`

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/httpserver/... -run TestAdminUI`
Expected: FAIL with missing SPA fallback or asset handler

- [ ] **Step 3: 实现嵌入式静态资源处理器**

要求：
- Go 可嵌入前端 `dist` 产物
- 支持静态资源返回
- 支持前端路由 fallback

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/httpserver/... -run TestAdminUI -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver
git commit -m "feat: serve embedded admin ui assets"
```

---

## Chunk 2: 鉴权、路由与基础数据访问层

### Task 4: 实现登录页与 Token 存储

**Files:**
- Create: `web/admin/src/pages/LoginPage.tsx`
- Create: `web/admin/src/lib/auth.ts`
- Test: `web/admin/src/pages/LoginPage.test.tsx`

- [ ] **Step 1: 写失败测试，验证输入 token 后可完成本地登录态保存**

测试点：
- 输入框可编辑
- 提交后 token 写入 `sessionStorage`
- 登录成功后跳转到 Dashboard

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web/admin && npm run test -- LoginPage.test.tsx`
Expected: FAIL

- [ ] **Step 3: 实现最小登录页与 auth helper**

要求：
- 仅保存 Admin Token
- 不引入额外用户体系
- 提供 `getToken`, `setToken`, `clearToken`

- [ ] **Step 4: 跑测试确认通过**

Run: `cd web/admin && npm run test -- LoginPage.test.tsx`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/admin/src/pages/LoginPage.tsx web/admin/src/lib/auth.ts
 git commit -m "feat: add admin token login flow"
```

### Task 5: 实现统一 HTTP Client 与 401 处理

**Files:**
- Create: `web/admin/src/lib/http.ts`
- Create: `web/admin/src/lib/query.ts`
- Test: `web/admin/src/lib/http.test.ts`

- [ ] **Step 1: 写失败测试，验证请求自动带 Bearer Token，401 时清理登录态**

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web/admin && npm run test -- http.test.ts`
Expected: FAIL

- [ ] **Step 3: 实现最小 HTTP 层**

要求：
- 自动读取 `sessionStorage` token
- 为 `/admin/*` 自动加 `Authorization` 头
- 对 401/403 提供统一错误抛出与跳登录机制

- [ ] **Step 4: 跑测试确认通过**

Run: `cd web/admin && npm run test -- http.test.ts`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/admin/src/lib
git commit -m "feat: add admin api client"
```

### Task 6: 实现前端主路由与受保护页面壳

**Files:**
- Modify: `web/admin/src/router.tsx`
- Create: `web/admin/src/components/auth/ProtectedRoute.tsx`
- Test: `web/admin/src/router.test.tsx`

- [ ] **Step 1: 写失败测试，验证未登录跳转登录页，已登录可进入受保护页面**
- [ ] **Step 2: 运行测试确认失败**
- [ ] **Step 3: 实现受保护路由**
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: Commit**

---

## Chunk 3: Config Center 与 Releases

### Task 7: 对接配置版本列表与详情

**Files:**
- Create: `web/admin/src/types/admin.ts`
- Create: `web/admin/src/hooks/useConfigVersions.ts`
- Create: `web/admin/src/pages/ConfigCenterPage.tsx`
- Create: `web/admin/src/components/config/ConfigVersionTable.tsx`
- Create: `web/admin/src/components/config/ConfigVersionDrawer.tsx`
- Test: `web/admin/src/pages/ConfigCenterPage.test.tsx`

- [ ] **Step 1: 写失败测试，验证页面可加载版本列表并打开详情抽屉**
- [ ] **Step 2: 运行测试确认失败**
- [ ] **Step 3: 实现列表、筛选条与详情抽屉**
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: Commit**

### Task 8: 实现创建 inheritance draft 表单

**Files:**
- Create: `web/admin/src/components/config/CreateDraftForm.tsx`
- Modify: `web/admin/src/pages/ConfigCenterPage.tsx`
- Test: `web/admin/src/components/config/CreateDraftForm.test.tsx`

- [ ] **Step 1: 写失败测试，验证表单校验与成功回执**
- [ ] **Step 2: 运行测试确认失败**
- [ ] **Step 3: 实现最小 draft 表单**
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: Commit**

### Task 9: 实现 Release 与 Promotion 工作台

**Files:**
- Create: `web/admin/src/pages/ReleasesPage.tsx`
- Create: `web/admin/src/components/releases/ReleaseDraftPanel.tsx`
- Create: `web/admin/src/components/releases/PromotionPanel.tsx`
- Test: `web/admin/src/pages/ReleasesPage.test.tsx`

- [ ] **Step 1: 写失败测试，验证 release 与 promotion 表单提交流程**
- [ ] **Step 2: 运行测试确认失败**
- [ ] **Step 3: 实现双工作台页面**
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: Commit**

---

## Chunk 4: Audit、Runtime 与 System

### Task 10: 实现 Audit & Runtime 双标签页

**Files:**
- Create: `web/admin/src/pages/AuditRuntimePage.tsx`
- Create: `web/admin/src/components/events/AuditTable.tsx`
- Create: `web/admin/src/components/events/RuntimeTable.tsx`
- Test: `web/admin/src/pages/AuditRuntimePage.test.tsx`

- [ ] **Step 1: 写失败测试，验证 tab 切换、筛选与 summary 模式切换**
- [ ] **Step 2: 运行测试确认失败**
- [ ] **Step 3: 实现页面与事件表格**
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: Commit**

### Task 11: 实现 System 页面

**Files:**
- Create: `web/admin/src/pages/SystemPage.tsx`
- Test: `web/admin/src/pages/SystemPage.test.tsx`

- [ ] **Step 1: 写失败测试，验证 health / usage / audit 卡片渲染**
- [ ] **Step 2: 运行测试确认失败**
- [ ] **Step 3: 实现系统状态页面**
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: Commit**

---

## Chunk 5: Playground

### Task 12: 实现 Playground 请求编辑器

**Files:**
- Create: `web/admin/src/types/playground.ts`
- Create: `web/admin/src/lib/playground.ts`
- Create: `web/admin/src/pages/PlaygroundPage.tsx`
- Create: `web/admin/src/components/playground/RequestEditor.tsx`
- Create: `web/admin/src/components/playground/MessageEditor.tsx`
- Test: `web/admin/src/pages/PlaygroundPage.test.tsx`

- [ ] **Step 1: 写失败测试，验证表单能发送 `POST /v1/chat/completions`**
- [ ] **Step 2: 运行测试确认失败**
- [ ] **Step 3: 实现请求编辑器与发送逻辑**
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: Commit**

### Task 13: 实现 Playground 响应可视化

**Files:**
- Create: `web/admin/src/components/playground/ResponsePanel.tsx`
- Create: `web/admin/src/components/playground/ResponseMeta.tsx`
- Create: `web/admin/src/components/playground/HistoryPanel.tsx`
- Test: `web/admin/src/components/playground/ResponsePanel.test.tsx`

- [ ] **Step 1: 写失败测试，验证可展示响应头、状态码、耗时、cache 信号**
- [ ] **Step 2: 运行测试确认失败**
- [ ] **Step 3: 实现结果区与最近请求模板**
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: Commit**

### Task 14: 评估是否补 Playground 元信息聚合接口

**Files:**
- Modify: `internal/httpserver/admin_handler.go`
- Modify: `internal/httpserver/admin_handler_test.go`
- Create: `internal/httpserver/playground_metadata_handler.go`
- Create: `internal/httpserver/playground_metadata_handler_test.go`

- [ ] **Step 1: 先写一个前端痛点清单**

判断现有接口是否足以支撑：
- provider/model 结果展示
- cache/semantic 头解析
- 错误信息结构化展示

- [ ] **Step 2: 若现有响应已足够，显式记录“无需新增后端接口”并跳过实现**
- [ ] **Step 3: 若确实缺失，再先写失败测试**
- [ ] **Step 4: 实现最小只读聚合接口**
- [ ] **Step 5: 跑 Go 测试确认通过**
- [ ] **Step 6: Commit**

---

## Chunk 6: Dashboard、Observability、Quota

### Task 15: 评估并实现 Dashboard 聚合接口

**Files:**
- Create: `internal/httpserver/dashboard_handler.go`
- Create: `internal/httpserver/dashboard_handler_test.go`
- Modify: `internal/httpserver/admin_handler.go`
- Create: `web/admin/src/pages/DashboardPage.tsx`
- Test: `web/admin/src/pages/DashboardPage.test.tsx`

- [ ] **Step 1: 先写失败测试，验证 Dashboard 所需数据结构**
- [ ] **Step 2: 运行 Go 测试确认失败**
- [ ] **Step 3: 实现最小只读 summary 聚合接口**
- [ ] **Step 4: 实现前端 Dashboard 页面**
- [ ] **Step 5: 跑前后端测试确认通过**
- [ ] **Step 6: Commit**

### Task 16: 实现 Observability 页面

**Files:**
- Create: `web/admin/src/pages/ObservabilityPage.tsx`
- Create: `web/admin/src/components/observability/*`
- Test: `web/admin/src/pages/ObservabilityPage.test.tsx`

- [ ] **Step 1: 写失败测试，验证 summary 卡片、provider 表格、hotspots 列表渲染**
- [ ] **Step 2: 运行测试确认失败**
- [ ] **Step 3: 实现页面**
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: Commit**

### Task 17: 实现 Quota 页面

**Files:**
- Create: `web/admin/src/pages/QuotaPage.tsx`
- Create: `web/admin/src/components/quota/*`
- Test: `web/admin/src/pages/QuotaPage.test.tsx`

- [ ] **Step 1: 写失败测试，验证 quota summary 与 trends 渲染**
- [ ] **Step 2: 运行测试确认失败**
- [ ] **Step 3: 实现页面**
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: Commit**

### Task 18: 实现 Policies 页面最小只读能力

**Files:**
- Create: `web/admin/src/pages/PoliciesPage.tsx`
- Test: `web/admin/src/pages/PoliciesPage.test.tsx`

- [ ] **Step 1: 写失败测试，验证模型策略列表展示**
- [ ] **Step 2: 运行测试确认失败**
- [ ] **Step 3: 实现最小页面**
- [ ] **Step 4: 跑测试确认通过**
- [ ] **Step 5: Commit**

---

## Chunk 7: 端到端验证、文档与交付

### Task 19: 增加前端 smoke 测试

**Files:**
- Create: `web/admin/playwright.config.ts`
- Create: `web/admin/tests/smoke.spec.ts`

- [ ] **Step 1: 写 smoke 用例，覆盖登录、进入 Dashboard、打开 Config Center、进入 Playground**
- [ ] **Step 2: 运行测试确认失败**
- [ ] **Step 3: 补齐缺口直到 smoke 测试通过**
- [ ] **Step 4: 再跑 `npm run test` 与 Playwright**
- [ ] **Step 5: Commit**

### Task 20: 增加 Go 侧前端可用性验证脚本

**Files:**
- Create: `cmd/verify/admin_ui/main.go`

- [ ] **Step 1: 写失败验证脚本需求**

验证：
- 前端入口页可打开
- 静态资源返回正常
- 管理台登录页存在

- [ ] **Step 2: 实现最小验证脚本**
- [ ] **Step 3: 运行 `go run ./cmd/verify/admin_ui/`**
- [ ] **Step 4: Commit**

### Task 21: 更新文档

**Files:**
- Modify: `docs/plans/2026-03-25-llm-gateway-project-report.md`
- Create: `docs/plans/2026-03-25-llm-gateway-admin-console-usage.md`

- [ ] **Step 1: 更新项目说明报告，加入前端结构与使用方式**
- [ ] **Step 2: 新增 Admin 控制台使用文档**
- [ ] **Step 3: 记录本地开发、构建、嵌入与启动方式**
- [ ] **Step 4: Commit**

### Task 22: 最终全量验证

**Files:**
- Verify only

- [ ] **Step 1: 运行前端单测**

Run: `cd web/admin && npm run test`
Expected: PASS

- [ ] **Step 2: 运行前端构建**

Run: `cd web/admin && npm run build`
Expected: PASS

- [ ] **Step 3: 运行 Go 全量测试**

Run: `go test -count=1 ./...`
Expected: PASS

- [ ] **Step 4: 运行关键验证脚本**

Run:
- `go run ./cmd/verify/inheritance_promotion/`
- `go run ./cmd/verify/admin_ui/`

Expected: PASS or success output

- [ ] **Step 5: Commit**

```bash
git add .
git commit -m "feat: add embedded admin console and playground"
```

---

## 执行注意事项

1. 严格按 TDD 执行，每个页面和每个后端补充接口都先写失败测试。
2. 若某步发现现有接口已足够，不要为了“计划完整”强行新增后端接口。
3. 前端组件尽量按职责拆小，避免把整个控制台堆进单文件。
4. 第一阶段不要做 RBAC、用户体系、多租户前端身份模型、实时 websocket 推送等超出设计范围能力。
5. 任何新增 API 都应优先只读聚合，不改变既有控制面生命周期语义。

Plan complete and saved to `docs/superpowers/plans/2026-03-25-llm-gateway-admin-console-and-playground.md`. Ready to execute?