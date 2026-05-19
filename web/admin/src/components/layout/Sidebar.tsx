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
}

type NavGroup = {
  label: string
  children: NavItem[]
}

export const navGroups: NavGroup[] = [
  {
    label: '概览',
    children: [
      { label: '仪表盘', path: '/dashboard' },
      { label: 'AI 聊天', path: '/chat' },
      { label: 'WebSocket 聊天', path: '/ws-chat' },
    ],
  },
  {
    label: '管理',
    children: [
      { label: '渠道管理', path: '/channels' },
      { label: '资产管理', path: '/assets' },
      { label: '广播管理', path: '/broadcasts' },
      { label: '配置中心', path: '/config-center' },
      { label: '发布管理', path: '/releases' },
    ],
  },
  {
    label: '监控',
    children: [
      { label: '审计与运行时', path: '/audit-runtime' },
      { label: '审计导出', path: '/audit-export' },
      { label: '可观测性', path: '/observability' },
      { label: '漂移仪表盘', path: '/drifts' },
      { label: '运行时观测', path: '/runtime-observer' },
    ],
  },
  {
    label: '策略',
    children: [
      { label: '策略管理', path: '/policies' },
      { label: '策略版本', path: '/policy-versions' },
      { label: '审批管理', path: '/approvals' },
      { label: '灰度发布', path: '/rollouts' },
    ],
  },
  {
    label: '系统',
    children: [
      { label: '配额管理', path: '/quota' },
      { label: '记忆治理', path: '/memory-governance' },
      { label: '推荐管理', path: '/recommendations' },
      { label: 'Prompt & Mask', path: '/presets' },
      { label: 'API 密钥', path: '/api-keys' },
      { label: '租户密钥', path: '/tenant-keys' },
      { label: '在线测试', path: '/playground' },
      { label: '系统状态', path: '/system' },
      { label: '系统设置', path: '/system/settings' },
      { label: '账单管理', path: '/billing' },
      { label: '定价管理', path: '/billing-pricing' },
    ],
  },
  {
    label: '账户',
    children: [
      { label: '账户设置', path: '/account' },
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
      <div className="app-sidebar__brand">
        <div>
          <strong>LLM Gateway</strong>
          <p>管理控制台</p>
        </div>
        {mobile ? (
          <Button variant="ghost" size="sm" onClick={onClose} aria-label="关闭导航">
            关闭
          </Button>
        ) : null}
      </div>
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
                  {item.label}
                </button>
              )
            })}
          </div>
        ))}
      </nav>
      <div className="app-sidebar__footer">
        <button
          type="button"
          className="nav-item nav-item--logout"
          onClick={handleLogout}
        >
          {t('common.logout')}
        </button>
      </div>
    </aside>
  )
})
