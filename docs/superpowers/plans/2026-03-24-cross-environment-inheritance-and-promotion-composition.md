# 跨环境继承与 Promotion 组合 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为控制面补齐跨环境继承与 promotion 组合的最小可用实现边界，使继承只用于生成草稿候选配置，promotion 仍是唯一跨环境进入 Released 的动作。

**Architecture:** 方案以单环境求值为核心，不新增跨环境运行时解析链。实现重点放在控制面建模、草稿生成入口、promotion 约束校验、审计事件分型，以及 admin/read API 的可解释性输出，确保继承来源与正式发布动作严格分离。

**Tech Stack:** Go, net/http, internal control plane store/service, admin HTTP handlers, 审计事件模型, 现有 verify 命令与 Go tests

---

## 实施进展回填（2026-03-24）

### 已完成能力

- [x] 最小 [`internal/controlplane/`](internal/controlplane) 骨架已创建并通过测试
- [x] [`CreateInheritanceDraft()`](internal/controlplane/service.go:91) 已实现，确保继承只生成 Draft，不自动生效
- [x] [`GetVersion()`](internal/controlplane/service.go:176) 与 [`CurrentReleased()`](internal/controlplane/service.go:168) 已实现，用于版本查询与当前 Released 查询
- [x] [`ReleaseDraft()`](internal/controlplane/service.go:117) 已实现，支持显式把 Draft 提升为 Released
- [x] [`PromoteReleased()`](internal/controlplane/service.go:143) 已实现，支持将 source environment 当前 Released 复制为 target environment 新 Released
- [x] [`internal/httpserver/admin_handler.go`](internal/httpserver/admin_handler.go) 已暴露最小管理接口：
  - `POST /admin/inheritance-drafts`
  - `POST /admin/releases`
  - `POST /admin/promotions`
  - `GET /admin/config-versions/{versionID}`
  - `GET /admin/audit-events`
  - `GET /admin/runtime-events`
- [x] audit/runtime 查询接口已支持最小过滤参数：`tenant_id`、`environment`
- [x] audit/runtime 查询接口已支持最小 `limit` 参数
- [x] audit/runtime 查询结果已固定为最新优先倒序返回
- [x] [`internal/httpserver/admin_handler.go`](internal/httpserver/admin_handler.go) 已补最小必填字段校验，缺失统一返回 `400`
- [x] [`internal/audit/recorder.go`](internal/audit/recorder.go) 已提供最小控制面审计事件模型与内存记录器
- [x] [`internal/runtime/publisher.go`](internal/runtime/publisher.go) 已提供最小运行时发布器，并保证只有 Released 才会发布
- [x] [`ReleaseDraft()`](internal/controlplane/service.go:117) / [`PromoteReleased()`](internal/controlplane/service.go:143) 已接入 audit/runtime side effects
- [x] [`cmd/verify/inheritance_promotion/main.go`](cmd/verify/inheritance_promotion/main.go) 已升级为 API 化回归脚本，覆盖成功路径、查询路径、失败路径
- [x] [`docs/plans/2026-03-24-cross-environment-inheritance-and-promotion-composition-design.md`](docs/plans/2026-03-24-cross-environment-inheritance-and-promotion-composition-design.md) 已同步当前实现边界

### 已完成验证

- [x] [`go test ./internal/controlplane/...`](go.mod:1)
- [x] [`go test ./internal/httpserver/...`](go.mod:1)
- [x] [`go test ./internal/audit/...`](go.mod:1)
- [x] [`go test ./internal/runtime/...`](go.mod:1)
- [x] [`go run ./cmd/verify/inheritance_promotion`](cmd/verify/inheritance_promotion/main.go:1)
- [x] [`go test ./internal/controlplane/... ./internal/httpserver/... ./internal/audit/... ./internal/runtime/... && go run ./cmd/verify/inheritance_promotion`](cmd/verify/inheritance_promotion/main.go:1)

### 已实现的最小闭环语义

- [x] inheritance 只生成 Draft，不自动跨环境生效
- [x] Draft 只能通过显式 release 进入目标环境当前 Released
- [x] source environment 的 Released 只能通过显式 promotion 进入目标环境当前 Released
- [x] release / promotion 会触发审计记录与运行时发布
- [x] Draft 不会触发运行时发布
- [x] 管理 API、查询 API、回归脚本、设计文档已保持一致

## 下一阶段切片（细化版）

### 当前新增能力补充说明（2026-03-25）

- [x] 管理面查询已从“列表只读”扩展为“列表 + 摘要”双视图：
  - [`GET /admin/audit-events`](internal/httpserver/admin_handler.go:295)
  - [`GET /admin/runtime-events`](internal/httpserver/admin_handler.go:346)
- [x] 两类 summary 统一返回结构：
  - `total`
  - `by_type`
  - `by_environment`
- [x] runtime summary 的 `by_type` 目前按 [`released`](internal/controlplane/service.go:106) 聚合，即使用 [`controlplane.ConfigStatusReleased`](internal/httpserver/admin_handler.go:387)
- [x] 管理面固定 token 鉴权骨架已稳定落地：
  - [`WithAdminToken()`](internal/httpserver/admin_handler.go:107)
  - [`ServeHTTP()`](internal/httpserver/admin_handler.go:121)
  - [`isAuthorizedAdminRequest()`](internal/httpserver/admin_handler.go:129)
- [x] 当前默认安全姿态为：未配置 token 时，全部 [`/admin/*`](internal/httpserver/admin_handler.go:122) 请求返回 `401`
- [x] summary 在当前三层闭环中已完成对齐：
  - handler 实现
  - [`internal/httpserver/admin_handler_test.go`](internal/httpserver/admin_handler_test.go) 单测
  - [`cmd/verify/inheritance_promotion/main.go`](cmd/verify/inheritance_promotion/main.go) 回归

### 切片 A：查询接口继续增强（推荐）

**目标：** 让管理面查询接口从“最小可用”进入“更适合运营查看”。

**Files:**
- Modify: [`internal/httpserver/admin_handler.go`](internal/httpserver/admin_handler.go)
- Test: [`internal/httpserver/admin_handler_test.go`](internal/httpserver/admin_handler_test.go)
- Verify: [`cmd/verify/inheritance_promotion/main.go`](cmd/verify/inheritance_promotion/main.go)

- [ ] **Step 1: 为 audit/runtime 查询接口增加 `limit + filter` 组合断言**
- [ ] **Step 2: 为 `limit` 非法值（如 `0`、负数、非数字）补边界测试**
- [ ] **Step 3: 明确超大 `limit` 的截断上限行为，并补测试**
- [ ] **Step 4: 在回归脚本 [`cmd/verify/inheritance_promotion/main.go`](cmd/verify/inheritance_promotion/main.go) 中加入 `limit + filter` 组合校验**
- [ ] **Step 5: 运行 [`go test ./internal/httpserver/...`](go.mod:1) 与 [`go run ./cmd/verify/inheritance_promotion`](cmd/verify/inheritance_promotion/main.go:1)**

### 切片 B：输入校验继续增强

**目标：** 让管理接口从“最小必填字段校验”进入“更稳健的 HTTP 输入约束”。

**Files:**
- Modify: [`internal/httpserver/admin_handler.go`](internal/httpserver/admin_handler.go)
- Test: [`internal/httpserver/admin_handler_test.go`](internal/httpserver/admin_handler_test.go)

- [ ] **Step 1: 为 inheritance draft 请求增加 source / target 不可相同的最小校验**
- [ ] **Step 2: 为 release 请求增加空白字符串（仅空格）场景测试**
- [ ] **Step 3: 为 promotion 请求增加 source / target 不可相同的最小校验**
- [ ] **Step 4: 统一 `400` 错误文案为更明确的字段缺失 / 非法组合描述**
- [ ] **Step 5: 运行 [`go test ./internal/httpserver/...`](go.mod:1)**

### 切片 C：回归脚本继续增强

**目标：** 让 [`cmd/verify/inheritance_promotion/main.go`](cmd/verify/inheritance_promotion/main.go) 更接近稳定回归入口。

**Files:**
- Modify: [`cmd/verify/inheritance_promotion/main.go`](cmd/verify/inheritance_promotion/main.go)

- [ ] **Step 1: 加入 audit/runtime 倒序校验**
- [ ] **Step 2: 加入 `tenant_id` / `environment` 过滤结果校验**
- [ ] **Step 3: 加入 `limit=1` 查询结果校验**
- [ ] **Step 4: 加入缺失必填字段返回 `400` 的 HTTP 回归校验**
- [ ] **Step 5: 运行 [`go run ./cmd/verify/inheritance_promotion`](cmd/verify/inheritance_promotion/main.go:1)**

### 切片 D：文档同步补充

**目标：** 把新增的查询语义与管理面安全骨架完全回填到文档，降低后续理解成本。

**Files:**
- Modify: [`docs/plans/2026-03-24-cross-environment-inheritance-and-promotion-composition-design.md`](docs/plans/2026-03-24-cross-environment-inheritance-and-promotion-composition-design.md)
- Modify: [`docs/superpowers/plans/2026-03-24-cross-environment-inheritance-and-promotion-composition.md`](docs/superpowers/plans/2026-03-24-cross-environment-inheritance-and-promotion-composition.md)

- [x] **Step 1: 增加 audit/runtime 查询接口示例**
- [x] **Step 2: 明确 `tenant_id` / `environment` / `limit` / 倒序语义**
- [x] **Step 3: 明确当前 HTTP 输入校验边界**
- [x] **Step 4: 回填 summary 视图与固定 token 鉴权骨架的当前落地状态**

## 暂不进入本阶段的事项

- [ ] 不做自动跨环境继承直接生效
- [ ] 不做跨环境默认链求值引擎
- [ ] 不做持久化版 audit/runtime 查询存储
- [ ] 不做复杂分页、游标或多维排序
- [ ] 不做完整鉴权、多租户访问控制与审批流整合

## 当前文件结构与职责（更新后）

- [`internal/controlplane/`](internal/controlplane)  
  已落地最小控制面服务、release / promotion 路径与查询能力。
- [`internal/httpserver/`](internal/httpserver)
  已落地管理接口、结果查询接口、过滤/limit/倒序能力、summary 视图、固定 token 鉴权骨架与错误路径测试。
- [`internal/audit/`](internal/audit)  
  已落地最小控制面审计事件记录器。
- [`internal/runtime/`](internal/runtime)  
  已落地最小发布器与 Released-only 保护。
- [`cmd/verify/inheritance_promotion/main.go`](cmd/verify/inheritance_promotion/main.go)
  已落地 API 化回归脚本，并覆盖 summary 成功/401/405/缺失 Header 等最小失败路径。
- [`docs/plans/2026-03-24-cross-environment-inheritance-and-promotion-composition-design.md`](docs/plans/2026-03-24-cross-environment-inheritance-and-promotion-composition-design.md)  
  已同步当前实现边界。

## 完成定义（当前状态）

当前已满足以下验收结果：

- [x] 可以通过 admin/control plane 入口从 source environment Released 生成 target environment Draft
- [x] Draft 带有 inheritance source metadata
- [x] Draft 不会改变 target 当前 Released，也不会触发 runtime 热更新
- [x] 只有显式 promotion / release 后，target 才进入新的 Released
- [x] 单环境解析优先级保持不变，不存在跨环境自动求值
- [x] verify 与相关 Go tests 全部通过
- [x] 管理面 summary 与固定 token 鉴权骨架已在实现、单测、verify 三层闭环中对齐

Plan complete and updated at [`docs/superpowers/plans/2026-03-24-cross-environment-inheritance-and-promotion-composition.md`](docs/superpowers/plans/2026-03-24-cross-environment-inheritance-and-promotion-composition.md).