import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { AppShell } from '../components/layout/AppShell'
import { listGovernanceDrifts } from '../lib/drifts'

function formatDate(value?: string) {
  if (!value) {
    return '—'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString()
}

export function DriftDashboardPage() {
  const { t } = useTranslation()
  const driftsQuery = useQuery({
    queryKey: ['governance-drifts'],
    queryFn: listGovernanceDrifts,
  })

  const drifts = useMemo(() => driftsQuery.data?.data ?? [], [driftsQuery.data])

  const metrics = useMemo(() => {
    const total = drifts.length
    const detected = drifts.filter((item) => item.status === 'detected').length
    const accepted = drifts.filter((item) => item.status === 'accepted').length
    const resolved = drifts.filter((item) => item.status === 'resolved').length
    return { total, detected, accepted, resolved }
  }, [drifts])

  return (
    <AppShell
      title={t('drifts.title')}
      description={t('drifts.description')}
    >
      <div className="events-page">
        {driftsQuery.isLoading ? <div className="event-state">{t('drifts.loading')}</div> : null}
        {driftsQuery.error ? <div className="config-error">{t('drifts.loadError')}</div> : null}

        {!driftsQuery.isLoading && !driftsQuery.error ? (
          <>
            <div className="summary-card-grid">
              <section className="summary-card">
                <span>{t('drifts.total')}</span>
                <strong>{metrics.total}</strong>
              </section>
              <section className="summary-card">
                <span>{t('drifts.detected')}</span>
                <strong>{metrics.detected}</strong>
              </section>
              <section className="summary-card">
                <span>{t('drifts.accepted')}</span>
                <strong>{metrics.accepted}</strong>
              </section>
              <section className="summary-card">
                <span>{t('drifts.resolved')}</span>
                <strong>{metrics.resolved}</strong>
              </section>
            </div>

            <div className="event-table">
              <table>
                <thead>
                  <tr>
                    <th>{t('drifts.colId')}</th>
                    <th>{t('drifts.colEnvironment')}</th>
                    <th>{t('drifts.colAgent')}</th>
                    <th>{t('drifts.colActiveModel')}</th>
                    <th>{t('drifts.colRecommendedModel')}</th>
                    <th>{t('drifts.colStatus')}</th>
                    <th>{t('drifts.colDetectedAt')}</th>
                  </tr>
                </thead>
                <tbody>
                  {drifts.map((row) => (
                    <tr key={row.id}>
                      <td>{row.id}</td>
                      <td>{row.environment || '—'}</td>
                      <td>{row.agent_id || '—'}</td>
                      <td>{row.active_model || '—'}</td>
                      <td>{row.recommended_model || '—'}</td>
                      <td>
                        <span className={`status-pill ${row.status}`}>{row.status}</span>
                      </td>
                      <td>{formatDate(row.detected_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {drifts.length === 0 ? <div className="config-table__state">{t('drifts.noData')}</div> : null}
            </div>
          </>
        ) : null}
      </div>
    </AppShell>
  )
}
