import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { Badge } from '../components/ui/Badge'
import { StatusDot } from '../components/ui/StatusDot'
import { EmptyState } from '../components/ui/EmptyState'
import { TableSkeleton } from '../components/ui/Skeleton'
import { ChannelFormModal } from './ChannelFormModal'
import { listChannels, deleteChannel, testChannel, batchDeleteChannels, batchUpdateChannelsStatus } from '../lib/channels'
import type { Channel, ChannelStatus } from '../types/channel'

const statusBadgeVariant: Record<ChannelStatus, 'success' | 'warning' | 'danger' | 'info'> = {
  active: 'success',
  inactive: 'info',
  error: 'danger',
  maintenance: 'warning',
}

const statusDotMap: Record<ChannelStatus, 'healthy' | 'disabled' | 'error'> = {
  active: 'healthy',
  inactive: 'disabled',
  error: 'error',
  maintenance: 'disabled',
}

export function ChannelsPage() {
  const queryClient = useQueryClient()
  const [showForm, setShowForm] = useState(false)
  const [editingChannel, setEditingChannel] = useState<Channel | null>(null)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [statusFilter, setStatusFilter] = useState<ChannelStatus | 'all'>('all')
  const [searchQuery, setSearchQuery] = useState('')

  const { data: channels, isLoading } = useQuery({
    queryKey: ['channels'],
    queryFn: listChannels,
    refetchInterval: 30_000,
  })

  const deleteMutation = useMutation({
    mutationFn: deleteChannel,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['channels'] }),
  })

  const testMutation = useMutation({
    mutationFn: testChannel,
  })

  const batchDeleteMutation = useMutation({
    mutationFn: batchDeleteChannels,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['channels'] })
      setSelected(new Set())
    },
  })

  const batchStatusMutation = useMutation({
    mutationFn: ({ ids, status }: { ids: string[]; status: ChannelStatus }) =>
      batchUpdateChannelsStatus(ids, status),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['channels'] })
      setSelected(new Set())
    },
  })

  const filtered = (channels ?? []).filter((ch) => {
    if (statusFilter !== 'all' && ch.status !== statusFilter) return false
    if (searchQuery && !ch.name.toLowerCase().includes(searchQuery.toLowerCase())) return false
    return true
  })

  const toggleSelect = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const toggleSelectAll = () => {
    if (selected.size === filtered.length) {
      setSelected(new Set())
    } else {
      setSelected(new Set(filtered.map((ch) => ch.id)))
    }
  }

  return (
    <AppShell title="渠道管理" description="管理 LLM 供应商渠道，配置路由优先级和权重">
      <div className="channels-page">
        {/* Toolbar */}
        <div className="channels-toolbar">
          <div className="channels-toolbar__left">
            <input
              type="text"
              placeholder="搜索渠道名称..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="channels-search"
            />
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value as ChannelStatus | 'all')}
              className="channels-filter"
            >
              <option value="all">全部状态</option>
              <option value="active">启用</option>
              <option value="inactive">停用</option>
              <option value="error">异常</option>
              <option value="maintenance">维护中</option>
            </select>
          </div>
          <div className="channels-toolbar__right">
            {selected.size > 0 ? (
              <>
                <span className="channels-batch-count">已选 {selected.size} 项</span>
                <button
                  type="button"
                  className="btn btn--outline"
                  onClick={() =>
                    batchStatusMutation.mutate({ ids: Array.from(selected), status: 'active' })
                  }
                >
                  批量启用
                </button>
                <button
                  type="button"
                  className="btn btn--outline"
                  onClick={() =>
                    batchStatusMutation.mutate({ ids: Array.from(selected), status: 'inactive' })
                  }
                >
                  批量停用
                </button>
                <button
                  type="button"
                  className="btn btn--danger"
                  onClick={() => {
                    if (confirm(`确定删除选中的 ${selected.size} 个渠道？`)) {
                      batchDeleteMutation.mutate(Array.from(selected))
                    }
                  }}
                >
                  批量删除
                </button>
              </>
            ) : null}
            <button
              type="button"
              className="btn btn--primary"
              onClick={() => {
                setEditingChannel(null)
                setShowForm(true)
              }}
            >
              + 添加渠道
            </button>
          </div>
        </div>

        {/* Table */}
        {isLoading ? (
          <div className="channels-loading">
            <TableSkeleton rows={5} cols={6} />
          </div>
        ) : filtered.length === 0 ? (
          <EmptyState
            title="暂无渠道"
            description="添加第一个 LLM 供应商渠道以开始使用"
            action={{ label: '添加渠道', onClick: () => setShowForm(true) }}
          />
        ) : (
          <div className="channels-table">
            <table>
              <thead>
                <tr>
                  <th className="channels-table__check">
                    <input
                      type="checkbox"
                      checked={selected.size === filtered.length && filtered.length > 0}
                      onChange={toggleSelectAll}
                    />
                  </th>
                  <th>名称</th>
                  <th>供应商</th>
                  <th>状态</th>
                  <th>优先级</th>
                  <th>权重</th>
                  <th>延迟</th>
                  <th>请求数</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((ch) => (
                  <tr key={ch.id} className={selected.has(ch.id) ? 'row--selected' : ''}>
                    <td>
                      <input
                        type="checkbox"
                        checked={selected.has(ch.id)}
                        onChange={() => toggleSelect(ch.id)}
                      />
                    </td>
                    <td>
                      <span className="channel-name">{ch.name}</span>
                    </td>
                    <td>
                      <Badge variant="info">{ch.provider}</Badge>
                    </td>
                    <td>
                      <StatusDot status={statusDotMap[ch.status]} label={ch.status} />
                    </td>
                    <td>
                      <Badge variant={ch.priority === 'highest' || ch.priority === 'high' ? 'warning' : 'default'}>
                        {ch.priority}
                      </Badge>
                    </td>
                    <td>{ch.weight}</td>
                    <td>{ch.latencyMs ? `${ch.latencyMs}ms` : '-'}</td>
                    <td>{ch.totalRequests ?? 0}</td>
                    <td>
                      <div className="channels-actions">
                        <button
                          type="button"
                          className="btn btn--sm"
                          onClick={() => {
                            setEditingChannel(ch)
                            setShowForm(true)
                          }}
                        >
                          编辑
                        </button>
                        <button
                          type="button"
                          className="btn btn--sm btn--outline"
                          onClick={async () => {
                            const result = await testMutation.mutateAsync(ch.id)
                            alert(result.success ? '测试成功' : `测试失败: ${result.error}`)
                          }}
                          disabled={testMutation.isPending}
                        >
                          测试
                        </button>
                        <button
                          type="button"
                          className="btn btn--sm btn--danger-ghost"
                          onClick={() => {
                            if (confirm(`确定删除渠道 "${ch.name}"？`)) {
                              deleteMutation.mutate(ch.id)
                            }
                          }}
                        >
                          删除
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {showForm ? (
        <ChannelFormModal
          channel={editingChannel}
          onClose={() => {
            setShowForm(false)
            setEditingChannel(null)
          }}
        />
      ) : null}
    </AppShell>
  )
}
