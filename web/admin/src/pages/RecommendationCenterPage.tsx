import type { FormEvent } from 'react'
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { ApiError } from '../lib/http'
import { createGovernanceApproval, listGovernanceRecommendations } from '../lib/recommendations'
import type { RecommendationRow } from '../types/recommendation'

type ApprovalDialogState = {
  recommendationID: string
  environment: string
  open: boolean
}

type ApprovalFormState = {
  approvedBy: string
  scope: string
  environment: string
}

const defaultApprovalFormState: ApprovalFormState = {
  approvedBy: 'ops-bot',
  scope: 'agent',
  environment: 'prod',
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

export function RecommendationCenterPage() {
  const [dialogState, setDialogState] = useState<ApprovalDialogState>({
    recommendationID: '',
    environment: defaultApprovalFormState.environment,
    open: false,
  })
  const [approvalForm, setApprovalForm] = useState<ApprovalFormState>(defaultApprovalFormState)
  const [approvalSubmitting, setApprovalSubmitting] = useState(false)
  const [approvalError, setApprovalError] = useState('')
  const [approvalSuccess, setApprovalSuccess] = useState('')

  const recommendationsQuery = useQuery({
    queryKey: ['governance-recommendations'],
    queryFn: listGovernanceRecommendations,
  })

  const recommendations = useMemo(() => recommendationsQuery.data?.data ?? [], [recommendationsQuery.data])

  const metrics = useMemo(() => {
    const total = recommendations.length
    const pending = recommendations.filter((item) => item.status === 'pending').length
    const approved = recommendations.filter((item) => item.status === 'approved').length

    return {
      total,
      pending,
      approved,
      uniqueAgents: new Set(recommendations.map((item) => item.agent_id).filter(Boolean)).size,
    }
  }, [recommendations])

  function openApprovalDialog(row: RecommendationRow) {
    setApprovalError('')
    setApprovalSuccess('')
    setDialogState({ recommendationID: row.id, environment: row.environment || approvalForm.environment, open: true })
    setApprovalForm((previous) => ({
      ...previous,
      environment: row.environment || previous.environment,
    }))
  }

  function closeApprovalDialog() {
    setDialogState((previous) => ({ ...previous, open: false }))
  }

  async function handleApprovalSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (!dialogState.recommendationID || !approvalForm.approvedBy.trim() || !approvalForm.environment.trim()) {
      setApprovalError('请至少填写 approved_by 与 environment。')
      return
    }

    setApprovalSubmitting(true)
    setApprovalError('')
    setApprovalSuccess('')

    try {
      const response = await createGovernanceApproval({
        recommendation_id: dialogState.recommendationID,
        decision: 'approved',
        approved_by: approvalForm.approvedBy.trim(),
        effective_scope: {
          scope: approvalForm.scope.trim() || 'agent',
          environment: approvalForm.environment.trim(),
        },
      })

      setApprovalSuccess(`已创建审批：${response.id}`)
      setDialogState((previous) => ({ ...previous, open: false }))
      void recommendationsQuery.refetch()
    } catch (unknownError) {
      if (unknownError instanceof ApiError) {
        setApprovalError(`审批失败：${unknownError.message}`)
      } else {
        setApprovalError(unknownError instanceof Error ? unknownError.message : '审批失败')
      }
    } finally {
      setApprovalSubmitting(false)
    }
  }

  return (
    <AppShell
      title="Recommendation Center"
      description="查看治理推荐列表与摘要，并直接为推荐创建审批动作。"
    >
      <div className="events-page">
        {recommendationsQuery.isLoading ? <div className="event-state">正在加载推荐列表…</div> : null}
        {recommendationsQuery.error ? <div className="config-error">推荐列表加载失败，请检查 governance 接口状态。</div> : null}
        {approvalError ? <div className="config-error">{approvalError}</div> : null}
        {approvalSuccess ? <div className="event-state">{approvalSuccess}</div> : null}

        {!recommendationsQuery.isLoading && !recommendationsQuery.error ? (
          <>
            <div className="summary-card-grid">
              <section className="summary-card">
                <span>Total Recommendations</span>
                <strong>{metrics.total}</strong>
              </section>
              <section className="summary-card">
                <span>Pending</span>
                <strong>{metrics.pending}</strong>
              </section>
              <section className="summary-card">
                <span>Approved</span>
                <strong>{metrics.approved}</strong>
              </section>
              <section className="summary-card">
                <span>Agents</span>
                <strong>{metrics.uniqueAgents}</strong>
              </section>
            </div>

            <div className="event-table">
              <table>
                <thead>
                  <tr>
                    <th>Recommendation ID</th>
                    <th>Agent</th>
                    <th>Task Type</th>
                    <th>Environment</th>
                    <th>Recommended Model</th>
                    <th>Status</th>
                    <th>Updated At</th>
                    <th>Action</th>
                  </tr>
                </thead>
                <tbody>
                  {recommendations.map((row) => (
                    <tr key={row.id}>
                      <td>{row.id}</td>
                      <td>{row.agent_id || '—'}</td>
                      <td>{row.task_type || '—'}</td>
                      <td>{row.environment || '—'}</td>
                      <td>{row.recommended_model || '—'}</td>
                      <td>
                        <span className={`status-pill ${row.status}`}>{row.status}</span>
                      </td>
                      <td>{formatDate(row.updated_at)}</td>
                      <td>
                        <button type="button" className="rollouts-action" onClick={() => openApprovalDialog(row)}>
                          审批
                        </button>
                      </td>
                    </tr>
                  ))}
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
            aria-labelledby="approval-dialog-title"
          >
            <div className="dialog-card__header">
              <div>
                <h2 id="approval-dialog-title">审批 Recommendation</h2>
                <p>Recommendation ID: {dialogState.recommendationID} · Decision: approved</p>
              </div>
              <button type="button" onClick={closeApprovalDialog}>
                关闭
              </button>
            </div>

            <form className="release-panel__grid" aria-label="Governance Approval Form" onSubmit={handleApprovalSubmit}>
              <label>
                Approved By
                <input
                  value={approvalForm.approvedBy}
                  onChange={(event) => setApprovalForm((previous) => ({ ...previous, approvedBy: event.target.value }))}
                />
              </label>
              <label>
                Scope
                <input
                  value={approvalForm.scope}
                  onChange={(event) => setApprovalForm((previous) => ({ ...previous, scope: event.target.value }))}
                />
              </label>
              <label>
                Environment
                <input
                  value={approvalForm.environment}
                  onChange={(event) => setApprovalForm((previous) => ({ ...previous, environment: event.target.value }))}
                />
              </label>
              <div className="dialog-card__actions">
                <button type="button" onClick={closeApprovalDialog}>取消</button>
                <button type="submit" disabled={approvalSubmitting}>{approvalSubmitting ? '审批中…' : '确认审批'}</button>
              </div>
            </form>
          </section>
        </div>
      ) : null}
    </AppShell>
  )
}
