import { FormEvent, useState } from 'react'
import { AppShell } from '../components/layout/AppShell'
import { apiRequest } from '../lib/http'

type SystemResponse = {
  service?: string
  time?: string
  admin_auth?: string
  object?: string
  data?: unknown[]
}

type LoadedState = {
  health: SystemResponse | null
  usage: SystemResponse | null
  audit: SystemResponse | null
}

export function SystemPage() {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [loaded, setLoaded] = useState<LoadedState>({
    health: null,
    usage: null,
    audit: null,
  })

  async function handleLoad(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setLoading(true)
    setError('')

    try {
      const [health, usage, audit] = await Promise.all([
        apiRequest<SystemResponse>('/admin/health'),
        apiRequest<SystemResponse>('/admin/usage'),
        apiRequest<SystemResponse>('/admin/audit'),
      ])

      setLoaded({ health, usage, audit })
    } catch (unknownError) {
      const message = unknownError instanceof Error ? unknownError.message : '加载系统状态失败'
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <AppShell
      title="System"
      description="检查管理服务健康状态、最近 usage 列表和最近审计记录。"
    >
      <div className="system-page">
        <form className="system-toolbar" onSubmit={handleLoad}>
          <button type="submit">{loading ? '加载中…' : '刷新系统状态'}</button>
        </form>

        {error ? <div className="config-error">{error}</div> : null}

        <div className="summary-card-grid">
          <section className="summary-card">
            <span>Health Service</span>
            <strong>{loaded.health?.service ?? '—'}</strong>
          </section>
          <section className="summary-card">
            <span>Admin Auth</span>
            <strong>{loaded.health?.admin_auth ?? '—'}</strong>
          </section>
          <section className="summary-card">
            <span>Usage Items</span>
            <strong>{loaded.usage?.data?.length ?? 0}</strong>
          </section>
          <section className="summary-card">
            <span>Audit Items</span>
            <strong>{loaded.audit?.data?.length ?? 0}</strong>
          </section>
        </div>
      </div>
    </AppShell>
  )
}
