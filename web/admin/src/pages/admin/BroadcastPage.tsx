import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { AppShell } from '../../components/layout/AppShell'
import { listBroadcasts, createBroadcast, updateBroadcast, deleteBroadcast } from '../../lib/api/broadcasts'
import type { Broadcast, BroadcastType } from '../../types/broadcast'

const NOW = (): string => new Date().toISOString()
const IN_24H = (): string => new Date(Date.now() + 86400000).toISOString()

function formatDT(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false })
}

const TYPE_LABELS: Record<BroadcastType, string> = { info: 'info', warning: 'warning', critical: 'critical' }
const TYPE_CLASSES: Record<BroadcastType, string> = { info: 'badge-info', warning: 'badge-warning', critical: 'badge-critical' }

type FormMode = 'create' | 'edit'

export function BroadcastPage() {
  const { t } = useTranslation()
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
      setFormError(t('common.titleRequired'))
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
    <AppShell title={t('broadcast.title')} description={t('broadcast.description')}>
      <div className="page-header">
        <h2>{t('broadcast.title')}</h2>
      </div>

      <div className="page-surface" style={{ marginBottom: '1rem' }}>
        <h3 style={{ marginBottom: '0.75rem', fontWeight: 600 }}>
          {mode === 'create' ? t('broadcast.create') : t('broadcast.edit')}
        </h3>
        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem', maxWidth: 500 }}>
          <div>
            <label>{t('broadcast.formTitle')}</label>
            <input value={title} onChange={e => setTitle(e.target.value)} placeholder={t('broadcast.formTitlePlaceholder')} />
          </div>
          <div>
            <label>{t('broadcast.formContent')}</label>
            <textarea value={content} onChange={e => setContent(e.target.value)} placeholder={t('broadcast.formContentPlaceholder')} rows={3} />
          </div>
          <div>
            <label>{t('broadcast.formType')}</label>
            <select value={bType} onChange={e => setBType(e.target.value as BroadcastType)}>
              <option value="info">{t('broadcast.typeInfo')}</option>
              <option value="warning">{t('broadcast.typeWarning')}</option>
              <option value="critical">{t('broadcast.typeCritical')}</option>
            </select>
          </div>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <div style={{ flex: 1 }}>
              <label>{t('broadcast.formStartAt')}</label>
              <input type="datetime-local" value={toLocalDT(startAt)} onChange={e => setStartAt(toISO(e.target.value))} />
            </div>
            <div style={{ flex: 1 }}>
              <label>{t('broadcast.formEndAt')}</label>
              <input type="datetime-local" value={toLocalDT(endAt)} onChange={e => setEndAt(toISO(e.target.value))} />
            </div>
          </div>
          {formError && <div className="config-error" role="alert">{formError}</div>}
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <button type="submit" className="button-primary" disabled={pending}>
              {pending ? t('common.pending') : mode === 'create' ? t('common.create') : t('common.save')}
            </button>
            {mode === 'edit' && (
              <button type="button" onClick={resetForm}>{t('common.cancel')}</button>
            )}
          </div>
        </form>
      </div>

      <div className="page-surface">
        <h3 style={{ marginBottom: '0.75rem', fontWeight: 600 }}>{t('broadcast.list')}</h3>
        {listQuery.isLoading && <div className="event-state">{t('common.loading')}</div>}
        {listQuery.error && <div className="config-error" role="alert">{t('common.error')}</div>}
        {Array.isArray(listQuery.data?.data) && (
          <table className="data-table">
            <thead>
              <tr>
                 <th>{t('broadcast.colId')}</th>
                 <th>{t('broadcast.colTitle')}</th>
                 <th>{t('broadcast.colType')}</th>
                 <th>{t('broadcast.colStartAt')}</th>
                 <th>{t('broadcast.colEndAt')}</th>
                 <th>{t('broadcast.colCreatedBy')}</th>
                 <th>{t('broadcast.colActions')}</th>
              </tr>
            </thead>
            <tbody>
              {listQuery.data.data.map((b: Broadcast) => (
                <tr key={b.id}>
                  <td>{b.id}</td>
                  <td>{b.title}</td>
                  <td><span className={`badge ${TYPE_CLASSES[b.type] || 'badge-info'}`}>{t(`broadcast.${TYPE_LABELS[b.type]}`) || b.type}</span></td>
                  <td>{formatDT(b.start_at)}</td>
                  <td>{formatDT(b.end_at)}</td>
                  <td>{b.created_by}</td>
                  <td>
                    <button type="button" onClick={() => startEdit(b)} style={{ marginRight: '0.5rem' }}>{t('common.edit')}</button>
                    <button type="button" onClick={() => { if (confirm(t('common.confirmDelete'))) deleteMut.mutate(b.id) }}>{t('common.delete')}</button>
                  </td>
                </tr>
              ))}
              {listQuery.data.data.length === 0 && (
                <tr><td colSpan={7} style={{ textAlign: 'center', padding: '1rem' }}>{t('broadcast.noBroadcasts')}</td></tr>
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
