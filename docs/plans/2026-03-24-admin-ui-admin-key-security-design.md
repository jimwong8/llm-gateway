# 2026-03-24 Admin UI Admin Key 安全增强设计

## 1. 背景

当前管理端通过 [`ensureAdminKey()`](internal/httpserver/adminui/app.js:31) 在首次请求时使用 `prompt` 输入 Admin Key，并存放在浏览器会话存储中。该方案能工作，但存在明显短板：

- 没有显式的设置/更新入口
- 没有清除入口
- 无法直观看到当前是否已配置密钥
- 无法感知会话是否即将过期
- 无法在长时间不操作后自动失效并提醒重新设置

因此需要在不改动后端鉴权协议的前提下，为前端管理页补充显式安全状态与会话管理能力。

## 2. 目标

为 Admin UI 增加一套轻量但清晰的安全交互层，支持：

- Admin Key 显式设置/更新
- Admin Key 显式清除
- Admin Key 脱敏状态展示
- 剩余会话时间显示
- 30 分钟无操作自动过期
- 距离过期 5 分钟时 warning 提示
- 过期后阻止继续请求，直到重新设置密钥

## 3. 范围

### 3.1 In Scope
- 在 [`index.html`](internal/httpserver/adminui/index.html:20) 增加顶部安全状态栏
- 在 [`app.js`](internal/httpserver/adminui/app.js:27) 扩展会话元数据管理
- 使用 `sessionStorage` 存储密钥与最近活动时间
- 每次成功管理请求后自动刷新最近活动时间
- 到期后前端阻断后续管理请求

### 3.2 Out of Scope
- 后端 token 刷新机制
- 长期记住登录
- 多用户权限分级
- 浏览器外部密钥托管

## 4. 架构设计

### 4.1 状态栏位置
在 [`index.html`](internal/httpserver/adminui/index.html:20) 主内容区域顶部新增安全状态栏，位于筛选区上方。

状态栏包含：
- 当前 Admin Key 状态（未设置 / 已设置 / 即将过期 / 已过期）
- 脱敏显示（如 `sk-****abcd`）
- 剩余会话时间（分钟级）
- `设置/更新` 按钮
- `清除` 按钮

### 4.2 存储策略
继续使用 `sessionStorage`，但从“只存密钥”扩展为“密钥 + 元数据”：

- `admin_api_key`
- `admin_api_key_last_active_at`
- 可选：`admin_api_key_set_at`

这样不改变既有工作模式，也能实现会话时效控制。

### 4.3 过期策略
- 默认有效期：30 分钟无操作
- warning 阈值：剩余 5 分钟
- 每次成功管理请求后刷新 `last_active_at`
- 如果检测到已过期：
  - 前端把状态栏标记为 expired
  - 后续请求前直接抛出错误，提示重新设置密钥

## 5. 交互设计

### 5.1 设置/更新
点击“设置/更新”时：
- 复用现有输入思路，但不再只依赖首次请求时的 `prompt`
- 用户输入后立即写入 `sessionStorage`
- 更新状态栏显示与剩余时间

### 5.2 清除
点击“清除”时：
- 删除所有 Admin Key 相关 `sessionStorage` 项
- 清空状态栏剩余时间与脱敏状态
- 后续请求需重新设置

### 5.3 状态显示
状态优先级：
- 未设置：neutral
- 已设置：normal
- 距过期 5 分钟内：warning
- 已过期：expired

## 6. 与现有代码集成点

### 6.1 需要调整的函数
- [`getAdminKey()`](internal/httpserver/adminui/app.js:27)
- [`ensureAdminKey()`](internal/httpserver/adminui/app.js:31)
- [`apiRequest()`](internal/httpserver/adminui/app.js:49)

### 6.2 新增建议函数
- `getAdminKeyMeta()`
- `setAdminKey()`
- `clearAdminKey()`
- `maskAdminKey()`
- `getAdminKeyRemainingMs()`
- `isAdminKeyExpired()`
- `touchAdminKeyActivity()`
- `renderAdminKeyStatus()`

## 7. UI 样式
文件：[`styles.css`](internal/httpserver/adminui/styles.css:1)

新增样式：
- `.security-bar`
- `.security-status`
- `.security-status.warning`
- `.security-status.expired`
- `.security-key-mask`
- `.security-actions`

## 8. 验收标准

### 8.1 正常路径
- 可通过状态栏设置 Admin Key
- 设置后显示脱敏值与剩余时间
- 每次成功请求后剩余时间刷新

### 8.2 清除路径
- 点击清除后密钥与元数据全部移除
- 状态栏恢复未设置状态
- 后续请求要求重新设置

### 8.3 过期路径
- 30 分钟无操作后状态变为 expired
- 距离过期 5 分钟内出现 warning 样式
- 过期后阻止继续请求并提示重新设置

## 9. 风险与缓解

### 风险 1：前端时间与用户系统时间不一致
缓解：首版接受浏览器本地时间驱动，避免引入后端时间同步复杂度。

### 风险 2：状态栏与真实请求状态不同步
缓解：在 [`apiRequest()`](internal/httpserver/adminui/app.js:49) 统一刷新活动时间，并在每次渲染时重新计算剩余时间。

### 风险 3：脱敏逻辑泄露过多信息
缓解：仅展示前缀少量字符与尾部少量字符，中间统一隐藏。

## 10. 结论

推荐采用“顶部安全状态栏 + 显式设置/清除 + 30 分钟无操作过期 + 5 分钟 warning 提示”方案。它与现有 Admin UI 架构兼容、改动范围可控，能显著提升管理密钥的可见性与安全性。