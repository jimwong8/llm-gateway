# Admin UI 治理覆盖补完 实施计划

> **For Hermes:** 用 subagent-driven-development 按任务逐条执行。每条任务 TDD + 频繁 commit。

**Goal:** 补全 3 个真实 admin UI 缺口（governance/evaluations、control-plane/compensations、long-context 健康/卡住任务/手动触发），并给 Observability 加 cache / prefix-families 两张只读卡片。改纯前端，后端不动。

**Architecture:** 沿用既有 AppShell + useQuery + i18n + lib/xxx.ts 模式。新增 3 个 page + 3 个 lib + 3 个 type 文件，复用现有的 SummaryMetricCard / event-table 样式类。

**Tech Stack:** React 18 + Vite + TanStack Query + react-i18next + vitest + @testing-library。

**后端真实响应结构（已核对源码）：**
- `GET /admin/governance/evaluations` → `{object:"list", data:[{id,dataset_id,formula_id,agent_id,task_type,environment,status,started_at,completed_at,created_at}]}`
- `POST /admin/governance/evaluations` body `{dataset_id,agent_id,task_type,environment}` → 201 run 对象
- `GET /admin/control-plane/compensations?tenant_id=&environment=&failed_stage=&module=&limit=` → `{object:"list", data:[CompensationRecord], summary:{total,filtered_total,returned,filters}}`
  - `CompensationRecord{module,tenant_id,environment,version,failed_stage,error_summary,suggested_action,created_at}`
- `POST /admin/control-plane/compensations/replay` body `{tenant_id,environment,version,module}` → 200
- `GET /admin/long-context/health` → `{status:"healthy"|"unhealthy", db:"ok"|err}`
- `GET /admin/long-context/tasks/stuck` → `{tasks:[{id,tenant_id,status,phase,completed_chunks,chunk_count,pending_chunks,failed_chunks,lease_owner,lease_until,last_error,updated_at,minutes_since_update}], count}`
- `POST /admin/long-context/run` body `{task_id,tenant_id,channel,model,max_chunks,reader_model,reader_channel,reader_top_k}` → 200 `{processed,successes,...}`
- `GET /admin/observability/cache` → 后端 `adminObservabilityCache`（结构见 Task 9 读取）
- `GET /admin/observability/prefix-families` → 后端 `adminObservabilityPrefixFamilies`（结构见 Task 9 读取）

**约定：** 所有新 fetch 走 `apiRequest`（lib/http.ts 已注入 Bearer）。测试用 vitest + mocking fetch（参考 PolicyVersionsPage.test.tsx）。`npm run test` = `vitest run`，`npm run build` = `tsc -b && vite build`。

---

## Task 1: 新增 governanceEvals lib + type

**Objective:** 封装 evaluations 的 GET 列表与 POST 创建。

**Files:**
- Create: `web/admin/src/lib/governanceEvals.ts`
- Create: `web/admin/src/types/governanceEval.ts`

**Step 1: 写 type**
`web/admin/src/types/governanceEval.ts`:
```ts
export type EvaluationRunStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'

export interface EvaluationRun {
  id: string
  dataset_id: string
  formula_id: string
  agent_id: string
  task_type: string
  environment: string
  status: EvaluationRunStatus
  started_at: string
  completed_at: string
  created_at: string
}

export interface ListEvaluationRunsResponse {
  object: 'list'
  data: EvaluationRun[]
}

export interface StartEvaluationRunInput {
  dataset_id: string
  agent_id: string
  task_type: string
  environment: string
}
```

**Step 2: 写 lib**
`web/admin/src/lib/governanceEvals.ts`:
```ts
import { apiRequest, jsonRequest } from './http'
import type { ListEvaluationRunsResponse, StartEvaluationRunInput } from '../types/governanceEval'

export function listEvaluationRuns(limit = 50) {
  return apiRequest<ListEvaluationRunsResponse>(`/admin/governance/evaluations?limit=${limit}`)
}

export function startEvaluationRun(input: StartEvaluationRunInput) {
  return jsonRequest<unknown>('/admin/governance/evaluations', input, { method: 'POST' })
}
```

**Step 3: 类型检查**
Run: `cd web/admin && npx tsc -b --noEmit`
Expected: 无报错。

**Step 4: Commit**
```bash
git add web/admin/src/lib/governanceEvals.ts web/admin/src/types/governanceEval.ts
git commit -m "feat(admin): add governanceEvals lib + types"
```

---

## Task 2: EvaluationsPage + 测试

**Objective:** 列表展示 evaluation runs，带创建表单与状态 badge。

**Files:**
- Create: `web/admin/src/pages/EvaluationsPage.tsx`
- Create: `web/admin/src/pages/EvaluationsPage.test.tsx`

**Step 1: 写页面** 复用 AppShell + useQuery + useMutation：
```tsx
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { AppShell } from '../components/layout/AppShell'
import { listEvaluationRuns, startEvaluationRun } from '../lib/governanceEvals'
import type { EvaluationRun } from '../types/governanceEval'

function StatusBadge({ status }: { status: string }) {
  return <span className={`status-badge status-${status}`}>{status}</span>
}

export function EvaluationsPage() {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [datasetID, setDatasetID] = useState('')
  const [agentID, setAgentID] = useState('')
  const [taskType, setTaskType] = useState('chat')
  const [environment, setEnvironment] = useState('prod')
  const [error, setError] = useState('')

  const query = useQuery({
    queryKey: ['evaluation-runs'],
    queryFn: () => listEvaluationRuns(50),
  })

  const mutation = useMutation({
    mutationFn: () => startEvaluationRun({ dataset_id: datasetID, agent_id: agentID, task_type: taskType, environment }),
    onSuccess: () => {
      setError('')
      qc.invalidateQueries({ queryKey: ['evaluation-runs'] })
    },
    onError: (err: Error) => setError(err.message),
  })

  const runs = query.data?.data ?? []
  return (
    <AppShell title={t('evaluations.title')} description={t('evaluations.description')}>
      <form className="config-drawer" onSubmit={(e) => { e.preventDefault(); mutation.mutate() }}>
        <input placeholder="dataset_id" value={datasetID} onChange={(e) => setDatasetID(e.target.value)} />
        <input placeholder="agent_id" value={agentID} onChange={(e) => setAgentID(e.target.value)} />
        <input placeholder="task_type" value={taskType} onChange={(e) => setTaskType(e.target.value)} />
        <input placeholder="environment" value={environment} onChange={(e) => setEnvironment(e.target.value)} />
        <button type="submit" disabled={mutation.isPending}>{t('common.create')}</button>
      </form>
      {error && <div className="config-drawer__error">{error}</div>}
      <div className="event-table">
        <table>
          <thead><tr><th>ID</th><th>Dataset</th><th>Agent</th><th>Task</th><th>Env</th><th>Status</th><th>Created</th></tr></thead>
          <tbody>
            {runs.map((r: EvaluationRun) => (
              <tr key={r.id}>
                <td>{r.id}</td><td>{r.dataset_id}</td><td>{r.agent_id}</td>
                <td>{r.task_type}</td><td>{r.environment}</td>
                <td><StatusBadge status={r.status} /></td><td>{r.created_at}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </AppShell>
  )
}
```

**Step 2: 写测试** 仿 PolicyVersionsPage.test.tsx mock fetch：
```tsx
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { setToken, clearToken } from '../lib/auth'
import { EvaluationsPage } from './EvaluationsPage'

describe('EvaluationsPage', () => {
  beforeEach(() => setToken('demo-admin-token'))
  afterEach(() => { clearToken(); vi.restoreAllMocks() })

  it('renders evaluation runs', async () => {
    const fetchMock = vi.fn().mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/user/broadcasts')) {
        return new Response(JSON.stringify({ object: 'list', data: [], read_ids: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } })
      }
      if (url.includes('/admin/governance/evaluations')) {
        return new Response(JSON.stringify({ object: 'list', data: [{ id: 'run-1', dataset_id: 'd1', formula_id: 'f1', agent_id: 'a1', task_type: 'chat', environment: 'prod', status: 'succeeded', started_at: '', completed_at: '', created_at: '2026-04-19T10:00:00Z' }] }), { status: 200, headers: { 'Content-Type': 'application/json' } })
      }
      return new Response('{}', { status: 200, headers: { 'Content-Type': 'application/json' } })
    })
    vi.stubGlobal('fetch', fetchMock)
    render(<QueryClientProvider client={new QueryClient()}><MemoryRouter><EvaluationsPage /></MemoryRouter></QueryClientProvider>)
    await waitFor(() => expect(screen.getByText('run-1')).toBeTruthy())
    expect(screen.getByText('succeeded')).toBeTruthy()
  })
})
```

**Step 3: 跑测试**
Run: `cd web/admin && npx vitest run src/pages/EvaluationsPage.test.tsx`
Expected: 1 passed

**Step 4: Commit**
```bash
git add web/admin/src/pages/EvaluationsPage.tsx web/admin/src/pages/EvaluationsPage.test.tsx
git commit -m "feat(admin): add EvaluationsPage"
```

---

## Task 3: 注册 EvaluationsPage 路由 + Sidebar

**Objective:** 把 /evaluations 接入 router 与侧栏"策略"分组。

**Files:**
- Modify: `web/admin/src/router.tsx` (顶部 import + children path)
- Modify: `web/admin/src/components/layout/Sidebar.tsx` (策略分组 children 加一项)

**Step 1: router.tsx** 在 `const PolicyVersionsPage = ...` 附近加：
```ts
const EvaluationsPage = lazy(() => import('./pages/EvaluationsPage').then(m => ({ default: m.EvaluationsPage })))
```
在 children 数组 `path: 'policy-versions'` 行下方加：
```ts
{ path: 'evaluations', element: <EvaluationsPage /> },
```

**Step 2: Sidebar.tsx** 在 `label: '灰度发布'` 项后加：
```ts
{ label: '评估运行', path: '/evaluations', icon: '🧪' },
```

**Step 3: 类型检查 + 测试**
Run: `cd web/admin && npx tsc -b --noEmit && npx vitest run src/pages/EvaluationsPage.test.tsx`
Expected: 无报错，1 passed。

**Step 4: Commit**
```bash
git add web/admin/src/router.tsx web/admin/src/components/layout/Sidebar.tsx
git commit -m "feat(admin): register EvaluationsPage route + nav"
```

---

## Task 4: compensations lib + type

**Objective:** 封装补偿记录 GET 与 replay POST。

**Files:**
- Create: `web/admin/src/lib/compensations.ts`
- Create: `web/admin/src/types/compensation.ts`

**Step 1: type** `web/admin/src/types/compensation.ts`:
```ts
export interface CompensationRecord {
  module: string
  tenant_id: string
  environment: string
  version: string
  failed_stage: string
  error_summary: string
  suggested_action: string
  created_at: string
}

export interface ListCompensationsResponse {
  object: 'list'
  data: CompensationRecord[]
  summary: {
    total: number
    filtered_total: number
    returned: number
    filters: Record<string, string>
  }
}

export interface ReplayCompensationInput {
  tenant_id: string
  environment: string
  version: string
  module: string
}
```

**Step 2: lib** `web/admin/src/lib/compensations.ts`:
```ts
import { apiRequest, jsonRequest } from './http'
import type { ListCompensationsResponse, ReplayCompensationInput } from '../types/compensation'

export interface CompensationFilters {
  tenantID?: string
  environment?: string
  failedStage?: string
  module?: string
  limit?: number
}

export function listCompensations(filters: CompensationFilters = {}) {
  const params = new URLSearchParams()
  if (filters.tenantID) params.set('tenant_id', filters.tenantID)
  if (filters.environment) params.set('environment', filters.environment)
  if (filters.failedStage) params.set('failed_stage', filters.failedStage)
  if (filters.module) params.set('module', filters.module)
  if (filters.limit) params.set('limit', String(filters.limit))
  const qs = params.toString()
  return apiRequest<ListCompensationsResponse>(`/admin/control-plane/compensations${qs ? `?${qs}` : ''}`)
}

export function replayCompensation(input: ReplayCompensationInput) {
  return jsonRequest<unknown>('/admin/control-plane/compensations/replay', input, { method: 'POST' })
}
```

**Step 3: 类型检查**
Run: `cd web/admin && npx tsc -b --noEmit`
Expected: 无报错。

**Step 4: Commit**
```bash
git add web/admin/src/lib/compensations.ts web/admin/src/types/compensation.ts
git commit -m "feat(admin): add compensations lib + types"
```

---

## Task 5: CompensationsPage + 测试

**Objective:** 列表展示补偿记录，带筛选与 replay（二次确认）。

**Files:**
- Create: `web/admin/src/pages/CompensationsPage.tsx`
- Create: `web/admin/src/pages/CompensationsPage.test.tsx`

**Step 1: 页面**
```tsx
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { AppShell } from '../components/layout/AppShell'
import { listCompensations, replayCompensation } from '../lib/compensations'
import type { CompensationRecord } from '../types/compensation'

export function CompensationsPage() {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [tenantID, setTenantID] = useState('')
  const [environment, setEnvironment] = useState('')
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')

  const query = useQuery({
    queryKey: ['compensations', tenantID, environment],
    queryFn: () => listCompensations({ tenantID, environment, limit: 50 }),
  })

  const replay = useMutation({
    mutationFn: (rec: CompensationRecord) => replayCompensation({ tenant_id: rec.tenant_id, environment: rec.environment, version: rec.version, module: rec.module }),
    onSuccess: () => { setError(''); setMessage(t('compensations.replayOk')); qc.invalidateQueries({ queryKey: ['compensations'] }) },
    onError: (err: Error) => { setMessage(''); setError(err.message) },
  })

  const records = query.data?.data ?? []
  return (
    <AppShell title={t('compensations.title')} description={t('compensations.description')}>
      <div className="releases-grid">
        <input placeholder="tenant_id" value={tenantID} onChange={(e) => setTenantID(e.target.value)} />
        <input placeholder="environment" value={environment} onChange={(e) => setEnvironment(e.target.value)} />
      </div>
      {error && <div className="config-drawer__error">{error}</div>}
      {message && <div className="config-drawer__empty">{message}</div>}
      <div className="event-table">
        <table>
          <thead><tr><th>Module</th><th>Tenant</th><th>Env</th><th>Version</th><th>Failed Stage</th><th>Error</th><th>Action</th><th></th></tr></thead>
          <tbody>
            {records.map((r, i) => (
              <tr key={`${r.tenant_id}-${r.version}-${i}`}>
                <td>{r.module}</td><td>{r.tenant_id}</td><td>{r.environment}</td>
                <td>{r.version}</td><td>{r.failed_stage}</td><td>{r.error_summary}</td>
                <td>{r.suggested_action}</td>
                <td><button onClick={() => { if (confirm(t('compensations.confirmReplay'))) replay.mutate(r) }}>{t('compensations.replay')}</button></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </AppShell>
  )
}
```

**Step 2: 测试** 仿 Task 2，mock fetch 返回 `{object:'list', data:[{module:'reload',tenant_id:'t1',environment:'prod',version:'v1',failed_stage:'reload_failed',error_summary:'x',suggested_action:'retry',created_at:''}], summary:{total:1,filtered_total:1,returned:1,filters:{}}}`。

**Step 3: 跑测试**
Run: `cd web/admin && npx vitest run src/pages/CompensationsPage.test.tsx`
Expected: 1 passed

**Step 4: Commit**
```bash
git add web/admin/src/pages/CompensationsPage.tsx web/admin/src/pages/CompensationsPage.test.tsx
git commit -m "feat(admin): add CompensationsPage"
```

---

## Task 6: 注册 CompensationsPage 路由 + Sidebar

**Objective:** 接入 /compensations，放入"运营中心"分组。

**Files:**
- Modify: `web/admin/src/router.tsx`
- Modify: `web/admin/src/components/layout/Sidebar.tsx`

**Step 1: router.tsx** 加 import + children：
```ts
const CompensationsPage = lazy(() => import('./pages/CompensationsPage').then(m => ({ default: m.CompensationsPage })))
```
children 加 `{ path: 'compensations', element: <CompensationsPage /> }`。

**Step 2: Sidebar.tsx** 在"系统"分组前插入新分组"运营中心"：
```ts
{
  label: '运营中心',
  children: [
    { label: '补偿记录', path: '/compensations', icon: '🛠️' },
    { label: '运行时观测', path: '/runtime-observer', icon: '👁️' },
  ],
},
```
（runtime-observer 已有，仅归组；也可保留原监控分组，本任务只加补偿一项即可，分组可后续微调。）

**Step 3: 类型检查 + 测试**
Run: `cd web/admin && npx tsc -b --noEmit && npx vitest run src/pages/CompensationsPage.test.tsx`
Expected: 无报错。

**Step 4: Commit**
```bash
git add web/admin/src/router.tsx web/admin/src/components/layout/Sidebar.tsx
git commit -m "feat(admin): register CompensationsPage route + nav"
```

---

## Task 7: longContext lib + type

**Objective:** 封装 health / stuck tasks / run 三接口。

**Files:**
- Create: `web/admin/src/lib/longContext.ts`
- Create: `web/admin/src/types/longContext.ts`

**Step 1: type** `web/admin/src/types/longContext.ts`:
```ts
export interface LongContextHealth {
  status: 'healthy' | 'unhealthy'
  db: string
}

export interface StuckTask {
  id: string
  tenant_id: string
  status: string
  phase: string
  completed_chunks: number
  chunk_count: number
  pending_chunks: number
  failed_chunks: number
  lease_owner: string
  lease_until: string
  last_error: string
  updated_at: string
  minutes_since_update: number
}

export interface StuckTasksResponse {
  tasks: StuckTask[]
  count: number
}

export interface RunLongContextInput {
  task_id: string
  tenant_id: string
  channel: string
  model: string
  max_chunks?: number
  reader_model?: string
  reader_channel?: string
  reader_top_k?: number
}

export interface RunLongContextResponse {
  processed: number
  successes: number
  [key: string]: unknown
}
```

**Step 2: lib** `web/admin/src/lib/longContext.ts`:
```ts
import { apiRequest, jsonRequest } from './http'
import type { LongContextHealth, RunLongContextInput, RunLongContextResponse, StuckTasksResponse } from '../types/longContext'

export function getLongContextHealth() {
  return apiRequest<LongContextHealth>('/admin/long-context/health')
}

export function listStuckTasks() {
  return apiRequest<StuckTasksResponse>('/admin/long-context/tasks/stuck')
}

export function runLongContext(input: RunLongContextInput) {
  return jsonRequest<RunLongContextResponse>('/admin/long-context/run', input, { method: 'POST' })
}
```

**Step 3: 类型检查**
Run: `cd web/admin && npx tsc -b --noEmit`
Expected: 无报错。

**Step 4: Commit**
```bash
git add web/admin/src/lib/longContext.ts web/admin/src/types/longContext.ts
git commit -m "feat(admin): add longContext lib + types"
```

---

## Task 8: LongContextPage + 测试

**Objective:** health 卡 + stuck 任务表 + 手动 run 表单（危险操作 confirm）。

**Files:**
- Create: `web/admin/src/pages/LongContextPage.tsx`
- Create: `web/admin/src/pages/LongContextPage.test.tsx`

**Step 1: 页面**
```tsx
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { AppShell } from '../components/layout/AppShell'
import { getLongContextHealth, listStuckTasks, runLongContext } from '../lib/longContext'
import type { StuckTask } from '../types/longContext'

export function LongContextPage() {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [taskID, setTaskID] = useState('')
  const [tenantID, setTenantID] = useState('')
  const [channel, setChannel] = useState('')
  const [model, setModel] = useState('')
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')

  const health = useQuery({ queryKey: ['lc-health'], queryFn: getLongContextHealth, refetchInterval: 30000 })
  const stuck = useQuery({ queryKey: ['lc-stuck'], queryFn: listStuckTasks, refetchInterval: 15000 })

  const run = useMutation({
    mutationFn: () => runLongContext({ task_id: taskID, tenant_id: tenantID, channel, model }),
    onSuccess: (res) => { setError(''); setMessage(`${t('longContext.runOk')} processed=${res.processed} successes=${res.successes}`); qc.invalidateQueries({ queryKey: ['lc-stuck'] }) },
    onError: (err: Error) => { setMessage(''); setError(err.message) },
  })

  const tasks: StuckTask[] = stuck.data?.tasks ?? []
  return (
    <AppShell title={t('longContext.title')} description={t('longContext.description')}>
      <div className="summary-card-grid">
        <div className="summary-metric-card">
          <span className="summary-metric-card__label">{t('longContext.health')}</span>
          <span className={`status-badge status-${(health.data?.status ?? 'unknown')}`}>{health.data?.status ?? '—'}</span>
        </div>
        <div className="summary-metric-card">
          <span className="summary-metric-card__label">{t('longContext.stuckCount')}</span>
          <span className="summary-metric-card__value">{stuck.data?.count ?? 0}</span>
        </div>
      </div>

      <form className="config-drawer" onSubmit={(e) => { e.preventDefault(); if (confirm(t('longContext.confirmRun'))) run.mutate() }}>
        <input placeholder="task_id" value={taskID} onChange={(e) => setTaskID(e.target.value)} />
        <input placeholder="tenant_id" value={tenantID} onChange={(e) => setTenantID(e.target.value)} />
        <input placeholder="channel" value={channel} onChange={(e) => setChannel(e.target.value)} />
        <input placeholder="model" value={model} onChange={(e) => setModel(e.target.value)} />
        <button type="submit" disabled={run.isPending}>{t('longContext.run')}</button>
      </form>
      {error && <div className="config-drawer__error">{error}</div>}
      {message && <div className="config-drawer__empty">{message}</div>}

      <div className="event-table">
        <table>
          <thead><tr><th>ID</th><th>Tenant</th><th>Status</th><th>Phase</th><th>Done/Total</th><th>Pending</th><th>Failed</th><th>Minutes Idle</th><th>Last Error</th></tr></thead>
          <tbody>
            {tasks.map((tk) => (
              <tr key={tk.id}>
                <td>{tk.id}</td><td>{tk.tenant_id}</td><td>{tk.status}</td><td>{tk.phase}</td>
                <td>{tk.completed_chunks}/{tk.chunk_count}</td><td>{tk.pending_chunks}</td>
                <td>{tk.failed_chunks}</td><td>{tk.minutes_since_update}</td><td>{tk.last_error}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </AppShell>
  )
}
```

**Step 2: 测试** mock fetch：health `{status:'healthy',db:'ok'}`，stuck `{tasks:[{id:'t1',tenant_id:'x',status:'mapping',phase:'map',completed_chunks:1,chunk_count:4,pending_chunks:2,failed_chunks:0,lease_owner:'w',lease_until:'',last_error:'',updated_at:'',minutes_since_update:5}],count:1}`。断言显示 t1 与 healthy。

**Step 3: 跑测试**
Run: `cd web/admin && npx vitest run src/pages/LongContextPage.test.tsx`
Expected: 1 passed

**Step 4: Commit**
```bash
git add web/admin/src/pages/LongContextPage.tsx web/admin/src/pages/LongContextPage.test.tsx
git commit -m "feat(admin): add LongContextPage"
```

---

## Task 9: 注册 LongContextPage + Observability cache/prefix-families 卡片

**Objective:** 接入 /long-context 路由 + nav，并给 ObservabilityPage 加两张只读卡片。

**Files:**
- Modify: `web/admin/src/router.tsx`
- Modify: `web/admin/src/components/layout/Sidebar.tsx`
- Modify: `web/admin/src/pages/ObservabilityPage.tsx`
- Create: `web/admin/src/components/observability/ObservabilityCacheSection.tsx`
- Create: `web/admin/src/components/observability/ObservabilityPrefixFamiliesSection.tsx`
- Modify: `web/admin/src/types/observability.ts` (加 Cache/PrefixFamily 类型)
- Modify: `web/admin/src/lib/api/dashboard.ts` (加 fetch 封装，或直接在 page 用 apiRequest)

**Step 1: 读后端真实响应**
Run:
```bash
cd /home/jimwong/projects/llm-gateway && rg -n "func (s \*Server) adminObservabilityCache" internal/httpserver/server.go
rg -n "func (s \*Server) adminObservabilityPrefixFamilies" internal/httpserver/server.go
```
Expected: 找到函数名 → 读其返回结构（在 server.go 内 `sed -n '<start>,+40p'`），据此定义类型（常见为 `{cache_entries:[{key,hits,misses,ttl}], ...}` 与 `{families:[{prefix,count}]}`——以实际源码为准，不要臆测）。

**Step 2: 加类型** `web/admin/src/types/observability.ts` 末尾追加（字段按 Step 1 实测）：
```ts
export interface CacheEntry { key: string; hits: number; misses: number; ttl_seconds: number }
export interface ObservabilityCacheResponse { entries: CacheEntry[] }
export interface PrefixFamily { prefix: string; count: number }
export interface ObservabilityPrefixFamiliesResponse { families: PrefixFamily[] }
```

**Step 3: 加 section 组件** 仿 ObservabilityProvidersSection.tsx，纯表格渲染 props。

**Step 4: ObservabilityPage.tsx** 加两个 useQuery + 渲染 section（参照现有 summary/providers/hotspots 写法）。

**Step 5: router + sidebar** 同 Task 3/6 模式加 `/long-context` 与"运营中心"分组项。

**Step 6: 类型检查 + 全量测试**
Run: `cd web/admin && npx tsc -b --noEmit && npx vitest run`
Expected: 无报错，全部测试通过。

**Step 7: Commit**
```bash
git add web/admin/src/router.tsx web/admin/src/components/layout/Sidebar.tsx web/admin/src/pages/ObservabilityPage.tsx web/admin/src/components/observability/ObservabilityCacheSection.tsx web/admin/src/components/observability/ObservabilityPrefixFamiliesSection.tsx web/admin/src/types/observability.ts
git commit -m "feat(admin): register LongContextPage + observability cache/prefix cards"
```

---

## Task 10: 全量构建 + i18n key 补充

**Objective:** 保证 `npm run build` 通过，补 zh-CN/en-US 缺失 key。

**Files:**
- Modify: `web/admin/src/i18n/locales/zh-CN.json`
- Modify: `web/admin/src/i18n/locales/en-US.json`

**Step 1: 加 key** 在 zh-CN.json 的 `nav` 区与新增顶层区加：
```json
"nav.evaluations": "评估运行",
"nav.compensations": "补偿记录",
"nav.longContext": "长上下文",
"evaluations.title": "评估运行",
"evaluations.description": "模型治理评估任务列表与创建",
"compensations.title": "补偿记录",
"compensations.description": "跨环境补偿与回放",
"compensations.replay": "回放",
"compensations.replayOk": "回放已触发",
"compensations.confirmReplay": "确认回放该补偿？",
"longContext.title": "长上下文任务",
"longContext.description": "1Mvir 虚拟长上下文任务健康与卡住恢复",
"longContext.health": "健康",
"longContext.stuckCount": "卡住任务",
"longContext.run": "手动推进",
"longContext.runOk": "推进完成",
"longContext.confirmRun": "确认手动推进该任务？此操作会调用真实 Provider。",
"observability.cache": "缓存条目",
"observability.prefixFamilies": "前缀族分布"
```
en-US.json 对应英文化。

**Step 2: 构建**
Run: `cd web/admin && npm run build`
Expected: `tsc -b` 无类型错误，`vite build` 产出 dist/。

**Step 3: 全量测试**
Run: `cd web/admin && npx vitest run`
Expected: 全部通过。

**Step 4: Commit**
```bash
git add web/admin/src/i18n/locales/zh-CN.json web/admin/src/i18n/locales/en-US.json
git commit -m "feat(admin): i18n keys for new pages"
```

---

## Task 11: 真实验证（端到端）

**Objective:** 在 17 号机真实网关上验证新页面调用的 API 真实返回（不止 HTTP 200）。

**Step 1:** 起前端 dev 或 build 后部署；用浏览器/代理访问 `http://10.100.1.17:8085/admin/ui/evaluations` 等，确认：
- evaluations 列表真实渲染数据
- compensations 列表 + replay 触发真实补偿（核对后端日志）
- long-context health=healthy、stuck 任务可见、run 真实推进（核对 1Mvir task 状态变化）
- observability 两张新卡片有数据

**Step 2:** 若任一接口返回结构与计划假设不符，回到对应 Task 修正 type/page 并重新 commit。

**验收标准：**
- `npm run build` 通过
- `npx vitest run` 全绿
- 3 个新页面在真实网关上数据可见、操作生效（replay/run 有后端副作用验证）
- Sidebar 分组清晰，无破坏现有路由
