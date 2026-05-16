import { useInRouterContext, useLocation, useNavigate } from 'react-router-dom'

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

export const navItems: NavItem[] = [
  { label: '仪表盘', path: '/dashboard' },
  { label: '配置中心', path: '/config-center' },
  { label: '发布管理', path: '/releases' },
  { label: '审计与运行时', path: '/audit-runtime' },
  { label: '在线测试', path: '/playground' },
  { label: '可观测性', path: '/observability' },
  { label: '配额管理', path: '/quota' },
  { label: '策略管理', path: '/policies' },
  { label: '记忆治理', path: '/memory-governance' },
  { label: '推荐管理', path: '/recommendations' },
  { label: '审批管理', path: '/approvals' },
  { label: '策略版本', path: '/policy-versions' },
  { label: '灰度发布', path: '/rollouts' },
  { label: '运行时观测', path: '/runtime-observer' },
  { label: '漂移仪表盘', path: '/drifts' },
  { label: '系统状态', path: '/system' },
]

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

function SidebarLayout({ mobile = false, open = false, onClose, currentPath = '', onNavigate = () => undefined }: SidebarProps) {
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
          <button type="button" onClick={onClose} aria-label="关闭导航">
            关闭
          </button>
        ) : null}
      </div>
      <nav className="app-sidebar__nav">
        {navItems.map((item) => {
          const isActive = item.path ? currentPath === item.path : false

          return (
            <button
              key={item.label}
              type="button"
              className={isActive ? 'nav-item active' : 'nav-item'}
              onClick={() => {
                if (!item.path) {
                  return
                }
                onNavigate(item.path)
                onClose?.()
              }}
            >
              {item.label}
            </button>
          )
        })}
      </nav>
    </aside>
  )
}
