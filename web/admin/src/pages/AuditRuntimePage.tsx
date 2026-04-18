import { FormEvent, useState } from 'react'
import { AppShell } from '../components/layout/AppShell'
import { AuditTable } from '../components/events/AuditTable'
import { RuntimeTable } from '../components/events/RuntimeTable'
import { useAuditEvents, useRuntimeEvents } from '../hooks/useAdminEvents'

type Tab = 'audit' | 'runtime'

type FilterState = {
  tenantID: string
  environment: string
  limit: string
  summary: boolean
}

const initialFilters: FilterState = {
  tenantID: '',
  environment: '',
  limit: '',
  summary: false,
}

export function AuditRuntimePage() {
  const [activeTab, setActiveTab] = useState<Tab>('audit')
  const [draftFilters, setDraftFilters] = useState<FilterState>(initialFilters)
  const [filters, setFilters] = useState<FilterState>(initialFilters)

  const auditQuery = useAuditEvents(filters)
  const runtimeQuery = useRuntimeEvents(filters)

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setFilters({ ...draftFilters })
  }

  const currentQuery = activeTab === 'audit' ? auditQuery : runtimeQuery

  return (
    <AppShell
      title="Audit & Runtime"
      description="在一个页面中查看控制面审计记录与运行时发布事件，支持 summary 视图和基础筛选。"
    >
      <div className="events-page">
        <form className="config-filters" aria-label="Event Filters" onSubmit={handleSubmit}>
          <label>
            Tenant ID
            <input value={draftFilters.tenantID} onChange={(event) => setDraftFilters((prev) => ({ ...prev, tenantID: event.target.value }))} placeholder="tenant-a" />
          </label>
          <label>
            Environment
            <input value={draftFilters.environment} onChange={(event) => setDraftFilters((prev) => ({ ...prev, environment: event.target.value }))} placeholder="prod" />
          </label>
          <label>
            Limit
            <input value={draftFilters.limit} onChange={(event) => setDraftFilters((prev) => ({ ...prev, limit: event.target.value }))} placeholder="20" />
          </label>
          <label className="toggle-field">
            <span>Summary</span>
            <input type="checkbox" checked={draftFilters.summary} onChange={(event) => setDraftFilters((prev) => ({ ...prev, summary: event.target.checked }))} />
          </label>
          <div className="config-filters__actions">
            <button type="submit">应用筛选</button>
          </div>
        </form>

        <div className="tab-strip" role="tablist" aria-label="Audit runtime tabs">
          <button type="button" role="tab" aria-selected={activeTab === 'audit'} className={activeTab === 'audit' ? 'tab active' : 'tab'} onClick={() => setActiveTab('audit')}>
            Audit Events
          </button>
          <button type="button" role="tab" aria-selected={activeTab === 'runtime'} className={activeTab === 'runtime' ? 'tab active' : 'tab'} onClick={() => setActiveTab('runtime')}>
            Runtime Events
          </button>
        </div>

        {currentQuery.error ? <div className="config-error">事件查询失败，请检查 Admin Token 与接口状态。</div> : null}

        {activeTab === 'audit' ? (
          <AuditTable data={auditQuery.data} loading={auditQuery.isLoading} />
        ) : (
          <RuntimeTable data={runtimeQuery.data} loading={runtimeQuery.isLoading} />
        )}
      </div>
    </AppShell>
  )
}
