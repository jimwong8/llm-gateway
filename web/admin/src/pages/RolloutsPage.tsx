import { useMemo, useState } from 'react'
import type { FormEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { ApiError } from '../lib/http'
import { createGovernanceRollback, listRolloutDashboard } from '../lib/rollouts'
import type { RolloutRow } from '../types/rollout'

type RollbackDialogState = {
  rolloutID: string
  environment: string
  open: boolean
}

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
  const [dialogState, setDialogState] = useState<RollbackDialogState>({
    rolloutID: '',
    environment: 'prod',
    open: false,
  })
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
    setDialogState({ rolloutID: row.id, environment: row.environment || 'prod', open: true })
    setRollbackForm((previous) => ({
      ...previous,
    }))
  }

  function closeRollbackDialog() {
    setDialogState((previous) => ({ ...previous, open: false }))
  }

  async function handleRollbackSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (!dialogState.rolloutID || !rollbackForm.actor.trim()) {
      setRollbackError('请至少填写 actor。')
      return
    }

    setRollbackSubmitting(true)
    setRollbackError('')
    setRollbackSuccess('')

    try {
      const response = await createGovernanceRollback({
        rollout_id: dialogState.rolloutID,
        actor: rollbackForm.actor.trim(),
        reason: rollbackForm.reason.trim() || undefined,
      })

      setRollbackSuccess(`已触发回滚：${response.rollback.id}`)
      setDialogState((previous) => ({ ...previous, open: false }))
      void rolloutsQuery.refetch()
    } catch (unknownError) {
      if (unknownError instanceof ApiError) {
        setRollbackError(`回滚失败：${unknownError.message}`)
      } else {
        setRollbackError(unknownError instanceof Error ? unknownError.message : '回滚失败')
      }
    } finally {
      setRollbackSubmitting(false)
    }
  }

  return (
    <AppShell
      title="Governance Rollouts"
      description="查看模型治理 rollout 进度、质量信号与状态分层，并在需要时通过回滚入口恢复到已知稳定版本。"
    >
      <div className="events-page">
        {rolloutsQuery.isLoading ? <div className="event-state">正在加载 rollout 列表…</div> : null}
        {rolloutsQuery.error ? <div className="config-error">rollout 列表加载失败，请检查 governance 接口状态。</div> : null}
        {rollbackError ? <div className="config-error">{rollbackError}</div> : null}
        {rollbackSuccess ? <div className="event-state">{rollbackSuccess}</div> : null}

        {!rolloutsQuery.isLoading && !rolloutsQuery.error ? (
          <>
            <div className="summary-card-grid">
              <section className="summary-card">
                <span>Total Rollouts</span>
                <strong>{metrics.total}</strong>
              </section>
              <section className="summary-card">
                <span>Running</span>
                <strong>{metrics.running}</strong>
                <small>{formatRate(metrics.runningRate)}</small>
              </section>
              <section className="summary-card">
                <span>Promoted</span>
                <strong>{metrics.promoted}</strong>
                <small>{formatRate(metrics.promotedRate)}</small>
              </section>
              <section className="summary-card">
                <span>Average Percent</span>
                <strong>{formatPercent(Number(metrics.averagePercent.toFixed(1)))}</strong>
              </section>
            </div>

            <div className="summary-card-grid">
              <section className="summary-card">
                <span>Dashboard Error Rate</span>
                <strong>{formatRate(metrics.averageErrorRate)}</strong>
              </section>
              <section className="summary-card">
                <span>Dashboard P95 Latency</span>
                <strong>{formatLatency(metrics.averageP95Latency)}</strong>
              </section>
              <section className="summary-card">
                <span>Dashboard Fallback Rate</span>
                <strong>{formatRate(metrics.averageFallbackRate)}</strong>
              </section>
              <section className="summary-card">
                <span>Dashboard Samples</span>
                <strong>{formatSampleCount(metrics.totalSamples)}</strong>
              </section>
            </div>

            <div className="summary-card-grid">
              <section className="summary-card summary-card--status healthy">
                <span>Healthy</span>
                <strong>{metrics.healthy}</strong>
                <small>error &lt; 2% · fallback &lt; 1% · p95 &lt; 900ms</small>
              </section>
              <section className="summary-card summary-card--status watch">
                <span>Watch</span>
                <strong>{metrics.watch}</strong>
                <small>error ≥ 2% 或 fallback ≥ 1% 或 p95 ≥ 900ms</small>
              </section>
              <section className="summary-card summary-card--status critical">
                <span>Critical</span>
                <strong>{metrics.critical}</strong>
                <small>error ≥ 5% 或 fallback ≥ 3% 或 p95 ≥ 1500ms</small>
              </section>
              <section className="summary-card">
                <span>Rollback Ready</span>
                <strong>{metrics.total > 0 ? 'Enabled' : 'Idle'}</strong>
                <small>可直接在行级操作触发回滚</small>
              </section>
            </div>

            <div className="event-table">
              <table>
                <thead>
                  <tr>
                    <th>Rollout ID</th>
                    <th>Policy Version</th>
                    <th>Environment</th>
                    <th>Status</th>
                    <th>Rollout %</th>
                    <th>Error Rate</th>
                    <th>p95 Latency</th>
                    <th>Fallback Rate</th>
                    <th>Samples</th>
                    <th>Triggered By</th>
                    <th>Updated At</th>
                    <th>Action</th>
                  </tr>
                </thead>
                <tbody>
                  {rollouts.map((row) => {
                    const health = getRolloutHealth(row)
                    return (
                      <tr key={row.id}>
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
                            回滚
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

      {dialogState.open ? (
        <div className="dialog-backdrop" role="presentation">
          <section
            className="dialog-card"
            role="dialog"
            aria-modal="true"
            aria-labelledby="rollback-dialog-title"
          >
            <div className="dialog-card__header">
              <div>
                <h2 id="rollback-dialog-title">回滚 Rollout</h2>
                <p>Rollout ID: {dialogState.rolloutID} · Environment: {dialogState.environment}</p>
              </div>
              <button type="button" onClick={closeRollbackDialog}>
                关闭
              </button>
            </div>

            <form className="release-panel__grid" aria-label="Rollback Release Form" onSubmit={handleRollbackSubmit}>
              <label>
                Actor
                <input value={rollbackForm.actor} onChange={(event) => setRollbackForm((previous) => ({ ...previous, actor: event.target.value }))} />
              </label>
              <label>
                Reason
                <input value={rollbackForm.reason} onChange={(event) => setRollbackForm((previous) => ({ ...previous, reason: event.target.value }))} />
              </label>
              <div className="dialog-card__actions">
                <button type="button" onClick={closeRollbackDialog}>取消</button>
                <button type="submit" disabled={rollbackSubmitting}>{rollbackSubmitting ? '回滚中…' : '确认回滚'}</button>
              </div>
            </form>
          </section>
        </div>
      ) : null}
    </AppShell>
  )
}
