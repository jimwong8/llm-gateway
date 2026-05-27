# Routing Reliability v2 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add retry-aware, rate-limit-aware, fallback-capable routing execution to llm-gateway without breaking the existing OpenAI-compatible API or current router decision behavior.

**Architecture:** Keep `internal/router` responsible for route decision and retry/fallback orchestration, while `internal/providers` remains responsible for concrete upstream calls and provider health/circuit state. Introduce small, testable primitives: error classification, key pool selection, retry executor, and execution trace. Integrate them behind feature-compatible defaults so existing registry retry behavior and router tests keep passing during migration.

**Tech Stack:** Go 1.22+, standard `context`, table-driven Go tests, existing `internal/router`, `internal/providers`, `internal/httpserver` packages.

---

## 0. Current State Summary

Existing relevant files:

- `internal/router/router.go`
  - Defines `Router`, `Decision`, `Channel`, `Ability`.
  - `Router.Decide(req providers.ChatCompletionRequest) Decision` selects model/provider/channel/ability.
  - Supports manual, policy, control-plane channel/ability, and legacy task scoring.
  - Produces `FallbackChain` in decision but does not execute fallback calls.
- `internal/router/router_test.go`
  - Tests manual override, global policy, task classification, explicit route channel, route abilities, and legacy fallback behavior.
- `internal/providers/provider.go`
  - Defines `Provider` interface with `ChatCompletion(ctx, req)` and `Name()`.
  - No stream interface yet.
- `internal/providers/registry.go`
  - `Registry.ChatCompletion(ctx, providerName, req)` resolves provider and retries same provider using config `ProviderMaxRetries`.
  - Has provider health/circuit state.
  - `shouldRetry(name)` currently disables retry for mock providers.
- `internal/providers/provider_test.go`
  - Tests mock provider behavior and registry helper functions.

Important constraints:

- Do **not** change `Router.Decide` semantics in this plan.
- Do **not** remove `providers.Registry.ChatCompletion` retry behavior immediately; first add v2 primitives and integrate conservatively.
- Do **not** require database-backed provider keys in this first implementation. Start with in-memory/provider-channel keys so tests are deterministic.
- Streaming first-chunk detection must be designed now, but only fully wired if an existing streaming interface is present; otherwise add interfaces and tests with fake streams without changing public API.

## 1. Desired Behavior

Routing Reliability v2 adds:

1. Error classification:
   - `rate_limit`: 429 or provider-specific rate limit text.
   - `retryable_upstream`: 408/502/503/504, temporary network errors.
   - `client_cancelled`: context cancellation/deadline.
   - `auth`: invalid API key, unauthorized provider credentials.
   - `bad_request`: non-retryable caller/request issues.
   - `unknown`: default safe behavior.
2. Key pool selection:
   - Select enabled key for provider/channel.
   - Exclude keys already rate-limited in current execution.
   - Reset exclusion when all eligible keys are exhausted.
   - Ignore zero/negative weight keys.
3. Retry executor:
   - Generic or typed function to execute provider calls with retry trace.
   - On rate limit: rotate key when key pool exists.
   - On retryable upstream/network: retry according to max attempts.
   - On auth/bad request/client cancel: stop immediately.
   - Preserve context cancellation.
4. Fallback execution:
   - If selected provider/model fails with retry-exhausted retryable error, try `Decision.FallbackChain` in order.
   - Do not fallback for auth/bad_request/client_cancelled unless explicitly configured later.
5. Execution trace:
   - Attempt number, provider, model, channel, key id suffix/hash, error class, latency, final selected fallback.
   - Trace should be returned to caller or available for logging/testing.

## 2. File Structure

### Create

- `internal/router/error_classification.go`
  - Error class enum and classifier.
- `internal/router/error_classification_test.go`
  - Table-driven classification tests.
- `internal/router/key_pool.go`
  - Provider key model and deterministic weighted selector.
- `internal/router/key_pool_test.go`
  - Key pool behavior tests.
- `internal/router/retry.go`
  - Retry executor, retry config, attempt trace.
- `internal/router/retry_test.go`
  - Retry and fallback orchestration tests with fake functions.
- `internal/router/executor.go`
  - High-level route execution helper that bridges `Decision` + provider call function.
- `internal/router/executor_test.go`
  - Tests fallback chain execution based on `Decision`.

### Modify

- `internal/router/router.go`
  - Add helper to expose ordered execution candidates from a `Decision` without changing `Decide` behavior.
- `internal/providers/provider.go`
  - Add optional structured error type only if needed by classifier.
- `internal/providers/registry.go`
  - Keep existing behavior; optionally expose health state query needed by key pool/executor.
- `internal/httpserver/...chat completions handler...`
  - Wire v2 executor only after all router package tests pass. Exact file must be identified by implementation worker via grep for `ChatCompletion(` route handler.

### Do Not Modify Initially

- Existing admin UI pages.
- Existing billing/memory/governance modules.
- Existing public OpenAI response schemas.

## 3. Public/Internal API Design

### 3.1 Error Classification

```go
type ErrorClass string

const (
    ErrorClassNone ErrorClass = "none"
    ErrorClassRateLimit ErrorClass = "rate_limit"
    ErrorClassRetryableUpstream ErrorClass = "retryable_upstream"
    ErrorClassClientCancelled ErrorClass = "client_cancelled"
    ErrorClassAuth ErrorClass = "auth"
    ErrorClassBadRequest ErrorClass = "bad_request"
    ErrorClassUnknown ErrorClass = "unknown"
)

type ClassifiedError struct {
    Class ErrorClass
    StatusCode int
    Provider string
    Retryable bool
    RotateKey bool
    Err error
}

func ClassifyError(ctx context.Context, provider string, err error) ClassifiedError
```

### 3.2 Key Pool

```go
type ProviderKey struct {
    ID string
    Provider string
    Channel string
    SecretRef string
    Weight int
    Enabled bool
}

type KeyPool interface {
    Next(ctx context.Context, provider string, channel string, used map[string]bool) (ProviderKey, error)
}

type StaticKeyPool struct {
    Keys []ProviderKey
}
```

Selector must be deterministic for tests. Do not use crypto randomness here. Weighted behavior can be stable by sorting keys and expanding by weight or choosing highest weight first.

### 3.3 Retry Executor

Prefer package-level generic function because Go does not allow type parameters on methods in some versions/patterns:

```go
type RetryConfig struct {
    MaxAttempts int
    BaseDelay time.Duration
    MaxDelay time.Duration
}

type AttemptTrace struct {
    Attempt int
    Provider string
    Model string
    Channel string
    KeyID string
    ErrorClass ErrorClass
    Error string
    Latency time.Duration
}

type RetryResult struct {
    Attempts []AttemptTrace
    FinalProvider string
    FinalModel string
    FinalKeyID string
}

type ExecuteWithKeyFunc[T any] func(ctx context.Context, key ProviderKey) (T, error)

func ExecuteWithRetries[T any](ctx context.Context, cfg RetryConfig, pool KeyPool, provider string, channel string, fn ExecuteWithKeyFunc[T]) (T, RetryResult, error)
```

### 3.4 Route Executor

```go
type RouteTarget struct {
    Provider string
    Model string
    Channel string
    Reason string
}

func TargetsFromDecision(decision Decision) []RouteTarget

type ExecuteRouteFunc[T any] func(ctx context.Context, target RouteTarget, key ProviderKey) (T, error)

func ExecuteDecision[T any](ctx context.Context, decision Decision, cfg RetryConfig, pool KeyPool, fn ExecuteRouteFunc[T]) (T, RetryResult, error)
```

Rules:

- First target is `decision.Provider` + `decision.Model`.
- Then append each `FallbackChain` item.
- Do not duplicate same provider/model/channel target.
- Stop after first success.
- Fallback only after retry-exhausted retryable/rate-limit/upstream classes.

## 4. Chunk 1: Error Classification

### Task 1: Add error class tests

**Files:**

- Create: `internal/router/error_classification_test.go`
- Create: `internal/router/error_classification.go`

- [ ] **Step 1: Write failing tests**

Add table-driven tests covering:

```go
func TestClassifyError(t *testing.T) {
    ctx := context.Background()
    tests := []struct {
        name string
        err error
        wantClass ErrorClass
        wantRetryable bool
        wantRotateKey bool
    }{
        {name: "nil", err: nil, wantClass: ErrorClassNone},
        {name: "rate limit status", err: ProviderHTTPError{StatusCode: 429, Message: "too many requests"}, wantClass: ErrorClassRateLimit, wantRetryable: true, wantRotateKey: true},
        {name: "rate limit text", err: errors.New("provider rate limit exceeded"), wantClass: ErrorClassRateLimit, wantRetryable: true, wantRotateKey: true},
        {name: "bad gateway", err: ProviderHTTPError{StatusCode: 502, Message: "bad gateway"}, wantClass: ErrorClassRetryableUpstream, wantRetryable: true},
        {name: "service unavailable", err: ProviderHTTPError{StatusCode: 503, Message: "unavailable"}, wantClass: ErrorClassRetryableUpstream, wantRetryable: true},
        {name: "unauthorized", err: ProviderHTTPError{StatusCode: 401, Message: "invalid api key"}, wantClass: ErrorClassAuth},
        {name: "bad request", err: ProviderHTTPError{StatusCode: 400, Message: "model required"}, wantClass: ErrorClassBadRequest},
        {name: "unknown", err: errors.New("boom"), wantClass: ErrorClassUnknown},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := ClassifyError(ctx, "openai", tt.err)
            if got.Class != tt.wantClass { t.Fatalf("class = %s, want %s", got.Class, tt.wantClass) }
            if got.Retryable != tt.wantRetryable { t.Fatalf("retryable = %v", got.Retryable) }
            if got.RotateKey != tt.wantRotateKey { t.Fatalf("rotateKey = %v", got.RotateKey) }
        })
    }
}
```

Also add cancellation test:

```go
func TestClassifyError_ContextCancelled(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    cancel()
    got := ClassifyError(ctx, "openai", context.Canceled)
    if got.Class != ErrorClassClientCancelled { t.Fatalf("class = %s", got.Class) }
    if got.Retryable { t.Fatalf("client cancellation must not retry") }
}
```

- [ ] **Step 2: Run test to verify failure**

Run:

```bash
go test ./internal/router -run 'TestClassifyError' -v
```

Expected: FAIL because `ErrorClass`, `ProviderHTTPError`, and `ClassifyError` do not exist.

- [ ] **Step 3: Implement minimal classifier**

Create `internal/router/error_classification.go` with:

- `ErrorClass` constants.
- `ProviderHTTPError` type implementing `Error()`.
- `ClassifiedError`.
- `ClassifyError` using `errors.Is`, status code, and lower-cased error text.

- [ ] **Step 4: Run tests**

```bash
go test ./internal/router -run 'TestClassifyError' -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/router/error_classification.go internal/router/error_classification_test.go
git commit -m "feat(router): classify retryable provider errors"
```

## 5. Chunk 2: Key Pool

### Task 2: Add static key pool

**Files:**

- Create: `internal/router/key_pool.go`
- Create: `internal/router/key_pool_test.go`

- [ ] **Step 1: Write failing tests**

Tests:

```go
func TestStaticKeyPool_NextSkipsUsedDisabledAndZeroWeight(t *testing.T)
func TestStaticKeyPool_NextResetsWhenAllEligibleKeysUsed(t *testing.T)
func TestStaticKeyPool_NextFiltersByProviderAndChannel(t *testing.T)
func TestStaticKeyPool_NextReturnsErrorWhenNoEligibleKeys(t *testing.T)
func TestStaticKeyPool_NextPrefersHigherWeightDeterministically(t *testing.T)
```

Expected behavior:

- Disabled keys skipped.
- `Weight <= 0` skipped.
- Provider comparison case-insensitive.
- Empty channel in key means usable for all channels; non-empty channel must match requested channel.
- If all eligible keys are marked used, clear `used` entries for those keys and return best eligible key.
- “Higher weight deterministic” means sort by weight desc then ID asc.

- [ ] **Step 2: Run failing tests**

```bash
go test ./internal/router -run 'TestStaticKeyPool' -v
```

Expected: FAIL.

- [ ] **Step 3: Implement StaticKeyPool**

`internal/router/key_pool.go` should include:

```go
var ErrNoProviderKey = errors.New("no eligible provider key")
```

Implementation notes:

- Do not mutate `p.Keys`.
- Mutate `used` only for reset behavior.
- Return safe `ProviderKey` value, not pointer.

- [ ] **Step 4: Run tests**

```bash
go test ./internal/router -run 'TestStaticKeyPool' -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/router/key_pool.go internal/router/key_pool_test.go
git commit -m "feat(router): add static provider key pool"
```

## 6. Chunk 3: Retry Executor

### Task 3: Add generic retry executor

**Files:**

- Create: `internal/router/retry.go`
- Create: `internal/router/retry_test.go`

- [ ] **Step 1: Write failing tests**

Test names and behavior:

```go
func TestExecuteWithRetries_SuccessFirstAttempt(t *testing.T)
```

Expected: one attempt, no error, result returned.

```go
func TestExecuteWithRetries_RateLimitRotatesKey(t *testing.T)
```

Setup two keys. First call returns 429 for key A, second succeeds with key B. Assert attempts contain A then B and final key is B.

```go
func TestExecuteWithRetries_NetworkErrorRetries(t *testing.T)
```

Function returns 503 once then success. Assert two attempts.

```go
func TestExecuteWithRetries_AuthErrorStopsImmediately(t *testing.T)
```

Function returns 401. Assert one attempt and no retry.

```go
func TestExecuteWithRetries_ContextCancelledStops(t *testing.T)
```

Cancel context after first failure. Assert no further retry.

```go
func TestExecuteWithRetries_AllKeysExhaustedResets(t *testing.T)
```

Two keys both 429, third attempt returns first key again after used reset when max attempts allows it.

- [ ] **Step 2: Run failing tests**

```bash
go test ./internal/router -run 'TestExecuteWithRetries' -v
```

Expected: FAIL.

- [ ] **Step 3: Implement retry executor**

Implementation notes:

- Normalize `MaxAttempts <= 0` to 1.
- Use `used := map[string]bool{}` inside execution.
- If pool is nil, use a synthetic empty key with provider.
- Record latency per attempt.
- Delay can be skipped in tests if `BaseDelay == 0`.
- Do not sleep if context is cancelled.
- On `RotateKey`, mark current key used.
- Return zero value of T on final failure.

- [ ] **Step 4: Run tests**

```bash
go test ./internal/router -run 'TestExecuteWithRetries' -v
```

Expected: PASS.

- [ ] **Step 5: Run all router tests**

```bash
go test ./internal/router -v
```

Expected: PASS and existing `Router.Decide` tests remain unchanged.

- [ ] **Step 6: Commit**

```bash
git add internal/router/retry.go internal/router/retry_test.go
git commit -m "feat(router): add retry executor with key rotation"
```

## 7. Chunk 4: Decision Fallback Executor

### Task 4: Convert decision to executable targets

**Files:**

- Create: `internal/router/executor.go`
- Create: `internal/router/executor_test.go`
- Modify: `internal/router/router.go` only if helper needs access to existing types; prefer new file in same package.

- [ ] **Step 1: Write target extraction tests**

Test:

```go
func TestTargetsFromDecision_IncludesPrimaryAndFallbacks(t *testing.T)
```

Given:

```go
Decision{
    Provider: "openai",
    Model: "gpt-4o-mini",
    Channel: "primary",
    FallbackChain: []FallbackRoute{
        {Provider: "anthropic", Model: "claude-sonnet", Reason: "second"},
        {Provider: "google", Model: "gemini", Reason: "third"},
    },
}
```

Expect 3 ordered targets.

Test duplicate suppression:

```go
func TestTargetsFromDecision_DeduplicatesPrimaryFallback(t *testing.T)
```

- [ ] **Step 2: Write ExecuteDecision tests**

Tests:

```go
func TestExecuteDecision_PrimarySuccessDoesNotFallback(t *testing.T)
func TestExecuteDecision_PrimaryRetryExhaustedFallsBack(t *testing.T)
func TestExecuteDecision_BadRequestDoesNotFallback(t *testing.T)
func TestExecuteDecision_AllTargetsFailReturnsTrace(t *testing.T)
```

Important: For primary retry exhausted, fake primary returns 503 for all attempts, fallback succeeds. Assert final provider/model is fallback and attempts include both targets.

- [ ] **Step 3: Run failing tests**

```bash
go test ./internal/router -run 'TestTargetsFromDecision|TestExecuteDecision' -v
```

Expected: FAIL.

- [ ] **Step 4: Implement executor**

Implementation notes:

- `ExecuteDecision` calls `ExecuteWithRetries` for each target.
- It appends attempt traces across targets.
- It falls back only when final classified error is `RateLimit`, `RetryableUpstream`, or `Unknown` if config says unknown retryable. For P0, do not fallback on `Unknown` unless tests require. Prefer safer no fallback for unknown.
- It sets request model at integration layer, not here.

- [ ] **Step 5: Run tests**

```bash
go test ./internal/router -run 'TestTargetsFromDecision|TestExecuteDecision' -v
```

Expected: PASS.

- [ ] **Step 6: Run all router tests**

```bash
go test ./internal/router -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/router/executor.go internal/router/executor_test.go
git commit -m "feat(router): execute decisions with fallback trace"
```

## 8. Chunk 5: Provider Error Normalization

### Task 5: Normalize provider HTTP errors into classifier-compatible errors

**Files:**

- Modify: `internal/providers/openai.go`
- Modify: `internal/providers/anthropic.go`
- Modify: `internal/providers/domestic.go`
- Modify: `internal/providers/edgefn.go`
- Modify: `internal/providers/local.go` if it makes HTTP calls
- Test: existing or new provider tests for one representative HTTP adapter

- [ ] **Step 1: Inspect provider HTTP error behavior**

Use grep/read to identify response status handling. Find code paths that currently return `fmt.Errorf("... status ...")`.

- [ ] **Step 2: Add tests for one adapter**

Prefer OpenAI adapter because it is core.

Test should fake HTTP server returning:

- 429
- 401
- 503
- 400

Assert returned error can be classified correctly by `router.ClassifyError`.

If importing router from providers creates cycle, move `ProviderHTTPError` to `internal/providers` and make router classifier recognize interface:

```go
type HTTPStatusError interface { HTTPStatusCode() int }
```

Preferred to avoid cycle:

- Define `ProviderHTTPError` in `internal/providers`.
- Router classifier uses interface instead of concrete type.

If this design adjustment is needed, update Chunk 1 accordingly.

- [ ] **Step 3: Implement normalized errors**

Provider HTTP adapters should return errors that expose status code and message.

- [ ] **Step 4: Run provider tests**

```bash
go test ./internal/providers -v
```

Expected: PASS.

- [ ] **Step 5: Run router tests**

```bash
go test ./internal/router -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/providers internal/router
git commit -m "feat(providers): expose upstream status for retry classification"
```

## 9. Chunk 6: HTTP Server Integration

### Task 6: Wire ExecuteDecision into chat completion path behind compatibility defaults

**Files:**

- Modify: chat completion handler file under `internal/httpserver`.
- Possibly modify: `internal/httpserver/server.go`.
- Tests: existing `internal/httpserver/*chat*test.go` plus new test file if needed.

- [ ] **Step 1: Locate current chat completion handler**

Use grep for:

```bash
rg "ChatCompletion\(|/v1/chat/completions|chat/completions" internal/httpserver internal
```

Do not edit until exact path is identified.

- [ ] **Step 2: Write integration tests**

Create/modify test covering:

```go
func TestChatCompletions_UsesFallbackWhenPrimaryProviderRetryExhausted(t *testing.T)
func TestChatCompletions_DoesNotFallbackOnBadRequest(t *testing.T)
func TestChatCompletions_RecordsRetryTraceHeaderOrAuditField(t *testing.T)
```

Use fake providers:

- primary returns 503
- fallback returns success

Expected response model/provider corresponds fallback route if existing response exposes model. If provider not exposed, assert body success and trace location.

- [ ] **Step 3: Add route executor dependency**

Do not instantiate global state directly in handler if existing server struct has router/provider registry fields. Follow current constructor pattern.

Pseudo-integration:

```go
decision := s.router.Decide(req)
resp, trace, err := router.ExecuteDecision(ctx, decision, retryCfg, keyPool, func(ctx context.Context, target router.RouteTarget, key router.ProviderKey) (providers.ChatCompletionResponse, error) {
    routedReq := req
    routedReq.Model = target.Model
    return s.providers.ChatCompletion(ctx, target.Provider, routedReq)
})
```

Initial key pool can be nil or static from config; do not block integration on database key pool.

- [ ] **Step 4: Preserve legacy behavior**

If no fallback chain and no key pool, result should match old code except trace logs.

- [ ] **Step 5: Run httpserver tests**

```bash
go test ./internal/httpserver -run 'ChatCompletions|Fallback|Retry' -v
```

Expected: PASS.

- [ ] **Step 6: Run relevant packages**

```bash
go test ./internal/router ./internal/providers ./internal/httpserver -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/httpserver internal/router
git commit -m "feat(httpserver): route chat completions through retry executor"
```

## 10. Chunk 7: Streaming First-Chunk Detection Design Hook

### Task 7: Add stream-safe interface and fake tests without forcing full SSE implementation

**Files:**

- Modify: `internal/providers/provider.go`
- Create: `internal/router/stream_retry.go`
- Create: `internal/router/stream_retry_test.go`

- [ ] **Step 1: Check existing stream support**

Search for stream response types. If none exist, add only internal helper types and tests.

- [ ] **Step 2: Add minimal stream chunk type if absent**

Potential shape:

```go
type ChatCompletionStreamChunk struct {
    Data []byte
    Err error
    Done bool
}
```

But avoid disrupting provider interface. Prefer defining in router until streaming feature plan.

- [ ] **Step 3: Write first-chunk tests**

Tests:

```go
func TestCheckFirstStreamChunkForError_ReturnsErrorBeforeExposingChunk(t *testing.T)
func TestCheckFirstStreamChunkForError_ForwardsFirstChunkOnSuccess(t *testing.T)
func TestCheckFirstStreamChunkForError_DrainsOrClosesOnContextCancel(t *testing.T)
```

- [ ] **Step 4: Implement helper**

Behavior:

- Read one chunk from source channel.
- If first chunk contains classified error, return error and a drained/closed channel.
- If first chunk is data, return new channel that emits first chunk then remaining chunks.
- Respect context cancellation.

- [ ] **Step 5: Run tests**

```bash
go test ./internal/router -run 'TestCheckFirstStreamChunkForError' -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/router/stream_retry.go internal/router/stream_retry_test.go internal/providers/provider.go
git commit -m "feat(router): add stream first chunk error gate"
```

## 11. Chunk 8: Observability and Trace Surfacing

### Task 8: Make retry/fallback trace visible for debugging

**Files:**

- Modify: `internal/router/retry.go`
- Modify: `internal/router/executor.go`
- Modify: chat completion handler in `internal/httpserver`
- Test: HTTP handler trace test

- [ ] **Step 1: Decide trace exposure**

P0 safe option:

- Add response header `X-LLM-Gateway-Route-Trace: <compact-json-or-request-id>` only in admin/debug mode.
- Always log structured trace with request_id.
- Do not expose provider key id except redacted/hash suffix.

- [ ] **Step 2: Add redaction test**

Ensure key secret/ref never appears in trace.

- [ ] **Step 3: Implement compact trace**

Trace fields:

- attempts count
- final provider
- final model
- fallback_used bool
- last_error_class

- [ ] **Step 4: Run tests**

```bash
go test ./internal/router ./internal/httpserver -run 'Trace|Retry|Fallback' -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/router internal/httpserver
git commit -m "feat(router): expose redacted retry trace"
```

## 12. Full Verification

After all chunks:

- [ ] **Step 1: Run router tests**

```bash
go test ./internal/router -v
```

Expected: PASS.

- [ ] **Step 2: Run provider tests**

```bash
go test ./internal/providers -v
```

Expected: PASS.

- [ ] **Step 3: Run httpserver tests**

```bash
go test ./internal/httpserver -v
```

Expected: PASS or document unrelated existing failures.

- [ ] **Step 4: Run broad backend tests**

```bash
go test ./...
```

Expected: PASS or document unrelated existing failures in `docs/plans/p0-baseline-known-failures.md`.

- [ ] **Step 5: Manual smoke**

Start gateway and call:

```bash
curl -sS http://localhost:8080/v1/models -H 'Authorization: Bearer <key>'
```

Then:

```bash
curl -sS http://localhost:8080/v1/chat/completions \
  -H 'Authorization: Bearer <key>' \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}'
```

Expected: Same compatible response shape as before.

- [ ] **Step 6: Failure injection smoke**

Use mock/fake provider config if available:

- Primary returns 503.
- Fallback returns 200.
- Verify response succeeds and trace indicates fallback.

## 13. Rollback Plan

If integration causes regression:

1. Revert only HTTP server integration commit:

```bash
git revert <commit-for-httpserver-integration>
```

2. Keep router primitives if tests pass; they are unused and safe.
3. If provider error normalization causes issue, revert provider normalization commit.
4. Existing `providers.Registry.ChatCompletion` retry behavior remains fallback path.

## 14. Acceptance Criteria

Routing Reliability v2 is accepted when:

- Existing `Router.Decide` tests pass unchanged.
- New classifier/key-pool/retry/executor tests pass.
- Chat completion handler can fallback from primary retryable failure to fallback route.
- Auth/bad request/client cancellation do not retry/fallback.
- Rate-limit errors rotate provider key when key pool has alternatives.
- Retry/fallback trace is redacted and test-covered.
- External OpenAI-compatible response shape is unchanged for normal successful calls.

## 15. Implementation Notes for Workers

- Do not optimize with `sync.Pool` in this plan. Performance object pooling belongs to P2 after benchmark evidence.
- Avoid introducing randomness into tests.
- Avoid adding database schema in this plan unless a provider key table already exists and is easy to reuse.
- If import cycles appear between `router` and `providers`, move shared low-level error status interface into `internal/providers` or a tiny `internal/upstream` package; do not create a large common package.
- Keep commits small and aligned with chunks.
- After each chunk, run the exact tests listed before moving forward.

Plan complete and saved to `docs/plans/2026-05-18-routing-reliability-v2-implementation.md`. Ready to execute.
