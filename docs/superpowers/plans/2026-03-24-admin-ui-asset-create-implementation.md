# Admin UI 资产创建能力 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在管理页新增“新建资产”弹窗，接入 `POST /admin/assets`，并在创建成功后刷新列表且高亮新记录。

**Architecture:** 基于现有 Admin UI 单页结构，在资产视图增加创建入口与独立 create modal。复用既有 API 封装与错误映射，新增最小状态字段 `lastCreatedAssetID` 和短时高亮机制，避免改动后端语义。通过前端必填校验与按钮禁用控制，确保可用性与避免重复提交。

**Tech Stack:** 原生 HTML/CSS/JavaScript、Go `net/http` 静态托管、远程 SSH + curl 联调

---

## Chunk 1: 文件结构与边界确认

### Task 1: 明确改动文件与责任

**Files:**
- Modify: `internal/httpserver/adminui/index.html`
- Modify: `internal/httpserver/adminui/styles.css`
- Modify: `internal/httpserver/adminui/app.js`
- Modify: `docs/plans/2026-03-24-admin-ui-implementation-plan.md`（如已有实施计划总览，追加本次子任务）

- [ ] **Step 1: 记录本次功能边界**

在任务备注中固定：
- 只新增“创建资产”能力
- 不改后端接口定义
- 不引入前端框架或构建工具

- [ ] **Step 2: 确认复用点**

阅读并确认复用以下已有函数：
- `apiPost()`
- `normalizeError()`
- `parseTagsInput()`
- `renderAssets()`

- [ ] **Step 3: 创建一次基线提交（可选）**

Run: `git status`
Expected: 工作区状态清晰可追踪

Run:
```bash
git add docs/plans/2026-03-24-admin-ui-asset-create-design.md
git commit -m "docs: add admin ui asset-create design"
```
Expected: 设计文档独立提交，后续实现可回滚

---

## Chunk 2: UI 结构实现（按钮 + Create Modal）

### Task 2: 在资产视图增加“新建资产”入口

**Files:**
- Modify: `internal/httpserver/adminui/index.html`

- [ ] **Step 1: 添加创建按钮容器（先写标记）**

在主视图区域增加资产操作栏，例如：
```html
<section class="view-actions" id="asset-actions">
  <button id="create-asset-open">新建资产</button>
</section>
```

- [ ] **Step 2: 刷新页面静态检查**

Run: 打开 `/admin/ui` 并确认按钮可见
Expected: 页面可正常渲染，无控制台语法错误

- [ ] **Step 3: 提交结构变更**

```bash
git add internal/httpserver/adminui/index.html
git commit -m "feat(admin-ui): add create asset entry button"
```

### Task 3: 增加 Create Modal 字段

**Files:**
- Modify: `internal/httpserver/adminui/index.html`

- [ ] **Step 1: 新增 create modal 结构（独立于 edit modal）**

字段集合：
- 必填：`create_tenant_id` / `create_source_model` / `create_task_type` / `create_title` / `create_summary`
- 可选：`create_tags` / `create_user_id` / `create_session_id` / `create_source_request_id`

- [ ] **Step 2: 添加控制按钮**

新增：
- `create-close`
- `create-cancel`
- `create-save`

- [ ] **Step 3: 静态验收**

Run: 手动触发显示（临时移除 hidden 或控制台设置）
Expected: modal 布局完整、字段齐全

- [ ] **Step 4: 提交**

```bash
git add internal/httpserver/adminui/index.html
git commit -m "feat(admin-ui): add create asset modal markup"
```

---

## Chunk 3: 样式实现（Create Modal + 行高亮）

### Task 4: 扩展创建相关样式

**Files:**
- Modify: `internal/httpserver/adminui/styles.css`

- [ ] **Step 1: 新增操作栏样式**

为 `.view-actions` 添加布局，使“刷新筛选”与“新建资产”视觉分层清晰。

- [ ] **Step 2: 复用 modal 基础样式**

仅补充 create 场景的细节样式，避免重复定义。

- [ ] **Step 3: 新增高亮样式**

示例：
```css
.table tr.row-highlight {
  background: #ecfeff;
  transition: background-color 0.2s ease;
}
```

- [ ] **Step 4: 手工验收**

Expected:
- 按钮位置不挤压筛选栏
- create modal 与 edit modal 风格一致
- 高亮样式在浅色主题可辨识

- [ ] **Step 5: 提交**

```bash
git add internal/httpserver/adminui/styles.css
git commit -m "feat(admin-ui): add create modal and row highlight styles"
```

---

## Chunk 4: 交互实现（状态 + 校验 + 提交 + 高亮）

### Task 5: 增加创建状态与弹窗开关

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 扩展状态结构**

在 `state` 中新增：
- `lastCreatedAssetID: 0`

- [ ] **Step 2: 新增弹窗控制函数**

实现：
- `openCreateModal()`
- `closeCreateModal()`

行为要求：
- 打开时默认注入当前筛选 tenant（若存在）
- 关闭时仅隐藏，不清空全局筛选状态

- [ ] **Step 3: 绑定按钮事件**

在 `bootstrap()` 中绑定：
- `create-asset-open`
- `create-close`
- `create-cancel`

- [ ] **Step 4: 提交**

```bash
git add internal/httpserver/adminui/app.js
git commit -m "feat(admin-ui): add create modal open close behavior"
```

### Task 6: 实现 `saveCreateAsset()` 与前端校验

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 先写最小失败验证（手工）**

场景：留空必填字段点击保存。
Expected: 显示“必填字段缺失”错误，且不发起网络请求。

- [ ] **Step 2: 实现必填校验逻辑**

必填字段：
- `tenant_id`
- `source_model`
- `task_type`
- `title`
- `summary`

可选字段透传：
- `tags`（使用 `parseTagsInput()`）
- `user_id`
- `session_id`
- `source_request_id`

- [ ] **Step 3: 接入创建请求**

调用：
- `apiPost("/admin/assets", payload)`

成功后：
- 读取返回 `id`
- 写入 `state.lastCreatedAssetID`
- 关闭 create modal
- 调用 `render()` 刷新列表

失败后：
- 使用 `normalizeError()` 对接消息
- 保留弹窗和用户输入

- [ ] **Step 4: 防重复提交**

保存按钮请求期间禁用，请求结束恢复。

- [ ] **Step 5: 提交**

```bash
git add internal/httpserver/adminui/app.js
git commit -m "feat(admin-ui): implement create asset validation and post flow"
```

### Task 7: 在资产表渲染中加入新建高亮逻辑

**Files:**
- Modify: `internal/httpserver/adminui/app.js`
- Modify: `internal/httpserver/adminui/styles.css`（若需微调）

- [ ] **Step 1: 在 `renderAssets()` 标记目标行**

当 `row.id === state.lastCreatedAssetID` 时为 `<tr>` 添加 `row-highlight`。

- [ ] **Step 2: 增加自动移除机制**

刷新后启动一次 6 秒定时器：
- 到时清理高亮状态（`lastCreatedAssetID = 0`）
- 避免永久高亮

- [ ] **Step 3: 手工验收**

Expected:
- 创建成功后目标行高亮
- 6 秒后自动恢复常态

- [ ] **Step 4: 提交**

```bash
git add internal/httpserver/adminui/app.js internal/httpserver/adminui/styles.css
git commit -m "feat(admin-ui): highlight newly created asset row"
```

---

## Chunk 5: 验证与回归

### Task 8: 本地/工作区验证

**Files:**
- Test: `internal/httpserver/adminui/index.html`
- Test: `internal/httpserver/adminui/app.js`
- Test: `internal/httpserver/adminui/styles.css`

- [ ] **Step 1: 功能冒烟测试**

手工验证：
1. 打开 `/admin/ui`
2. 点击“新建资产”
3. 输入最小必填并保存
4. 观察成功消息、列表刷新、高亮

Expected: 创建链路可用

- [ ] **Step 2: 异常路径测试**

场景：
- 不填必填直接保存
- 使用错误 Admin Key

Expected:
- 分别提示“必填缺失”与“401 错误映射消息”

- [ ] **Step 3: 回归测试**

验证以下既有能力未受影响：
- 编辑资产
- 软删除
- 版本抽屉与回滚
- 统计页与审计页切换

### Task 9: 远程联调验证（10.100.1.13）

**Files:**
- Test: 远程运行中服务（无新增仓库文件）

- [ ] **Step 1: 同步并编译**

Run（示例）：
```bash
ssh jimwong@10.100.1.13 'cd /home/jimwong/llm-gateway/gateway && /usr/local/go/bin/go build -o server ./cmd/server'
```
Expected: build 成功

- [ ] **Step 2: 重启服务**

Run（示例）：
```bash
ssh jimwong@10.100.1.13 'sudo systemctl restart llm-gateway'
```
Expected: 服务恢复运行

- [ ] **Step 3: 创建接口实测**

Run: 使用 `curl -X POST /admin/assets` 创建测试资产
Expected: 返回新 `id`

- [ ] **Step 4: 列表回读验证**

Run: `GET /admin/assets?tenant_id=...`
Expected: 新记录字段与提交内容一致

- [ ] **Step 5: UI 验证截图（可选）**

Expected: 截图可见新建成功消息 + 高亮行

---

## Chunk 6: 文档与收尾

### Task 10: 更新实施记录并准备合并

**Files:**
- Modify: `docs/plans/2026-03-24-admin-ui-implementation-plan.md`（如存在）
- Create/Modify: 变更说明（PR 描述或发布记录）

- [ ] **Step 1: 记录实现差异**

补充实际落地与设计偏差（若有）。

- [ ] **Step 2: 汇总验证证据**

至少包含：
- 本地手工验证结果
- 远程接口验证结果

- [ ] **Step 3: 最终提交**

```bash
git add internal/httpserver/adminui/index.html internal/httpserver/adminui/styles.css internal/httpserver/adminui/app.js docs/plans/2026-03-24-admin-ui-implementation-plan.md
git commit -m "feat(admin-ui): add asset create modal and post workflow"
```

---

## 回滚策略

- UI 级回滚：仅回退以下文件到前一提交即可完全撤销本功能
  - `internal/httpserver/adminui/index.html`
  - `internal/httpserver/adminui/styles.css`
  - `internal/httpserver/adminui/app.js`
- 服务级回滚：远程恢复上一版二进制并重启服务
- 数据级说明：已创建测试资产可通过管理端软删除处理

## 验收标准（Definition of Done）

- 用户可在 Admin UI 完成资产创建
- 必填校验符合已确认规则
- 创建成功后列表刷新并高亮新记录
- 异常提示清晰，失败时不丢输入
- 既有编辑/删除/版本/统计/审计能力无回归
