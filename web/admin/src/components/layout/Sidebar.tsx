import { useLocation, useNavigate } from 'react-router-dom'

type SidebarProps = {
  mobile?: boolean
  open?: boolean
  onClose?: () => void
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
  { label: 'System', path: '/system' },
]

export function Sidebar({ mobile = false, open = false, onClose }: SidebarProps) {
  const navigate = useNavigate()
  const location = useLocation()

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
          const isActive = item.path ? location.pathname === item.path : false

          return (
            <button
              key={item.label}
              type="button"
              className={isActive ? 'nav-item active' : 'nav-item'}
              onClick={() => {
                if (!item.path) {
                  return
                }
                navigate(item.path)
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
