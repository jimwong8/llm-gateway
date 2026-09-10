# Project 级隔离与配置 Override 设计文档

日期：2026-03-24

## 1. 背景

当前系统已经完成：

- tenant 级控制面配置管理设计与实施计划
- 配置版本化 / 回滚设计与实施计划
- 审批 / 发布流程设计与实施计划
- 运行时热更新、多实例同步、实例状态聚合设计与实施计划
- tenant tier / environment / scope 隔离边界设计与实施计划

在上述基础上，下一步最自然的扩展就是：

> 如何在保持 tenant 级默认配置的前提下，引入 project 级更细粒度配置，并定义清晰的 override 优先级。

本阶段目标是定义最小可用版：

- project 级 scope
- tenant 默认配置与 project override 的优先级
- 与版本化 / 发布 / 热更新 / 多实例同步的衔接

明确不做：
- 跨 project 继承
- project 间模板共享

## 2. 方案对比

### 方案 A：最小可用版（推荐）

只定义：
- tenant default
- project override
- 两层优先级

优点：
- 模型清晰
- 最容易与现有 tenant 级模型衔接
- 风险可控

缺点：
- 不能表达更复杂的 project 继承关系

### 方案 B：增强版

在最小可用版基础上加入：
- project 模板
- 批量 override

优点：
- 对大型租户更友好

缺点：
- 当前阶段明显扩大范围

### 方案 C：复杂版

直接引入：
- 跨 project 继承
- project 间模板共享
- 多层 override 解析链

优点：
- 表达力最强

缺点：
- 当前阶段过重
- 安全与治理复杂度高

### 结论

采用方案 A：**先定义 project 级 scope、tenant 默认配置与 project override 的优先级，不进入跨 project 继承。**

## 3. project 级 scope 与优先级规则

### 3.1 scope 扩展

在现有 `scope` 模型上，首版增加：

- `tenant`
- `project`

其中：
- `tenant` 表示租户默认配置
- `project` 表示租户下具体 project 的局部 override

### 3.2 最小优先级模型

解析顺序固定为：

1. `project override`
2. `tenant default`

即：
- project 层有值时优先使用 project 配置
- project 层缺失时显式回退到 tenant 默认

### 3.3 回退语义

当 project 层没有配置时：
- 使用 tenant 默认
- 不复制 tenant 配置生成影子版本

这样可以避免：
- 冗余版本爆炸
- project 层与 tenant 层配置难以对账

### 3.4 project 级版本主体

同一 `(tenant_id, environment, module)` 下，若配置属于 project 级：
- 必须显式带 `project_id`
- 才能成为独立版本主体

这样才能保证：
- 不同 project 的配置历史互不覆盖
- diff / rollback / 发布动作都有明确边界

## 4. 版本化 / 发布 / 热更新衔接

### 4.1 版本化

tenant default 与 project override 都进入统一版本模型，但：

- `scope=tenant` 的版本不带 `project_id`
- `scope=project` 的版本必须带 `project_id`

### 4.2 发布与回滚

发布与回滚动作遵循同样的两层优先级：

- project 的 Released 只影响对应 project
- 不影响 tenant default

也就是说：
- tenant 默认发布不会自动覆盖已有 project override
- project 回滚也不会回滚 tenant 默认

### 4.3 热更新与多实例同步

配置事件在热更新与多实例同步中必须显式携带：

- `scope=tenant|project`
- 可选 `project_id`

实例消费事件时，应先判断：
- 这是 tenant 默认变更
- 还是某个 project 的局部 override 变更

然后再决定是否覆盖本地运行态。

## 5. 非目标与控制边界

首版明确不支持：

- 跨 project 继承
- project 间模板共享
- project 对 project 的 fallback 链
- 自动合并多层 project 配置

这样可保证首版的配置解析规则始终简单可解释。

## 6. 风险与控制

### 风险 1：tenant default 与 project override 逻辑混淆

控制方式：
- 固定两层优先级
- project 缺失时显式回退，不生成影子配置

### 风险 2：版本历史在 project 维度下膨胀

控制方式：
- 只有真的 project override 才产生 project 级版本
- 不复制 tenant 默认为每个 project 生成初始版本

### 风险 3：发布与回滚误作用到错误范围

控制方式：
- 所有动作都必须带 `scope`
- `scope=project` 时必须带 `project_id`
- `scope=tenant` 明确不能影响 project 历史

## 7. 成功标准

本阶段完成后应满足：

- 已定义 project 级 scope
- 已固定 tenant default 与 project override 的两层优先级
- 已明确 project 级版本主体必须带 `project_id`
- 已明确与版本化 / 发布 / 热更新 / 多实例同步的衔接方式
- 当前模型可支撑后续 project 模板与更复杂 override，但无需推翻现有设计
