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
  return Array.isArray(value?.issues) ? value.issues.length : 0
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
  const issues = Array.isArray(data?.alerts?.issues) ? data.alerts.issues.slice(0, 3) : []
  const advice = data?.ai_ops_advice
  const recommendedActions = Array.isArray(advice?.recommended_actions) ? advice.recommended_actions.slice(0, 3) : []
  const operations = Array.isArray(data?.operation_history) ? data.operation_history.slice(0, 3) : []

  return (
    <section className="panel" style={{ marginTop: '1.5rem' }}>
      <div className="page-header" style={{ marginBottom: '1rem' }}>
        <div>
          <h2>Session / Ops 概览</h2>
          <p>来自 Python session dashboard（/api/v1/admin/dashboard）的聚合状态摘要。</p>
        </div>
      </div>

      {loading ? <div className="event-state">正在加载 Session/Ops 概览…</div> : null}
      {hasError ? <div className="config-error">Session/Ops 数据暂不可用，但现有首页概览仍可继续使用。</div> : null}

      {!loading && !hasError ? (
        <>
          <div className="summary-card-grid">
            <SummaryMetricCard label="Overall Status" value={statusLabel(data?.overall_status)} />
            <SummaryMetricCard label="Health" value={statusLabel(data?.health?.status)} />
            <SummaryMetricCard label="Continuation" value={statusLabel(data?.continuation?.status)} />
            <SummaryMetricCard label="Duplicate Groups" value={data?.duplicates?.duplicate_group_count ?? 0} />
            <SummaryMetricCard label="Alert Issues" value={countIssues(data?.alerts)} />
            <SummaryMetricCard label="Shared Project Sessions" value={data?.shared_memory?.project_session_count ?? 0} />
            <SummaryMetricCard label="Pending Continuations" value={data?.continuation_metrics?.pending ?? 0} />
            <SummaryMetricCard label="KG Success / Fail" value={formatKgSummary(data?.kg)} />
          </div>

          <div className="event-table" style={{ marginTop: '1rem' }}>
            <div className="page-header" style={{ marginBottom: '0.75rem' }}>
              <div>
                <h3>Recent Alerts</h3>
                <p>仅显示前 3 条问题摘要。</p>
              </div>
            </div>
            {issues.length > 0 ? (
              <ul>
                {issues.map((issue, index) => (
                  <li key={`issue-${index}`}>{stringifyIssue(issue)}</li>
                ))}
              </ul>
            ) : (
              <div className="event-state">当前没有告警问题。</div>
            )}
          </div>

          <div className="event-table" style={{ marginTop: '1rem' }}>
            <div className="page-header" style={{ marginBottom: '0.75rem' }}>
              <div>
                <h3>AI Ops Advice</h3>
                <p>{advice?.summary || '暂无建议摘要。'}</p>
              </div>
            </div>
            <div className="summary-card-grid" style={{ marginBottom: '0.75rem' }}>
              <SummaryMetricCard label="Risk Level" value={statusLabel(advice?.risk_level)} />
              <SummaryMetricCard label="Recommended Actions" value={recommendedActions.length} />
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
              <div className="event-state">当前没有推荐动作。</div>
            )}
          </div>

          <div className="event-table" style={{ marginTop: '1rem' }}>
            <div className="page-header" style={{ marginBottom: '0.75rem' }}>
              <div>
                <h3>Recent Operations</h3>
                <p>展示最近 3 条操作历史。</p>
              </div>
            </div>
            {operations.length > 0 ? (
              <table>
                <thead>
                  <tr>
                    <th>Action</th>
                    <th>Status</th>
                    <th>Target</th>
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
              <div className="event-state">当前没有操作历史。</div>
            )}
          </div>
        </>
      ) : null}
    </section>
  )
}
