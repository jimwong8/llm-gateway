# 2026-03-24 Admin UI 批量操作可观测性设计

## 1. 背景

当前管理端已经具备多种批量操作能力：
- 批量标签
- 批量回滚
- 批量软删除

这些能力已经可以工作，但执行时的用户反馈仍然偏弱：
- 只能看到最终成功/失败摘要
- 无法观察执行进度
- 无法快速查看失败明细
- 无法只重试失败项

因此需要在不改变后端接口语义的前提下，为前端批量操作增加统一的可观测层。

## 2. 目标

为 [`internal/httpserver/adminui/app.js`](internal/httpserver/adminui/app.js:1) 中的三类批量任务提供统一状态面板，支持：
- 执行中进度展示
- 当前处理项展示
- 成功/失败数量统计
- 失败明细列表
- 失败项一键重试

## 3. 范围

### 3.1 In Scope
- 覆盖三类批量操作：批量标签、批量回滚、批量软删除
- 新增统一的批量任务状态面板
- 前端维护最近 1 个活动/最近完成任务状态
- 失败项保留 `id`、`action`、`reason`、`retry_payload`
- 支持对最近一次任务的失败项执行“重试失败项”

### 3.2 Out of Scope
- 多任务历史中心
- 服务端持久化任务日志
- 跨页面保留批量任务状态
- 后端接口扩展专用 batch API

## 4. 架构设计

### 4.1 统一状态面板
在 [`internal/httpserver/adminui/index.html`](internal/httpserver/adminui/index.html:1) 的操作区下方新增统一批量任务状态面板。

面板显示：
- 任务类型（delete / tags / rollback）
- 总数
- 已完成数
- 失败数
- 当前处理资产 ID
- 百分比进度条
- 失败明细列表
- “重试失败项”按钮

### 4.2 前端统一任务状态模型
在 [`internal/httpserver/adminui/app.js`](internal/httpserver/adminui/app.js:1) 中新增单一批量任务状态对象，例如：

```js
state.batchJob = {
  visible: false,
  action: "",
  total: 0,
  completed: 0,
  failed: 0,
  current_id: 0,
  running: false,
  failures: [],
  retryable: []
}
```

该状态被三类批量流程共用，而不是为每类操作维护一套独立实现。

### 4.3 执行模型
每个批量动作在循环执行前：
- 初始化状态面板
- 写入总数
- 清空旧失败列表

每处理 1 个资产：
- 更新当前 ID
- 累加 completed
- 若失败则写入 failures
- 实时刷新面板

执行完成后：
- 若存在失败项，保留“重试失败项”按钮
- 若全部成功，显示完成态但仍保留最近一次任务结果

## 5. 失败明细与重试机制

### 5.1 失败项结构
失败项最少包含：
- `id`
- `action`
- `reason`
- `retry_payload`

其中：
- 批量标签的 `retry_payload` 为对应 [`apiPut()`](internal/httpserver/adminui/app.js:97) 所需 payload
- 批量回滚的 `retry_payload` 为对应 [`apiPost()`](internal/httpserver/adminui/app.js:89) 到 rollback 接口的 payload
- 批量删除的 `retry_payload` 为对应 [`apiDelete()`](internal/httpserver/adminui/app.js:105) 所需 query 参数

### 5.2 明细展示
失败明细按执行顺序展示，内容至少包括：
- 资产 ID
- 操作类型
- 失败原因

### 5.3 重试策略
“重试失败项”只针对最近一次批量任务中的失败集合执行。

重试行为：
- 成功后从失败列表中移除
- 失败则覆盖原错误原因
- 进度条按“本次重试任务”重新计算

## 6. UI 设计

### 6.1 新增面板区域
文件：[`internal/httpserver/adminui/index.html`](internal/httpserver/adminui/index.html:1)

建议结构：
- 状态摘要卡片
- 进度条
- 当前处理项文本
- 失败明细列表
- 重试失败项按钮

### 6.2 样式
文件：[`internal/httpserver/adminui/styles.css`](internal/httpserver/adminui/styles.css:1)

新增样式：
- 面板容器
- 进度条轨道与进度值
- 失败列表滚动区
- 错误行高亮
- 重试按钮

## 7. 与现有代码的集成点

### 7.1 需要接入的现有函数
- [`batchApplyTagsSelected()`](internal/httpserver/adminui/app.js:576)
- [`batchRollbackSelected()`](internal/httpserver/adminui/app.js:660)
- [`batchSoftDeleteSelected()`](internal/httpserver/adminui/app.js:719)

### 7.2 可复用能力
- [`normalizeError()`](internal/httpserver/adminui/app.js:38)
- [`apiPost()`](internal/httpserver/adminui/app.js:89)
- [`apiPut()`](internal/httpserver/adminui/app.js:97)
- [`apiDelete()`](internal/httpserver/adminui/app.js:105)
- 现有消息提示区 [`#message`](internal/httpserver/adminui/index.html:46)

### 7.3 新增建议函数
- `createBatchJob(action, total)`
- `updateBatchJobProgress(partial)`
- `appendBatchJobFailure(failure)`
- `finishBatchJob()`
- `retryFailedBatchJobItems()`
- `renderBatchJobPanel()`

## 8. 验收标准

### 8.1 功能验收
- 批量标签执行时可看到实时进度
- 批量回滚执行时可看到实时进度
- 批量软删除执行时可看到实时进度
- 失败项会进入失败明细列表
- 点击“重试失败项”后仅重试失败集合

### 8.2 边界验收
- 当全部成功时，失败列表为空且重试按钮禁用或隐藏
- 当部分失败时，失败数与明细一致
- 重试成功后，失败列表会减少
- 重试再次失败时，错误原因会更新

## 9. 风险与缓解

### 风险 1：三类批量任务逻辑重复膨胀
缓解：抽出统一的 batch job 状态与渲染函数，只把具体 API 调用留在各自执行器中。

### 风险 2：失败重试 payload 不一致
缓解：在首次失败时即记录标准化的 `retry_payload`，避免重试阶段二次推导。

### 风险 3：面板渲染频率过高导致 UI 抖动
缓解：每次循环结束后再刷新一次 UI，不做更细粒度的频繁重绘。

## 10. 结论

推荐采用“统一批量任务状态面板”方案。该方案能在较低复杂度下，同时提升批量标签、批量回滚、批量软删除三类能力的可观测性与可恢复性，并且与当前前端架构保持一致，适合作为下一轮实现目标。
