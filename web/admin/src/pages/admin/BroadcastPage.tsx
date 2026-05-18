import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { AppShell } from '../../components/layout/AppShell'
import { listBroadcasts, createBroadcast, updateBroadcast, deleteBroadcast } from '../../lib/api/broadcasts'
import type { Broadcast, BroadcastType } from '../../types/broadcast'

const NOW = (): string => new Date().toISOString()
const IN_24H = (): string => new Date(Date.now() + 86400000).toISOString()

function formatDT(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false })
}

const TYPE_LABELS: Record<BroadcastType, string> = { info: '信息', warning: '警告', critical: '严重' }
const TYPE_CLASSES: Record<BroadcastType, string> = { info: 'badge-info', warning: 'badge-warning', critical: 'badge-critical' }

type FormMode = 'create' | 'edit'

export function BroadcastPage() {
  const queryClient = useQueryClient()
  const [mode, setMode] = useState<FormMode>('create')
  const [editId, setEditId] = useState<number | null>(null)
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [bType, setBType] = useState<BroadcastType>('info')
  const [startAt, setStartAt] = useState(NOW)
  const [endAt, setEndAt] = useState(IN_24H)
  const [formError, setFormError] = useState('')

  const listQuery = useQuery<{ object: string; data: Broadcast[] }>({
    queryKey: ['admin-broadcasts'],
    queryFn: listBroadcasts,
    refetchInterval: 30000,
  })

  const createMut = useMutation({
    mutationFn: createBroadcast,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-broadcasts'] })
      resetForm()
    },
    onError: (err: Error) => setFormError(err.message),
  })

  const updateMut = useMutation({
    mutationFn: ({ id, input }: { id: number; input: Parameters<typeof updateBroadcast>[1] }) => updateBroadcast(id, input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-broadcasts'] })
      resetForm()
    },
    onError: (err: Error) => setFormError(err.message),
  })

  const deleteMut = useMutation({
    mutationFn: deleteBroadcast,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin-broadcasts'] }),
    onError: (err: Error) => setFormError(err.message),
  })

  function resetForm() {
    setMode('create')
    setEditId(null)
    setTitle('')
    setContent('')
    setBType('info')
    setStartAt(NOW)
    setEndAt(IN_24H)
    setFormError('')
  }

  function startEdit(b: Broadcast) {
    setMode('edit')
    setEditId(b.id)
    setTitle(b.title)
    setContent(b.content)
    setBType(b.type)
    setStartAt(b.start_at)
    setEndAt(b.end_at)
    setFormError('')
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!title.trim() || !content.trim()) {
      setFormError('标题和内容不能为空')
      return
    }
    const input = { title: title.trim(), content: content.trim(), type: bType, start_at: startAt, end_at: endAt }
    if (mode === 'create') {
      createMut.mutate(input)
    } else if (editId !== null) {
      updateMut.mutate({ id: editId, input })
    }
  }

  const pending = createMut.isPending || updateMut.isPending

  return (
    <AppShell title="广播管理" description="管理系统广播通知，可创建信息、警告、紧急类型的广播消息。">
      <div className="page-header">
        <h2>广播管理</h2>
      </div>

      <div className="page-surface" style={{ marginBottom: '1rem' }}>
        <h3 style={{ marginBottom: '0.75rem', fontWeight: 600 }}>
          {mode === 'create' ? '创建广播' : '编辑广播'}
        </h3>
        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem', maxWidth: 500 }}>
          <div>
            <label>标题 *</label>
            <input value={title} onChange={e => setTitle(e.target.value)} placeholder="广播标题" />
          </div>
          <div>
            <label>内容 *</label>
            <textarea value={content} onChange={e => setContent(e.target.value)} placeholder="广播内容" rows={3} />
          </div>
          <div>
            <label>类型</label>
            <select value={bType} onChange={e => setBType(e.target.value as BroadcastType)}>
              <option value="info">信息</option>
              <option value="warning">警告</option>
              <option value="critical">严重</option>
            </select>
          </div>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <div style={{ flex: 1 }}>
              <label>开始时间</label>
              <input type="datetime-local" value={toLocalDT(startAt)} onChange={e => setStartAt(toISO(e.target.value))} />
            </div>
            <div style={{ flex: 1 }}>
              <label>结束时间</label>
              <input type="datetime-local" value={toLocalDT(endAt)} onChange={e => setEndAt(toISO(e.target.value))} />
            </div>
          </div>
          {formError && <div className="config-error" role="alert">{formError}</div>}
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <button type="submit" className="button-primary" disabled={pending}>
              {pending ? '保存中…' : mode === 'create' ? '创建' : '保存'}
            </button>
            {mode === 'edit' && (
              <button type="button" onClick={resetForm}>取消</button>
            )}
          </div>
        </form>
      </div>

      <div className="page-surface">
        <h3 style={{ marginBottom: '0.75rem', fontWeight: 600 }}>广播列表</h3>
        {listQuery.isLoading && <div className="event-state">加载中…</div>}
        {listQuery.error && <div className="config-error" role="alert">加载失败</div>}
        {listQuery.data && (
          <table className="data-table">
            <thead>
              <tr>
                <th>ID</th>
                <th>标题</th>
                <th>类型</th>
                <th>开始时间</th>
                <th>结束时间</th>
                <th>创建者</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {listQuery.data.data.map((b: Broadcast) => (
                <tr key={b.id}>
                  <td>{b.id}</td>
                  <td>{b.title}</td>
                  <td><span className={`badge ${TYPE_CLASSES[b.type] || 'badge-info'}`}>{TYPE_LABELS[b.type] || b.type}</span></td>
                  <td>{formatDT(b.start_at)}</td>
                  <td>{formatDT(b.end_at)}</td>
                  <td>{b.created_by}</td>
                  <td>
                    <button type="button" onClick={() => startEdit(b)} style={{ marginRight: '0.5rem' }}>编辑</button>
                    <button type="button" onClick={() => { if (confirm('确认删除？')) deleteMut.mutate(b.id) }}>删除</button>
                  </td>
                </tr>
              ))}
              {listQuery.data.data.length === 0 && (
                <tr><td colSpan={7} style={{ textAlign: 'center', padding: '1rem' }}>暂无广播</td></tr>
              )}
            </tbody>
          </table>
        )}
      </div>
    </AppShell>
  )
}

function toLocalDT(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function toISO(local: string): string {
  if (!local) return new Date().toISOString()
  return new Date(local).toISOString()
}
