# 控制面配置管理增强设计文档

日期：2026-03-24

## 1. 背景

当前后端已经完成多条关键主链路：

- 路由策略引擎
- L1/L2/L3/L4 缓存与复用链路
- Billing & Observability
- Quota Observability
- Policy Engine 增强 / 深化 / 闭环

但控制面仍然是分散式的：

- 策略配置散落在 [`internal/policy`](internal/policy)
- 配额能力散落在 [`internal/quota`](internal/quota)
- 观测接口散落在 [`internal/httpserver/server.go`](internal/httpserver/server.go)
- Admin 管理入口虽然存在，但缺少统一的“控制面配置管理”视角

本阶段目标不是重建一个完整控制面系统，而是先做**最小可用版**：

- 统一管理策略、配额、观测配置的读取与写入接口
- 保持 tenant 级范围
- 提供一致的配置视图与更新入口
- 不引入复杂版本控制或回滚机制

## 2. 方案对比

### 方案 A：最小可用版（推荐）

在现有 `/admin/*` 基础上新增统一控制面配置入口，由控制层协调现有 [`internal/policy`](internal/policy)、[`internal/quota`](internal/quota)、[`internal/billing`](internal/billing) 和观测配置。

优点：
- 改动最小
- 与当前架构最兼容
- 能最快形成“统一入口”体验

缺点：
- 仍然依赖底层模块各自存储
- 不是完整控制面平台

### 方案 B：增强版

在方案 A 基础上加入配置版本化、审计关联与回滚。

优点：
- 更接近企业控制面

缺点：
- 实现复杂度显著增加
- 当前阶段容易超 scope

### 方案 C：重构版

先抽离统一配置中心，再让策略、配额、观测全部迁移到统一配置模型。

优点：
- 长期结构最清晰

缺点：
- 当前成本最高
- 会打断现有已稳定模块

### 结论

采用方案 A：**最小可用版**。

## 3. 统一配置模型与边界

### 3.1 控制面统一范围

首版统一三类配置：

- Policy 配置
- Quota 配置
- Observability 配置

### 3.2 作用域

首版只支持 **tenant 级** 配置，不引入：

- project 级覆盖
- environment 级覆盖
- 多层优先级合并

### 3.3 配置模型原则

控制面接口负责对外提供统一视图，但底层仍分别落在现有模块中：

- [`internal/policy`](internal/policy)
- [`internal/quota`](internal/quota)
- [`internal/billing`](internal/billing)
- [`internal/httpserver/server.go`](internal/httpserver/server.go) 中的观测入口

这意味着首版的目标是**统一入口**，而不是统一底层物理存储。

## 4. 接口组织与最小交互模型

### 4.1 接口组织

沿用现有 `/admin/*` 风格，在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 中新增：

- `/admin/control-plane/config`
- `/admin/control-plane/policy`
- `/admin/control-plane/quota`
- `/admin/control-plane/observability`

### 4.2 支持的操作

首版支持：

- `GET`：读取统一配置视图
- `POST` / `PUT`：更新配置

首版不做：

- `PATCH`
- 复杂 diff 更新
- 批量事务写入

### 4.3 统一返回结构

统一包含：

- `scope`
- `tenant_id`
- `module`
- `config`
- `updated_at`

目标是让前端或控制台以一致方式消费，而不必知道底层来自哪个模块。

## 5. 配置写入与测试策略

### 5.1 配置写入方式

首版不引入新的统一配置表。

控制面接口只负责：

- 聚合读取现有模块配置
- 调用现有模块的写入能力

即：
- Policy 继续写 [`internal/policy`](internal/policy)
- Quota 继续写 [`internal/quota`](internal/quota)
- Observability 配置继续落在现有配置模型与 handler 上

### 5.2 一致性边界

首版不保证跨模块强事务一致性。

原因：
- 当前目标是统一入口，而不是分布式事务
- 先保证“能统一读写”，再考虑“强一致编排”

### 5.3 测试策略

#### Handler 测试

在 [`internal/httpserver/server.go`](internal/httpserver/server.go) 对应测试中验证：

- 控制面配置接口已注册
- GET 返回结构一致
- POST / PUT 能正确调用底层模块

#### 底层回归测试

继续复用并扩展：

- [`internal/policy`](internal/policy)
- [`internal/quota`](internal/quota)
- [`internal/billing`](internal/billing)

#### 远程联调

只验证：

- 控制面统一入口可读
- 更新后下游配置状态正确变化
- 不强求版本回滚与审计联动

## 6. 风险与控制

### 风险 1：统一入口掩盖底层差异

控制方式：
- 返回体保留 `module`
- 统一视图中明确说明哪些字段来自哪个底层模块

### 风险 2：写入顺序导致局部不一致

控制方式：
- 首版明确不做跨模块事务
- 写接口文档清楚说明最小一致性边界

### 风险 3：后续扩展困难

控制方式：
- 先统一接口语义与返回结构
- 为后续版本化与回滚预留 `updated_at` / `scope` 等字段

## 7. 成功标准

完成后应满足：

- 管理端可从统一入口读取策略、配额、观测配置
- 管理端可通过统一入口更新对应配置
- 返回结构一致，不再需要上层分别感知多个子模块
- 本地测试与远程联调可验证统一入口有效
