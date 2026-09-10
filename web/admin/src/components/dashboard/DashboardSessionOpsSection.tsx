import { useTranslation } from 'react-i18next'
import type { SessionAdminDashboard } from '../../types/sessionDashboard'
import { SummaryMetricCard } from './SummaryMetricCard'

type DashboardSessionOpsSectionProps = {
  loading: boolean
  hasError: boolean
  data: SessionAdminDashboard | undefined
}

function statusLabel(value: string | undefined) {
  return value ?? '—'
}

function countIssues(value: SessionAdminDashboard['alerts']) {
  return Array.isArray(value) ? value.length : 0
}

function formatKgSummary(value: SessionAdminDashboard['kg']) {
  const success = value?.kg_success ?? 0
  const failJson = value?.kg_fail_json_extract ?? 0
  const fail429 = value?.kg_fail_429 ?? 0
  return `${success}/${failJson + fail429}`
}

function stringifyIssue(issue: string | { [key: string]: unknown }) {
  if (typeof issue === 'string') return issue
  if (issue.summary && typeof issue.summary === 'string') return issue.summary
  if (issue.message && typeof issue.message === 'string') return issue.message
  if (issue.code && typeof issue.code === 'string') return issue.code
  return JSON.stringify(issue)
}

export function DashboardSessionOpsSection({ loading, hasError, data }: DashboardSessionOpsSectionProps) {
  const { t } = useTranslation()
  const issues = Array.isArray(data?.alerts) ? data.alerts.slice(0, 3) : []
  const advice = data?.ai_ops_advice
  const recommendedActions = Array.isArray(advice?.recommended_actions) ? advice.recommended_actions.slice(0, 3) : []
  const operations = Array.isArray(data?.operation_history) ? data.operation_history.slice(0, 3) : []

  return (
    <section className="panel dashboard-session-ops">
      <div className="page-header dashboard-session-ops__header">
        <div>
          <h2>{t('sessionOps.title')}</h2>
          <p>{t('sessionOps.description')}</p>
        </div>
      </div>

      {loading ? <div className="event-state">{t('sessionOps.loading')}</div> : null}
      {hasError ? <div className="config-error">{t('sessionOps.partialError')}</div> : null}

      {!loading && !hasError ? (
        <>
          <div className="summary-card-grid">
            <SummaryMetricCard label={t('sessionOps.overallStatus')} value={statusLabel(data?.overall_status)} />
            <SummaryMetricCard label={t('sessionOps.healthStatus')} value={statusLabel(data?.health?.status)} />
            <SummaryMetricCard label={t('sessionOps.continuationStatus')} value={statusLabel(data?.continuation?.status)} />
            <SummaryMetricCard label={t('sessionOps.duplicateGroups')} value={data?.duplicates?.duplicate_group_count ?? 0} />
            <SummaryMetricCard label={t('sessionOps.alertIssues')} value={countIssues(data?.alerts)} />
            <SummaryMetricCard label={t('sessionOps.sharedProjectSessions')} value={data?.shared_memory?.project_session_count ?? 0} />
            <SummaryMetricCard label={t('sessionOps.pendingContinuations')} value={data?.continuation_metrics?.pending ?? 0} />
            <SummaryMetricCard label={t('sessionOps.kgSuccessFail')} value={formatKgSummary(data?.kg)} />
          </div>

          <div className="event-table dashboard-session-ops__section">
            <div className="page-header dashboard-session-ops__section-header">
              <div>
                <h3>{t('sessionOps.recentAlerts')}</h3>
                <p>{t('sessionOps.recentAlertsHint')}</p>
              </div>
            </div>
            {issues.length > 0 ? (
              <ul>
                {issues.map((issue, index) => (
                  <li key={`issue-${index}`}>{stringifyIssue(issue)}</li>
                ))}
              </ul>
            ) : (
              <div className="event-state">{t('sessionOps.noAlerts')}</div>
            )}
          </div>

          <div className="event-table dashboard-session-ops__section">
            <div className="page-header dashboard-session-ops__section-header">
              <div>
                <h3>{t('sessionOps.aiOpsAdvice')}</h3>
                <p>{advice?.summary || t('sessionOps.noAdviceSummary')}</p>
              </div>
            </div>
            <div className="summary-card-grid dashboard-session-ops__metrics">
              <SummaryMetricCard label={t('sessionOps.riskLevel')} value={statusLabel(advice?.risk_level)} />
              <SummaryMetricCard label={t('sessionOps.recommendedActions')} value={recommendedActions.length} />
            </div>
            {recommendedActions.length > 0 ? (
              <ul>
                {recommendedActions.map((item, index) => (
                  <li key={`advice-${index}`}>
                    <strong>{item.area ?? 'general'}</strong>: {item.action}
                  </li>
                ))}
              </ul>
            ) : (
              <div className="event-state">{t('sessionOps.noRecommendedActions')}</div>
            )}
          </div>

          <div className="event-table dashboard-session-ops__section">
            <div className="page-header dashboard-session-ops__section-header">
              <div>
                <h3>{t('sessionOps.recentOperations')}</h3>
                <p>{t('sessionOps.recentOperationsHint')}</p>
              </div>
            </div>
            {operations.length > 0 ? (
              <table>
                <thead>
                  <tr>
                    <th>{t('sessionOps.colAction')}</th>
                    <th>{t('sessionOps.colStatus')}</th>
                    <th>{t('sessionOps.colTarget')}</th>
                  </tr>
                </thead>
                <tbody>
                  {operations.map((item, index) => (
                    <tr key={`operation-${index}`}>
                      <td>{item.action}</td>
                      <td>{item.status}</td>
                      <td>{item.target_id || item.target_type || '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : (
              <div className="event-state">{t('sessionOps.noOperations')}</div>
            )}
          </div>
        </>
      ) : null}
    </section>
  )
}
