# 2026-03-24 Admin UI 审计联动设计

## 1. 背景

当前管理端已经具备较完整的控制面能力：
- 资产创建/编辑/删除
- 批量标签 / 批量回滚 / 批量软删除
- 筛选预设
- 导出 JSON / CSV
- Admin Key 安全状态管理

但这些动作目前主要依赖即时提示，缺少统一的本地审计留痕。对于管理端而言，至少应能回答：
- 谁在当前浏览器会话里做了什么操作
- 操作何时发生
- 操作成功还是失败
- 操作涉及哪些资产或参数
- 如何把这些记录导出留档

因此需要补一层“前端本地操作日志”能力，不改变后端接口，仅在浏览器本地为高价值管理动作做留痕与导出。

## 2. 目标

在 [`/admin/ui`](internal/httpserver/admin_ui_handler.go:1) 中增加本地操作日志能力：
- 记录高价值管理动作
- 在页面中展示最近日志
- 支持导出日志为 JSON / CSV
- 日志保存在浏览器 `localStorage`

## 3. 范围

### 3.1 In Scope
记录以下高价值动作：
- 设置 / 清除 Admin Key
- 创建资产
- 编辑资产
- 删除资产
- 批量标签
- 批量回滚
- 批量删除
- 导出资产 JSON / CSV

新增能力：
- 本地操作日志模型
- 最近日志列表
- 导出操作日志 JSON / CSV
- 日志保留上限与淘汰策略

### 3.2 Out of Scope
- 后端持久化审计表
- 多浏览器同步日志
- 服务端搜索审计
- 按用户身份做强审计归属

## 4. 架构设计

### 4.1 存储方式
采用浏览器 `localStorage` 保存本地操作日志。

建议新增 storage key：
- `admin_ui_operation_logs_v1`

理由：
- 与现有筛选预设方案一致，前端可独立完成
- 不依赖后端 schema 变更
- 可直接复用 JSON 序列化与导出逻辑

### 4.2 日志模型
每条日志至少包含：
- `id`
- `action`
- `created_at`
- `status`
- `summary`
- `payload`

建议字段说明：
- `id`: 前端生成的唯一 ID（时间戳 + 随机片段）
- `action`: 动作类型，如 `asset.create`、`asset.edit`、`asset.delete`、`batch.tags`、`admin_key.set`
- `created_at`: ISO 时间戳
- `status`: `success` / `error`
- `summary`: 给人读的摘要文案
- `payload`: 关键参数快照（可裁剪、避免泄露完整敏感信息）

## 5. 记录入口设计

在 [`internal/httpserver/adminui/app.js`](internal/httpserver/adminui/app.js:1) 中增加统一日志函数，例如：
- `readOperationLogs()`
- `writeOperationLogs()`
- `appendOperationLog(entry)`
- `renderOperationLogs()`
- `exportOperationLogs(format)`

所有高价值动作在成功/失败分支中调用统一日志入口，而不是分散拼接本地存储逻辑。

## 6. 展示设计

### 6.1 最近日志列表
在 [`internal/httpserver/adminui/index.html`](internal/httpserver/adminui/index.html:47) 的操作区附近新增最近日志列表区域。

建议展示：
- 时间
- 动作类型
- 状态
- 摘要

首版不做搜索与筛选，只展示最近若干条即可。

### 6.2 保留策略
首版建议：
- 最多保留最近 200 条日志
- 超出时自动淘汰最旧记录

理由：
- 控制 `localStorage` 体积
- 满足“最近会话操作留痕”需求

## 7. 导出设计

### 7.1 导出入口
在当前导出区附近增加：
- 导出操作日志 JSON
- 导出操作日志 CSV

### 7.2 导出内容
导出字段与日志模型一致：
- `id`
- `action`
- `created_at`
- `status`
- `summary`
- `payload`

其中：
- JSON 保留完整结构
- CSV 中 `payload` 采用 JSON 字符串序列化

### 7.3 文件命名
文件名携带时间戳，例如：
- `admin-operation-logs-2026-03-24T06-00-00.json`
- `admin-operation-logs-2026-03-24T06-00-00.csv`

## 8. 与现有能力集成点

需要在以下现有动作中接入日志：
- [`promptAdminKeyUpdate()`](internal/httpserver/adminui/app.js:120)
- [`clearAdminKeyStorage()`](internal/httpserver/adminui/app.js:113)
- [`saveCreateAsset()`](internal/httpserver/adminui/app.js:1082)
- [`saveEditAsset()`](internal/httpserver/adminui/app.js:1130)
- 单项删除按钮流转（资产列表动作）
- [`batchApplyTagsSelected()`](internal/httpserver/adminui/app.js:723)
- [`batchRollbackSelected()`](internal/httpserver/adminui/app.js:819)
- [`batchSoftDeleteSelected()`](internal/httpserver/adminui/app.js:906)
- [`exportCurrentAssets()`](internal/httpserver/adminui/app.js:132)

## 9. 安全与隐私边界

- 不记录完整 Admin Key，只记录动作类型与脱敏摘要
- `payload` 中避免保存完整敏感密钥
- 对资产类操作，只保留必要字段，如 `asset_id`、`tenant_id`、`version`、`tags` 摘要等

## 10. 验收标准

### 10.1 记录能力
- 指定的高价值动作会写入本地日志
- 成功与失败都会被记录

### 10.2 展示能力
- 页面中能看到最近日志列表
- 日志至少包含时间、动作、状态、摘要

### 10.3 导出能力
- 可导出操作日志 JSON
- 可导出操作日志 CSV
- 文件名带时间戳

### 10.4 存储策略
- 本地最多保留 200 条
- 超出后会自动淘汰最旧记录

## 11. 风险与缓解

### 风险 1：日志写入分散导致遗漏
缓解：统一使用 `appendOperationLog()`，所有动作通过该函数入库。

### 风险 2：日志 payload 过大
缓解：首版对 payload 做裁剪，只保留关键字段。

### 风险 3：误记录敏感信息
缓解：Admin Key 仅记录“设置/清除”事件，不记录明文，只显示脱敏摘要或空 payload。

## 12. 结论

推荐采用“前端 localStorage 日志 + 最近日志列表 + JSON/CSV 导出”的方案。它实现成本低、改动边界清晰，且能显著提升管理端的可追踪性与留档能力。