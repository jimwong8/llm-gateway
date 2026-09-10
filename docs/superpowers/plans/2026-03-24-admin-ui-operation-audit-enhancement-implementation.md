# Admin UI 操作日志增强 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为本地操作日志增加动作筛选、关键词搜索，并补齐剩余高价值动作埋点与导出联动。

**Architecture:** 在 [`internal/httpserver/adminui/app.js`](internal/httpserver/adminui/app.js:241) 的本地日志模型上扩展过滤状态与渲染函数，让最近日志列表支持按动作和关键词筛选。关键词同时匹配 `action`、`summary`、`payload` 的 JSON 字符串；同时把尚未记录的高价值管理动作统一接入 [`appendOperationLog()`](internal/httpserver/adminui/app.js:271)。

**Tech Stack:** 原生 HTML/CSS/JavaScript、浏览器 `localStorage`、现有 Admin UI 操作流、Blob 下载导出

---

## Chunk 1: 文件边界与增强范围

### Task 1: 固定本次改动文件与范围

**Files:**
- Modify: `internal/httpserver/adminui/index.html`
- Modify: `internal/httpserver/adminui/app.js`
- Modify: `internal/httpserver/adminui/styles.css`
- Reference: `docs/plans/2026-03-24-admin-ui-operation-audit-design.md`

- [ ] **Step 1: 固定增强范围**

本次只做：
- 最近日志列表动作筛选
- 最近日志列表关键词搜索
- 补齐剩余高价值动作埋点
- 保持现有 JSON/CSV 日志导出能力

- [ ] **Step 2: 确认搜索规则**

关键词匹配：
- `action`
- `summary`
- `payload` 的 JSON 字符串

- [ ] **Step 3: 确认动作筛选方式**

使用固定下拉选项，默认：`全部动作`。

---

## Chunk 2: UI 增强

### Task 2: 在最近日志面板增加筛选器与搜索框

**Files:**
- Modify: `internal/httpserver/adminui/index.html`

- [ ] **Step 1: 在 [`operation-log-panel`](internal/httpserver/adminui/index.html:69) 中加入工具条**

建议结构：
```html
<div class="operation-log-toolbar">
  <select id="operation-log-action-filter">
    <option value="">全部动作</option>
    <option value="admin_key">admin_key</option>
    <option value="asset">asset</option>
    <option value="batch">batch</option>
    <option value="assets.export">assets.export</option>
  </select>
  <input id="operation-log-search" placeholder="搜索 action / summary / payload" />
</div>
```

- [ ] **Step 2: 静态检查布局**

Expected:
- 不挤压已有日志标题与导出按钮
- 工具条位于最近日志列表上方

- [ ] **Step 3: 提交**

```bash
git add internal/httpserver/adminui/index.html
git commit -m "feat(admin-ui): add operation log filters UI"
```

### Task 3: 增加日志筛选工具条样式

**Files:**
- Modify: `internal/httpserver/adminui/styles.css`

- [ ] **Step 1: 新增筛选工具条样式**

新增：
- `.operation-log-toolbar`
- `.operation-log-toolbar select`
- `.operation-log-toolbar input`

- [ ] **Step 2: 新增最近日志项辅助样式**

补充：
- `.operation-log-meta`
- `.operation-log-empty`

- [ ] **Step 3: 提交**

```bash
git add internal/httpserver/adminui/styles.css
git commit -m "feat(admin-ui): style operation log filters"
```

---

## Chunk 3: 日志过滤与渲染

### Task 4: 扩展日志渲染函数支持筛选

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 新增筛选辅助函数**

实现：
- `getOperationLogFilterState()`
- `filterOperationLogs(logs, filterState)`

其中关键词匹配逻辑：
```js
const haystack = [log.action, log.summary, JSON.stringify(log.payload || {})]
  .join(" ")
  .toLowerCase();
```

- [ ] **Step 2: 改造 [`renderOperationLogs()`](internal/httpserver/adminui/app.js:278)**

让它从：
- 原始 `readOperationLogs()`
- 经过 `filterOperationLogs()`
- 再渲染最近若干条（首版可维持最近 10 条）

- [ ] **Step 3: 空结果提示**

当过滤后为空时显示：
- `暂无匹配的操作日志`

- [ ] **Step 4: 提交**

```bash
git add internal/httpserver/adminui/app.js
git commit -m "feat(admin-ui): filter operation logs by action and keyword"
```

### Task 5: 绑定动作筛选与关键词搜索事件

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 在 [`bootstrap()`](internal/httpserver/adminui/app.js:1508) 绑定筛选控件**

绑定：
- `#operation-log-action-filter`
- `#operation-log-search`

- [ ] **Step 2: 事件策略**

建议：
- `select` 用 `change`
- `input` 用 `input` 或 `keydown(Enter)`

- [ ] **Step 3: 提交**

```bash
git add internal/httpserver/adminui/app.js
git commit -m "feat(admin-ui): bind operation log filter controls"
```

---

## Chunk 4: 补齐剩余高价值动作埋点

### Task 6: 为单项删除与批量操作补齐日志

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 单项删除埋点**

在单项删除成功/失败路径中加入：
- `asset.delete`

- [ ] **Step 2: 批量标签埋点**

在 [`batchApplyTagsSelected()`](internal/httpserver/adminui/app.js:723) 完成后记录：
- `batch.tags`
- payload 包含：模式、成功数、失败数

- [ ] **Step 3: 批量回滚埋点**

在 [`batchRollbackSelected()`](internal/httpserver/adminui/app.js:819) 完成后记录：
- `batch.rollback`
- payload 包含：模式、目标版本/上一版本、成功数、失败数

- [ ] **Step 4: 批量删除埋点**

在 [`batchSoftDeleteSelected()`](internal/httpserver/adminui/app.js:906) 完成后记录：
- `batch.delete`
- payload 包含：成功数、失败数

- [ ] **Step 5: 提交**

```bash
git add internal/httpserver/adminui/app.js
git commit -m "feat(admin-ui): log remaining high-value operations"
```

### Task 7: 导出联动补齐

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 资产导出已保持记录**

确认已有：
- `assets.export.json`
- `assets.export.csv`

- [ ] **Step 2: 操作日志导出联动**

在 [`exportOperationLogs()`](internal/httpserver/adminui/app.js:295) 的按钮调用处增加一条 UI 提示，并注意不要把“导出日志本身”递归写回正在导出的日志数据中。

- [ ] **Step 3: 提交**

```bash
git add internal/httpserver/adminui/app.js
git commit -m "feat(admin-ui): finalize operation log export linkage"
```

---

## Chunk 5: 验证

### Task 8: 本地语法与功能验证

**Files:**
- Test: `internal/httpserver/adminui/app.js`
- Test: `internal/httpserver/adminui/index.html`
- Test: `internal/httpserver/adminui/styles.css`

- [ ] **Step 1: 语法检查**

Run: `node --check ./internal/httpserver/adminui/app.js`
Expected: PASS

- [ ] **Step 2: 搜索/筛选验证**

手工验证：
- 动作下拉切换后日志列表变化
- 输入关键词可匹配 action / summary / payload

- [ ] **Step 3: 埋点验证**

至少触发：
- Admin Key 设置/清除
- 资产创建/编辑/删除
- 批量标签/回滚/删除
- 资产导出 JSON/CSV

Expected: 最近日志列表可见对应记录

- [ ] **Step 4: 导出验证**

验证：
- 操作日志 JSON 导出
- 操作日志 CSV 导出
- 文件名带时间戳

---

## Chunk 6: 收尾与回滚

### Task 9: 收尾与回滚策略

**Files:**
- Modify: `docs/plans/2026-03-24-admin-ui-operation-audit-design.md`（如实现偏差）

- [ ] **Step 1: 记录实现偏差**

若实现上与设计略有差异，补充说明。

- [ ] **Step 2: 最终提交**

```bash
git add internal/httpserver/adminui/index.html internal/httpserver/adminui/app.js internal/httpserver/adminui/styles.css docs/plans/2026-03-24-admin-ui-operation-audit-design.md
git commit -m "feat(admin-ui): enhance local operation audit logs"
```

- [ ] **Step 3: 回滚策略**

回滚仅需恢复：
- `internal/httpserver/adminui/index.html`
- `internal/httpserver/adminui/app.js`
- `internal/httpserver/adminui/styles.css`

不会影响后端接口与数据库。

---

## 验收标准（Definition of Done）

- 最近日志列表支持动作筛选与关键词搜索
- 关键词匹配 `action`、`summary`、`payload`
- 剩余高价值动作埋点补齐
- 操作日志 JSON / CSV 导出可用
- 本地最多保留 200 条日志
- 不记录 Admin Key 明文
- 不影响现有创建、编辑、删除、批量、导出与安全状态功能
