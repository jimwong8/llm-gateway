import { useState, useEffect, useCallback } from 'react'
import { listActiveBroadcasts, markBroadcastRead } from '../../lib/api/broadcasts'
import type { Broadcast, BroadcastType } from '../../types/broadcast'

const TYPE_STYLES: Record<BroadcastType, string> = {
  info: '#1890ff',
  warning: '#faad14',
  critical: '#ff4d4f',
}
const TYPE_LABELS: Record<BroadcastType, string> = {
  info: '信息',
  warning: '警告',
  critical: '紧急',
}

function getDismissed(): Set<number> {
  try {
    const raw = sessionStorage.getItem('broadcast_dismissed')
    return raw ? new Set<number>(JSON.parse(raw)) : new Set()
  } catch {
    return new Set()
  }
}

function addDismissed(id: number) {
  try {
    const s = getDismissed()
    s.add(id)
    sessionStorage.setItem('broadcast_dismissed', JSON.stringify([...s]))
  } catch { /* noop */ }
}

export function Topbar({ onToggleNavigation }: { onToggleNavigation: () => void }) {
  const [banners, setBanners] = useState<Broadcast[]>([])
  const [dismissed, setDismissed] = useState<Set<number>>(getDismissed)

  const fetchBanners = useCallback(async () => {
    try {
      const res = await listActiveBroadcasts()
      const readSet = new Set(res.read_ids || [])
      const unread = (res.data || []).filter(b => !dismissed.has(b.id) && !readSet.has(b.id))
      setBanners(unread)
    } catch { /* ignore */ }
  }, [dismissed])

  useEffect(() => {
    fetchBanners()
    const interval = setInterval(fetchBanners, 60000)
    return () => clearInterval(interval)
  }, [fetchBanners])

  function handleDismiss(id: number) {
    addDismissed(id)
    setDismissed(prev => { const n = new Set(prev); n.add(id); return n })
    setBanners(prev => prev.filter(b => b.id !== id))
    markBroadcastRead(id).catch(() => { /* ignore */ })
  }

  return (
    <header className="topbar">
      {banners.length > 0 && (
        <div className="broadcast-banners" style={{ position: 'absolute', top: 0, left: 0, right: 0, zIndex: 100 }}>
          {banners.map(b => (
            <div
              key={b.id}
              className={`broadcast-banner broadcast-banner--${b.type}`}
              style={{ background: TYPE_STYLES[b.type] || TYPE_STYLES.info, color: '#fff', padding: '6px 16px', display: 'flex', alignItems: 'center', gap: '8px', fontSize: '13px' }}
            >
              <strong>[{TYPE_LABELS[b.type] || b.type}] {b.title}</strong>
              <span style={{ flex: 1 }}>{b.content}</span>
              <button
                type="button"
                onClick={() => handleDismiss(b.id)}
                style={{ background: 'transparent', border: 'none', color: '#fff', cursor: 'pointer', fontWeight: 'bold', padding: '0 4px' }}
                aria-label="关闭"
              >
                ✕
              </button>
            </div>
          ))}
        </div>
      )}
      <div className="topbar__left">
        <button type="button" aria-label="切换导航" onClick={onToggleNavigation}>
          菜单
        </button>
        <div>
          <strong>LLM Gateway Console</strong>
          <p>管理控制台与在线测试台</p>
        </div>
      </div>
      <div className="topbar__right">
        <span className="env-badge">环境: Local</span>
      </div>
    </header>
  )
}
