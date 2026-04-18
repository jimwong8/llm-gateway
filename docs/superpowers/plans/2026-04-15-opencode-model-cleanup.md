# OpenCode 可用大模型清理 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 清理仓库中 OpenCode 可用大模型相关的失效配置项，仅保留实际可用且代码路径完整的模型，并确保默认值、路由、provider 注册和接口展示保持一致。

**Architecture:** 采用“先定位真实来源，再验证失效，再最小化删除”的收敛策略。实施从配置、provider 注册、router 候选和接口展示四层建立映射，只删除经过验证明确失效的模型项，并同步修复默认模型/fallback/展示与测试，避免出现悬空引用或行为分叉。

**Tech Stack:** Go（config/provider/router/httpserver）、`go test`、LSP diagnostics、仓库现有设计文档。

---

## File Structure（实施前锁定）

### 核心运行时
- Modify: `internal/config/config.go`
  - 校准默认 provider / default model / 相关环境变量默认值
- Modify: `internal/providers/provider.go`
  - 仅在模型/provider 契约确需同步时调整
- Modify: `internal/providers/registry.go`
  - 移除失效 provider 对应模型项映射或注册引用
- Modify: `internal/router/router.go`
  - 清理模型候选、fallback model、评分输入中的失效项
- Modify: `internal/httpserver/server.go`
  - 清理 `/v1/models` 或管理面模型列表的脏数据来源
- Modify: `internal/health/providers.go`
  - 仅在 provider 健康检查显式依赖已删除项时调整

### 启动入口
- Modify: `cmd/server/main.go`
- Modify: `cmd/server_main.go`

### 测试
- Modify/Create: `internal/providers/registry_test.go`
- Modify/Create: `internal/router/router_test.go`
- Modify/Create: `internal/httpserver/*_test.go`
- Modify/Create: `internal/health/*_test.go`

### 文档
- Modify: `docs/plans/2026-04-15-opencode-model-cleanup-design.md`
  - 如实施中发现边界变化，补充最终结论

---

## Chunk 1: 识别真实模型来源与失效判定清单

### Task 1: 梳理模型项的真实来源与生效路径

**Files:**
- Read/Modify: `internal/config/config.go`
- Read/Modify: `internal/providers/registry.go`
- Read/Modify: `internal/router/router.go`
- Read/Modify: `internal/httpserver/server.go`
- Read/Modify: `cmd/server/main.go`
- Read/Modify: `cmd/server_main.go`

- [ ] **Step 1: 列出所有 OpenCode 可用大模型相关入口**

输出一个工作清单，至少包含：
- 默认模型来源
- provider 注册来源
- router 候选/回退来源
- `/v1/models` 或管理接口展示来源

- [ ] **Step 2: 记录每个模型项的链路映射**

为每个待整理模型项记录：
- 模型名
- 对应 provider
- 是否进入 router 候选
- 是否对外暴露
- 当前疑点（未注册/悬空/仅展示/测试失败等）

- [ ] **Step 3: 标记“明确失效”和“待确认”两类项**

判定原则：
- 代码链路已断裂 → 明确失效
- 仅依赖外部凭证、但代码仍有效 → 待确认，不直接删除

- [ ] **Step 4: 形成实施清单并保存到工作说明中**

建议写入临时工作笔记或直接映射到提交说明，避免后续删错。

- [ ] **Step 5: Commit**

```bash
git add docs/plans/2026-04-15-opencode-model-cleanup-design.md
git commit -m "docs: refine opencode model cleanup scope"
```

### Task 2: 为失效判定建立测试或验证基线

**Files:**
- Modify/Create: `internal/providers/registry_test.go`
- Modify/Create: `internal/router/router_test.go`
- Modify/Create: `internal/httpserver/*_test.go`

- [ ] **Step 1: 先补一个失败测试覆盖“失效项仍被暴露/使用”的现状**

示例方向：
- router 仍会返回已失效模型候选
- `/v1/models` 仍包含已确认失效的模型名
- registry 仍接受已失效 provider/model 映射

- [ ] **Step 2: 运行相关单测确认当前失败**

Run: `go test ./internal/router ./internal/providers ./internal/httpserver`

Expected:
- 至少命中与本次清理目标对应的失败或不满足预期的断言

- [ ] **Step 3: 如果现有测试结构不适配，补最小可维护测试夹具**

要求：
- 不引入过度抽象
- 优先复用现有测试风格

- [ ] **Step 4: 重新运行相关测试，确保基线稳定可复现**

- [ ] **Step 5: Commit**

```bash
git add internal/providers/registry_test.go internal/router/router_test.go internal/httpserver
git commit -m "test: add cleanup regression coverage for invalid models"
```

---

## Chunk 2: 删除失效模型项并修复悬空引用

### Task 3: 删除真实来源中的失效模型项

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/providers/registry.go`
- Modify: `internal/router/router.go`
- Modify: `internal/httpserver/server.go`
- Modify: `cmd/server/main.go`
- Modify: `cmd/server_main.go`

- [ ] **Step 1: 从最上游真实来源删除已确认失效项**

顺序：
1. 配置默认值/模型列表来源
2. provider 注册映射
3. router 候选与 fallback
4. 接口展示来源

- [ ] **Step 2: 同步修复默认模型与 fallback**

要求：
- `DEFAULT_MODEL` 指向仍有效模型
- `FallbackModel` 不引用已删除项
- provider 默认值与模型默认值互相匹配

- [ ] **Step 3: 检查双入口 wiring 一致性**

核对：
- `cmd/server/main.go`
- `cmd/server_main.go`

如果一个入口依赖已删除项而另一个没有，必须统一修正。

- [ ] **Step 4: 清理展示层脏数据**

确保：
- `/v1/models` 不再返回失效项
- 若管理接口/前端展示依赖同源列表，也同步清理

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/providers/registry.go internal/router/router.go internal/httpserver/server.go cmd/server/main.go cmd/server_main.go
git commit -m "refactor: remove invalid opencode model entries"
```

### Task 4: 处理因删除引出的健康检查或边界逻辑

**Files:**
- Modify: `internal/health/providers.go`
- Modify/Create: `internal/health/*_test.go`
- Modify/Create: `internal/providers/*_test.go`

- [ ] **Step 1: 检查 health probe 是否仍引用已删除项**

重点确认：
- 是否以旧模型名/旧 provider 名做显式探测
- mock 模式下 disabled 逻辑是否被误改

- [ ] **Step 2: 只修复本次删除导致的边界问题**

不要顺手重做健康检查架构。

- [ ] **Step 3: 为边界修复补测试**

示例：
- 删除后 health 接口仍能稳定返回
- 已删除 provider/model 不再被误报为 active candidate

- [ ] **Step 4: 运行相关包测试确认通过**

Run: `go test ./internal/health ./internal/providers`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/health internal/providers
git commit -m "fix: align health checks with cleaned model set"
```

---

## Chunk 3: 验证、收尾与交付说明

### Task 5: 完成静态诊断与针对性测试验证

**Files:**
- Verify: 所有变更文件

- [ ] **Step 1: 对所有改动文件运行 LSP diagnostics**

Expected:
- 无新增 error

- [ ] **Step 2: 运行相关包测试**

Run:
```bash
go test ./internal/router ./internal/providers ./internal/httpserver ./internal/health
```
Expected: `PASS`

- [ ] **Step 3: 视改动跨度运行全量测试兜底**

Run:
```bash
go test ./...
```
Expected: `PASS`

- [ ] **Step 4: 做一次运行时行为抽查**

至少确认一项：
- 服务能启动
- `/v1/models` 不再出现已删除项
- 管理面相关模型列表不再出现脏数据

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "test: verify cleaned opencode model configuration"
```

### Task 6: 更新文档并准备实施交接

**Files:**
- Modify: `docs/plans/2026-04-15-opencode-model-cleanup-design.md`
- Modify: `docs/superpowers/plans/2026-04-15-opencode-model-cleanup.md`

- [ ] **Step 1: 在设计文档补充最终删除清单与原因**

至少写明：
- 删除了哪些模型项
- 每项为何判定为失效
- 默认值/fallback 如何调整

- [ ] **Step 2: 在实施计划上勾选已完成步骤或记录偏差**

如果实施过程中与原计划有偏差，要写明原因。

- [ ] **Step 3: 整理交付说明**

包括：
- 改动文件列表
- 运行的验证命令
- 剩余风险或待外部确认项

- [ ] **Step 4: 最终复核无悬空引用、无未说明 blocker**

- [ ] **Step 5: Commit**

```bash
git add docs/plans/2026-04-15-opencode-model-cleanup-design.md docs/superpowers/plans/2026-04-15-opencode-model-cleanup.md
git commit -m "docs: finalize opencode model cleanup plan"
```
