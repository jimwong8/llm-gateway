# Provider Adapter Enhancement Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 Provider Adapter 落地统一错误分类与 usage 口径归一化，保证上游 Registry、Billing 与 Observability 可以稳定消费标准化输出。

**Architecture:** 在 [`internal/providers/provider.go`](internal/providers/provider.go) 中增加统一的 provider error 结构与 usage 归一化辅助逻辑，并首先在 [`internal/providers/openai.go`](internal/providers/openai.go) 中接入。[`internal/providers/registry.go`](internal/providers/registry.go) 只做最小兼容验证，确保现有重试与熔断逻辑不被破坏。

**Tech Stack:** Go, net/http, JSON, existing provider registry, existing billing/observability integration

---

## File Map

- Modify: [`internal/providers/provider.go`](internal/providers/provider.go)
  - 新增统一 provider error 类型与 usage 归一化辅助函数
- Modify: [`internal/providers/openai.go`](internal/providers/openai.go)
  - 接入错误分类与 usage 归一化
- Modify: [`internal/providers/registry.go`](internal/providers/registry.go)
  - 最小兼容统一错误包装
- Create: [`internal/providers/openai_test.go`](internal/providers/openai_test.go)
  - 覆盖错误分类与 usage 补全测试
- Create: [`internal/providers/registry_test.go`](internal/providers/registry_test.go)
  - 覆盖包装后错误对重试与熔断的兼容性
- Optional Modify: [`docs/plans/2026-03-24-provider-adapter-enhancement-design.md`](docs/plans/2026-03-24-provider-adapter-enhancement-design.md)
  - 补充实现备注（可选）

---

## Chunk 1: 统一 provider 错误与 usage 结构

### Task 1: 在 [`internal/providers/provider.go`](internal/providers/provider.go) 中引入统一错误结构

**Files:**
- Modify: [`internal/providers/provider.go`](internal/providers/provider.go)
- Test: [`internal/providers/openai_test.go`](internal/providers/openai_test.go)

- [ ] **Step 1: 写失败测试，约束 provider error 形状**

新增测试先固定一个最小错误结构，例如：

```go
type ProviderError struct {
    Type       string
    StatusCode int
    Message    string
    Retryable  bool
}
```

并断言：
- `Type` 至少支持 `auth` / `rate_limit` / `timeout` / `upstream_4xx` / `upstream_5xx` / `network` / `decode`

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/providers -run TestProviderErrorShape -v`
Expected: FAIL，提示结构不存在。

- [ ] **Step 3: 写最小实现**

在 [`internal/providers/provider.go`](internal/providers/provider.go) 中新增：
- [`ProviderError`](internal/providers/provider.go)
- `func (e *ProviderError) Error() string`
- `func normalizeUsage(resp *ChatCompletionResponse)`

其中 `normalizeUsage(...)` 规则：
- 缺 `total_tokens` 时补 `prompt + completion`
- 三者都缺时补零

- [ ] **Step 4: 运行测试并确认通过**

Run: `go test ./internal/providers -run TestProviderErrorShape -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/providers/provider.go internal/providers/openai_test.go
git commit -m "feat: add standardized provider error and usage normalization"
```

### Task 2: 在 [`internal/providers/openai.go`](internal/providers/openai.go) 中落地错误分类与 usage 补全

**Files:**
- Modify: [`internal/providers/openai.go`](internal/providers/openai.go)
- Test: [`internal/providers/openai_test.go`](internal/providers/openai_test.go)

- [ ] **Step 1: 写失败测试，覆盖 HTTP 4xx / 5xx / decode 错误**

建议测试：
- 401 -> `auth`
- 429 -> `rate_limit`
- 4xx -> `upstream_4xx`
- 5xx -> `upstream_5xx`
- 非法 JSON -> `decode`

测试名建议：
- `TestOpenAIProvider_ClassifiesErrors`
- `TestOpenAIProvider_NormalizesUsage`

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/providers -run 'TestOpenAIProvider_(ClassifiesErrors|NormalizesUsage)' -v`
Expected: FAIL

- [ ] **Step 3: 写最小实现**

在 [`internal/providers/openai.go`](internal/providers/openai.go) 中：
- 针对 `resp.StatusCode` 返回 [`ProviderError`](internal/providers/provider.go)
- 网络错误分类为 `network`
- `context deadline exceeded` 归类为 `timeout`
- 解码失败归类为 `decode`
- decode 成功后调用 `normalizeUsage(&out)`

- [ ] **Step 4: 运行测试并确认通过**

Run: `go test ./internal/providers -run 'TestOpenAIProvider_(ClassifiesErrors|NormalizesUsage)' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/providers/openai.go internal/providers/openai_test.go
git commit -m "feat: classify openai provider errors and normalize usage"
```

---

## Chunk 2: 保证 Registry 兼容统一错误

### Task 3: 为 [`internal/providers/registry.go`](internal/providers/registry.go) 增加最小兼容测试

**Files:**
- Modify: [`internal/providers/registry.go`](internal/providers/registry.go)
- Test: [`internal/providers/registry_test.go`](internal/providers/registry_test.go)

- [ ] **Step 1: 写失败测试，覆盖错误包装不破坏重试**

新增测试：
- `ProviderError{Type: "upstream_5xx"}` 不应破坏重试路径
- `ProviderError{Type: "auth"}` 不应该被当作可无限重试的普通错误

测试名建议：
- `TestRegistry_ProviderErrorCompatibility`

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/providers -run TestRegistry_ProviderErrorCompatibility -v`
Expected: FAIL

- [ ] **Step 3: 写最小兼容实现**

在 [`internal/providers/registry.go`](internal/providers/registry.go) 中：
- 如有必要，增加 `errors.As(err, *ProviderError)` 判断
- 保持现有 `recordFailure(...)` / `shouldRetry(...)` 逻辑最小调整
- 不重构 Registry 主体

- [ ] **Step 4: 运行测试并确认通过**

Run: `go test ./internal/providers -run TestRegistry_ProviderErrorCompatibility -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/providers/registry.go internal/providers/registry_test.go
git commit -m "test: ensure registry handles provider errors consistently"
```

---

## Chunk 3: 全量回归与远程验证

### Task 4: 补齐 provider 层回归测试

**Files:**
- Create: [`internal/providers/openai_test.go`](internal/providers/openai_test.go)
- Create: [`internal/providers/registry_test.go`](internal/providers/registry_test.go)

- [ ] **Step 1: 运行 providers 全量测试**

Run: `go test ./internal/providers -v`
Expected: PASS

- [ ] **Step 2: 运行关键上游回归**

Run:
- `go test ./internal/providers ./internal/httpserver ./internal/billing -v`
Expected: PASS

- [ ] **Step 3: 如有远程环境，做最小远程编译验证**

Run（远程）:
- `go test ./internal/providers ./internal/httpserver ./internal/billing`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/providers/provider.go internal/providers/openai.go internal/providers/registry.go internal/providers/openai_test.go internal/providers/registry_test.go
git commit -m "chore: finalize provider adapter enhancement"
```

---

Plan complete and saved to `docs/superpowers/plans/2026-03-24-provider-adapter-enhancement.md`. Ready to execute?
