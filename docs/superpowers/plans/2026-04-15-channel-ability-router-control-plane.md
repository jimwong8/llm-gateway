# Channel + Ability + Router Control Plane Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce a first-stage control-plane routing path built around Channel + Ability while keeping the existing provider execution path and `/v1/chat/completions` contract working.

**Architecture:** Add lightweight control-plane data structures to the router, extend request/decision payloads with channel and ability fields, and let the router resolve explicit channels or ability-bound channels into the existing provider/model decision shape. Keep `providers.Registry` as the execution and resilience layer for this phase.

**Tech Stack:** Go, net/http, existing router/providers/httpserver packages, go test

---

## File impact map

- Modify: `internal/providers/provider.go`
  - Extend `ChatCompletionRequest` with control-plane request fields.
- Modify: `internal/router/router.go`
  - Add `Channel`, `Ability`, control-plane candidate resolution, and richer `Decision` metadata.
- Modify: `internal/router/router_test.go`
  - Add tests for explicit channel routing and ability-based routing fallback.
- Modify: `internal/httpserver/server.go`
  - Surface control-plane route metadata in headers and audit payloads.

## Chunk 1: Request and router control-plane slice

### Task 1: Extend request/decision shapes

**Files:**
- Modify: `internal/providers/provider.go`
- Modify: `internal/router/router.go`

- [ ] Add `RouteChannel`, `RouteAbilities`, and `RoutePolicyKey` fields to `ChatCompletionRequest`.
- [ ] Add `Channel` and `Ability` metadata fields to `router.Decision`.
- [ ] Keep all existing request/decision fields for compatibility.

### Task 2: Add lightweight Channel + Ability routing

**Files:**
- Modify: `internal/router/router.go`

- [ ] Define lightweight `Channel` and `Ability` structs inside router package.
- [ ] Seed default channels from the existing model registry.
- [ ] Add router methods to register/replace channels and abilities for tests and future runtime loading.
- [ ] Implement explicit channel routing when `req.RouteChannel` is present.
- [ ] Implement ability-based candidate routing when `req.RouteAbilities` is present.
- [ ] Preserve legacy scoring and `FallbackModel` behavior when control-plane fields are absent.

## Chunk 2: HTTP exposure and compatibility

### Task 3: Expose route metadata through HTTP

**Files:**
- Modify: `internal/httpserver/server.go`

- [ ] Add `X-Route-Channel` and `X-Route-Ability` headers when present.
- [ ] Include new request fields in `requestToMap` audit payload.
- [ ] Keep existing `/v1/chat/completions` response behavior unchanged.

## Chunk 3: Tests and verification

### Task 4: Add router tests for new routing path

**Files:**
- Modify: `internal/router/router_test.go`

- [ ] Add a test that explicit `RouteChannel` selects the bound model/provider.
- [ ] Add a test that `RouteAbilities` route to enabled channels and produce a fallback model when multiple channels are available.
- [ ] Ensure existing manual override and policy tests still pass.

### Task 5: Verify packages

**Files:**
- No code changes required beyond previous tasks.

- [ ] Run `go test ./internal/router ./internal/providers ./internal/httpserver`.
- [ ] Run diagnostics on changed packages/files.
- [ ] If package tests pass, run `go test ./...` as final verification.

Plan complete and saved to `docs/superpowers/plans/2026-04-15-channel-ability-router-control-plane.md`.
