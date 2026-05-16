import { useQuery } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { apiRequest } from '../lib/http'

type SystemResponse = {
  service?: string
  time?: string
  admin_auth?: string
  object?: string
  data?: unknown[]
}

export function SystemPage() {
  const query = useQuery({
    queryKey: ['system'],
    queryFn: async () => {
      const results = await Promise.allSettled([
        apiRequest<SystemResponse>('/admin/health'),
        apiRequest<SystemResponse>('/admin/usage'),
        apiRequest<SystemResponse>('/admin/audit'),
      ])

      const [health, usage, audit] = results

      return {
        health: health.status === 'fulfilled' ? health.value : null,
        usage: usage.status === 'fulfilled' ? usage.value : null,
        audit: audit.status === 'fulfilled' ? audit.value : null,
      }
    },
  })

  return (
    <AppShell
      title="系统状态"
      description="检查管理服务健康状态、最近用量列表和最近审计记录。"
    >
      <div className="system-page">
        <form className="system-toolbar" onSubmit={(e) => { e.preventDefault(); query.refetch() }}>
          <button type="submit">{query.isLoading ? '加载中…' : '刷新'}</button>
        </form>

        {query.error ? <div className="config-error">{(query.error as Error).message}</div> : null}

        <div className="summary-card-grid">
          <section className="summary-card">
            <span>健康服务</span>
            <strong>{query.data?.health?.service ?? '—'}</strong>
          </section>
          <section className="summary-card">
            <span>管理员认证</span>
            <strong>{query.data?.health?.admin_auth ?? '—'}</strong>
          </section>
          <section className="summary-card">
            <span>用量条目数</span>
            <strong>{query.data?.usage?.data?.length ?? 0}</strong>
          </section>
          <section className="summary-card">
            <span>审计条目数</span>
            <strong>{query.data?.audit?.data?.length ?? 0}</strong>
          </section>
        </div>
      </div>
    </AppShell>
  )
}
