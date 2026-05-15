import { useMemo, useState, type FormEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { ApiError } from '../lib/http'
import type { ApprovalRequest } from '../types/recommendation'
import { createGovernanceApproval, listGovernanceRecommendations } from '../lib/recommendations'
import { Link, useSearchParams } from 'react-router-dom'

type ApprovalFormState = {
  recommendationID: string
  decision: ApprovalRequest['decision']
  finalModel: string
  approvalReason: string
  approvedBy: string
  scope: string
  environment: string
  projectID: string
}

const initialForm: ApprovalFormState = {
  recommendationID: '',
  decision: 'approved',
  finalModel: '',
  approvalReason: '',
  approvedBy: 'ops-bot',
  scope: 'agent',
  environment: 'prod',
  projectID: '',
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

export function ApprovalsPage() {
  const [searchParams] = useSearchParams()
  const initialRecommendationID = searchParams.get('recommendationId') ?? ''
  const initialEnvironment = searchParams.get('environment') ?? initialForm.environment
  const [form, setForm] = useState<ApprovalFormState>({
    ...initialForm,
    recommendationID: initialRecommendationID,
    environment: initialEnvironment,
  })
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [successMessage, setSuccessMessage] = useState('')

  const recommendationsQuery = useQuery({
    queryKey: ['governance-recommendations'],
    queryFn: listGovernanceRecommendations,
  })

  const recommendations = useMemo(() => recommendationsQuery.data?.data ?? [], [recommendationsQuery.data])

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (!form.recommendationID.trim() || !form.approvedBy.trim() || !form.scope.trim() || !form.environment.trim()) {
      setError('请填写 recommendation、审批人和生效范围信息。')
      setSuccessMessage('')
      return
    }

    if (form.decision === 'overridden' && !form.finalModel.trim()) {
      setError('override 决策必须填写 final model。')
      setSuccessMessage('')
      return
    }

    if (form.decision === 'rejected' && !form.approvalReason.trim()) {
      setError('reject 决策必须填写审批原因。')
      setSuccessMessage('')
      return
    }

    setSubmitting(true)
    setError('')
    setSuccessMessage('')

    try {
      const approval = await createGovernanceApproval({
        recommendation_id: form.recommendationID.trim(),
        decision: form.decision,
        final_model: form.finalModel.trim() || undefined,
        approval_reason: form.approvalReason.trim() || undefined,
        approved_by: form.approvedBy.trim(),
        effective_scope: {
          scope: form.scope.trim(),
          project_id: form.projectID.trim() || undefined,
          environment: form.environment.trim(),
        },
      })

      setSuccessMessage(`审批成功：${approval.id}（${form.decision}）`)
      if (form.decision !== 'overridden') {
        setForm((previous) => ({ ...previous, finalModel: '' }))
      }
      if (form.decision !== 'rejected') {
        setForm((previous) => ({ ...previous, approvalReason: '' }))
      }
      void recommendationsQuery.refetch()
    } catch (unknownError) {
      if (unknownError instanceof ApiError) {
        setError(`审批失败：${unknownError.message}`)
      } else {
        setError(unknownError instanceof Error ? unknownError.message : '审批失败')
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <AppShell
      title="Approvals"
      description="对推荐模型执行 approve / override / reject，产出可追踪的审批记录并刷新推荐队列。"
    >
      <div className="events-page">
        {recommendationsQuery.isLoading ? <div className="event-state">正在加载 recommendations…</div> : null}
        {recommendationsQuery.error ? <div className="config-error">recommendation 列表加载失败，请检查 governance 接口状态。</div> : null}

        <div className="event-table">
          <table>
            <thead>
              <tr>
                <th>Recommendation ID</th>
                <th>Agent</th>
                <th>Task</th>
                <th>Environment</th>
                <th>Recommended Model</th>
                <th>Status</th>
                <th>Updated At</th>
                <th>Action</th>
              </tr>
            </thead>
            <tbody>
              {recommendations.map((item) => (
                <tr key={item.id}>
                  <td>{item.id}</td>
                  <td>{item.agent_id}</td>
                  <td>{item.task_type}</td>
                  <td>{item.environment}</td>
                  <td>{item.recommended_model || '—'}</td>
                  <td>{item.status || '—'}</td>
                  <td>{formatDate(item.updated_at)}</td>
                  <td>
                    <button
                      type="button"
                      className="rollouts-action"
                      onClick={() => setForm((previous) => ({ ...previous, recommendationID: item.id }))}
                    >
                      选择
                    </button>
                  </td>
                </tr>
              ))}
              {recommendations.length === 0 && !recommendationsQuery.isLoading ? (
                <tr>
                  <td colSpan={8}>暂无 recommendation 数据</td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>

        <form className="release-panel" aria-label="Governance Approval Form" onSubmit={handleSubmit}>
          <div className="release-panel__header">
            <div>
              <h2>Approval Console</h2>
              <p>选择 recommendation 后提交决策；override 需要 final model，reject 需要审批理由。</p>
            </div>
            <div className="policy-actions">
              {successMessage ? (
                <Link className="rollouts-action" to={`/policy-versions?environment=${encodeURIComponent(form.environment || 'prod')}`}>
                  去策略版本页
                </Link>
              ) : null}
              <button type="submit" disabled={submitting}>
                {submitting ? '提交中…' : '提交审批'}
              </button>
            </div>
          </div>

          <div className="release-panel__grid">
            <label>
              Recommendation ID
              <input
                value={form.recommendationID}
                onChange={(event) => setForm((previous) => ({ ...previous, recommendationID: event.target.value }))}
                placeholder="rec-1"
              />
            </label>
            <label>
              Decision
              <select
                value={form.decision}
                onChange={(event) => setForm((previous) => ({ ...previous, decision: event.target.value as ApprovalFormState['decision'] }))}
              >
                <option value="approved">approve</option>
                <option value="overridden">override</option>
                <option value="rejected">reject</option>
              </select>
            </label>
            <label>
              Final Model
              <input
                value={form.finalModel}
                onChange={(event) => setForm((previous) => ({ ...previous, finalModel: event.target.value }))}
                placeholder="gpt-4o-mini"
                disabled={form.decision !== 'overridden'}
              />
            </label>
            <label>
              Approval Reason
              <input
                value={form.approvalReason}
                onChange={(event) => setForm((previous) => ({ ...previous, approvalReason: event.target.value }))}
                placeholder="reason"
              />
            </label>
            <label>
              Approved By
              <input
                value={form.approvedBy}
                onChange={(event) => setForm((previous) => ({ ...previous, approvedBy: event.target.value }))}
                placeholder="ops-bot"
              />
            </label>
            <label>
              Scope
              <select
                value={form.scope}
                onChange={(event) => setForm((previous) => ({ ...previous, scope: event.target.value }))}
              >
                <option value="agent">agent</option>
                <option value="project">project</option>
                <option value="tenant">tenant</option>
              </select>
            </label>
            <label>
              Environment
              <input
                value={form.environment}
                onChange={(event) => setForm((previous) => ({ ...previous, environment: event.target.value }))}
                placeholder="prod"
              />
            </label>
            <label>
              Project ID
              <input
                value={form.projectID}
                onChange={(event) => setForm((previous) => ({ ...previous, projectID: event.target.value }))}
                placeholder="project-x"
              />
            </label>
          </div>

          {error ? <div className="config-error">{error}</div> : null}
          {successMessage ? <div className="config-success">{successMessage}</div> : null}
        </form>
      </div>
    </AppShell>
  )
}
