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
  { label: 'Dashboard', path: '/dashboard' },
  { label: 'Config Center', path: '/config-center' },
  { label: 'Releases', path: '/releases' },
  { label: 'Audit & Runtime', path: '/audit-runtime' },
  { label: 'Playground', path: '/playground' },
  { label: 'Observability', path: '/observability' },
  { label: 'Quota', path: '/quota' },
  { label: 'Policies', path: '/policies' },
  { label: 'Memory Governance', path: '/memory-governance' },
  { label: 'Recommendations', path: '/recommendations' },
  { label: 'Approvals', path: '/approvals' },
  { label: 'Policy Versions', path: '/policy-versions' },
  { label: 'Rollouts', path: '/rollouts' },
  { label: 'Runtime Observer', path: '/runtime-observer' },
  { label: 'Drift Dashboard', path: '/drifts' },
  { label: 'System', path: '/system' },
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
      aria-label={mobile ? 'Mobile navigation' : 'Primary navigation'}
      className={mobile ? 'app-sidebar mobile' : 'app-sidebar'}
      data-open={mobile ? String(open) : undefined}
      data-testid={mobile ? 'mobile-drawer' : undefined}
    >
      <div className="app-sidebar__brand">
        <div>
          <strong>LLM Gateway</strong>
          <p>Admin Console</p>
        </div>
        {mobile ? (
          <button type="button" onClick={onClose} aria-label="Close navigation">
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
