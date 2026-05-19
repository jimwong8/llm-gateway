import { useState, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { Badge } from '../components/ui/Badge'
import { EmptyState } from '../components/ui/EmptyState'
import { TableSkeleton } from '../components/ui/Skeleton'
import { listAssets, getAssetStats, deleteAsset } from '../lib/api/assets'
import type { Asset, AssetStats } from '../lib/api/assets'

const TASK_VARIANT: Record<string, 'success' | 'warning' | 'danger' | 'info'> = {
  code: 'success',
  analysis: 'warning',
  general: 'info',
}

export function AssetsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [keyword, setKeyword] = useState('')
  const [taskFilter, setTaskFilter] = useState<string>('all')
  const [page, setPage] = useState(0)
  const pageSize = 20

  const { data: assetsData, isLoading } = useQuery({
    queryKey: ['assets', keyword, taskFilter, page],
    queryFn: () => listAssets({
      keyword: keyword || undefined,
      task_type: taskFilter !== 'all' ? taskFilter : undefined,
      limit: pageSize,
      offset: page * pageSize,
    }),
    refetchInterval: 30_000,
  })

  const { data: stats, isLoading: statsLoading } = useQuery({
    queryKey: ['assets-stats'],
    queryFn: getAssetStats,
    refetchInterval: 60_000,
  })

  const deleteMutation = useMutation({
    mutationFn: deleteAsset,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['assets'] })
      queryClient.invalidateQueries({ queryKey: ['assets-stats'] })
    },
  })

  const assets = assetsData?.data ?? []
  const total = assetsData?.total ?? assets.length

  const filteredAssets = useMemo(() => {
    return assets.filter((a) => {
      if (taskFilter !== 'all' && a.task_type !== taskFilter) return false
      if (keyword && !a.title.toLowerCase().includes(keyword.toLowerCase()) && !a.summary.toLowerCase().includes(keyword.toLowerCase())) return false
      return true
    })
  }, [assets, taskFilter, keyword])

  const taskTypes = useMemo(() => {
    const types = new Set<string>()
    assets.forEach((a) => types.add(a.task_type))
    return Array.from(types).sort()
  }, [assets])

  return (
    <AppShell
      title={t('assets.pageTitle')}
      description={t('assets.pageDescription')}
    >
      <div className="channels-page">
        <div className="channels-toolbar" style={{ marginBottom: '1rem' }}>
          <div style={{ display: 'flex', gap: '1rem', alignItems: 'center', flexWrap: 'wrap' }}>
            <input
              type="text"
              placeholder={t('assets.searchPlaceholder')}
              value={keyword}
              onChange={(e) => { setKeyword(e.target.value); setPage(0) }}
              style={{ padding: '0.5rem', borderRadius: '4px', border: '1px solid #e2e8f0', minWidth: '200px' }}
            />
            <select
              value={taskFilter}
              onChange={(e) => { setTaskFilter(e.target.value); setPage(0) }}
              style={{ padding: '0.5rem', borderRadius: '4px', border: '1px solid #e2e8f0' }}
            >
              <option value="all">{t('assets.typeAll')}</option>
              {taskTypes.map((t) => (
                <option key={t} value={t}>{t}</option>
              ))}
            </select>
          </div>
        </div>

        {statsLoading ? null : stats ? (
          <div className="summary-card-grid" style={{ marginBottom: '1rem' }}>
            <div className="summary-card">
              <span>{t('assets.totalAssets')}</span>
              <strong>{stats.total_assets}</strong>
            </div>
            <div className="summary-card">
              <span>{t('assets.totalHits')}</span>
              <strong>{stats.total_hits}</strong>
            </div>
            {stats.by_task.slice(0, 3).map((t) => (
              <div key={t.task_type} className="summary-card">
                <span>{t.task_type}</span>
                <strong>{t.count}</strong>
              </div>
            ))}
          </div>
        ) : null}

        {isLoading ? (
          <TableSkeleton rows={5} />
        ) : filteredAssets.length === 0 ? (
          <EmptyState title={t('assets.emptyTitle')} description={t('assets.emptyDescription')} />
        ) : (
          <>
            <table className="channels-table">
              <thead>
            <tr>
                   <th>{t('assets.id')}</th>
                   <th>{t('assets.title')}</th>
                   <th>{t('assets.type')}</th>
                   <th>{t('assets.sourceModel')}</th>
                   <th>{t('assets.hitCount')}</th>
                   <th>{t('assets.createdAt')}</th>
                   <th>{t('assets.actions')}</th>
                 </tr>
              </thead>
              <tbody>
                {filteredAssets.map((asset) => (
                  <tr key={asset.id}>
                    <td style={{ color: '#94a3b8', fontSize: '0.85rem' }}>{asset.id}</td>
                    <td>
                      <span className="channel-name" title={asset.summary}>
                        {asset.title.length > 40 ? asset.title.slice(0, 40) + '…' : asset.title}
                      </span>
                    </td>
                    <td>
                      <Badge variant={TASK_VARIANT[asset.task_type] || 'info'}>{asset.task_type}</Badge>
                    </td>
                    <td>{asset.source_model}</td>
                    <td>{asset.hit_count}</td>
                    <td style={{ color: '#94a3b8', fontSize: '0.85rem' }}>
                      {new Date(asset.created_at).toLocaleDateString('zh-CN')}
                    </td>
                    <td>
                      <button
                        type="button"
                        className="btn btn--sm btn--danger-ghost"
                        disabled={deleteMutation.isPending}
                        onClick={() => {
                          if (confirm(t('assets.confirmDelete', { id: asset.id, title: asset.title }))) {
                            deleteMutation.mutate(asset.id)
                          }
                        }}
                      >
                        {t('assets.delete')}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>

            {total > pageSize && (
              <div style={{ display: 'flex', justifyContent: 'center', gap: '0.5rem', marginTop: '1rem' }}>
                <button
                  type="button"
                  className="btn btn--outline btn--sm"
                  disabled={page === 0}
                  onClick={() => setPage((p) => Math.max(0, p - 1))}
                >
                  {t('common.prev')}
                </button>
                <span style={{ padding: '0.5rem 1rem', color: '#94a3b8' }}>
                  {t('assets.pageInfo', { page: page + 1, total: Math.ceil(total / pageSize) })}
                </span>
                <button
                  type="button"
                  className="btn btn--outline btn--sm"
                  disabled={(page + 1) * pageSize >= total}
                  onClick={() => setPage((p) => p + 1)}
                >
                  {t('common.next')}
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </AppShell>
  )
}
