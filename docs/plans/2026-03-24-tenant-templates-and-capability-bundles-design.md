# 多租户默认模板体系与能力包设计文档

日期：2026-03-24

## 1. 背景

当前系统已经完成：

- tenant / environment / scope 隔离边界设计
- project scope 与 override 设计
- 控制面配置管理、版本化、审批流、热更新、多实例同步等后续控制面能力规划

在这些能力之上，下一步需要解决的问题是：

> 新租户如何拥有一套合理的默认能力配置，而又不牺牲 tenant 自定义与 project override 的灵活性。

本阶段目标是定义增强版的“多租户默认模板体系与能力包边界”：

- tenant 默认模板
- capability bundle
- 单层模板继承
- tenant tier 映射

## 2. 方案对比

### 方案 A：最小可用版

只定义 tenant 默认模板与 capability bundle。

优点：
- 结构最简单
- 能较快提供默认值能力

缺点：
- 无法表达 tier 与模板的组织关系
- 不利于后续产品化分层

### 方案 B：增强版（推荐）

在方案 A 基础上加入：

- 模板继承
- tenant tier 与模板 / bundle 的映射

优点：
- 适合 SaaS 场景下的租户分层治理
- 可降低新租户初始化成本
- 保持 tenant override 与 project override 的灵活性

缺点：
- 需要额外定义模板和 bundle 的职责边界

### 方案 C：复杂版

直接引入：
- 跨 tenant 模板共享
- 模板市场
- 多级继承链

优点：
- 表达力最强

缺点：
- 当前阶段过重
- 会让控制面模型迅速膨胀

### 结论

采用方案 B：**定义 tenant 默认模板、capability bundle、模板继承与 tenant tier 关联映射。**

## 3. 模板模型与 tier 映射

### 3.1 tenant 默认模板

tenant 默认模板是一类独立配置对象，用于承载该租户的默认控制面配置，例如：

- 默认路由策略
- 默认配额阈值
- 默认观测开关
- 默认治理策略开关

它的职责是：
- 作为 tenant 的基础配置底座
- 为 project override 提供回退来源

### 3.2 capability bundle

capability bundle 用来表达一组能力组合，而不是直接保存完整运行参数。

例如可以包含：

- 缓存能力
- 观测能力
- 治理能力
- promotion 能力

capability bundle 更像“能力包”，而 tenant 默认模板更像“配置模板”。

### 3.3 tenant tier 映射

`tenant_tier` 通过映射关系选择：

- 默认模板
- 默认 capability bundle

这样可以让：
- `free`
- `pro`
- `enterprise`

拥有不同起始能力组合，但 tenant 仍然可以在模板基础上做自己的覆盖。

### 3.4 模板继承

首版只允许：

- **单层继承**

即一个模板只能继承一个上层模板，不做多级模板链。

这样能在表达力和复杂度之间取得平衡。

## 4. 优先级与非目标

### 4.1 最终解析顺序

最终配置解析顺序固定为：

1. `project override`
2. `tenant override`
3. `tenant template`
4. `tier-mapped default bundle`

这一定义了控制面配置解析的清晰层次。

### 4.2 capability bundle 的定位

capability bundle 只提供默认能力组合。

它不直接替代：
- 配置版本
- tenant 自定义配置
- project override

也就是说，它是默认值来源，不是最终事实源。

### 4.3 非目标

首版不支持：

- 跨 tenant 模板共享
- 模板市场
- 多级模板继承
- project 级模板链

### 4.4 目标边界

模板与 bundle 的目标是：
- 降低新租户初始化成本
- 提供标准化起点

而不是取代 tenant 自定义能力。

## 5. 与现有模型的衔接

### 5.1 与 tenant / environment / scope 的关系

tenant 模板仍然必须受：
- `tenant_id`
- `environment`
- `scope`

约束。

### 5.2 与 project override 的关系

project override 仍然保持最高优先级，不会被模板覆盖。

### 5.3 与版本化 / 发布 / 热更新的关系

模板与 bundle 一旦参与实际配置生效，也必须进入：
- 版本化
- 发布
- 热更新
- 多实例同步

链路。

也就是说，它们不能成为“配置旁路”。

## 6. 风险与控制

### 风险 1：模板与 tenant override 语义混淆

控制方式：
- 明确模板是默认底座
- override 永远优先于模板

### 风险 2：bundle 与模板职责重叠

控制方式：
- bundle 只表达能力组合
- 模板表达具体配置结构

### 风险 3：继承过早复杂化

控制方式：
- 首版只允许单层继承
- 明确不支持多级模板链

## 7. 成功标准

本阶段完成后应满足：

- 已定义 tenant 默认模板模型
- 已定义 capability bundle 模型
- 已定义 tenant tier 与模板 / bundle 的映射关系
- 已固定最终配置解析顺序
- 已明确模板与 bundle 的职责边界与非目标

这将为后续真正落地多租户默认配置体系提供稳定边界。