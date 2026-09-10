# Admin UI E2E Regression Report - 2026-05-17

## Scope

Target: `http://10.100.1.17:8080/admin/ui`
Browser automation: Puppeteer real browser session
Login token used: administrator bearer token provided during the test session

Covered modules:

- Login / session bootstrap
- Dashboard
- Providers
- API Keys
- Models
- Routing Policies
- Budget Policies
- Usage
- Audit Logs
- Tool Management
- Configuration
- Online test console

## Summary

The admin UI is reachable and most primary list/detail/edit flows work. Several regressions were found and fixed during this pass:

1. Provider save API rejected UI payloads because the admin handler decoded directly into `gateway.Provider` and therefore did not accept legacy UI field names such as `provider_type`, `base_url`, `api_key`, and `is_active`.
2. Provider discovery API panicked when a provider had nil/empty config or missing base URL, returning `socket hang up` to the UI.
3. Provider health check could not run for providers without a model and returned `model required` instead of using a discovered/default model.
4. Configuration save sent an invalid JSON scalar through `fetchJson`, then tried to parse an empty response body as JSON.

After fixes, provider edit/create flows, provider discovery, provider health check, and configuration save were re-tested successfully in the browser.

## Tested Flows

### Login

- Opened `/admin/ui`, redirected to `/login`.
- Entered admin bearer token.
- Landed on Dashboard.

Result: pass.

### Dashboard

Observed cards and sections:

- Request statistics
- Average latency
- Estimated cost
- Success rate
- System status
- Provider health
- Recent activity
- Quick actions

Result: pass. Backend metrics displayed zero/empty values in this environment, but UI rendered correctly.

### Providers

Actions performed:

- Opened provider list.
- Inspected existing provider `google_gemini`.
- Ran provider health check.
- Ran model discovery.
- Edited existing provider metadata.
- Created `E2E Browser Provider`, then deleted it.

Initial defects:

- Save failed with `provider type is required`.
- Discovery failed with `socket hang up` due backend panic.
- Health check failed with `model required`.

Fix verification:

- Edit succeeded and returned success toast.
- Discovery succeeded and displayed models.
- Health check succeeded using discovered/default model.
- Create/delete provider succeeded.

Result: pass after fixes.

### API Keys

Actions performed:

- Opened key list.
- Created a test key named `e2e-temp-key`.
- Verified generated key modal/message.
- Used search field.
- Edited key status/name.
- Deleted the test key.

Result: pass.

### Models

Actions performed:

- Opened model list.
- Used provider search/filter.
- Created test model `e2e-browser-model`.
- Edited model display/capabilities.
- Deleted test model.

Result: pass.

### Routing Policies

Actions performed:

- Opened route policies.
- Created test policy `e2e-route-policy`.
- Edited the policy.
- Deleted test policy.

Result: pass.

### Budget Policies

Actions performed:

- Opened budget policies.
- Created a temporary budget policy.
- Edited values/status.
- Deleted temporary policy.

Result: pass.

### Usage

Actions performed:

- Opened usage page.
- Used time granularity filters.
- Exported usage data.

Result: pass.

### Audit Logs

Actions performed:

- Opened audit log page.
- Applied action/status filters.
- Verified table rendering and recent admin actions.

Result: pass.

### Tool Management

Actions performed:

- Opened tools page.
- Created temporary tool `e2e_temp_tool`.
- Edited tool.
- Deleted tool.

Result: pass.

### Configuration

Actions performed:

- Opened configuration page.
- Adjusted admin token expiration setting.
- Saved configuration.

Initial defect:

- Save failed with JSON parse error because request body was not valid JSON and response body could be empty.

Fix verification:

- Save succeeded and success toast was shown.

Result: pass after fixes.

### Online Test Console

Actions performed:

- Opened online test page.
- Verified provider/model selectors.
- Sent a simple test message.

Result: page controls rendered. In this environment provider availability may depend on upstream credentials/network, so no broad upstream quality assertion was made.

## Code Changes Made

### Backend

- `internal/admin/handlers.go`
  - Added provider request normalization (`adminProviderRequest`) so UI legacy payload names map to backend provider fields.
  - Made provider discovery robust for nil/empty configs and missing base URLs.
  - Added default model selection for provider health checks.
  - Added safe provider config coercion helpers.

### Frontend

- `web/admin/src/api/client.ts`
  - Added support for `rawBody` request payloads.
  - Avoid parsing empty HTTP response bodies as JSON.

- `web/admin/src/pages/ConfigPage.tsx`
  - Sends config values as valid raw JSON bodies.

## Verification Commands

Executed from repository root:

```bash
go test ./internal/admin
npm test -- --runInBand
```

Results:

- `go test ./internal/admin`: pass
- `npm test -- --runInBand`: pass, 5 frontend tests passing

## Remaining Notes

- Playwright MCP profile was locked by another browser instance, so Puppeteer was used for the real browser pass.
- The environment contained existing providers, keys, and policies; temporary E2E artifacts were removed after creation.
- Backend process was restarted multiple times during fixes via `killall llm-gateway` followed by `go run ./cmd/gateway -config config/config.yaml`.
