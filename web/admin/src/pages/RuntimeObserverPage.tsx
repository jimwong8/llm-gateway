import { useMemo, useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { getRuntimeObserver } from '../lib/runtimeObserver'

function formatDate(value?: string) {
  if (!value) {
    return '—'
  }
  if (value.startsWith('0001-01-01')) {
    return '—'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString()
}

export function RuntimeObserverPage() {
  const { t } = useTranslation()
  const [draftEnvironment, setDraftEnvironment] = useState('prod')
  const [environment, setEnvironment] = useState('prod')

  const observerQuery = useQuery({
    queryKey: ['runtime-observer', environment],
    queryFn: () => getRuntimeObserver(environment, 20),
  })

  const runtimeFacts = useMemo(() => observerQuery.data?.facts.runtime_decisions ?? [], [observerQuery.data])
  const distributionFacts = useMemo(() => observerQuery.data?.facts.distribution_events ?? [], [observerQuery.data])
  const cacheEntries = useMemo(() => observerQuery.data?.cache.entries ?? [], [observerQuery.data])

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setEnvironment(draftEnvironment.trim() || 'prod')
  }

  return (
    <AppShell
      title={t('runtimeObserver.title')}
      description={t('runtimeObserver.description')}
    >
      <div className="events-page">
        <form className="config-filters" aria-label={t('runtimeObserver.filtersLabel')} onSubmit={handleSubmit}>
          <label>
            {t('runtimeObserver.environment')}
            <input value={draftEnvironment} onChange={(event) => setDraftEnvironment(event.target.value)} placeholder={t('runtimeObserver.environmentPlaceholder')} />
          </label>
          <div className="config-filters__actions">
            <button type="submit">{t('runtimeObserver.refresh')}</button>
          </div>
        </form>

        {observerQuery.isLoading ? <div className="event-state">{t('runtimeObserver.loading')}</div> : null}
        {observerQuery.error ? <div className="config-error">{t('runtimeObserver.loadError')}</div> : null}

        {!observerQuery.isLoading && !observerQuery.error && observerQuery.data ? (
          <>
            <div className="summary-card-grid">
              <section className="summary-card">
                <span>{t('runtimeObserver.environment')}</span>
                <strong>{observerQuery.data.environment}</strong>
              </section>
              <section className="summary-card">
                <span>{t('runtimeObserver.activePolicy')}</span>
                <strong>{observerQuery.data.active_policy.version_id || '—'}</strong>
                <small>{formatDate(observerQuery.data.active_policy.updated_at)}</small>
              </section>
              <section className="summary-card">
                <span>{t('runtimeObserver.cacheEntries')}</span>
                <strong>{observerQuery.data.cache.entry_count}</strong>
              </section>
              <section className="summary-card">
                <span>{t('runtimeObserver.invalidationCount')}</span>
                <strong>{observerQuery.data.cache.invalidation_count}</strong>
                <small>{formatDate(observerQuery.data.cache.last_invalidated_at)}</small>
              </section>
            </div>

            <div className="event-table">
              <table>
                <thead>
                   <tr>
                     <th>{t('runtimeObserver.colCacheEnvironment')}</th>
                     <th>{t('runtimeObserver.colPolicyVersion')}</th>
                     <th>{t('runtimeObserver.colCachedAt')}</th>
                   </tr>
                </thead>
                <tbody>
                  {cacheEntries.map((entry) => (
                    <tr key={`${entry.environment}-${entry.policy_version_id}-${entry.cached_at}`}>
                      <td>{entry.environment}</td>
                      <td>{entry.policy_version_id}</td>
                      <td>{formatDate(entry.cached_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            <div className="runtime-observer-grid">
              <section className="event-table">
                <table>
                  <thead>
                     <tr>
                       <th>{t('runtimeObserver.colRequestId')}</th>
                       <th>{t('runtimeObserver.colResolvedModel')}</th>
                       <th>{t('runtimeObserver.colScope')}</th>
                       <th>{t('runtimeObserver.colCreatedAt')}</th>
                     </tr>
                  </thead>
                  <tbody>
                    {runtimeFacts.map((fact) => (
                      <tr key={fact.request_id}>
                        <td>{fact.request_id}</td>
                        <td>{fact.resolved_model}</td>
                        <td>{fact.matched_scope_type || '—'}</td>
                        <td>{formatDate(fact.created_at)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </section>

              <section className="event-table">
                <table>
                  <thead>
                     <tr>
                       <th>{t('runtimeObserver.colEventId')}</th>
                       <th>{t('runtimeObserver.colEventType')}</th>
                       <th>{t('runtimeObserver.colRollout')}</th>
                       <th>{t('runtimeObserver.colCreatedAt')}</th>
                     </tr>
                  </thead>
                  <tbody>
                    {distributionFacts.map((fact) => (
                      <tr key={fact.event_id}>
                        <td>{fact.event_id}</td>
                        <td>{fact.event_type}</td>
                        <td>{fact.rollout_id || '—'}</td>
                        <td>{formatDate(fact.created_at)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </section>
            </div>
          </>
        ) : null}
      </div>
    </AppShell>
  )
}
