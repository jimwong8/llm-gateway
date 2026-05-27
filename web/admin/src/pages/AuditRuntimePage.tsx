import { FormEvent, useState } from 'react'
import { useTranslation } from 'react-i18next'
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
  const { t } = useTranslation()
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
      title={t('auditRuntime.title')}
      description={t('auditRuntime.description')}
    >
      <div className="events-page">
        <form className="config-filters" aria-label={t('auditRuntime.filtersLabel')} onSubmit={handleSubmit}>
          <label>
            {t('auditRuntime.tenantId')}
            <input value={draftFilters.tenantID} onChange={(event) => setDraftFilters((prev) => ({ ...prev, tenantID: event.target.value }))} placeholder={t('auditRuntime.tenantIdPlaceholder')} />
          </label>
          <label>
            {t('auditRuntime.environment')}
            <input value={draftFilters.environment} onChange={(event) => setDraftFilters((prev) => ({ ...prev, environment: event.target.value }))} placeholder={t('auditRuntime.environmentPlaceholder')} />
          </label>
          <label>
            {t('auditRuntime.limit')}
            <input value={draftFilters.limit} onChange={(event) => setDraftFilters((prev) => ({ ...prev, limit: event.target.value }))} placeholder={t('auditRuntime.limitPlaceholder')} />
          </label>
          <label className="toggle-field">
            <span>{t('auditRuntime.summaryView')}</span>
            <input type="checkbox" checked={draftFilters.summary} onChange={(event) => setDraftFilters((prev) => ({ ...prev, summary: event.target.checked }))} />
          </label>
          <div className="config-filters__actions">
            <button type="submit">{t('common.filter')}</button>
          </div>
        </form>

        <div className="tab-strip" role="tablist" aria-label={t('auditRuntime.tabLabel')}>
          <button type="button" role="tab" aria-selected={activeTab === 'audit'} className={activeTab === 'audit' ? 'tab active' : 'tab'} onClick={() => setActiveTab('audit')}>
            {t('auditRuntime.auditEvents')}
          </button>
          <button type="button" role="tab" aria-selected={activeTab === 'runtime'} className={activeTab === 'runtime' ? 'tab active' : 'tab'} onClick={() => setActiveTab('runtime')}>
            {t('auditRuntime.runtimeEvents')}
          </button>
        </div>

        {currentQuery.error ? <div className="config-error">{t('auditRuntime.queryError')}</div> : null}

        {activeTab === 'audit' ? (
          <AuditTable data={auditQuery.data} loading={auditQuery.isLoading} />
        ) : (
          <RuntimeTable data={runtimeQuery.data} loading={runtimeQuery.isLoading} />
        )}
      </div>
    </AppShell>
  )
}
