import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { Badge } from '../components/ui/Badge'
import { EmptyState } from '../components/ui/EmptyState'
import { TableSkeleton } from '../components/ui/Skeleton'
import { listTenantKeys, putTenantKey, deleteTenantKey } from '../lib/api/tenant-keys'
import type { TenantKey } from '../lib/api/tenant-keys'

const PROVIDERS = ['openai', 'anthropic', 'google', 'azure', 'xstx', 'custom']

export function TenantKeysPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [showForm, setShowForm] = useState(false)
  const [tenantID, setTenantID] = useState('')
  const [provider, setProvider] = useState('openai')
  const [apiKey, setApiKey] = useState('')
  const [searchTenant, setSearchTenant] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['tenant-keys', searchTenant],
    queryFn: () => listTenantKeys(searchTenant || undefined),
    refetchInterval: 30_000,
  })

  const putMutation = useMutation({
    mutationFn: putTenantKey,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tenant-keys'] })
      setShowForm(false)
      setTenantID('')
      setProvider('openai')
      setApiKey('')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: ({ tenantID, provider }: { tenantID: string; provider: string }) => deleteTenantKey(tenantID, provider),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['tenant-keys'] }),
  })

  const keys = data?.data ?? []

  return (
    <AppShell title={t('tenantKeys.pageTitle')} description={t('tenantKeys.pageDescription')}>
      <div className="channels-page">
        <div className="channels-toolbar" style={{ marginBottom: '1rem' }}>
          <div style={{ display: 'flex', gap: '1rem', alignItems: 'center', flexWrap: 'wrap' }}>
            <input
              type="text"
              placeholder={t('tenantKeys.searchPlaceholder')}
              value={searchTenant}
              onChange={(e) => setSearchTenant(e.target.value)}
              style={{ padding: '0.5rem', borderRadius: '4px', border: '1px solid #e2e8f0', minWidth: '200px' }}
            />
            <button type="button" className="btn btn--primary" onClick={() => setShowForm(!showForm)}>
              {showForm ? t('common.cancel') : t('tenantKeys.addKey')}
            </button>
          </div>
        </div>

        {showForm && (
          <div className="page-surface" style={{ marginBottom: '1rem', padding: '1rem' }}>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr auto', gap: '1rem', alignItems: 'end' }}>
              <div>
                <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.85rem', color: '#64748b' }}>{t('tenantKeys.tenantId')}</label>
                <input
                  type="text"
                  value={tenantID}
                  onChange={(e) => setTenantID(e.target.value)}
                  placeholder={t('tenantKeys.tenantIdPlaceholder')}
                  style={{ width: '100%', padding: '0.5rem', borderRadius: '4px', border: '1px solid #e2e8f0' }}
                />
              </div>
              <div>
                <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.85rem', color: '#64748b' }}>{t('tenantKeys.provider')}</label>
                <select
                  value={provider}
                  onChange={(e) => setProvider(e.target.value)}
                  style={{ width: '100%', padding: '0.5rem', borderRadius: '4px', border: '1px solid #e2e8f0' }}
                >
                  {PROVIDERS.map((p) => (
                    <option key={p} value={p}>{p}</option>
                  ))}
                </select>
              </div>
              <div>
                <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.85rem', color: '#64748b' }}>{t('tenantKeys.apiKey')}</label>
                <input
                  type="password"
                  value={apiKey}
                  onChange={(e) => setApiKey(e.target.value)}
                  placeholder={t('tenantKeys.apiKeyPlaceholder')}
                  style={{ width: '100%', padding: '0.5rem', borderRadius: '4px', border: '1px solid #e2e8f0' }}
                />
              </div>
              <button
                type="button"
                className="btn btn--primary"
                disabled={putMutation.isPending || !tenantID || !apiKey}
                onClick={() => putMutation.mutate({ tenant_id: tenantID, provider, api_key: apiKey })}
              >
                {putMutation.isPending ? t('tenantKeys.saving') : t('common.save')}
              </button>
            </div>
          </div>
        )}

        {isLoading ? (
          <TableSkeleton rows={5} />
        ) : keys.length === 0 ? (
          <EmptyState title={t('tenantKeys.emptyTitle')} description={t('tenantKeys.emptyDescription')} />
        ) : (
          <table className="channels-table">
            <thead>
            <tr>
                 <th>{t('tenantKeys.tenantId')}</th>
                 <th>{t('tenantKeys.provider')}</th>
                 <th>{t('tenantKeys.status')}</th>
                 <th>{t('tenantKeys.createdAt')}</th>
                 <th>{t('tenantKeys.actions')}</th>
               </tr>
            </thead>
            <tbody>
              {keys.map((key: TenantKey) => (
                <tr key={`${key.tenant_id}-${key.provider}`}>
                  <td>{key.tenant_id}</td>
                  <td><Badge variant="info">{key.provider}</Badge></td>
                  <td>
                     <Badge variant={key.is_active ? 'success' : 'danger'}>
                       {key.is_active ? t('tenantKeys.active') : t('tenantKeys.disabled')}
                     </Badge>
                  </td>
                  <td style={{ color: '#94a3b8', fontSize: '0.85rem' }}>
                    {new Date(key.created_at).toLocaleDateString('zh-CN')}
                  </td>
                  <td>
                    <button
                      type="button"
                      className="btn btn--sm btn--danger-ghost"
                      disabled={deleteMutation.isPending}
                      onClick={() => {
                        if (confirm(t('tenantKeys.confirmDelete', { tenant: key.tenant_id, provider: key.provider }))) {
                          deleteMutation.mutate({ tenantID: key.tenant_id, provider: key.provider })
                        }
                      }}
                    >
                      {t('tenantKeys.delete')}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </AppShell>
  )
}
