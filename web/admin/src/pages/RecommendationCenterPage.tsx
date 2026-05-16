import type { FormEvent } from 'react'
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
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

  const 推荐Query = useQuery({
    queryKey: ['governance-推荐'],
    queryFn: listGovernanceRecommendations,
  })

  const 推荐 = useMemo(() => 推荐Query.data?.data ?? [], [推荐Query.data])

  const metrics = useMemo(() => {
    const total = 推荐.length
    const pending = 推荐.filter((item) => item.status === 'pending').length
    const approved = 推荐.filter((item) => item.status === 'approved').length

    return {
      total,
      pending,
      approved,
      uniqueAgents: new Set(推荐.map((item) => item.agent_id).filter(Boolean)).size,
    }
  }, [推荐])

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
      void 推荐Query.refetch()
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
      title="推荐管理"
      description="查看治理推荐列表与摘要，并直接为推荐创建审批动作。"
    >
      <div className="events-page">
        {推荐Query.isLoading ? <div className="event-state">正在加载推荐列表…</div> : null}
        {推荐Query.error ? <div className="config-error">推荐列表加载失败，请检查 governance 接口状态。</div> : null}
        {approvalError ? <div className="config-error">{approvalError}</div> : null}
        {approvalSuccess ? <div className="event-state">{approvalSuccess}</div> : null}

        {!推荐Query.isLoading && !推荐Query.error ? (
          <>
            <div className="summary-card-grid">
              <section className="summary-card">
                <span>推荐总数</span>
                <strong>{metrics.total}</strong>
              </section>
              <section className="summary-card">
                <span>待处理</span>
                <strong>{metrics.pending}</strong>
              </section>
              <section className="summary-card">
                <span>已审批</span>
                <strong>{metrics.approved}</strong>
              </section>
              <section className="summary-card">
                <span>智能体数</span>
                <strong>{metrics.uniqueAgents}</strong>
              </section>
            </div>

            <div className="event-table">
              <table>
                <thead>
                  <tr>
                    <th>推荐 ID</th>
                    <th>智能体</th>
                    <th>任务类型</th>
                    <th>环境</th>
                    <th>推荐模型</th>
                    <th>状态</th>
                    <th>更新时间</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {推荐.map((row) => (
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
                        <div className="policy-actions">
                          <button type="button" className="rollouts-action" onClick={() => openApprovalDialog(row)}>
                            审批
                          </button>
                          <Link
                            className="rollouts-action"
                            to={`/approvals?recommendationId=${encodeURIComponent(row.id)}&environment=${encodeURIComponent(row.environment || 'prod')}`}
                          >
                            去审批页
                          </Link>
                        </div>
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
                <h2 id="approval-dialog-title">审批推荐</h2>
                <p>推荐 ID: {dialogState.recommendationID} · 决策: approved</p>
              </div>
              <button type="button" onClick={closeApprovalDialog}>
                关闭
              </button>
            </div>

            <form className="release-panel__grid" aria-label="Governance Approval Form" onSubmit={handleApprovalSubmit}>
                <label>
                审批人
                <input
                  value={approvalForm.approvedBy}
                  onChange={(event) => setApprovalForm((previous) => ({ ...previous, approvedBy: event.target.value }))}
                />
              </label>
              <label>
                作用域
                <input
                  value={approvalForm.scope}
                  onChange={(event) => setApprovalForm((previous) => ({ ...previous, scope: event.target.value }))}
                />
              </label>
              <label>
                环境
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
