# Admin UI（内置托管）Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在网关进程内新增企业控制面前端（`/admin/ui`），可视化管理 L4 资产、统计、版本和复用审计，并支持筛选、搜索、分页、软删除标记与回滚入口。

**Architecture:** 采用“后端托管静态前端 + 前端直连现有 Admin API”的同域方案。后端仅增加 UI 静态入口与资源分发，不改动既有业务 API 协议。前端以页面级模块组织（资产页、统计页、版本抽屉、审计页），以 URL Query 持久化筛选状态，并通过统一 API Client 处理鉴权与错误。

**Tech Stack:** Go `net/http`（静态托管与路由）、HTML/CSS/Vanilla JS（首版 UI）、现有 Admin API（`/admin/assets*`）、Go `httptest`（后端入口测试）。

---

## File Structure（实施前锁定）

### Backend（网关）
- Modify: `internal/httpserver/server.go`
  - 新增 `/admin/ui` 与 `/admin/ui/*` 路由
  - 挂载 UI Handler（不影响现有 `/admin/*` API）
- Create: `internal/httpserver/admin_ui_handler.go`
  - 提供 UI 静态资源处理逻辑
  - 处理 `/admin/ui` 默认文档与缓存头
- Create: `internal/httpserver/admin_ui_handler_test.go`
  - 验证 UI 路由可访问、content-type 正确、回退逻辑正确

### Frontend（内置静态资源）
- Create: `internal/httpserver/adminui/index.html`
  - 左侧导航 + 顶部筛选栏 + 主区域容器
- Create: `internal/httpserver/adminui/styles.css`
  - 企业后台布局、表格、抽屉、状态标签、错误态样式
- Create: `internal/httpserver/adminui/app.js`
  - 页面路由（hash 或 query）
  - API Client、状态管理、数据拉取、交互动作（回滚/删除/刷新）
- Create: `internal/httpserver/adminui/components.js`
  - 可复用渲染函数（表格、卡片、分页、提示框、抽屉）

### Documentation
- Modify: `docs/plans/2026-03-24-admin-ui-design.md`
  - 补充“实施结果链接”和最终接口映射

---

## Chunk 1: 后端托管入口与静态资源管线

### Task 1: 增加 `/admin/ui` 路由入口

**Files:**
- Modify: `internal/httpserver/server.go`
- Test: `internal/httpserver/admin_ui_handler_test.go`

- [ ] **Step 1: 写失败测试（路由未注册时应失败）**

```go
func TestAdminUIRouteRegistered(t *testing.T) {
    srv := New(testCfg(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
    rr := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/admin/ui", nil)
    srv.Handler().ServeHTTP(rr, req)
    if rr.Code == http.StatusNotFound {
        t.Fatalf("expected /admin/ui to be registered")
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/httpserver -run TestAdminUIRouteRegistered -v`
Expected: `FAIL`（404/not found）

- [ ] **Step 3: 在 `server.go` 注册 `/admin/ui` 与 `/admin/ui/`**

```go
mux.HandleFunc("/admin/ui", s.adminUI)
mux.HandleFunc("/admin/ui/", s.adminUI)
```

- [ ] **Step 4: 再跑测试确认通过**

Run: `go test ./internal/httpserver -run TestAdminUIRouteRegistered -v`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/server.go internal/httpserver/admin_ui_handler_test.go
git commit -m "feat(admin-ui): register admin ui routes"
```

### Task 2: 实现静态资源 Handler（含默认文档）

**Files:**
- Create: `internal/httpserver/admin_ui_handler.go`
- Create: `internal/httpserver/admin_ui_handler_test.go`

- [ ] **Step 1: 写失败测试（/admin/ui 返回 index）**

```go
func TestAdminUIIndexServed(t *testing.T) {
    rr := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/admin/ui", nil)
    srv := newServerForUIOnly()
    srv.Handler().ServeHTTP(rr, req)
    if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
        t.Fatalf("expected html content-type, got %s", ct)
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/httpserver -run TestAdminUIIndexServed -v`
Expected: `FAIL`

- [ ] **Step 3: 实现 `adminUI` Handler（按路径分发 index/css/js）**

最小实现：
- `/admin/ui`、`/admin/ui/` -> `index.html`
- `/admin/ui/styles.css` -> css
- `/admin/ui/app.js`、`/admin/ui/components.js` -> js
- 其他路径 -> 404

- [ ] **Step 4: 运行单测通过**

Run: `go test ./internal/httpserver -run TestAdminUI -v`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/admin_ui_handler.go internal/httpserver/admin_ui_handler_test.go
git commit -m "feat(admin-ui): serve static admin ui assets"
```

---

## Chunk 2: 前端骨架与页面渲染

### Task 3: 交付页面骨架（导航+容器+筛选栏）

**Files:**
- Create: `internal/httpserver/adminui/index.html`
- Create: `internal/httpserver/adminui/styles.css`

- [ ] **Step 1: 写前端 smoke 脚本（页面关键节点存在）**

Run: `grep -E "id=\"app\"|id=\"filters\"|id=\"nav\"" internal/httpserver/adminui/index.html`
Expected: 命中 3 处

- [ ] **Step 2: 创建 `index.html` 最小骨架**

包含：
- 左侧导航：Assets / Stats / Reuse Audits
- 顶部过滤：tenant、task、model、tag、keyword、include_deleted、limit
- 主区域：`#view`
- 版本抽屉：`#version-drawer`

- [ ] **Step 3: 创建样式 `styles.css`**

包含：
- 后台布局（sidebar + main）
- 表格、按钮、tag、badge
- 错误提示、loading、empty
- 抽屉与遮罩

- [ ] **Step 4: 运行 smoke 检查**

Run: `grep -E "#version-drawer|.layout|.table" internal/httpserver/adminui/styles.css`
Expected: 命中对应样式

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/adminui/index.html internal/httpserver/adminui/styles.css
git commit -m "feat(admin-ui): add admin ui layout and base styles"
```

### Task 4: 建立前端状态与页面路由

**Files:**
- Create: `internal/httpserver/adminui/app.js`
- Create: `internal/httpserver/adminui/components.js`

- [ ] **Step 1: 写失败测试（最小 e2e 手工验证用例）**

Manual Expected:
1) 打开 `/admin/ui`
2) 默认进入 Assets 视图
3) 修改筛选后 URL query 同步变化

- [ ] **Step 2: 实现状态模型**

在 `app.js` 定义：
- `uiState.view` (`assets|stats|audits`)
- `queryState` (`tenant_id/task_type/source_model/tag/keyword/include_deleted/limit/offset`)
- `cache`（最近一次接口响应）

- [ ] **Step 3: 实现页面切换与 query 持久化**

- 初始化从 `location.search` 读取
- 交互更新后 `history.replaceState` 写回 URL

- [ ] **Step 4: 手工验证状态保持**

Run: 打开页面->设筛选->刷新
Expected: 筛选值保持

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/adminui/app.js internal/httpserver/adminui/components.js
git commit -m "feat(admin-ui): add state management and view routing"
```

---

## Chunk 3: API 集成与业务交互

### Task 5: 资产页（筛选/搜索/分页/软删除标记）

**Files:**
- Modify: `internal/httpserver/adminui/app.js`
- Modify: `internal/httpserver/adminui/components.js`

- [ ] **Step 1: 先写失败路径（接口返回 401/500 的 UI 表现）**

Manual Expected:
- 401 显示“管理密钥无效”
- 500 显示“请求失败，可重试”

- [ ] **Step 2: 实现 API Client（统一 Header 注入 `X-Admin-Key`）**

```js
async function apiGet(path, query) { /* fetch + error mapping */ }
```

- [ ] **Step 3: 接入 `GET /admin/assets` 并渲染表格**

- 展示列：ID、tenant、title、task、model、hit_count、is_deleted、created_at
- 支持 `limit/offset`
- 筛选变化时 offset 重置为 0

- [ ] **Step 4: 增加软删除标记与空态/错误态**

- `is_deleted=true` 显示 badge
- 空列表显示 Empty

- [ ] **Step 5: 验证并 Commit**

Run: 手工验证 `/admin/ui`
Expected: 可筛选、可翻页、可看到软删除标记

```bash
git add internal/httpserver/adminui/app.js internal/httpserver/adminui/components.js
git commit -m "feat(admin-ui): implement assets table with filters and pagination"
```

### Task 6: 统计页（overview + by_task/by_model/by_tag）

**Files:**
- Modify: `internal/httpserver/adminui/app.js`
- Modify: `internal/httpserver/adminui/components.js`

- [ ] **Step 1: 写失败路径（stats 拉取失败显示 retry）**
- [ ] **Step 2: 接入 `GET /admin/assets/stats`**
- [ ] **Step 3: 渲染 6 个 overview 指标卡**
  - `asset_count/active_count/deleted_count/version_count/reuse_count/total_hit_count`
- [ ] **Step 4: 渲染三组分布表（task/model/tag）**
- [ ] **Step 5: 验证并 Commit**

```bash
git add internal/httpserver/adminui/app.js internal/httpserver/adminui/components.js
git commit -m "feat(admin-ui): implement stats dashboard"
```

### Task 7: 版本抽屉 + 回滚操作

**Files:**
- Modify: `internal/httpserver/adminui/app.js`
- Modify: `internal/httpserver/adminui/components.js`

- [ ] **Step 1: 写失败路径（版本拉取失败、回滚失败）**
- [ ] **Step 2: 接入 `GET /admin/assets/versions`（按 asset_id）**
- [ ] **Step 3: 抽屉渲染版本列表与回滚按钮**
- [ ] **Step 4: 接入 `POST /admin/assets/rollback`**
- [ ] **Step 5: 回滚成功后失效并重拉（assets + versions + stats）**
- [ ] **Step 6: 验证并 Commit**

```bash
git add internal/httpserver/adminui/app.js internal/httpserver/adminui/components.js
git commit -m "feat(admin-ui): add version drawer and rollback action"
```

### Task 8: 复用审计页

**Files:**
- Modify: `internal/httpserver/adminui/app.js`
- Modify: `internal/httpserver/adminui/components.js`

- [ ] **Step 1: 写失败路径（audits 失败态）**
- [ ] **Step 2: 接入 `GET /admin/assets/reuse-audits`**
- [ ] **Step 3: 渲染审计表（request_id/asset_id/route_model/route_task/hit_source/created_at）**
- [ ] **Step 4: 支持分页与 tenant 过滤联动**
- [ ] **Step 5: 验证并 Commit**

```bash
git add internal/httpserver/adminui/app.js internal/httpserver/adminui/components.js
git commit -m "feat(admin-ui): implement reuse audits page"
```

---

## Chunk 4: 质量保障、文档与交付

### Task 9: 后端入口测试补齐

**Files:**
- Modify: `internal/httpserver/admin_ui_handler_test.go`

- [ ] **Step 1: 增加 3 个测试用例**
  - `/admin/ui` 返回 html
  - `/admin/ui/styles.css` 返回 css
  - `/admin/ui/not-exists.js` 返回 404

- [ ] **Step 2: 运行测试**

Run: `go test ./internal/httpserver -run TestAdminUI -v`
Expected: `PASS`

- [ ] **Step 3: Commit**

```bash
git add internal/httpserver/admin_ui_handler_test.go
git commit -m "test(admin-ui): add static handler route tests"
```

### Task 10: 文档更新与验收脚本

**Files:**
- Modify: `docs/plans/2026-03-24-admin-ui-design.md`
- Create: `docs/plans/2026-03-24-admin-ui-acceptance.md`

- [ ] **Step 1: 记录最终接口映射与页面映射**
- [ ] **Step 2: 写验收 checklist（含 401/404/5xx 场景）**
- [ ] **Step 3: 记录手工验收命令（curl + 浏览器）**
- [ ] **Step 4: Commit**

```bash
git add docs/plans/2026-03-24-admin-ui-design.md docs/plans/2026-03-24-admin-ui-acceptance.md
git commit -m "docs(admin-ui): add acceptance checklist and endpoint mapping"
```

---

## 执行顺序与里程碑

1) 里程碑 M1（可访问）：`/admin/ui` 可打开，静态资源可加载
2) 里程碑 M2（可用）：资产页 + 统计页可用
3) 里程碑 M3（闭环）：版本回滚 + 审计页可用
4) 里程碑 M4（可交付）：测试与文档齐备

---

## 风险与回避

- 风险：首版前端无构建工具，代码膨胀
  - 回避：拆 `app.js` / `components.js`，函数级模块化
- 风险：管理密钥输入/持久化处理不当
  - 回避：仅内存态，不写入 localStorage
- 风险：大数据量分页体验差
  - 回避：后端分页优先，前端不做全量缓存

---

## Definition of Done

- `/admin/ui` 与 `/admin/ui/*` 在网关进程内可访问
- 资产、统计、版本、审计四类页面能力可用
- 支持筛选/搜索/分页/软删除标记/版本回滚入口
- 401/404/5xx 错误处理符合设计
- 测试与验收文档完成，能复现实测流程
