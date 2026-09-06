import React from 'react'
import { useInRouterContext, useLocation, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Button } from '../ui'
import { clearToken } from '../../lib/auth'
import { clearUserToken } from '../../lib/api/identity'

type SidebarProps = {
  mobile?: boolean
  open?: boolean
  onClose?: () => void
  currentPath?: string
  onNavigate?: (path: string) => void
}

type NavItem = {
  label: string
  path?: string
  icon?: string
}

type NavGroup = {
  label: string
  children: NavItem[]
}

export const navGroups: NavGroup[] = [
  {
    label: '概览',
    children: [
      { label: '仪表盘', path: '/dashboard', icon: '📊' },
      { label: 'AI 聊天', path: '/chat', icon: '💬' },
      { label: 'WebSocket 聊天', path: '/ws-chat', icon: '🔌' },
    ],
  },
  {
    label: '管理',
    children: [
      { label: '渠道管理', path: '/channels', icon: '🔗' },
      { label: '资产管理', path: '/assets', icon: '📦' },
      { label: '广播管理', path: '/broadcasts', icon: '📢' },
      { label: '配置中心', path: '/config-center', icon: '⚙️' },
      { label: '发布管理', path: '/releases', icon: '🚀' },
    ],
  },
  {
    label: '监控',
    children: [
      { label: '审计与运行时', path: '/audit-runtime', icon: '🔍' },
      { label: '审计导出', path: '/audit-export', icon: '📤' },
      { label: '可观测性', path: '/observability', icon: '📈' },
      { label: '漂移仪表盘', path: '/drifts', icon: '🌊' },
      { label: '运行时观测', path: '/runtime-observer', icon: '👁️' },
    ],
  },
  {
    label: '策略',
    children: [
      { label: '策略管理', path: '/policies', icon: '📋' },
      { label: '策略版本', path: '/policy-versions', icon: '🔄' },
      { label: '审批管理', path: '/approvals', icon: '✅' },
      { label: '灰度发布', path: '/rollouts', icon: '🎯' },
    ],
  },
  {
    label: '系统',
    children: [
      { label: '配额管理', path: '/quota', icon: '📏' },
      { label: '记忆治理', path: '/memory-governance', icon: '🧠' },
      { label: '推荐管理', path: '/recommendations', icon: '💡' },
      { label: 'Prompt & Mask', path: '/presets', icon: '🎭' },
      { label: 'API 密钥', path: '/api-keys', icon: '🔑' },
      { label: '租户密钥', path: '/tenant-keys', icon: '🗝️' },
      { label: '在线测试', path: '/playground', icon: '🧪' },
      { label: '系统设置', path: '/system', icon: '🔧' },
      { label: '账单管理', path: '/billing', icon: '💰' },
      { label: '定价管理', path: '/billing-pricing', icon: '💵' },
      { label: 'Webhook', path: '/webhooks', icon: '🔔' },
    ],
  },
  {
    label: '账户',
    children: [
      { label: '账户设置', path: '/account', icon: '👤' },
    ],
  },
]

export const navItems: NavItem[] = navGroups.flatMap((g) => g.children)

export function Sidebar({ mobile = false, open = false, onClose, currentPath, onNavigate }: SidebarProps) {
  const inRouter = useInRouterContext()
  if (!inRouter) {
    return <SidebarLayout mobile={mobile} open={open} onClose={onClose} currentPath={currentPath ?? ''} onNavigate={onNavigate ?? (() => undefined)} />
  }
  return <RoutedSidebar mobile={mobile} open={open} onClose={onClose} currentPath={currentPath} onNavigate={onNavigate} />
}

function RoutedSidebar(props: SidebarProps) {
  const navigate = useNavigate()
  const location = useLocation()
  return (
    <SidebarLayout
      {...props}
      currentPath={props.currentPath ?? location.pathname}
      onNavigate={props.onNavigate ?? ((path) => navigate(path))}
    />
  )
}

export const SidebarLayout = React.memo(function SidebarLayout({ mobile = false, open = false, onClose, currentPath = '', onNavigate = () => undefined }: SidebarProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()

  const handleLogout = () => {
    clearToken()
    clearUserToken()
    navigate('/login')
  }

  return (
    <aside
      aria-label={mobile ? '移动端导航' : '主导航'}
      className={mobile ? 'app-sidebar mobile' : 'app-sidebar'}
      data-open={mobile ? String(open) : undefined}
      data-testid={mobile ? 'mobile-drawer' : undefined}
    >
      {/* Brand */}
      <div className="app-sidebar__brand">
        <div className="sidebar-brand">
          <div className="sidebar-brand__logo">
            <svg width="28" height="28" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
              <rect width="32" height="32" rx="8" fill="#ffffff"/>
              <path d="M8 10L16 6L24 10V22L16 26L8 22V10Z" stroke="#171717" strokeWidth="1.5" strokeLinejoin="round"/>
              <path d="M16 6V16M16 16V26M16 16L8 10M16 16L24 10" stroke="#171717" strokeWidth="1.5" strokeLinejoin="round"/>
            </svg>
          </div>
          <div className="sidebar-brand__text">
            <strong>LLM Gateway</strong>
            <span>管理控制台</span>
          </div>
        </div>
        {mobile ? (
          <Button variant="ghost" size="sm" onClick={onClose} aria-label="关闭导航">
            ✕
          </Button>
        ) : null}
      </div>

      {/* Navigation */}
      <nav className="app-sidebar__nav">
        {navGroups.map((group) => (
          <div key={group.label} className="nav-group">
            <span className="nav-group-label">{group.label}</span>
            {group.children.map((item) => {
              const isActive = item.path ? currentPath === item.path : false
              return (
                <button
                  key={item.label}
                  type="button"
                  className={isActive ? 'nav-item active' : 'nav-item'}
                  onClick={() => {
                    if (!item.path) return
                    onNavigate(item.path)
                    onClose?.()
                  }}
                >
                  {item.icon && <span className="nav-item__icon">{item.icon}</span>}
                  <span className="nav-item__label">{item.label}</span>
                </button>
              )
            })}
          </div>
        ))}
      </nav>

      {/* Footer */}
      <div className="app-sidebar__footer">
        <button
          type="button"
          className="nav-item nav-item--logout"
          onClick={handleLogout}
        >
          <span className="nav-item__icon">🚪</span>
          <span className="nav-item__label">{t('common.logout')}</span>
        </button>
      </div>
    </aside>
  )
})
