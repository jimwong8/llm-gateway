import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { listActiveBroadcasts, markBroadcastRead } from '../../lib/api/broadcasts'
import { clearToken, getToken } from '../../lib/auth'
import { clearUserToken } from '../../lib/api/identity'
import type { Broadcast, BroadcastType } from '../../types/broadcast'

const TYPE_STYLES: Record<BroadcastType, string> = {
  info: '#3b82f6',
  warning: '#f59e0b',
  critical: '#ef4444',
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
  const { t } = useTranslation()
  const [banners, setBanners] = useState<Broadcast[]>([])
  const [dismissed, setDismissed] = useState<Set<number>>(getDismissed)
  const [showUserMenu, setShowUserMenu] = useState(false)

  const getUserEmail = useCallback(() => {
    const token = getToken()
    if (!token) return ''
    try {
      const payload = JSON.parse(atob(token.split('.')[1]))
      return payload.email || payload.sub || ''
    } catch {
      return ''
    }
  }, [])

  const navigate = useNavigate()

  const handleLogout = useCallback(() => {
    clearToken()
    clearUserToken()
    navigate('/login')
  }, [navigate])

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
      {/* Broadcast banners */}
      {banners.length > 0 && (
        <div className="broadcast-banners">
          {banners.map(b => (
            <div
              key={b.id}
              className={`broadcast-banner broadcast-banner--${b.type}`}
              style={{ background: TYPE_STYLES[b.type] || TYPE_STYLES.info }}
            >
              <span className="broadcast-banner__type">[{TYPE_LABELS[b.type] || b.type}]</span>
              <strong className="broadcast-banner__title">{b.title}</strong>
              <span className="broadcast-banner__content">{b.content}</span>
              <button
                type="button"
                className="broadcast-banner__dismiss"
                onClick={() => handleDismiss(b.id)}
                aria-label="关闭"
              >
                ✕
              </button>
            </div>
          ))}
        </div>
      )}

      <div className="topbar__left">
        <button type="button" className="topbar__menu-btn" aria-label="切换导航" onClick={onToggleNavigation}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
            <line x1="3" y1="6" x2="21" y2="6"/><line x1="3" y1="12" x2="21" y2="12"/><line x1="3" y1="18" x2="21" y2="18"/>
          </svg>
        </button>
        <div className="topbar__title">
          <strong>LLM Gateway Console</strong>
          <span>管理控制台与在线测试台</span>
        </div>
      </div>

      <div className="topbar__right">
        <span className="env-badge">Local</span>

        {/* User menu */}
        <div className="topbar__user">
          <button
            type="button"
            className="topbar__user-btn"
            onClick={() => setShowUserMenu(!showUserMenu)}
          >
            <span className="topbar__user-avatar">
              {getUserEmail()?.charAt(0)?.toUpperCase() || 'U'}
            </span>
            <span className="topbar__user-email">{getUserEmail() || t('common.user')}</span>
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <polyline points="6 9 12 15 18 9"/>
            </svg>
          </button>

          {showUserMenu && (
            <div className="topbar__dropdown">
              <button type="button" className="topbar__dropdown-item" onClick={() => navigate('/account')}>
                <span>👤</span> 账户设置
              </button>
              <div className="topbar__dropdown-sep" />
              <button type="button" className="topbar__dropdown-item topbar__dropdown-item--danger" onClick={handleLogout}>
                <span>🚪</span> {t('common.logout')}
              </button>
            </div>
          )}
        </div>
      </div>
    </header>
  )
}
