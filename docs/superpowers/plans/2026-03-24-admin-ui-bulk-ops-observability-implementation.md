# Admin UI 批量操作可观测性 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为批量标签、批量回滚、批量软删除统一提供任务状态面板、实时进度、失败明细和失败项重试能力。

**Architecture:** 在 [`internal/httpserver/adminui/app.js`](internal/httpserver/adminui/app.js:1) 引入单一批量任务状态对象和统一渲染函数，让三类批量操作共享同一套执行观测框架。UI 通过 [`internal/httpserver/adminui/index.html`](internal/httpserver/adminui/index.html:1) 新增状态面板，在 [`internal/httpserver/adminui/styles.css`](internal/httpserver/adminui/styles.css:1) 中补充进度条、失败列表和重试操作样式。

**Tech Stack:** 原生 HTML/CSS/JavaScript、浏览器 DOM API、现有 Admin UI 批量操作接口（[`apiPut()`](internal/httpserver/adminui/app.js:97)、[`apiPost()`](internal/httpserver/adminui/app.js:89)、[`apiDelete()`](internal/httpserver/adminui/app.js:105)）

---

## Chunk 1: 结构与状态模型

### Task 1: 明确文件边界与新增职责

**Files:**
- Modify: `internal/httpserver/adminui/index.html`
- Modify: `internal/httpserver/adminui/app.js`
- Modify: `internal/httpserver/adminui/styles.css`
- Reference: `docs/plans/2026-03-24-admin-ui-bulk-ops-observability-design.md`

- [ ] **Step 1: 固定作用范围**

只覆盖三类批量操作：
- 批量标签
- 批量回滚
- 批量软删除

不新增后端 batch API，不做历史任务中心。

- [ ] **Step 2: 确认现有接入点**

复核以下函数是首批接入目标：
- [`batchApplyTagsSelected()`](internal/httpserver/adminui/app.js:576)
- [`batchRollbackSelected()`](internal/httpserver/adminui/app.js:660)
- [`batchSoftDeleteSelected()`](internal/httpserver/adminui/app.js:719)

- [ ] **Step 3: 创建初始提交（可选）**

Run: `git status`
Expected: 工作区变更清晰可辨识

---

## Chunk 2: 面板 UI 骨架

### Task 2: 在页面增加统一批量任务状态面板

**Files:**
- Modify: `internal/httpserver/adminui/index.html`

- [ ] **Step 1: 在操作区下方新增状态面板容器**

增加类似结构：
```html
<section id="batch-job-panel" class="batch-job-panel" hidden>
  <div id="batch-job-summary"></div>
  <div id="batch-job-progress" class="batch-job-progress">
    <div id="batch-job-progress-bar"></div>
  </div>
  <div id="batch-job-current"></div>
  <div id="batch-job-failures"></div>
  <div class="batch-job-actions">
    <button id="batch-job-retry-btn" disabled>重试失败项</button>
  </div>
</section>
```

- [ ] **Step 2: 静态检查骨架位置**

Expected:
- 位于 [`preset-bar`](internal/httpserver/adminui/index.html:37) 与资产操作区之间，或紧接操作区之后
- 不影响既有筛选区域布局

- [ ] **Step 3: 提交**

```bash
git add internal/httpserver/adminui/index.html
git commit -m "feat(admin-ui): add bulk job status panel skeleton"
```

### Task 3: 增加状态面板样式

**Files:**
- Modify: `internal/httpserver/adminui/styles.css`

- [ ] **Step 1: 添加状态面板容器样式**

新增：
- `.batch-job-panel`
- `.batch-job-summary`
- `.batch-job-current`
- `.batch-job-actions`

- [ ] **Step 2: 添加进度条样式**

新增：
- `.batch-job-progress`
- `.batch-job-progress-bar`

要求：
- 支持 0-100% 宽度更新
- 与当前后台风格一致

- [ ] **Step 3: 添加失败列表样式**

新增：
- `.batch-job-failures`
- `.batch-job-failure-item`
- `.batch-job-failure-reason`

- [ ] **Step 4: 提交**

```bash
git add internal/httpserver/adminui/styles.css
git commit -m "feat(admin-ui): add bulk job panel styles"
```

---

## Chunk 3: 前端统一状态模型

### Task 4: 在状态树中引入统一批量任务对象

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 在 [`state`](internal/httpserver/adminui/app.js:2) 中新增批量任务字段**

示例：
```js
batchJob: {
  visible: false,
  action: "",
  running: false,
  total: 0,
  completed: 0,
  failed: 0,
  current_id: 0,
  failures: [],
  retryable: []
}
```

- [ ] **Step 2: 增加状态辅助函数**

实现：
- `createBatchJob(action, total)`
- `updateBatchJobProgress(partial)`
- `appendBatchJobFailure(failure)`
- `finishBatchJob()`
- `clearBatchJob()`（若需要）

- [ ] **Step 3: 实现统一渲染函数**

实现：
- `renderBatchJobPanel()`

渲染内容至少包括：
- 任务类型
- 总数/已完成/失败数
- 当前处理资产 ID
- 百分比进度条
- 失败明细列表
- 重试按钮状态

- [ ] **Step 4: 提交**

```bash
git add internal/httpserver/adminui/app.js
git commit -m "feat(admin-ui): add unified bulk job state model"
```

---

## Chunk 4: 接入批量标签

### Task 5: 为 [`batchApplyTagsSelected()`](internal/httpserver/adminui/app.js:576) 接入进度与失败明细

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 执行前初始化任务**

调用：
- `createBatchJob("tags", ids.length)`
- `renderBatchJobPanel()`

- [ ] **Step 2: 每次循环后更新进度**

每处理 1 个资产后：
- 设置 `current_id`
- 更新 `completed`
- 若失败则记录 `id/action/reason/retry_payload`

- [ ] **Step 3: 结束时完成任务**

- 成功全部完成：保留面板并显示完成态
- 部分失败：保留失败明细与重试按钮

- [ ] **Step 4: 提交**

```bash
git add internal/httpserver/adminui/app.js
git commit -m "feat(admin-ui): observe bulk tag jobs"
```

### Task 6: 为批量标签生成可重试 payload

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 失败时记录原始更新 payload**

失败项结构：
```js
{
  id,
  action: "tags",
  reason,
  retry_payload: payload
}
```

- [ ] **Step 2: 验证失败项结构完整**

Expected:
- 后续无需重新推导 tags 逻辑即可重试

---

## Chunk 5: 接入批量回滚

### Task 7: 为 [`batchRollbackSelected()`](internal/httpserver/adminui/app.js:660) 接入进度与失败明细

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 初始化回滚任务**

调用：
- `createBatchJob("rollback", ids.length)`

- [ ] **Step 2: 每项处理时记录当前资产与目标版本**

对于：
- 固定版本号模式
- 自动上一版本模式

都要统一记录当前处理项。

- [ ] **Step 3: 无上一版本时记录失败项**

失败项中应保留：
```js
{
  id,
  action: "rollback",
  reason: "无上一版本可回滚",
  retry_payload: { tenant_id, asset_id, version, mode }
}
```

- [ ] **Step 4: 提交**

```bash
git add internal/httpserver/adminui/app.js
git commit -m "feat(admin-ui): observe bulk rollback jobs"
```

---

## Chunk 6: 接入批量软删除

### Task 8: 为 [`batchSoftDeleteSelected()`](internal/httpserver/adminui/app.js:719) 接入进度与失败明细

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 初始化删除任务**

调用：
- `createBatchJob("delete", ids.length)`

- [ ] **Step 2: 每次删除后更新面板**

记录：
- 当前资产 ID
- 已完成数
- 失败数

- [ ] **Step 3: 失败时记录 retry payload**

```js
{
  id,
  action: "delete",
  reason,
  retry_payload: { id, tenant_id }
}
```

- [ ] **Step 4: 提交**

```bash
git add internal/httpserver/adminui/app.js
git commit -m "feat(admin-ui): observe bulk delete jobs"
```

---

## Chunk 7: 失败项重试能力

### Task 9: 增加“重试失败项”统一执行器

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 实现 `retryFailedBatchJobItems()`**

仅针对最近一次任务的失败集合执行。

逻辑：
- 读取 `state.batchJob.failures`
- 针对不同 `action` 使用对应 API 重新执行
- 成功则从失败列表移除
- 失败则更新 reason

- [ ] **Step 2: 绑定面板按钮**

绑定：
- `#batch-job-retry-btn`

- [ ] **Step 3: 重试时也显示进度条**

重试过程应作为当前任务的一次新的运行态，但不丢失原 action 上下文。

- [ ] **Step 4: 提交**

```bash
git add internal/httpserver/adminui/app.js internal/httpserver/adminui/index.html
git commit -m "feat(admin-ui): add retry for failed bulk items"
```

---

## Chunk 8: 验证

### Task 10: 本地静态与交互验证

**Files:**
- Test: `internal/httpserver/adminui/app.js`
- Test: `internal/httpserver/adminui/index.html`
- Test: `internal/httpserver/adminui/styles.css`

- [ ] **Step 1: 语法检查**

Run: `node --check ./internal/httpserver/adminui/app.js`
Expected: 无语法错误

- [ ] **Step 2: 手工验证批量标签**

Expected:
- 面板显示总数/进度/失败明细
- 失败后可点击重试失败项

- [ ] **Step 3: 手工验证批量回滚**

Expected:
- 固定版本号模式显示进度
- 自动上一版本模式在无上一版本时记录失败项

- [ ] **Step 4: 手工验证批量软删除**

Expected:
- 删除过程中可见进度递增
- 若失败，失败项进入列表

### Task 11: 远程联调验证（10.100.1.13）

**Files:**
- Test: 远程服务运行实例

- [ ] **Step 1: 准备失败样例数据**

构造：
- 至少 1 个可成功操作资产
- 至少 1 个会失败的资产/条件

- [ ] **Step 2: 验证批量回滚失败明细**

使用“自动上一版本”模式验证：
- 一个资产可成功回滚
- 一个资产无上一版本，进入失败列表

- [ ] **Step 3: 验证重试失败项**

Expected:
- 重试按钮仅重试失败集合
- 成功后从失败列表移除
- 再失败则更新原因

---

## Chunk 9: 收尾与回滚

### Task 12: 收尾与回滚策略

**Files:**
- Modify: `docs/plans/2026-03-24-admin-ui-bulk-ops-observability-design.md`（如有设计偏差需补充）

- [ ] **Step 1: 记录实现差异**

如果最终实现与设计略有不同，补充说明。

- [ ] **Step 2: 准备最终提交**

```bash
git add internal/httpserver/adminui/index.html internal/httpserver/adminui/app.js internal/httpserver/adminui/styles.css docs/plans/2026-03-24-admin-ui-bulk-ops-observability-design.md
git commit -m "feat(admin-ui): add bulk operation observability"
```

- [ ] **Step 3: 回滚策略**

回滚仅需还原：
- `internal/httpserver/adminui/index.html`
- `internal/httpserver/adminui/app.js`
- `internal/httpserver/adminui/styles.css`

不会涉及后端 schema 或接口变更。

---

## 验收标准（Definition of Done）

- 三类批量操作均显示统一状态面板
- 面板显示总数、已完成、失败数、当前项、进度条
- 失败项以列表形式可见
- “重试失败项”只针对最近一次任务中的失败集合执行
- 重试成功后失败列表减少，重试失败后原因更新
- 前端语法检查通过，且不破坏既有批量功能
