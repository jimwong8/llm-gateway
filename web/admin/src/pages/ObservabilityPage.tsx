import { FormEvent, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { apiRequest } from '../lib/http'
import type {
  BillingSummary,
  HotspotsResult,
  ListResponse,
  ProviderBreakdownRow,
} from '../types/observability'

type FilterState = {
  tenantID: string
  provider: string
  model: string
  limit: string
}

const initialFilters: FilterState = {
  tenantID: '',
  provider: '',
  model: '',
  limit: '',
}

function buildQuery(path: string, filters: FilterState) {
  const params = new URLSearchParams()
  if (filters.tenantID.trim()) params.set('tenant_id', filters.tenantID.trim())
  if (filters.provider.trim()) params.set('provider', filters.provider.trim())
  if (filters.model.trim()) params.set('model', filters.model.trim())
  if (filters.limit.trim()) params.set('limit', filters.limit.trim())
  const query = params.toString()
  return query ? `${path}?${query}` : path
}

export function ObservabilityPage() {
  const [draftFilters, setDraftFilters] = useState(initialFilters)
  const [filters, setFilters] = useState(initialFilters)

  const summaryQuery = useQuery({
    queryKey: ['observability-summary', filters],
    queryFn: () => apiRequest<BillingSummary>(buildQuery('/admin/observability/summary', filters)),
  })

  const providersQuery = useQuery({
    queryKey: ['observability-providers', filters],
    queryFn: () =>
      apiRequest<ListResponse<ProviderBreakdownRow>>(buildQuery('/admin/observability/providers', filters)),
  })

  const hotspotsQuery = useQuery({
    queryKey: ['observability-hotspots', filters],
    queryFn: () => apiRequest<HotspotsResult>(buildQuery('/admin/observability/hotspots', filters)),
  })

  const providers = useMemo(() => providersQuery.data?.data ?? [], [providersQuery.data])
  const hotspots = hotspotsQuery.data

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setFilters({ ...draftFilters })
  }

  return (
    <AppShell
      title="Observability"
      description="展示请求摘要、Provider 统计与热点租户/模型，用于快速判断网关流量质量。"
    >
      <div className="events-page">
        <form className="config-filters" aria-label="Observability Filters" onSubmit={handleSubmit}>
          <label>
            Tenant ID
            <input value={draftFilters.tenantID} onChange={(event) => setDraftFilters((prev) => ({ ...prev, tenantID: event.target.value }))} placeholder="tenant-a" />
          </label>
          <label>
            Provider
            <input value={draftFilters.provider} onChange={(event) => setDraftFilters((prev) => ({ ...prev, provider: event.target.value }))} placeholder="openai" />
          </label>
          <label>
            Model
            <input value={draftFilters.model} onChange={(event) => setDraftFilters((prev) => ({ ...prev, model: event.target.value }))} placeholder="gpt-4o-mini" />
          </label>
          <label>
            Limit
            <input value={draftFilters.limit} onChange={(event) => setDraftFilters((prev) => ({ ...prev, limit: event.target.value }))} placeholder="10" />
          </label>
          <div className="config-filters__actions">
            <button type="submit">应用筛选</button>
          </div>
        </form>

        {summaryQuery.error || providersQuery.error || hotspotsQuery.error ? (
          <div className="config-error">观测数据加载失败，请检查 Admin API 与 billing store 状态。</div>
        ) : null}

        <div className="summary-card-grid">
          <section className="summary-card">
            <span>Requests</span>
            <strong>{summaryQuery.data?.requests ?? 0}</strong>
          </section>
          <section className="summary-card">
            <span>Cache Hit Rate</span>
            <strong>{((summaryQuery.data?.cache_hit_rate ?? 0) * 100).toFixed(1)}%</strong>
          </section>
          <section className="summary-card">
            <span>Provider Error Rate</span>
            <strong>{((summaryQuery.data?.provider_error_rate ?? 0) * 100).toFixed(1)}%</strong>
          </section>
          <section className="summary-card">
            <span>Avg Latency</span>
            <strong>{(summaryQuery.data?.avg_latency_ms ?? 0).toFixed(1)} ms</strong>
          </section>
        </div>

        <div className="event-table">
          <table>
            <thead>
              <tr>
                <th>Provider</th>
                <th>Requests</th>
                <th>Total Tokens</th>
                <th>Error Rate</th>
              </tr>
            </thead>
            <tbody>
              {providers.map((row) => (
                <tr key={row.provider}>
                  <td>{row.provider}</td>
                  <td>{row.requests}</td>
                  <td>{row.total_tokens}</td>
                  <td>{(row.provider_error_rate * 100).toFixed(1)}%</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div className="summary-card-grid">
          <section className="summary-card">
            <span>Top Tenant</span>
            <strong>{hotspots?.tenants[0]?.key ?? '—'}</strong>
          </section>
          <section className="summary-card">
            <span>Top Model</span>
            <strong>{hotspots?.models[0]?.key ?? '—'}</strong>
          </section>
        </div>
      </div>
    </AppShell>
  )
}
