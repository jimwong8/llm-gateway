import { useMemo, useState, type FormEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
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
  const { t } = useTranslation()
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
      setError(t('approvals.formRequired'))
      setSuccessMessage('')
      return
    }

    if (form.decision === 'overridden' && !form.finalModel.trim()) {
      setError(t('approvals.overriddenRequiresModel'))
      setSuccessMessage('')
      return
    }

    if (form.decision === 'rejected' && !form.approvalReason.trim()) {
      setError(t('approvals.rejectedRequiresReason'))
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

      setSuccessMessage(t('approvals.submitSuccess', { id: approval.id, decision: form.decision }))
      if (form.decision !== 'overridden') {
        setForm((previous) => ({ ...previous, finalModel: '' }))
      }
      if (form.decision !== 'rejected') {
        setForm((previous) => ({ ...previous, approvalReason: '' }))
      }
      void recommendationsQuery.refetch()
    } catch (unknownError) {
      if (unknownError instanceof ApiError) {
        setError(t('approvals.submitFailed', { message: unknownError.message }))
      } else {
        setError(unknownError instanceof Error ? unknownError.message : t('approvals.submitFailedGeneric'))
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <AppShell
      title={t('approvals.title')}
      description={t('approvals.description')}
    >
      <div className="events-page">
        {recommendationsQuery.isLoading ? <div className="event-state">{t('approvals.loading')}</div> : null}
        {recommendationsQuery.error ? <div className="config-error">{t('approvals.loadError')}</div> : null}

        <div className="event-table">
          <table>
            <thead>
              <tr>
                 <th>{t('approvals.colId')}</th>
                 <th>{t('approvals.colAgent')}</th>
                 <th>{t('approvals.colTask')}</th>
                 <th>{t('approvals.colEnvironment')}</th>
                 <th>{t('approvals.colRecommendedModel')}</th>
                 <th>{t('approvals.colStatus')}</th>
                 <th>{t('approvals.colUpdatedAt')}</th>
                 <th>{t('approvals.colActions')}</th>
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
                      {t('approvals.select')}
                    </button>
                  </td>
                </tr>
              ))}
              {recommendations.length === 0 && !recommendationsQuery.isLoading ? (
                <tr>
                  <td colSpan={8}>{t('approvals.noData')}</td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>

        <form className="release-panel" aria-label="Governance Approval Form" onSubmit={handleSubmit}>
          <div className="release-panel__header">
            <div>
              <h2>{t('approvals.consoleTitle')}</h2>
              <p>{t('approvals.consoleDescription')}</p>
            </div>
            <div className="policy-actions">
              {successMessage ? (
                <Link className="rollouts-action" to={`/policy-versions?environment=${encodeURIComponent(form.environment || 'prod')}`}>
                  {t('approvals.goToPolicyVersions')}
                </Link>
              ) : null}
              <button type="submit" disabled={submitting}>
                {submitting ? t('approvals.submitting') : t('approvals.submitApproval')}
              </button>
            </div>
          </div>

          <div className="release-panel__grid">
            <label>
              {t('approvals.labelRecommendationId')}
              <input
                value={form.recommendationID}
                onChange={(event) => setForm((previous) => ({ ...previous, recommendationID: event.target.value }))}
                placeholder="推荐-1"
              />
            </label>
            <label>
              {t('approvals.labelDecision')}
              <select
                value={form.decision}
                onChange={(event) => setForm((previous) => ({ ...previous, decision: event.target.value as ApprovalFormState['decision'] }))}
              >
                <option value="approved">{t('approvals.decisionApproved')}</option>
                <option value="overridden">{t('approvals.decisionOverridden')}</option>
                <option value="rejected">{t('approvals.decisionRejected')}</option>
              </select>
            </label>
            <label>
              {t('approvals.labelFinalModel')}
              <input
                value={form.finalModel}
                onChange={(event) => setForm((previous) => ({ ...previous, finalModel: event.target.value }))}
                placeholder="gpt-4o-mini"
                disabled={form.decision !== 'overridden'}
              />
            </label>
            <label>
              {t('approvals.labelApprovalReason')}
              <input
                value={form.approvalReason}
                onChange={(event) => setForm((previous) => ({ ...previous, approvalReason: event.target.value }))}
                placeholder="原因"
              />
            </label>
            <label>
              {t('approvals.labelApprover')}
              <input
                value={form.approvedBy}
                onChange={(event) => setForm((previous) => ({ ...previous, approvedBy: event.target.value }))}
                placeholder="运维机器人"
              />
            </label>
            <label>
              {t('approvals.labelScope')}
              <select
                value={form.scope}
                onChange={(event) => setForm((previous) => ({ ...previous, scope: event.target.value }))}
              >
                <option value="agent">{t('approvals.scopeAgent')}</option>
                <option value="project">{t('approvals.scopeProject')}</option>
                <option value="tenant">{t('approvals.scopeTenant')}</option>
              </select>
            </label>
            <label>
              {t('approvals.labelEnvironment')}
              <input
                value={form.environment}
                onChange={(event) => setForm((previous) => ({ ...previous, environment: event.target.value }))}
                placeholder="生产环境"
              />
            </label>
            <label>
              {t('approvals.labelProjectId')}
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
