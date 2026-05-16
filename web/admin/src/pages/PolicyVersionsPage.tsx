import { useMutation, useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { AppShell } from '../components/layout/AppShell'
import {
  activatePolicyVersion,
  approvePolicyVersion,
  getPolicyVersionDiff,
  listPolicyVersions,
} from '../lib/policyVersions'
import type { PolicyVersionDiffPayload, PolicyVersionRow } from '../types/policyVersion'
import { Link, useSearchParams } from 'react-router-dom'

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

function formatDiffPayload(payload: PolicyVersionDiffPayload | undefined) {
  if (payload === undefined) {
    return ''
  }
  if (typeof payload === 'string') {
    return payload
  }
  try {
    return JSON.stringify(payload, null, 2)
  } catch {
    return String(payload)
  }
}

export function PolicyVersionsPage() {
  const [searchParams] = useSearchParams()
  const [selectedVersionID, setSelectedVersionID] = useState('')
  const [actionMessage, setActionMessage] = useState('')
  const [actionError, setActionError] = useState('')

  const versionsQuery = useQuery({
    queryKey: ['governance-policy-versions'],
    queryFn: () => listPolicyVersions(50),
  })

  const versions = useMemo(() => versionsQuery.data?.data ?? [], [versionsQuery.data])

  const selectedVersion = useMemo(() => (
    versions.find((item) => item.id === selectedVersionID) ?? null
  ), [versions, selectedVersionID])

  const selectedVersionEffective = selectedVersion ?? versions[0] ?? null
  const selectedVersionEffectiveID = selectedVersionEffective?.id ?? ''

  useEffect(() => {
    if (versions.length === 0) {
      return
    }
    const versionIDFromQuery = searchParams.get('versionId') ?? ''
    if (versionIDFromQuery) {
      const matchedVersion = versions.find((item) => item.id === versionIDFromQuery)
      if (matchedVersion && matchedVersion.id !== selectedVersionID) {
        setSelectedVersionID(matchedVersion.id)
      }
      return
    }
    const environmentFromQuery = searchParams.get('environment') ?? ''
    if (!environmentFromQuery || selectedVersionID) {
      return
    }
    const matchedEnvironmentVersion = versions.find((item) => item.environment === environmentFromQuery)
    if (matchedEnvironmentVersion) {
      setSelectedVersionID(matchedEnvironmentVersion.id)
    }
  }, [versions, searchParams, selectedVersionID])

  const diffQuery = useQuery({
    queryKey: ['governance-policy-version-diff', selectedVersionEffectiveID],
    queryFn: () => getPolicyVersionDiff(selectedVersionEffectiveID),
    enabled: Boolean(selectedVersionEffectiveID),
    retry: false,
  })

  const approveMutation = useMutation({
    mutationFn: (version: PolicyVersionRow) => approvePolicyVersion(version.id, 'admin-ui'),
    onSuccess: async (data) => {
      setActionError('')
      setActionMessage(`已审批策略版本 ${data.id}`)
      await versionsQuery.refetch()
    },
    onError: (error) => {
      setActionMessage('')
      setActionError(error instanceof Error ? error.message : '审批失败')
    },
  })

  const activateMutation = useMutation({
    mutationFn: (version: PolicyVersionRow) => activatePolicyVersion(version.id),
    onSuccess: async (data) => {
      setActionError('')
      setActionMessage(`已激活策略版本 ${data.id}`)
      await versionsQuery.refetch()
    },
    onError: (error) => {
      setActionMessage('')
      setActionError(error instanceof Error ? error.message : '激活失败')
    },
  })

  function isActionPending(versionID: string) {
    return approveMutation.isPending && approveMutation.variables?.id === versionID
      || activateMutation.isPending && activateMutation.variables?.id === versionID
  }

  return (
    <AppShell
      title="策略版本"
      description="查看策略版本生命周期、当前激活状态，并在右侧对比区检查选中版本的差异内容。"
    >
      <div className="policy-version-center">
        {versionsQuery.isLoading ? <div className="event-state">正在加载策略版本列表…</div> : null}
        {versionsQuery.error ? <div className="config-error">策略版本列表加载失败，请检查 governance 接口状态。</div> : null}
        {actionError ? <div className="config-error">操作失败：{actionError}</div> : null}
        {actionMessage ? <div className="event-state">{actionMessage}</div> : null}

        {!versionsQuery.isLoading && !versionsQuery.error ? (
          <>
            <div className="summary-card-grid">
              <section className="summary-card">
                <span>版本总数</span>
                <strong>{versions.length}</strong>
              </section>
              <section className="summary-card">
                <span>当前激活版本</span>
                <strong>{versions.find((item) => item.status === 'active')?.id ?? '—'}</strong>
              </section>
              <section className="summary-card">
                <span>当前选中</span>
                <strong>{selectedVersionEffective?.id ?? '—'}</strong>
              </section>
              <section className="summary-card">
                <span>已审批数</span>
                <strong>{versions.filter((item) => item.status === 'approved').length}</strong>
              </section>
            </div>

            <div className="policy-version-center__content">
              <div className="event-table">
                <table>
                  <thead>
                    <tr>
                      <th>版本 ID</th>
                      <th>状态</th>
                      <th>环境</th>
                      <th>创建人</th>
                      <th>审批人</th>
                      <th>激活时间</th>
                      <th>操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    {versions.map((row) => {
                      const selected = row.id === selectedVersionEffective?.id
                      const pending = isActionPending(row.id)
                      const canApprove = row.status === 'draft'
                      const canActivate = row.status === 'approved'

                      return (
                        <tr key={row.id} data-selected={selected ? 'true' : 'false'}>
                          <td>
                            <button
                              type="button"
                              className={selected ? 'policy-select policy-select--active' : 'policy-select'}
                              onClick={() => {
                                setSelectedVersionID(row.id)
                                setActionError('')
                              }}
                            >
                              {row.id}
                            </button>
                          </td>
                          <td>
                            <span className={`status-pill ${row.status}`}>{row.status}</span>
                          </td>
                          <td>{row.environment || '—'}</td>
                          <td>{row.created_by || '—'}</td>
                          <td>{row.approved_by || '—'}</td>
                          <td>{formatDate(row.activated_at)}</td>
                          <td>
                            <div className="policy-actions">
                              <button
                                type="button"
                                className="rollouts-action"
                                disabled={!canApprove || pending}
                                onClick={() => {
                                  setActionMessage('')
                                  setActionError('')
                                  approveMutation.mutate(row)
                                }}
                              >
                                {pending && canApprove ? '审批中…' : '批准'}
                              </button>
                              <button
                                type="button"
                                className="rollouts-action"
                                disabled={!canActivate || pending}
                                onClick={() => {
                                  setActionMessage('')
                                  setActionError('')
                                  activateMutation.mutate(row)
                                }}
                              >
                                {pending && canActivate ? '激活中…' : '激活'}
                              </button>
                              {(row.status === 'approved' || row.status === 'active') ? (
                                  <Link
                                    className="rollouts-action"
                                    to={`/rollouts?policyVersionId=${encodeURIComponent(row.id)}&environment=${encodeURIComponent(row.environment || 'prod')}`}
                                  >
                                    查看灰度发布
                                  </Link>
                              ) : null}
                            </div>
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
                {versions.length === 0 ? <div className="config-table__state">当前没有策略版本。</div> : null}
              </div>

              <aside className="config-drawer" aria-label="Policy version diff">
                <div className="config-drawer__header">
                  <div>
                    <h2>版本 Diff</h2>
                    <p>
                      {selectedVersionEffective
                        ? `选中版本 ${selectedVersionEffective.id} 的差异内容`
                        : '请选择一个策略版本查看差异'}
                    </p>
                  </div>
                </div>

                {!selectedVersionEffective ? (
                  <div className="config-drawer__empty">暂无可展示的策略版本。</div>
                ) : null}

                {selectedVersionEffective && diffQuery.isLoading ? (
                  <div className="config-drawer__empty">正在加载版本差异…</div>
                ) : null}

                {selectedVersionEffective && diffQuery.error ? (
                  <div className="config-error policy-diff__error">
                    版本差异暂不可用（diff API 尚未就绪或返回异常）。
                  </div>
                ) : null}

                {selectedVersionEffective && !diffQuery.isLoading && !diffQuery.error ? (
                  <pre className="policy-diff-block" data-testid="policy-diff-content">
                    {formatDiffPayload(diffQuery.data)}
                  </pre>
                ) : null}
              </aside>
            </div>
          </>
        ) : null}
      </div>
    </AppShell>
  )
}
