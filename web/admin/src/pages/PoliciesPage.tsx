import { useQuery } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
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
      title="Policies"
      description="查看当前允许的模型列表，为后续策略编辑功能保留最小可用入口。"
    >
      <div className="events-page">
        {query.isLoading ? <div className="event-state">正在加载策略模型…</div> : null}
        {query.error ? <div className="config-error">策略模型加载失败，请检查 policy store 状态。</div> : null}

        {!query.isLoading && !query.error ? (
          <div className="summary-card-grid">
            <section className="summary-card">
              <span>Tenant</span>
              <strong>{query.data?.tenant_id || '—'}</strong>
            </section>
            <section className="summary-card">
              <span>Allowed Models</span>
              <strong>{query.data?.models?.length ?? 0}</strong>
            </section>
          </div>
        ) : null}

        {!query.isLoading && !query.error ? (
          <div className="event-table">
            <table>
              <thead>
                <tr>
                  <th>Model</th>
                </tr>
              </thead>
              <tbody>
                {(query.data?.models ?? []).map((model) => (
                  <tr key={model}>
                    <td>{model}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </div>
    </AppShell>
  )
}
