import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { Badge } from '../components/ui/Badge'
import { StatusDot } from '../components/ui/StatusDot'
import { EmptyState } from '../components/ui/EmptyState'
import { TableSkeleton } from '../components/ui/Skeleton'
import { ChannelFormModal } from './ChannelFormModal'
import { listChannels, deleteChannel, testChannel, batchDeleteChannels, batchUpdateChannelsStatus } from '../lib/channels'
import type { Channel, ChannelStatus } from '../types/channel'

const statusBadgeVariant: Record<string, 'success' | 'warning' | 'danger' | 'info'> = {
  active: 'success',
  inactive: 'info',
  error: 'danger',
  maintenance: 'warning',
}

const statusDotMap: Record<string, 'healthy' | 'disabled' | 'error'> = {
  active: 'healthy',
  inactive: 'disabled',
  error: 'error',
  maintenance: 'disabled',
}

export function ChannelsPage() {
  const { t } = useTranslation()
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
    <AppShell title={t('channels.pageTitle')} description={t('channels.pageDescription')}>
      <div className="channels-page">
        {/* Toolbar */}
        <div className="channels-toolbar">
          <div className="channels-toolbar__left">
            <input
              type="text"
              placeholder={t('channels.searchPlaceholder')}
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="channels-search"
            />
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value as ChannelStatus | 'all')}
              className="channels-filter"
            >
              <option value="all">{t('channels.statusAll')}</option>
              <option value="active">{t('channels.statusActive')}</option>
              <option value="inactive">{t('channels.statusInactive')}</option>
              <option value="error">{t('channels.statusError')}</option>
              <option value="maintenance">{t('channels.statusMaintenance')}</option>
            </select>
          </div>
          <div className="channels-toolbar__right">
            {selected.size > 0 ? (
              <>
                <span className="channels-batch-count">{t('channels.selectedCount', { count: selected.size })}</span>
                <button
                  type="button"
                  className="btn btn--outline"
                  onClick={() =>
                    batchStatusMutation.mutate({ ids: Array.from(selected), status: 'active' })
                  }
                >
                  {t('channels.batchEnable')}
                </button>
                <button
                  type="button"
                  className="btn btn--outline"
                  onClick={() =>
                    batchStatusMutation.mutate({ ids: Array.from(selected), status: 'inactive' })
                  }
                >
                  {t('channels.batchDisable')}
                </button>
                <button
                  type="button"
                  className="btn btn--danger"
                  onClick={() => {
                    if (confirm(t('channels.confirmBatchDelete', { count: selected.size }))) {
                      batchDeleteMutation.mutate(Array.from(selected))
                    }
                  }}
                >
                  {t('channels.batchDelete')}
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
              + {t('channels.addChannel')}
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
            title={t('channels.emptyTitle')}
            description={t('channels.emptyDescription')}
            action={{ label: t('channels.addChannel'), onClick: () => setShowForm(true) }}
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
                  <th>{t('channels.name')}</th>
                  <th>{t('channels.provider')}</th>
                  <th>{t('channels.status')}</th>
                  <th>{t('channels.priority')}</th>
                  <th>{t('channels.weight')}</th>
                  <th>{t('channels.latency')}</th>
                  <th>{t('channels.requests')}</th>
                  <th>{t('channels.actions')}</th>
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
                     <td>{ch.latency_ms ? `${ch.latency_ms}ms` : '-'}</td>
                     <td>{ch.total_requests ?? 0}</td>
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
                             alert(result.success ? t('channels.testSuccess') : `${t('channels.testFailed')}: ${result.error}`)
                           }}
                           disabled={testMutation.isPending}
                         >
                           {t('channels.test')}
                         </button>
                         <button
                           type="button"
                           className="btn btn--sm btn--danger-ghost"
                           onClick={() => {
                             if (confirm(t('channels.confirmDelete', { name: ch.name }))) {
                               deleteMutation.mutate(ch.id)
                             }
                           }}
                         >
                           {t('channels.delete')}
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
