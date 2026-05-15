import { FormEvent, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { QuotaSummarySection } from '../components/quota/QuotaSummarySection'
import { QuotaTrendsSection } from '../components/quota/QuotaTrendsSection'
import { apiRequest } from '../lib/http'
import type { QuotaSummary, QuotaTrendsResponse } from '../types/quota'

type FilterState = {
  tenantID: string
  windowMinutes: string
}

const initialFilters: FilterState = {
  tenantID: 'tenant-a',
  windowMinutes: '15',
}

function buildSummaryQuery(tenantID: string) {
  const params = new URLSearchParams()
  if (tenantID.trim()) params.set('tenant_id', tenantID.trim())
  const query = params.toString()
  return query ? `/admin/observability/quota?${query}` : '/admin/observability/quota'
}

function buildTrendsQuery(filters: FilterState) {
  const params = new URLSearchParams()
  if (filters.tenantID.trim()) params.set('tenant_id', filters.tenantID.trim())
  if (filters.windowMinutes.trim()) params.set('window_minutes', filters.windowMinutes.trim())
  const query = params.toString()
  return query ? `/admin/observability/quota/trends?${query}` : '/admin/observability/quota/trends'
}

export function QuotaPage() {
  const [draftFilters, setDraftFilters] = useState(initialFilters)
  const [filters, setFilters] = useState(initialFilters)

  const summaryQuery = useQuery({
    queryKey: ['quota-summary', filters.tenantID],
    queryFn: () => apiRequest<QuotaSummary>(buildSummaryQuery(filters.tenantID)),
  })

  const trendsQuery = useQuery({
    queryKey: ['quota-trends', filters],
    queryFn: () => apiRequest<QuotaTrendsResponse>(buildTrendsQuery(filters)),
  })

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setFilters({ ...draftFilters })
  }

  return (
    <AppShell
      title="Quota"
      description="查看指定 tenant 的 quota 摘要与时间窗口趋势，用于发现逼近限制与被拒绝请求。"
    >
      <div className="events-page">
        <form className="config-filters" aria-label="Quota Filters" onSubmit={handleSubmit}>
          <label>
            Tenant ID
            <input value={draftFilters.tenantID} onChange={(event) => setDraftFilters((prev) => ({ ...prev, tenantID: event.target.value }))} placeholder="tenant-a" />
          </label>
          <label>
            Window Minutes
            <input value={draftFilters.windowMinutes} onChange={(event) => setDraftFilters((prev) => ({ ...prev, windowMinutes: event.target.value }))} placeholder="15" />
          </label>
          <div className="config-filters__actions">
            <button type="submit">应用筛选</button>
          </div>
        </form>

        {summaryQuery.error || trendsQuery.error ? (
          <div className="config-error">Quota 数据加载失败，请检查 Redis / limiter 状态。</div>
        ) : null}

        <QuotaSummarySection
          tenantId={summaryQuery.data?.tenant_id ?? '—'}
          used={summaryQuery.data?.used ?? 0}
          remaining={summaryQuery.data?.remaining ?? 0}
          rejectRate={summaryQuery.data?.reject_rate ?? 0}
        />

        <QuotaTrendsSection trends={trendsQuery.data} />
      </div>
    </AppShell>
  )
}
