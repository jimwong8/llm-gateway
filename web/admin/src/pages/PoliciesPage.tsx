import { useQuery } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { PoliciesSummarySection } from '../components/policies/PoliciesSummarySection'
import { PoliciesModelsSection } from '../components/policies/PoliciesModelsSection'
import { apiRequest } from '../lib/http'

type PoliciesResponse = {
  tenant_id: string
  models: string[]
}

export function PoliciesPage() {
  const query = useQuery({
    queryKey: ['policies-models'],
    queryFn: () => apiRequest<PoliciesResponse>('/admin/policies/models'),
  })

  return (
    <AppShell
      title="策略管理"
      description="查看当前允许的模型列表，为后续策略编辑功能保留最小可用入口。"
    >
      <div className="events-page">
        {query.isLoading ? <div className="event-state">正在加载策略模型…</div> : null}
        {query.error ? <div className="config-error">策略模型加载失败，请检查 policy store 状态。</div> : null}

        {!query.isLoading && !query.error ? (
          <PoliciesSummarySection
            tenantId={query.data?.tenant_id ?? '—'}
            modelCount={query.data?.models?.length ?? 0}
          />
        ) : null}

        {!query.isLoading && !query.error ? (
          <PoliciesModelsSection models={query.data?.models ?? []} />
        ) : null}
      </div>
    </AppShell>
  )
}
