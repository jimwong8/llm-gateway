# 跨环境 Promotion 与配置发布流水线设计文档

日期：2026-03-24

## 1. 背景

当前系统已经完成：

- 控制面配置管理设计
- 配置版本化与回滚设计
- 变更审批 / 发布流程设计
- 运行时热更新设计
- 多实例配置同步与消息总线边界设计
- project scope 与 override 设计

在此基础上，下一步自然问题是：

> 已发布的配置如何从 `dev` 推进到 `staging` 再推进到 `prod`，同时保持版本、审计和热更新链路可追踪。

本阶段目标是定义最小可用版的跨环境 promotion 边界：

- 只支持 `dev -> staging -> prod` 的手动 promotion
- 不做自动流水线
- 不做自动验证 gate
- 不做自动失败回退

## 2. 方案对比

### 方案 A：最小可用版（推荐）

只定义：
- `dev -> staging -> prod` 单向手动 promotion
- Released 版本可被提升到下一个环境
- 目标环境生成新的 Released 版本

优点：
- 最容易和当前版本化、审批流模型衔接
- 风险最小
- 可审计性强

缺点：
- 仍然依赖人工操作
- 不具备自动 gate 能力

### 方案 B：增强版

在最小版基础上加入：
- promotion gate
- 验证钩子
- 失败回退点

优点：
- 更接近企业发布流水线

缺点：
- 当前阶段明显扩大范围

### 方案 C：自动流水线版

直接引入自动审批与自动发布链路。

优点：
- 长期自动化程度最高

缺点：
- 当前阶段复杂度过高
- 难以快速稳定落地

### 结论

采用方案 A：**先定义 dev -> staging -> prod 的手动 promotion 边界，不做自动流水线。**

## 3. promotion 状态机与边界

### 3.1 promotion 的前提

只有 **Released** 版本才能参与跨环境 promotion。

明确不允许：
- Draft 直接 promotion
- Pending 直接 promotion

### 3.2 环境流向

首版只支持以下单向路径：

- `dev -> staging`
- `staging -> prod`

不支持：
- `prod -> staging`
- `staging -> dev`
- 任意跨级跳转

### 3.3 promotion 的对象

promotion 传播的是：
- 已 Released 的配置版本
- 其配置内容快照
- 对应来源元数据

不是直接迁移运行时状态。

### 3.4 目标环境版本语义

当配置被 promotion 到目标环境后：

- 在目标环境生成一个**新的** Released 版本记录
- 不复用源环境版本号作为事实主键
- 但保留 `source_environment` 与 `source_version` 关系

这样可以保证：
- 各环境版本历史独立
- promotion 来源仍然可追踪

## 4. promotion 与版本 / 审计衔接

### 4.1 与版本模型的关系

promotion 本质上是一种特殊的发布动作。

因此目标环境的新 Released 版本记录至少应保留：

- `source_environment`
- `source_version`
- `promoted_from`

### 4.2 与审计链路的关系

promotion 必须进入统一审计链路。

审计至少记录：

- 操作者
- 来源接口
- 模块
- 源环境
- 目标环境
- 源版本
- 目标新版本
- 时间

### 4.3 与热更新、多实例同步的关系

热更新与多实例同步只感知：
- 目标环境中新生成的 Released 版本

它们不需要理解 promotion 细节，只需要理解：
- 当前环境已有一个新的可生效版本

### 4.4 非目标

首版不做：
- 自动执行验证钩子
- promotion 失败自动回退
- 多阶段自动 gate
- 基于健康检查自动阻断 promotion

## 5. 风险与控制

### 风险 1：环境间版本号混淆

控制方式：
- 目标环境必须生成自己的 Released 版本
- 不直接复用源环境主键

### 风险 2：promotion 误作用到错误环境

控制方式：
- 只允许固定的单向路径
- 请求必须显式声明源环境与目标环境

### 风险 3：审计链路不完整

控制方式：
- 把 promotion 明确视为“特殊发布动作”
- 强制写入统一审计事件

## 6. 成功标准

本阶段完成后应满足：

- 已明确 dev / staging / prod 的手动 promotion 边界
- 已明确只有 Released 版本才能 promotion
- 已明确目标环境必须生成新的 Released 版本记录
- 已明确 promotion 必须写审计
- 已明确热更新 / 多实例同步只面向目标环境新版本工作

这将为后续加入 promotion gate、验证钩子与失败回退提供稳定边界。