# Admin UI Admin Key 安全增强 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 Admin UI 增加显式的 Admin Key 设置/清除入口、脱敏状态栏显示，以及 30 分钟无操作过期与 5 分钟 warning 提示。

**Architecture:** 在 [`internal/httpserver/adminui/index.html`](internal/httpserver/adminui/index.html:1) 主内容顶部增加安全状态栏，在 [`internal/httpserver/adminui/app.js`](internal/httpserver/adminui/app.js:1) 扩展 `sessionStorage` 存储的密钥元数据，并在统一请求入口里刷新会话活动时间。通过一个前端时效模型计算剩余时间、warning 和 expired 状态，并由状态栏统一渲染。

**Tech Stack:** 原生 HTML/CSS/JavaScript、浏览器 `sessionStorage`、现有 Admin API 请求封装 [`apiRequest()`](internal/httpserver/adminui/app.js:49)

---

## Chunk 1: 文件边界与职责确认

### Task 1: 固定改动范围

**Files:**
- Modify: `internal/httpserver/adminui/index.html`
- Modify: `internal/httpserver/adminui/app.js`
- Modify: `internal/httpserver/adminui/styles.css`
- Reference: `docs/plans/2026-03-24-admin-ui-admin-key-security-design.md`

- [ ] **Step 1: 明确只做前端安全增强**

本次不改动后端接口协议，不新增后端认证接口。

- [ ] **Step 2: 确认现有接入点**

重点接入现有函数：
- [`getAdminKey()`](internal/httpserver/adminui/app.js:27)
- [`ensureAdminKey()`](internal/httpserver/adminui/app.js:31)
- [`apiRequest()`](internal/httpserver/adminui/app.js:49)

- [ ] **Step 3: 确认会话策略**

按设计固定：
- 存储介质：`sessionStorage`
- 过期策略：30 分钟无操作失效
- warning 阈值：5 分钟

---

## Chunk 2: 状态栏 UI 骨架

### Task 2: 在页面加入安全状态栏

**Files:**
- Modify: `internal/httpserver/adminui/index.html`

- [ ] **Step 1: 在筛选栏上方增加状态栏容器**

增加结构示例：
```html
<section class="security-bar" id="security-bar">
  <div class="security-status" id="security-status">未设置 Admin Key</div>
  <div class="security-key-mask" id="security-key-mask">--</div>
  <div class="security-expiry" id="security-expiry">--</div>
  <div class="security-actions">
    <button id="admin-key-set-btn">设置/更新</button>
    <button id="admin-key-clear-btn">清除</button>
  </div>
</section>
```

- [ ] **Step 2: 检查状态栏布局位置**

Expected:
- 位于 [`#filters`](internal/httpserver/adminui/index.html:21) 上方
- 不影响 [`preset-bar`](internal/httpserver/adminui/index.html:37) 与批量操作区

- [ ] **Step 3: 提交**

```bash
git add internal/httpserver/adminui/index.html
git commit -m "feat(admin-ui): add admin key security status bar"
```

### Task 3: 增加安全状态栏样式

**Files:**
- Modify: `internal/httpserver/adminui/styles.css`

- [ ] **Step 1: 增加状态栏容器样式**

新增：
- `.security-bar`
- `.security-actions`
- `.security-key-mask`
- `.security-expiry`

- [ ] **Step 2: 增加状态样式**

新增：
- `.security-status`
- `.security-status.warning`
- `.security-status.expired`

Expected:
- normal: 中性可读
- warning: 黄色/橙色提醒
- expired: 红色高亮

- [ ] **Step 3: 提交**

```bash
git add internal/httpserver/adminui/styles.css
git commit -m "feat(admin-ui): style admin key security bar"
```

---

## Chunk 3: Admin Key 元数据模型

### Task 4: 在前端增加会话元数据常量与工具函数

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 新增 storage key 常量**

增加：
- `ADMIN_KEY_STORAGE_KEY`
- `ADMIN_KEY_LAST_ACTIVE_KEY`
- `ADMIN_KEY_SET_AT_KEY`
- `ADMIN_KEY_IDLE_TTL_MS`
- `ADMIN_KEY_WARNING_MS`

- [ ] **Step 2: 新增工具函数**

实现：
- `setAdminKey(key)`
- `clearAdminKey()`
- `getAdminKeyLastActiveAt()`
- `touchAdminKeyActivity()`
- `getAdminKeyRemainingMs()`
- `isAdminKeyExpired()`
- `maskAdminKey(key)`

- [ ] **Step 3: 确认脱敏规则**

建议：
- 长度较短时仅显示前 2 后 2
- 常规情况下显示前 3 后 4，中间全部 `*`

- [ ] **Step 4: 提交**

```bash
git add internal/httpserver/adminui/app.js
git commit -m "feat(admin-ui): add admin key session metadata helpers"
```

---

## Chunk 4: 请求生命周期接入

### Task 5: 让请求入口支持过期判断与活动刷新

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 改造 [`getAdminKey()`](internal/httpserver/adminui/app.js:27)**

行为：
- 若密钥不存在，返回空
- 若存在但已过期，清理并返回空

- [ ] **Step 2: 改造 [`ensureAdminKey()`](internal/httpserver/adminui/app.js:31)**

行为：
- 保留兜底输入能力
- 但优先配合显式按钮入口使用
- 输入成功后调用 `setAdminKey()`

- [ ] **Step 3: 改造 [`apiRequest()`](internal/httpserver/adminui/app.js:49)**

在请求成功后调用：
- `touchAdminKeyActivity()`
- `renderAdminKeyStatus()`

在请求前若检测过期：
- 抛出明确错误：`Admin Key 已过期，请重新设置`

- [ ] **Step 4: 提交**

```bash
git add internal/httpserver/adminui/app.js
git commit -m "feat(admin-ui): enforce admin key idle expiration"
```

---

## Chunk 5: 状态栏渲染与交互

### Task 6: 实现状态栏渲染函数

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 实现 `renderAdminKeyStatus()`**

渲染内容：
- 是否已设置
- 脱敏 key
- 剩余时间（分钟级）
- 状态类名（normal / warning / expired）

- [ ] **Step 2: 剩余时间显示规则**

建议文案：
- 未设置：`未设置 Admin Key`
- 正常：`剩余 23 分钟`
- warning：`即将过期：剩余 4 分钟`
- expired：`已过期，请重新设置`

- [ ] **Step 3: 页面初始化时调用**

在 [`bootstrap()`](internal/httpserver/adminui/app.js:1237) 中加入：
- 初始渲染
- 定时刷新（例如每 30 秒）

- [ ] **Step 4: 提交**

```bash
git add internal/httpserver/adminui/app.js
git commit -m "feat(admin-ui): render admin key security state"
```

### Task 7: 绑定“设置/更新”和“清除”按钮

**Files:**
- Modify: `internal/httpserver/adminui/app.js`

- [ ] **Step 1: 绑定 `#admin-key-set-btn`**

行为：
- 点击后显式弹出输入
- 写入 key 与时间戳
- 刷新状态栏

- [ ] **Step 2: 绑定 `#admin-key-clear-btn`**

行为：
- 清除所有 Admin Key 相关存储
- 清空状态栏提醒与剩余时间显示

- [ ] **Step 3: 失败提示与确认**

建议：
- 清除前可用 `confirm`
- 设置失败时保留旧值（若已有）

- [ ] **Step 4: 提交**

```bash
git add internal/httpserver/adminui/app.js
git commit -m "feat(admin-ui): add admin key set and clear actions"
```

---

## Chunk 6: 过期提醒与 warning 态

### Task 8: 实现 warning / expired 状态切换

**Files:**
- Modify: `internal/httpserver/adminui/app.js`
- Modify: `internal/httpserver/adminui/styles.css`

- [ ] **Step 1: 剩余 5 分钟内切换 warning 类**

Expected:
- 状态栏颜色变化明显
- 仍允许请求

- [ ] **Step 2: 已过期切换 expired 类**

Expected:
- 状态栏显示已过期
- 后续请求被前端阻断

- [ ] **Step 3: 清除过期后残留状态**

当重新设置密钥后：
- warning / expired 样式应恢复正常

- [ ] **Step 4: 提交**

```bash
git add internal/httpserver/adminui/app.js internal/httpserver/adminui/styles.css
git commit -m "feat(admin-ui): add admin key warning and expired states"
```

---

## Chunk 7: 验证

### Task 9: 本地验证

**Files:**
- Test: `internal/httpserver/adminui/app.js`
- Test: `internal/httpserver/adminui/index.html`
- Test: `internal/httpserver/adminui/styles.css`

- [ ] **Step 1: 语法检查**

Run: `node --check ./internal/httpserver/adminui/app.js`
Expected: PASS

- [ ] **Step 2: 设置/清除验证**

验证：
- 设置后状态栏显示脱敏值
- 清除后恢复未设置状态

- [ ] **Step 3: URL/筛选不回归**

验证：
- 不影响 [`writeQueryToURL()`](internal/httpserver/adminui/app.js:213)
- 不影响筛选预设、批量功能、创建编辑流程

### Task 10: 手工过期验证

**Files:**
- Test: 浏览器会话存储与页面交互

- [ ] **Step 1: 模拟接近过期**

通过开发者工具把 `last_active_at` 调整到剩余 4 分钟。
Expected: 状态栏进入 warning

- [ ] **Step 2: 模拟已过期**

通过开发者工具把 `last_active_at` 调整到超时。
Expected: 状态栏进入 expired，后续请求前端直接报错

- [ ] **Step 3: 重新设置后恢复**

Expected: 状态恢复 normal

---

## Chunk 8: 收尾与回滚

### Task 11: 收尾提交与回滚策略

**Files:**
- Modify: `docs/plans/2026-03-24-admin-ui-admin-key-security-design.md`（如有实际偏差）

- [ ] **Step 1: 记录实现偏差**

若实现细节与设计略有差异，补充说明。

- [ ] **Step 2: 最终提交**

```bash
git add internal/httpserver/adminui/index.html internal/httpserver/adminui/app.js internal/httpserver/adminui/styles.css docs/plans/2026-03-24-admin-ui-admin-key-security-design.md
git commit -m "feat(admin-ui): add admin key security status bar"
```

- [ ] **Step 3: 回滚策略**

回滚仅需恢复：
- `internal/httpserver/adminui/index.html`
- `internal/httpserver/adminui/app.js`
- `internal/httpserver/adminui/styles.css`

不会影响后端接口或数据库。

---

## 验收标准（Definition of Done）

- 状态栏可显示 Admin Key 是否已设置
- 状态栏显示脱敏密钥与剩余时间
- `设置/更新` 与 `清除` 按钮可用
- 30 分钟无操作后会话过期
- 距过期 5 分钟进入 warning 状态
- 过期后后续请求被前端阻断，直到重新设置密钥
- 不破坏现有筛选、批量操作、导出与编辑能力
