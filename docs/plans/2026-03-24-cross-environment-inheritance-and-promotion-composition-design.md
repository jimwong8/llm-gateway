# 跨环境继承与 Promotion 组合设计文档

日期：2026-03-24

## 1. 背景

当前系统已经完成：

- tenant / environment / scope 隔离边界设计
- project scope 与 override 设计
- 配置版本化与回滚设计
- 变更审批 / 发布流程设计
- 跨环境 promotion 设计
- promotion gate / 验证钩子 / 失败回退点设计

在这些能力之上，进一步的问题是：

> 当系统未来既支持 project 级 override，又要支持跨环境 promotion 时，跨环境继承应如何与 promotion 组合，才能既保留默认值能力，又不让环境边界失控。

本阶段目标是先定义**最小可用版**边界：

- 可以讨论跨环境继承
- 但不允许自动跨环境继承直接生效
- promotion 仍然是唯一让配置跨环境进入生效路径的动作

## 2. 方案对比

### 方案 A：最小可用版（推荐）

先定义跨环境继承与 promotion 的组合边界，但不允许自动跨环境继承直接生效。

优点：
- 保持环境边界清晰
- 不破坏现有 promotion 语义
- 便于后续逐步增强

缺点：
- 还不能实现真正的跨环境默认链

### 方案 B：增强版

在方案 A 基础上定义可控的跨环境默认继承链与 promotion 覆盖规则。

优点：
- 更接近完整平台能力

缺点：
- 当前阶段范围偏大
- 容易把“边界梳理”变成“继承引擎设计”

### 方案 C：自动继承版

直接允许跨环境自动继承并与 promotion 混合求值。

优点：
- 自动化最强

缺点：
- 风险最高
- 当前阶段完全不适合

### 结论

采用方案 A：**先定义跨环境继承与 promotion 的组合边界，但不允许自动跨环境继承生效。**

## 3. 组合边界与优先级

### 3.1 正交关系

`project override` 与 `cross-environment promotion` 属于两条正交能力：

- `project override`：解决同一环境内部的局部优先级问题
- `promotion`：解决配置如何从一个环境传播到另一个环境的问题

它们不应互相隐式触发。

### 3.2 promotion 的作用范围

跨环境 promotion 只作用于：
- 当前环境中已经 Released 的版本

它不会自动把上游环境的 tenant 默认值或 project override 直接注入到下游环境。

### 3.3 不允许自动跨环境继承生效

即使未来引入“跨环境默认继承链”，首版也必须遵守：

- 继承只能用于辅助生成草稿或初始化候选配置
- 不能绕过 promotion 直接改写目标环境当前生效版本

### 3.4 解析优先级

最终配置解析仍然只在**单个环境内部**求值：

- `project override > tenant override > tenant template > tier-mapped default bundle`

不做跨环境合并求值。

## 4. 与版本 / 热更新 / 审计的衔接

### 4.1 与版本化的关系

如果未来存在跨环境默认继承链，它只能作为：
- 创建新环境 Draft 的初始化来源

不能直接成为 Released 版本。

### 4.2 与 promotion 的关系

promotion 仍然是唯一允许让配置跨环境进入 Released 的动作。

换言之：
- 继承只能辅助生成候选配置
- promotion 才能使其成为目标环境真实生效版本

### 4.3 与审计的关系

审计必须明确区分两类动作：

1. **继承生成草稿**
2. **正式 promotion 发布**

避免后续排查时把“参考来源”误认为“已生效动作”。

### 4.4 与热更新 / 多实例同步的关系

热更新与多实例同步只响应：
- 目标环境中新生成的 Released 版本

它们不响应：
- 任何继承型草稿
- 任何尚未进入 Released 的候选版本

### 4.5 当前最小实现落地状态

当前代码已经落地的最小能力包括：

- [`internal/controlplane/service.go`](internal/controlplane/service.go)
  已支持：
  - inheritance draft 生成
  - draft 显式 release
  - source environment Released 显式 promotion
  - released / version 查询
- [`internal/httpserver/admin_handler.go`](internal/httpserver/admin_handler.go)
  已暴露最小管理接口：
  - `POST /admin/inheritance-drafts`
  - `POST /admin/releases`
  - `POST /admin/promotions`
  - `GET /admin/config-versions/{versionID}`
- [`internal/audit/recorder.go`](internal/audit/recorder.go)
  已区分控制面 `inheritance-draft` 与 `release` 事件模型。
- [`internal/runtime/publisher.go`](internal/runtime/publisher.go)
  已锁定只有 Released 才会触发发布，Draft 不触发运行时事件。

### 4.6 当前发布闭环语义

当前最小闭环已经满足：

1. 继承只生成 Draft，不自动成为当前 Released。
2. Draft 必须通过显式 release 才能成为目标环境当前 Released。
3. source environment 的 Released 必须通过显式 promotion 才能跨环境进入目标环境当前 Released。
4. `release` 与 `promotion` 成功后都会触发：
   - 控制面审计事件写入
   - 运行时 Released 发布事件投递
5. Draft 创建本身不会触发运行时发布。

### 4.7 当前 verify 方式

当前已有两层验证：

- 单元测试：
  - [`internal/controlplane/service_test.go`](internal/controlplane/service_test.go)
  - [`internal/httpserver/admin_handler_test.go`](internal/httpserver/admin_handler_test.go)
  - [`internal/runtime/publisher_test.go`](internal/runtime/publisher_test.go)
  - [`internal/audit/recorder_test.go`](internal/audit/recorder_test.go)
- 端到端 verify：
  - [`cmd/verify/inheritance_promotion/main.go`](cmd/verify/inheritance_promotion/main.go)

其中 [`cmd/verify/inheritance_promotion/main.go`](cmd/verify/inheritance_promotion/main.go) 已通过 [`internal/httpserver/`](internal/httpserver) API 串起：

- draft 创建
- draft 查询
- release
- promotion
- audit 摘要输出
- runtime 摘要输出

因此当前系统已经具备“控制面能力 → 管理 API → 审计 / 运行时副作用 → verify 演示”的最小闭环。

## 5. 风险与控制

### 风险 1：继承与 promotion 语义混淆

控制方式：
- 明确继承只生成候选 Draft
- 明确 promotion 才能进入 Released

### 风险 2：跨环境默认链破坏环境隔离

控制方式：
- 首版不允许自动跨环境继承生效
- 任何跨环境变化都必须显式 promotion

### 风险 3：审计记录无法区分“参考来源”与“生效动作”

控制方式：
- 审计事件类型必须区分 inheritance-draft 与 promotion-release

## 6. 成功标准

本阶段完成后应满足：

- 已明确 project override 与跨环境 promotion 的正交边界
- 已明确不允许自动跨环境继承直接生效
- 已明确 promotion 是唯一跨环境进入 Released 的动作
- 已明确与版本化、审计、热更新、多实例同步的衔接规则
- 后续如果进入增强版跨环境默认链，也不需要推翻当前边界模型
