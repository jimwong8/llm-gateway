import { useTranslation } from 'react-i18next'
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
  const { t } = useTranslation()
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
      setActionMessage(t('policyVersions.approved', { id: data.id }))
      await versionsQuery.refetch()
    },
    onError: (error) => {
      setActionMessage('')
      setActionError(error instanceof Error ? error.message : t('policyVersions.approveFailed'))
    },
  })

  const activateMutation = useMutation({
    mutationFn: (version: PolicyVersionRow) => activatePolicyVersion(version.id),
    onSuccess: async (data) => {
      setActionError('')
      setActionMessage(t('policyVersions.activated', { id: data.id }))
      await versionsQuery.refetch()
    },
    onError: (error) => {
      setActionMessage('')
      setActionError(error instanceof Error ? error.message : t('policyVersions.activateFailed'))
    },
  })

  function isActionPending(versionID: string) {
    return approveMutation.isPending && approveMutation.variables?.id === versionID
      || activateMutation.isPending && activateMutation.variables?.id === versionID
  }

  return (
    <AppShell
      title={t('policyVersions.title')}
      description={t('policyVersions.description')}
    >
      <div className="policy-version-center">
        {versionsQuery.isLoading ? <div className="event-state">{t('policyVersions.loading')}</div> : null}
        {versionsQuery.error ? <div className="config-error">{t('policyVersions.loadError')}</div> : null}
        {actionError ? <div className="config-error">{t('policyVersions.actionError', { error: actionError })}</div> : null}
        {actionMessage ? <div className="event-state">{actionMessage}</div> : null}

        {!versionsQuery.isLoading && !versionsQuery.error ? (
          <>
            <div className="summary-card-grid">
              <section className="summary-card">
                <span>{t('policyVersions.totalVersions')}</span>
                <strong>{versions.length}</strong>
              </section>
              <section className="summary-card">
                <span>{t('policyVersions.activeVersion')}</span>
                <strong>{versions.find((item) => item.status === 'active')?.id ?? '—'}</strong>
              </section>
              <section className="summary-card">
                <span>{t('policyVersions.currentSelected')}</span>
                <strong>{selectedVersionEffective?.id ?? '—'}</strong>
              </section>
              <section className="summary-card">
                <span>{t('policyVersions.approvedCount')}</span>
                <strong>{versions.filter((item) => item.status === 'approved').length}</strong>
              </section>
            </div>

            <div className="policy-version-center__content">
              <div className="event-table">
                <table>
                  <thead>
                     <tr>
                       <th>{t('policyVersions.colVersionId')}</th>
                       <th>{t('policyVersions.colStatus')}</th>
                       <th>{t('policyVersions.colEnvironment')}</th>
                       <th>{t('policyVersions.colCreatedBy')}</th>
                       <th>{t('policyVersions.colApprovedBy')}</th>
                       <th>{t('policyVersions.colActivatedAt')}</th>
                       <th>{t('policyVersions.colActions')}</th>
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
                                 {pending && canApprove ? t('policyVersions.approving') : t('policyVersions.approve')}
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
                                 {pending && canActivate ? t('policyVersions.activating') : t('policyVersions.activate')}
                              </button>
                              {(row.status === 'approved' || row.status === 'active') ? (
                                  <Link
                                    className="rollouts-action"
                                    to={`/rollouts?policyVersionId=${encodeURIComponent(row.id)}&environment=${encodeURIComponent(row.environment || 'prod')}`}
                                  >
                                     {t('policyVersions.viewRollout')}
                                  </Link>
                              ) : null}
                            </div>
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
                 {versions.length === 0 ? <div className="config-table__state">{t('policyVersions.noVersions')}</div> : null}
              </div>

              <aside className="config-drawer" aria-label="Policy version diff">
                <div className="config-drawer__header">
                  <div>
                    <h2>{t('policyVersions.versionDiff')}</h2>
                    <p>
                      {selectedVersionEffective
                        ? t('policyVersions.selectedDiff', { id: selectedVersionEffective.id })
                        : t('policyVersions.selectVersionPrompt')}
                    </p>
                  </div>
                </div>

                {!selectedVersionEffective ? (
                  <div className="config-drawer__empty">{t('policyVersions.noVersions')}</div>
                ) : null}

                {selectedVersionEffective && diffQuery.isLoading ? (
                  <div className="config-drawer__empty">{t('policyVersions.loadingDiff')}</div>
                ) : null}

                {selectedVersionEffective && diffQuery.error ? (
                  <div className="config-error policy-diff__error">
                    {t('policyVersions.diffUnavailable')}
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
