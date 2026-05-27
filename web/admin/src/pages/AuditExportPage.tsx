import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { Badge } from '../components/ui/Badge'
import { exportAuditData, triggerCleanup, getRetentionPolicy } from '../lib/api/audit'

export function AuditExportPage() {
  const { t } = useTranslation()
  const [tenantID, setTenantID] = useState('')
  const [exportFormat, setExportFormat] = useState<'json' | 'csv'>('json')
  const [retentionDays, setRetentionDays] = useState(90)
  const [cleanupResult, setCleanupResult] = useState<{ deleted: number; retention_days: number } | null>(null)

  const { data: policy } = useQuery({
    queryKey: ['audit-retention'],
    queryFn: getRetentionPolicy,
  })

  const cleanupMutation = useMutation({
    mutationFn: (days: number) => triggerCleanup(days),
    onSuccess: (result) => setCleanupResult({ deleted: result.deleted, retention_days: result.retention_days }),
  })

  const handleExport = async () => {
    if (!tenantID) return
    try {
      const blob = await exportAuditData(tenantID, exportFormat)
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `audit-export-${tenantID}.${exportFormat}`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
    } catch {
      alert(t('auditExport.exportFailed'))
    }
  }

  return (
    <AppShell title={t('auditExport.pageTitle')} description={t('auditExport.pageDescription')}>
      <div className="channels-page">
        <div className="page-surface" style={{ marginBottom: '1rem', padding: '1rem' }}>
          <h3 style={{ marginBottom: '1rem' }}>{t('auditExport.dataExport')}</h3>
          <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr auto', gap: '1rem', alignItems: 'end' }}>
            <div>
              <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.85rem', color: '#64748b' }}>{t('auditExport.tenantId')}</label>
              <input
                type="text"
                value={tenantID}
                onChange={(e) => setTenantID(e.target.value)}
                placeholder={t('auditExport.tenantIdPlaceholder')}
                style={{ width: '100%', padding: '0.5rem', borderRadius: '4px', border: '1px solid #e2e8f0' }}
              />
            </div>
            <div>
              <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.85rem', color: '#64748b' }}>{t('auditExport.format')}</label>
              <select
                value={exportFormat}
                onChange={(e) => setExportFormat(e.target.value as 'json' | 'csv')}
                style={{ width: '100%', padding: '0.5rem', borderRadius: '4px', border: '1px solid #e2e8f0' }}
              >
                <option value="json">JSON</option>
                <option value="csv">CSV</option>
              </select>
            </div>
            <button
              type="button"
              className="btn btn--primary"
              disabled={!tenantID}
              onClick={handleExport}
            >
              {t('common.export')}
            </button>
          </div>
        </div>

        <div className="page-surface" style={{ marginBottom: '1rem', padding: '1rem' }}>
          <h3 style={{ marginBottom: '1rem' }}>{t('auditExport.retentionPolicy')}</h3>
          <div className="summary-card-grid" style={{ marginBottom: '1rem' }}>
            <div className="summary-card">
              <span>{t('auditExport.currentRetentionDays')}</span>
              <strong>{t('auditExport.days', { days: policy?.retention_days ?? 90 })}</strong>
            </div>
            {cleanupResult && (
              <div className="summary-card">
                <span>{t('auditExport.lastCleanup')}</span>
                <strong>{t('auditExport.deletedCount', { count: cleanupResult.deleted })}</strong>
              </div>
            )}
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr auto', gap: '1rem', alignItems: 'end' }}>
            <div>
              <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.85rem', color: '#64748b' }}>{t('auditExport.retentionDays')}</label>
              <input
                type="number"
                value={retentionDays}
                onChange={(e) => setRetentionDays(parseInt(e.target.value) || 90)}
                min={1}
                max={365}
                style={{ width: '100%', padding: '0.5rem', borderRadius: '4px', border: '1px solid #e2e8f0' }}
              />
            </div>
            <div>
              <Badge variant="warning">{t('auditExport.autoCleanupInfo')}</Badge>
            </div>
            <button
              type="button"
              className="btn btn--outline"
              disabled={cleanupMutation.isPending}
              onClick={() => cleanupMutation.mutate(retentionDays)}
            >
              {cleanupMutation.isPending ? t('auditExport.cleaningUp') : t('auditExport.cleanupNow')}
            </button>
          </div>
        </div>
      </div>
    </AppShell>
  )
}
