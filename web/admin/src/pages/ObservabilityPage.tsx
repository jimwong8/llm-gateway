import { FormEvent, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { ObservabilitySummarySection } from '../components/observability/ObservabilitySummarySection'
import { ObservabilityProvidersSection } from '../components/observability/ObservabilityProvidersSection'
import { ObservabilityHotspotsSection } from '../components/observability/ObservabilityHotspotsSection'
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
      title="可观测性"
      description="展示请求摘要、Provider 统计与热点租户/模型，用于快速判断网关流量质量。"
    >
      <div className="events-page">
        <form className="config-filters" aria-label="可观测性筛选" onSubmit={handleSubmit}>
          <label>
            租户 ID
            <input value={draftFilters.tenantID} onChange={(event) => setDraftFilters((prev) => ({ ...prev, tenantID: event.target.value }))} placeholder="租户-a" />
          </label>
          <label>
            Provider
            <input value={draftFilters.provider} onChange={(event) => setDraftFilters((prev) => ({ ...prev, provider: event.target.value }))} placeholder="openai" />
          </label>
          <label>
            模型
            <input value={draftFilters.model} onChange={(event) => setDraftFilters((prev) => ({ ...prev, model: event.target.value }))} placeholder="gpt-4o-mini" />
          </label>
          <label>
            条数限制
            <input value={draftFilters.limit} onChange={(event) => setDraftFilters((prev) => ({ ...prev, limit: event.target.value }))} placeholder="10" />
          </label>
          <div className="config-filters__actions">
            <button type="submit">筛选</button>
          </div>
        </form>

        {summaryQuery.error || providersQuery.error || hotspotsQuery.error ? (
          <div className="config-error">观测数据加载失败，请检查 Admin API 与计费存储状态。</div>
        ) : null}

        <ObservabilitySummarySection
          requests={summaryQuery.data?.requests ?? 0}
          cacheHitRate={summaryQuery.data?.cache_hit_rate ?? 0}
          providerErrorRate={summaryQuery.data?.provider_error_rate ?? 0}
          avgLatencyMs={summaryQuery.data?.avg_latency_ms ?? 0}
        />

        <ObservabilityProvidersSection providers={providers} />

        <ObservabilityHotspotsSection hotspots={hotspots} />
      </div>
    </AppShell>
  )
}
