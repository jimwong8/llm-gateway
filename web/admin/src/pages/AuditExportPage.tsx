import { useState } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { Badge } from '../components/ui/Badge'
import { exportAuditData, triggerCleanup, getRetentionPolicy } from '../lib/api/audit'

export function AuditExportPage() {
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
      alert('导出失败，请检查租户 ID 是否正确')
    }
  }

  return (
    <AppShell title="审计与合规" description="审计日志导出、数据保留策略管理。">
      <div className="channels-page">
        <div className="page-surface" style={{ marginBottom: '1rem', padding: '1rem' }}>
          <h3 style={{ marginBottom: '1rem' }}>数据导出</h3>
          <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr auto', gap: '1rem', alignItems: 'end' }}>
            <div>
              <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.85rem', color: '#64748b' }}>租户 ID</label>
              <input
                type="text"
                value={tenantID}
                onChange={(e) => setTenantID(e.target.value)}
                placeholder="输入租户 ID..."
                style={{ width: '100%', padding: '0.5rem', borderRadius: '4px', border: '1px solid #e2e8f0' }}
              />
            </div>
            <div>
              <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.85rem', color: '#64748b' }}>格式</label>
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
              导出
            </button>
          </div>
        </div>

        <div className="page-surface" style={{ marginBottom: '1rem', padding: '1rem' }}>
          <h3 style={{ marginBottom: '1rem' }}>数据保留策略</h3>
          <div className="summary-card-grid" style={{ marginBottom: '1rem' }}>
            <div className="summary-card">
              <span>当前保留天数</span>
              <strong>{policy?.retention_days ?? 90} 天</strong>
            </div>
            {cleanupResult && (
              <div className="summary-card">
                <span>上次清理</span>
                <strong>{cleanupResult.deleted} 条</strong>
              </div>
            )}
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr auto', gap: '1rem', alignItems: 'end' }}>
            <div>
              <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.85rem', color: '#64748b' }}>保留天数</label>
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
              <Badge variant="warning">自动清理每 24 小时执行</Badge>
            </div>
            <button
              type="button"
              className="btn btn--outline"
              disabled={cleanupMutation.isPending}
              onClick={() => cleanupMutation.mutate(retentionDays)}
            >
              {cleanupMutation.isPending ? '清理中...' : '立即清理'}
            </button>
          </div>
        </div>
      </div>
    </AppShell>
  )
}
