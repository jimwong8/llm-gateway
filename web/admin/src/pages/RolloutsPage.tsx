import { useMemo, useState } from 'react'
import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { AppShell } from '../components/layout/AppShell'
import { RollbackDialog } from '../components/rollouts/RollbackDialog'
import { ApiError } from '../lib/http'
import { createGovernanceRollback, listRolloutDashboard } from '../lib/rollouts'
import type { RolloutRow } from '../types/rollout'

type RollbackFormState = {
  actor: string
  reason: string
}

type RolloutHealth = 'healthy' | 'watch' | 'critical'

const defaultRollbackFormState: RollbackFormState = {
  actor: 'ops-bot',
  reason: 'rollback to known good',
}

function formatPercent(value: number) {
  if (!Number.isFinite(value)) {
    return '0%'
  }
  return `${value}%`
}

function formatRate(value: number | undefined) {
  if (!Number.isFinite(value ?? 0)) {
    return '0.0%'
  }
  return `${((value ?? 0) * 100).toFixed(1)}%`
}

function formatLatency(value: number | undefined) {
  if (!Number.isFinite(value ?? 0)) {
    return '0 ms'
  }
  return `${Math.round(value ?? 0)} ms`
}

function formatSampleCount(value: number | undefined) {
  const count = Number(value ?? 0)
  if (!Number.isFinite(count) || count <= 0) {
    return '0'
  }
  return String(Math.round(count))
}

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

function getRolloutHealth(row: RolloutRow): RolloutHealth {
  const errorRate = row.error_rate ?? 0
  const fallbackRate = row.fallback_rate ?? 0
  const p95Latency = row.p95_latency ?? 0

  if (errorRate >= 0.05 || fallbackRate >= 0.03 || p95Latency >= 1500) {
    return 'critical'
  }

  if (errorRate >= 0.02 || fallbackRate >= 0.01 || p95Latency >= 900) {
    return 'watch'
  }

  return 'healthy'
}

export function RolloutsPage() {
  const { t } = useTranslation()
  const [searchParams] = useSearchParams()
  const highlightedPolicyVersionID = searchParams.get('policyVersionId') ?? ''
  const [rolloutID, setRolloutID] = useState('')
  const [environment, setEnvironment] = useState('prod')
  const [dialogOpen, setDialogOpen] = useState(false)
  const [rollbackForm, setRollbackForm] = useState<RollbackFormState>(defaultRollbackFormState)
  const [rollbackSubmitting, setRollbackSubmitting] = useState(false)
  const [rollbackError, setRollbackError] = useState('')
  const [rollbackSuccess, setRollbackSuccess] = useState('')

  const rolloutsQuery = useQuery({
    queryKey: ['governance-rollout-dashboard'],
    queryFn: listRolloutDashboard,
  })

  const rollouts = useMemo(() => rolloutsQuery.data?.data ?? [], [rolloutsQuery.data])

  const metrics = useMemo(() => {
    const total = rollouts.length
    const running = rollouts.filter((item) => item.status === 'running').length
    const promoted = rollouts.filter((item) => item.status === 'promoted').length
    const averagePercent = total > 0
      ? rollouts.reduce((sum, item) => sum + (Number(item.rollout_percent) || 0), 0) / total
      : 0

    const averageErrorRate = total > 0
      ? rollouts.reduce((sum, item) => sum + (item.error_rate ?? 0), 0) / total
      : 0

    const averageP95Latency = total > 0
      ? rollouts.reduce((sum, item) => sum + (item.p95_latency ?? 0), 0) / total
      : 0

    const averageFallbackRate = total > 0
      ? rollouts.reduce((sum, item) => sum + (item.fallback_rate ?? 0), 0) / total
      : 0

    const totalSamples = rollouts.reduce((sum, item) => sum + (item.sample_count ?? 0), 0)

    const critical = rollouts.filter((item) => getRolloutHealth(item) === 'critical').length
    const watch = rollouts.filter((item) => getRolloutHealth(item) === 'watch').length
    const healthy = total - critical - watch

    return {
      total,
      running,
      promoted,
      averagePercent,
      runningRate: total > 0 ? running / total : 0,
      promotedRate: total > 0 ? promoted / total : 0,
      averageErrorRate,
      averageP95Latency,
      averageFallbackRate,
      totalSamples,
      critical,
      watch,
      healthy,
    }
  }, [rollouts])

  function openRollbackDialog(row: RolloutRow) {
    setRollbackError('')
    setRollbackSuccess('')
    setRolloutID(row.id)
    setEnvironment(row.environment || 'prod')
    setDialogOpen(true)
    setRollbackForm((previous) => ({ ...previous }))
  }

  function closeRollbackDialog() {
    setDialogOpen(false)
  }

  async function handleRollbackSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (!rolloutID || !rollbackForm.actor.trim()) {
      setRollbackError(t('rollouts.actorRequired'))
      return
    }

    setRollbackSubmitting(true)
    setRollbackError('')
    setRollbackSuccess('')

    try {
      const response = await createGovernanceRollback({
        rollout_id: rolloutID,
        actor: rollbackForm.actor.trim(),
        reason: rollbackForm.reason.trim() || undefined,
      })

      setRollbackSuccess(t('rollouts.rollbackTriggered', { id: response.id }))
      setDialogOpen(false)
      void rolloutsQuery.refetch()
    } catch (unknownError) {
      if (unknownError instanceof ApiError) {
        setRollbackError(t('rollouts.rollbackFailed', { message: unknownError.message }))
      } else {
        setRollbackError(unknownError instanceof Error ? unknownError.message : t('rollouts.rollbackFailedGeneric'))
      }
    } finally {
      setRollbackSubmitting(false)
    }
  }

  return (
    <AppShell
      title={t('rollouts.title')}
      description={t('rollouts.description')}
    >
      <div className="events-page">
        {rolloutsQuery.isLoading ? <div className="event-state">{t('rollouts.loading')}</div> : null}
        {rolloutsQuery.error ? <div className="config-error">{t('rollouts.loadError')}</div> : null}
        {rollbackError ? <div className="config-error">{rollbackError}</div> : null}
        {rollbackSuccess ? <div className="event-state">{rollbackSuccess}</div> : null}

        {!rolloutsQuery.isLoading && !rolloutsQuery.error ? (
          <>
            <div className="summary-card-grid">
              <section className="summary-card">
                <span>{t('rollouts.totalRollouts')}</span>
                <strong>{metrics.total}</strong>
              </section>
              <section className="summary-card">
                <span>{t('rollouts.running')}</span>
                <strong>{metrics.running}</strong>
                <small>{formatRate(metrics.runningRate)}</small>
              </section>
              <section className="summary-card">
                <span>{t('rollouts.completed')}</span>
                <strong>{metrics.promoted}</strong>
                <small>{formatRate(metrics.promotedRate)}</small>
              </section>
              <section className="summary-card">
                <span>{t('rollouts.averagePercent')}</span>
                <strong>{formatPercent(Number(metrics.averagePercent.toFixed(1)))}</strong>
              </section>
            </div>

            <div className="summary-card-grid">
              <section className="summary-card">
                <span>{t('rollouts.avgErrorRate')}</span>
                <strong>{formatRate(metrics.averageErrorRate)}</strong>
              </section>
              <section className="summary-card">
                <span>{t('rollouts.avgP95Latency')}</span>
                <strong>{formatLatency(metrics.averageP95Latency)}</strong>
              </section>
              <section className="summary-card">
                <span>{t('rollouts.avgFallbackRate')}</span>
                <strong>{formatRate(metrics.averageFallbackRate)}</strong>
              </section>
              <section className="summary-card">
                <span>{t('rollouts.totalSamples')}</span>
                <strong>{formatSampleCount(metrics.totalSamples)}</strong>
              </section>
            </div>

            <div className="summary-card-grid">
              <section className="summary-card summary-card--status healthy">
                <span>{t('rollouts.healthy')}</span>
                <strong>{metrics.healthy}</strong>
                <small>error &lt; 2% · fallback &lt; 1% · p95 &lt; 900ms</small>
              </section>
              <section className="summary-card summary-card--status watch">
                <span>{t('rollouts.watch')}</span>
                <strong>{metrics.watch}</strong>
                <small>error ≥ 2% 或 fallback ≥ 1% 或 p95 ≥ 900ms</small>
              </section>
              <section className="summary-card summary-card--status critical">
                <span>{t('rollouts.critical')}</span>
                <strong>{metrics.critical}</strong>
                <small>error ≥ 5% 或 fallback ≥ 3% 或 p95 ≥ 1500ms</small>
              </section>
              <section className="summary-card">
                <span>{t('rollouts.rollbackReady')}</span>
                <strong>{metrics.total > 0 ? t('rollouts.enabled') : t('rollouts.idle')}</strong>
                <small>{t('rollouts.rollbackHint')}</small>
              </section>
            </div>

            <div className="event-table">
              <table>
                <thead>
                    <tr>
                      <th>{t('rollouts.colRolloutId')}</th>
                      <th>{t('rollouts.colPolicyVersion')}</th>
                      <th>{t('rollouts.colEnvironment')}</th>
                      <th>{t('rollouts.colStatus')}</th>
                      <th>{t('rollouts.colRolloutPercent')}</th>
                      <th>{t('rollouts.colErrorRate')}</th>
                      <th>{t('rollouts.colP95Latency')}</th>
                      <th>{t('rollouts.colFallbackRate')}</th>
                      <th>{t('rollouts.colSampleCount')}</th>
                      <th>{t('rollouts.colTriggeredBy')}</th>
                      <th>{t('rollouts.colUpdatedAt')}</th>
                      <th>{t('rollouts.colActions')}</th>
                    </tr>
                </thead>
                <tbody>
                  {rollouts.map((row) => {
                    const health = getRolloutHealth(row)
                    return (
                      <tr key={row.id} data-highlighted={highlightedPolicyVersionID !== '' && row.policy_version_id === highlightedPolicyVersionID ? 'true' : 'false'}>
                        <td>{row.id}</td>
                        <td>{row.policy_version_id}</td>
                        <td>{row.environment}</td>
                        <td>
                          <div className="rollout-status-cell">
                            <span className={`status-pill ${row.status}`}>{row.status}</span>
                            <span className={`status-pill health-${health}`}>{health}</span>
                          </div>
                        </td>
                        <td>{formatPercent(row.rollout_percent ?? 0)}</td>
                        <td className="rollout-metric">{formatRate(row.error_rate)}</td>
                        <td className="rollout-metric">{formatLatency(row.p95_latency)}</td>
                        <td className="rollout-metric">{formatRate(row.fallback_rate)}</td>
                        <td className="rollout-metric">{formatSampleCount(row.sample_count)}</td>
                        <td>{row.triggered_by || '—'}</td>
                        <td>{formatDate(row.updated_at)}</td>
                        <td>
                          <button type="button" className="rollouts-action" onClick={() => openRollbackDialog(row)}>
                            {t('rollouts.rollback')}
                          </button>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          </>
        ) : null}
      </div>

      <RollbackDialog
        open={dialogOpen}
        onClose={closeRollbackDialog}
        onSubmit={handleRollbackSubmit}
        rolloutID={rolloutID}
        environment={environment}
        actor={rollbackForm.actor}
        onActorChange={(value) => setRollbackForm((prev) => ({ ...prev, actor: value }))}
        reason={rollbackForm.reason}
        onReasonChange={(value) => setRollbackForm((prev) => ({ ...prev, reason: value }))}
        loading={rollbackSubmitting}
        error={rollbackError}
      />
    </AppShell>
  )
}
