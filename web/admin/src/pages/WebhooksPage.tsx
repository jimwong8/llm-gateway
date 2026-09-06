import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import type { Webhook } from '../types/webhook'
import { WEBHOOK_EVENTS, WEBHOOK_EVENT_LABELS } from '../types/webhook'

async function listWebhooks(): Promise<{ data: Webhook[] }> {
  const res = await fetch('/api/webhooks')
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  return res.json()
}

async function createWebhook(data: { url: string; events: string[]; enabled: boolean }): Promise<Webhook> {
  const res = await fetch('/api/webhooks', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  return res.json()
}

async function deleteWebhook(id: string): Promise<void> {
  await fetch(`/api/webhooks/${id}`, { method: 'DELETE' })
}

export function WebhooksPage() {
  const queryClient = useQueryClient()
  const [showForm, setShowForm] = useState(false)
  const [url, setUrl] = useState('')
  const [events, setEvents] = useState<string[]>([])
  const [enabled, setEnabled] = useState(true)

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['webhooks'],
    queryFn: listWebhooks,
  })

  const createMutation = useMutation({
    mutationFn: () => createWebhook({ url, events, enabled }),
    onSuccess: () => {
      setShowForm(false)
      setUrl('')
      setEvents([])
      setEnabled(true)
      queryClient.invalidateQueries({ queryKey: ['webhooks'] })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteWebhook(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['webhooks'] }),
  })

  const webhooks = data?.data ?? []

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    createMutation.mutate()
  }

  const handleToggleEvent = (ev: string) => {
    setEvents(prev => prev.includes(ev) ? prev.filter(e => e !== ev) : [...prev, ev])
  }

  return (
    <AppShell title="Webhook 管理" description="管理 webhook 订阅">
      <div>
        <div style={{ marginBottom: '1.5rem', display: 'flex', gap: '0.75rem' }}>
          <button onClick={() => setShowForm(!showForm)} style={{ padding: '0.5rem 1.25rem', background: '#3b82f6', color: '#fff', border: 'none', borderRadius: '0.5rem', cursor: 'pointer', fontSize: '0.85rem', fontWeight: 500 }}>
            {showForm ? '取消' : '新建 Webhook'}
          </button>
          <button onClick={() => refetch()} style={{ padding: '0.5rem 1.25rem', background: 'transparent', color: '#f1f5f9', border: '1px solid rgba(255,255,255,0.12)', borderRadius: '0.5rem', cursor: 'pointer', fontSize: '0.85rem' }}>刷新</button>
        </div>

        {showForm && (
          <form onSubmit={handleSubmit} style={{ marginBottom: '1.5rem', padding: '1.5rem', background: 'rgba(255,255,255,0.04)', borderRadius: '0.75rem', border: '1px solid rgba(255,255,255,0.08)' }}>
            <h3 style={{ marginBottom: '1rem', fontSize: '1rem', fontWeight: 600, color: '#f1f5f9' }}>创建新 Webhook</h3>
            <div style={{ display: 'grid', gap: '1rem' }}>
              <div>
                <label style={{ display: 'block', fontSize: '0.85rem', marginBottom: '0.5rem', color: '#cbd5e1' }}>Webhook URL <span style={{ color: '#ef4444' }}>*</span></label>
                <input type="url" required placeholder="https://example.com/webhook" value={url} onChange={e => setUrl(e.target.value)} style={{ width: '100%', padding: '0.6rem', borderRadius: '0.5rem', border: '1px solid rgba(255,255,255,0.12)', background: 'rgba(255,255,255,0.06)', color: '#f1f5f9', fontSize: '0.85rem' }} />
              </div>
              <div>
                <label style={{ display: 'block', fontSize: '0.85rem', marginBottom: '0.5rem', color: '#cbd5e1' }}>订阅事件 <span style={{ color: '#ef4444' }}>*</span></label>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem' }}>
                  {WEBHOOK_EVENTS.map(ev => (
                    <button key={ev} type="button" onClick={() => handleToggleEvent(ev)} style={{ padding: '0.4rem 0.8rem', borderRadius: '9999px', border: `1px solid ${events.includes(ev) ? '#3b82f6' : 'rgba(255,255,255,0.12)'}`, background: events.includes(ev) ? 'rgba(59,130,246,0.15)' : 'transparent', color: events.includes(ev) ? '#60a5fa' : '#94a3b8', cursor: 'pointer', fontSize: '0.75rem' }}>
                      {WEBHOOK_EVENT_LABELS[ev] || ev}
                    </button>
                  ))}
                </div>
              </div>
              <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer', fontSize: '0.85rem' }}>
                <input type="checkbox" checked={enabled} onChange={e => setEnabled(e.target.checked)} />
                <span style={{ color: '#f1f5f9' }}>启用</span>
              </label>
              <button type="submit" disabled={!url || events.length === 0 || createMutation.isPending} style={{ padding: '0.6rem 1.5rem', background: '#3b82f6', color: '#fff', border: 'none', borderRadius: '0.5rem', cursor: 'pointer', fontSize: '0.85rem', fontWeight: 500, opacity: (!url || events.length === 0 || createMutation.isPending) ? 0.5 : 1 }}>
                {createMutation.isPending ? '创建中...' : '创建'}
              </button>
            </div>
            {createMutation.isError && <div style={{ marginTop: '1rem', color: '#ef4444', fontSize: '0.85rem' }}>创建失败: {(createMutation.error as Error)?.message}</div>}
          </form>
        )}

        {isLoading ? (
          <div style={{ padding: '2rem', textAlign: 'center', color: '#64748b' }}>加载中...</div>
        ) : webhooks.length === 0 ? (
          <div style={{ padding: '3rem', textAlign: 'center', background: 'rgba(255,255,255,0.04)', borderRadius: '0.75rem' }}>
            <div style={{ fontSize: '2rem', marginBottom: '1rem' }}>📭</div>
            <p style={{ color: '#64748b' }}>暂无 webhook 订阅</p>
          </div>
        ) : (
          <div style={{ overflowX: 'auto', borderRadius: '0.75rem', border: '1px solid rgba(255,255,255,0.08)' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
              <thead>
                <tr style={{ background: 'rgba(255,255,255,0.04)' }}>
                  <th style={{ padding: '0.75rem 1rem', textAlign: 'left', fontWeight: 600, fontSize: '0.75rem', textTransform: 'uppercase', letterSpacing: '0.05em', color: '#94a3b8', borderBottom: '1px solid rgba(255,255,255,0.08)' }}>URL</th>
                  <th style={{ padding: '0.75rem 1rem', textAlign: 'left', fontWeight: 600, fontSize: '0.75rem', textTransform: 'uppercase', letterSpacing: '0.05em', color: '#94a3b8', borderBottom: '1px solid rgba(255,255,255,0.08)' }}>事件</th>
                  <th style={{ padding: '0.75rem 1rem', textAlign: 'left', fontWeight: 600, fontSize: '0.75rem', textTransform: 'uppercase', letterSpacing: '0.05em', color: '#94a3b8', borderBottom: '1px solid rgba(255,255,255,0.08)' }}>状态</th>
                  <th style={{ padding: '0.75rem 1rem', textAlign: 'left', fontWeight: 600, fontSize: '0.75rem', textTransform: 'uppercase', letterSpacing: '0.05em', color: '#94a3b8', borderBottom: '1px solid rgba(255,255,255,0.08)' }}>操作</th>
                </tr>
              </thead>
              <tbody>
                {webhooks.map((wh: Webhook) => (
                  <tr key={wh.id} style={{ borderBottom: '1px solid rgba(255,255,255,0.08)' }}>
                    <td style={{ padding: '0.75rem 1rem', color: '#f1f5f9' }}>{wh.url}</td>
                    <td style={{ padding: '0.75rem 1rem' }}>
                      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.25rem' }}>
                        {wh.events.map(e => (
                          <span key={e} style={{ padding: '0.15rem 0.5rem', background: 'rgba(255,255,255,0.06)', borderRadius: '0.25rem', fontSize: '0.7rem', color: '#94a3b8' }}>
                            {WEBHOOK_EVENT_LABELS[e] || e}
                          </span>
                        ))}
                      </div>
                    </td>
                    <td style={{ padding: '0.75rem 1rem' }}>
                      <span style={{ padding: '0.25rem 0.6rem', borderRadius: '9999px', fontSize: '0.7rem', fontWeight: 500, background: wh.enabled ? 'rgba(34,197,94,0.15)' : 'rgba(255,255,255,0.06)', color: wh.enabled ? '#22c55e' : '#94a3b8' }}>
                        {wh.enabled ? '启用' : '禁用'}
                      </span>
                    </td>
                    <td style={{ padding: '0.75rem 1rem' }}>
                      <button onClick={() => deleteMutation.mutate(wh.id)} disabled={deleteMutation.isPending} style={{ padding: '0.3rem 0.6rem', background: 'transparent', border: '1px solid #ef4444', color: '#ef4444', borderRadius: '0.25rem', cursor: 'pointer', fontSize: '0.75rem' }}>删除</button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </AppShell>
  )
}
