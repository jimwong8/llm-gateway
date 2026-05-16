import { useState, useMemo } from 'react'
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
      title="资产管理"
      description="浏览和管理知识资产，包括标准化摘要、结构化抽取和复用审计。"
    >
      <div className="channels-page">
        <div className="channels-toolbar" style={{ marginBottom: '1rem' }}>
          <div style={{ display: 'flex', gap: '1rem', alignItems: 'center', flexWrap: 'wrap' }}>
            <input
              type="text"
              placeholder="搜索资产标题或内容..."
              value={keyword}
              onChange={(e) => { setKeyword(e.target.value); setPage(0) }}
              style={{ padding: '0.5rem', borderRadius: '4px', border: '1px solid #e2e8f0', minWidth: '200px' }}
            />
            <select
              value={taskFilter}
              onChange={(e) => { setTaskFilter(e.target.value); setPage(0) }}
              style={{ padding: '0.5rem', borderRadius: '4px', border: '1px solid #e2e8f0' }}
            >
              <option value="all">全部类型</option>
              {taskTypes.map((t) => (
                <option key={t} value={t}>{t}</option>
              ))}
            </select>
          </div>
        </div>

        {statsLoading ? null : stats ? (
          <div className="summary-card-grid" style={{ marginBottom: '1rem' }}>
            <div className="summary-card">
              <span>总资产数</span>
              <strong>{stats.total_assets}</strong>
            </div>
            <div className="summary-card">
              <span>总命中次数</span>
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
          <EmptyState title="暂无资产" description="还没有创建任何知识资产。资产会在请求处理过程中自动生成。" />
        ) : (
          <>
            <table className="channels-table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>标题</th>
                  <th>类型</th>
                  <th>来源模型</th>
                  <th>命中次数</th>
                  <th>创建时间</th>
                  <th>操作</th>
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
                          if (confirm(`确认删除资产 #${asset.id} "${asset.title}"？`)) {
                            deleteMutation.mutate(asset.id)
                          }
                        }}
                      >
                        删除
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
                  上一页
                </button>
                <span style={{ padding: '0.5rem 1rem', color: '#94a3b8' }}>
                  第 {page + 1} 页 / 共 {Math.ceil(total / pageSize)} 页
                </span>
                <button
                  type="button"
                  className="btn btn--outline btn--sm"
                  disabled={(page + 1) * pageSize >= total}
                  onClick={() => setPage((p) => p + 1)}
                >
                  下一页
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </AppShell>
  )
}
