import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { Badge } from '../components/ui/Badge'
import { EmptyState } from '../components/ui/EmptyState'
import { TableSkeleton } from '../components/ui/Skeleton'
import type { ApiKey } from '../types/identity'
import { listApiKeys, createApiKey, revokeApiKey } from '../lib/api/identity'

export function ApiKeysPage() {
  const queryClient = useQueryClient()
  const [showNewKey, setShowNewKey] = useState<string | null>(null)
  const [keyName, setKeyName] = useState('default')

  const { data, isLoading } = useQuery({
    queryKey: ['user-api-keys'],
    queryFn: listApiKeys,
    refetchInterval: 30_000,
  })

  const createMutation = useMutation({
    mutationFn: (name: string) => createApiKey({ name }),
    onSuccess: (res) => {
      setShowNewKey(res.key)
      setKeyName('default')
      queryClient.invalidateQueries({ queryKey: ['user-api-keys'] })
    },
  })

  const revokeMutation = useMutation({
    mutationFn: (id: number) => revokeApiKey(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['user-api-keys'] }),
  })

  const keys = data?.data ?? []

  return (
    <AppShell title="API 密钥管理" description="管理你的个人 API 密钥，用于访问 LLM Gateway 服务。">
      <div className="channels-page">
        <div className="channels-toolbar" style={{ marginBottom: '1rem' }}>
          <div style={{ display: 'flex', gap: '1rem', alignItems: 'center', flexWrap: 'wrap' }}>
            <input
              type="text"
              placeholder="密钥名称"
              value={keyName}
              onChange={(e) => setKeyName(e.target.value)}
              style={{ padding: '0.5rem', borderRadius: '4px', border: '1px solid #e2e8f0', minWidth: '200px' }}
            />
            <button
              type="button"
              className="btn btn--primary"
              disabled={createMutation.isPending}
              onClick={() => createMutation.mutate(keyName)}
            >
              {createMutation.isPending ? '生成中...' : '创建新密钥'}
            </button>
          </div>
        </div>

        {showNewKey && (
          <div className="page-surface" style={{ marginBottom: '1rem', padding: '1rem', border: '1px solid #22c55e' }}>
            <p style={{ marginBottom: '0.5rem', fontWeight: 600, color: '#166534' }}>
              新密钥已创建 — 请立即复制，关闭后将不再显示：
            </p>
            <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
              <code style={{
                flex: 1, padding: '0.75rem', background: '#f0fdf4', borderRadius: '4px',
                border: '1px solid #bbf7d0', wordBreak: 'break-all', fontSize: '0.9rem',
              }}>
                {showNewKey}
              </code>
              <button
                type="button"
                className="btn btn--sm btn--primary"
                onClick={() => { navigator.clipboard.writeText(showNewKey); setShowNewKey(null) }}
              >
                已复制
              </button>
            </div>
          </div>
        )}

        {isLoading ? (
          <TableSkeleton rows={5} />
        ) : keys.length === 0 ? (
          <EmptyState title="暂无 API 密钥" description="点击「创建新密钥」生成第一个密钥。" />
        ) : (
          <table className="channels-table">
            <thead>
              <tr>
                <th>名称</th>
                <th>前缀</th>
                <th>状态</th>
                <th>RPM 限制</th>
                <th>请求数</th>
                <th>总 Token</th>
                <th>总费用</th>
                <th>平均延迟</th>
                <th>最近使用</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {keys.map((key) => (
                <tr key={key.id}>
                  <td>{key.name}</td>
                  <td><code>{key.key_prefix}...</code></td>
                  <td><Badge variant={key.status === 'active' ? 'success' : 'danger'}>{key.status}</Badge></td>
                  <td style={{ fontSize: '0.85rem' }}>{key.rpm_limit}</td>
                  <td style={{ fontSize: '0.85rem' }}>{key.usage?.total_requests ?? '-'}</td>
                  <td style={{ fontSize: '0.85rem' }}>{key.usage ? key.usage.total_tokens.toLocaleString() : '-'}</td>
                  <td style={{ fontSize: '0.85rem' }}>{key.usage ? `$${key.usage.total_cost.toFixed(4)}` : '-'}</td>
                  <td style={{ fontSize: '0.85rem' }}>{key.usage ? `${key.usage.avg_latency_ms.toFixed(0)}ms` : '-'}</td>
                  <td style={{ color: '#94a3b8', fontSize: '0.85rem' }}>
                    {key.last_used_at ? new Date(key.last_used_at).toLocaleString('zh-CN') : '从未使用'}
                  </td>
                  <td>
                    <button
                      type="button"
                      className="btn btn--sm btn--danger-ghost"
                      disabled={revokeMutation.isPending}
                      onClick={() => {
                        if (confirm(`确认吊销密钥「${key.name}」(ID: ${key.id})？`)) {
                          revokeMutation.mutate(key.id)
                        }
                      }}
                    >
                      吊销
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
