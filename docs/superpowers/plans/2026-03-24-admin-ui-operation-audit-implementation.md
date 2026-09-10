# Admin UI 本地操作日志 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为管理端高价值操作增加本地操作日志、最近日志列表，以及 JSON/CSV 导出能力。

**Architecture:** 在 [`internal/httpserver/adminui/app.js`](internal/httpserver/adminui/app.js:1) 中引入统一的 localStorage 日志模型与写入/渲染/导出函数，在 [`internal/httpserver/adminui/index.html`](internal/httpserver/adminui/index.html:47) 的操作区附近增加最近日志面板与导出入口，并在 [`internal/httpserver/adminui/styles.css`](internal/httpserver/adminui/styles.css:1) 中补充列表与导出按钮样式。所有高价值操作在成功/失败分支统一调用日志入口，避免分散埋点。

**Tech Stack:** 原生 HTML/CSS/JavaScript、浏览器 `localStorage`、现有 Admin UI 操作流、Blob 下载导出

---

## Chunk 1: 文件边界与日志模型

### Task 1: 固定改动文件与职责

**Files:**
- Modify: `internal/httpserver/adminui/index.html`
- Modify: `internal/httpserver/adminui/app.js`
- Modify: `internal/httpserver/adminui/styles.css`
- Reference: `docs/plans/2026-03-24-admin-ui-operation-audit-design.md`

- [ ] **Step 1: 确认高价值动作范围**

本次只覆盖：
- Admin Key 设置 / 清除
- 资产创建 / 编辑 / 删除
- 批量标签 / 批量回滚 / 批量删除
- 资产导出 JSON / CSV

- [ ] **Step 2: 固定日志模型**

每条日志最少包含：
- `id`
- `action`
- `created_at`
- `status`
- `summary`
- `payload`

- [ ] **Step 3: 确认保留策略**

固定：
- `localStorage`
- 最多保留最近 200 条
- 超出自动淘汰最旧记录

---

## Chunk 2: UI 结构

### Task 2: 增加最近日志列表与导出入口

**Files:**
- Modify: `internal/httpserver/adminui/index.html`

- [ ] **Step 1: 在操作区附近新增日志导出按钮**

增加：
- `#export-oplog-json-btn`
- `#export-oplog-csv-btn`

- [ ] **Step 2: 增加最近日志列表容器**

建议结构：
```html
<section class="operation-log-panel" id="operation-log-panel">
  <div class="operation-log-header">
    <strong>最近操作日志</strong>
  </div>
  <div id="operation-log-list"></div>
</section>
```

- [ ] **Step 3: 静态检查位置**

Expected:
- 不挤压现有批量操作按钮
- 位于导出区附近，语义一致

- [ ] **Step 4: 提交**

```bash
git add internal/httpserver/adminui/index.html
git commit -m "feat(admin-ui): add operation log panel skeleton"
```

### Task 3: 为日志列表与导出入口增加样式

**Files:**
- Modify: `internal/httpserver/adminui/styles.css`

- [ ] **Step 1: 新增日志面板样式**

新增：
- `.operation-log-panel`
- `.operation-log-header`
- `.operation-log-list`
- `.operation-log-item`

- [ ] **Step 2: 新增状态样式**

新增：
- `.operation-log-item.success`
- `.operation-log-item.error`

- [ ] **Step 3: 新增导出按钮样式**

让操作日志导出按钮与资产导出按钮风格一致但可区分。

- [ ] **Step 4: 提交**

```bash
git add internal/httpserver/adminui/styles.css
git commit -m "feat(admin-ui): style operation log panel"
```

---

## Chunk 3: localStorage 模型与基础函数

### Task 4: 新增日志存储常量与读写函数

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 增加 storage key 常量**

例如：
```js
const OPLOG_STORAGE_KEY = "admin_ui_operation_logs_v1";
const OPLOG_LIMIT = 200;
```

- [ ] **Step 2: 实现基础函数**

实现：
- `readOperationLogs()`
- `writeOperationLogs(logs)`
- `appendOperationLog(entry)`
- `buildOperationLogEntry(action, status, summary, payload)`

- [ ] **Step 3: 淘汰策略**

在 `writeOperationLogs()` 中只保留最近 200 条。

- [ ] **Step 4: 提交**

```bash
git add internal/httpserver/adminui/app.js
git commit -m "feat(admin-ui): add local operation log storage"
```

### Task 5: 实现最近日志列表渲染

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 实现 `renderOperationLogs()`**

展示：
- 时间
- 动作
- 状态
- 摘要

- [ ] **Step 2: 页面初始化时渲染**

在 [`bootstrap()`](internal/httpserver/adminui/app.js:1237) 中调用 `renderOperationLogs()`。

- [ ] **Step 3: 每次写入后刷新列表**

Expected:
- 日志列表与本地存储保持同步

- [ ] **Step 4: 提交**

```bash
git add internal/httpserver/adminui/app.js
git commit -m "feat(admin-ui): render recent operation logs"
```

---

## Chunk 4: 导出能力

### Task 6: 增加操作日志 JSON / CSV 导出

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 实现 `exportOperationLogs(format)`**

字段：
- `id`
- `action`
- `created_at`
- `status`
- `summary`
- `payload`

- [ ] **Step 2: 复用现有下载能力**

复用：
- [`csvEscape()`](internal/httpserver/adminui/app.js:114)
- [`triggerDownload()`](internal/httpserver/adminui/app.js:120)

- [ ] **Step 3: 文件名带时间戳**

例如：
- `admin-operation-logs-2026-03-24T06-00-00.json`
- `admin-operation-logs-2026-03-24T06-00-00.csv`

- [ ] **Step 4: 绑定导出按钮**

绑定：
- `#export-oplog-json-btn`
- `#export-oplog-csv-btn`

- [ ] **Step 5: 提交**

```bash
git add internal/httpserver/adminui/app.js internal/httpserver/adminui/index.html
git commit -m "feat(admin-ui): export local operation logs"
```

---

## Chunk 5: 高价值动作埋点

### Task 7: 为 Admin Key 动作埋点

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 在设置成功后写入日志**

动作：
- `admin_key.set`

payload 仅保留脱敏摘要，不写明文。

- [ ] **Step 2: 在清除后写入日志**

动作：
- `admin_key.clear`

- [ ] **Step 3: 提交**

```bash
git add internal/httpserver/adminui/app.js
git commit -m "feat(admin-ui): log admin key operations"
```

### Task 8: 为资产创建/编辑/删除埋点

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 在 [`saveCreateAsset()`](internal/httpserver/adminui/app.js:1082) 成功/失败时写日志**

动作：
- `asset.create`

- [ ] **Step 2: 在 [`saveEditAsset()`](internal/httpserver/adminui/app.js:1130) 成功/失败时写日志**

动作：
- `asset.edit`

- [ ] **Step 3: 在单项删除动作中写日志**

动作：
- `asset.delete`

- [ ] **Step 4: 提交**

```bash
git add internal/httpserver/adminui/app.js
git commit -m "feat(admin-ui): log asset write operations"
```

### Task 9: 为批量动作埋点

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 为 [`batchApplyTagsSelected()`](internal/httpserver/adminui/app.js:723) 写日志**

动作：
- `batch.tags`

记录：
- 模式
- 成功数
- 失败数

- [ ] **Step 2: 为 [`batchRollbackSelected()`](internal/httpserver/adminui/app.js:819) 写日志**

动作：
- `batch.rollback`

记录：
- 模式
- 目标版本/自动上一版本
- 成功数
- 失败数

- [ ] **Step 3: 为 [`batchSoftDeleteSelected()`](internal/httpserver/adminui/app.js:906) 写日志**

动作：
- `batch.delete`

- [ ] **Step 4: 为资产导出写日志**

动作：
- `assets.export.json`
- `assets.export.csv`

- [ ] **Step 5: 提交**

```bash
git add internal/httpserver/adminui/app.js
git commit -m "feat(admin-ui): log bulk and export operations"
```

---

## Chunk 6: 验证

### Task 10: 本地验证

**Files:**
- Test: `internal/httpserver/adminui/app.js`
- Test: `internal/httpserver/adminui/index.html`
- Test: `internal/httpserver/adminui/styles.css`

- [ ] **Step 1: 语法检查**

Run: `node --check ./internal/httpserver/adminui/app.js`
Expected: PASS

- [ ] **Step 2: 手工验证日志写入**

执行以下动作并观察最近日志列表：
- 设置 Admin Key
- 创建资产
- 编辑资产
- 删除资产
- 批量标签
- 批量回滚
- 批量删除
- 导出资产 JSON/CSV

Expected: 每类动作都有对应日志项

- [ ] **Step 3: 手工验证保留上限**

模拟写入 >200 条，确认最旧日志被淘汰。

### Task 11: 导出验证

**Files:**
- Test: 浏览器下载内容

- [ ] **Step 1: 导出操作日志 JSON**

Expected:
- 文件可下载
- 字段完整

- [ ] **Step 2: 导出操作日志 CSV**

Expected:
- 文件可下载
- `payload` 为 JSON 字符串

- [ ] **Step 3: 文件名检查**

Expected:
- 文件名带当前时间戳

---

## Chunk 7: 收尾与回滚

### Task 12: 收尾提交与回滚策略

**Files:**
- Modify: `docs/plans/2026-03-24-admin-ui-operation-audit-design.md`（如实现有偏差）

- [ ] **Step 1: 记录实现偏差**

若实现细节与设计不同，补充说明。

- [ ] **Step 2: 最终提交**

```bash
git add internal/httpserver/adminui/index.html internal/httpserver/adminui/app.js internal/httpserver/adminui/styles.css docs/plans/2026-03-24-admin-ui-operation-audit-design.md
git commit -m "feat(admin-ui): add local operation audit logs"
```

- [ ] **Step 3: 回滚策略**

回滚只需恢复：
- `internal/httpserver/adminui/index.html`
- `internal/httpserver/adminui/app.js`
- `internal/httpserver/adminui/styles.css`

不会影响后端接口和数据库。

---

## 验收标准（Definition of Done）

- 高价值管理动作会写入本地日志
- 页面能展示最近操作日志
- 支持导出操作日志 JSON / CSV
- 最多保留 200 条，超出自动淘汰最旧记录
- 不记录 Admin Key 明文
- 不破坏现有创建、编辑、删除、批量、导出与安全状态能力
