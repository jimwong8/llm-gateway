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
      setError('请填写推荐、审批人和生效范围信息。')
      setSuccessMessage('')
      return
    }

    if (form.decision === 'overridden' && !form.finalModel.trim()) {
      setError('覆盖决策必须填写最终模型。')
      setSuccessMessage('')
      return
    }

    if (form.decision === 'rejected' && !form.approvalReason.trim()) {
      setError('拒绝决策必须填写审批原因。')
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
      title="审批管理"
      description="对推荐模型执行批准/覆盖/拒绝，产出可追踪的审批记录并刷新推荐队列。"
    >
      <div className="events-page">
        {recommendationsQuery.isLoading ? <div className="event-state">正在加载 recommendations…</div> : null}
        {recommendationsQuery.error ? <div className="config-error">recommendation 列表加载失败，请检查 governance 接口状态。</div> : null}

        <div className="event-table">
          <table>
            <thead>
              <tr>
                <th>推荐 ID</th>
                <th>智能体</th>
                <th>任务</th>
                <th>环境</th>
                <th>推荐模型</th>
                <th>状态</th>
                <th>更新时间</th>
                <th>操作</th>
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
                  <td colSpan={8}>暂无推荐数据</td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>

        <form className="release-panel" aria-label="Governance Approval Form" onSubmit={handleSubmit}>
          <div className="release-panel__header">
            <div>
              <h2>审批控制台</h2>
              <p>选择推荐后提交决策；覆盖需要指定最终模型，拒绝需要审批理由。</p>
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
              推荐 ID
              <input
                value={form.recommendationID}
                onChange={(event) => setForm((previous) => ({ ...previous, recommendationID: event.target.value }))}
                placeholder="推荐-1"
              />
            </label>
            <label>
              决策
              <select
                value={form.decision}
                onChange={(event) => setForm((previous) => ({ ...previous, decision: event.target.value as ApprovalFormState['decision'] }))}
              >
                <option value="approved">批准</option>
                <option value="overridden">覆盖</option>
                <option value="rejected">拒绝</option>
              </select>
            </label>
            <label>
              最终模型
              <input
                value={form.finalModel}
                onChange={(event) => setForm((previous) => ({ ...previous, finalModel: event.target.value }))}
                placeholder="gpt-4o-mini"
                disabled={form.decision !== 'overridden'}
              />
            </label>
            <label>
              审批原因
              <input
                value={form.approvalReason}
                onChange={(event) => setForm((previous) => ({ ...previous, approvalReason: event.target.value }))}
                placeholder="原因"
              />
            </label>
            <label>
              审批人
              <input
                value={form.approvedBy}
                onChange={(event) => setForm((previous) => ({ ...previous, approvedBy: event.target.value }))}
                placeholder="运维机器人"
              />
            </label>
            <label>
              作用域
              <select
                value={form.scope}
                onChange={(event) => setForm((previous) => ({ ...previous, scope: event.target.value }))}
              >
                <option value="agent">智能体</option>
                <option value="project">项目</option>
                <option value="tenant">租户</option>
              </select>
            </label>
            <label>
              环境
              <input
                value={form.environment}
                onChange={(event) => setForm((previous) => ({ ...previous, environment: event.target.value }))}
                placeholder="生产环境"
              />
            </label>
            <label>
              项目 ID
              <input
                value={form.projectID}
                onChange={(event) => setForm((previous) => ({ ...previous, projectID: event.target.value }))}
                placeholder="项目-x"
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
