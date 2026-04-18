import { PropsWithChildren, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { PageHeader } from '../common/PageHeader'
import { navItems, Sidebar } from './Sidebar'
import { Topbar } from './Topbar'

type AppShellProps = PropsWithChildren<{
  title?: string
  description?: string
}>

export function AppShell({
  children,
  title = 'Dashboard',
  description = '第一期先建立完整响应式后台布局骨架。',
}: AppShellProps) {
  const [mobileOpen, setMobileOpen] = useState(false)
  const navigate = useNavigate()
  const location = useLocation()

  return (
    <div className="app-shell">
      <Sidebar />
      <Sidebar mobile open={mobileOpen} onClose={() => setMobileOpen(false)} />
      <div className="app-shell__content">
        <Topbar onToggleNavigation={() => setMobileOpen((value) => !value)} />
        <div className="app-quick-nav" aria-label="Quick navigation">
          {navItems
            .filter((item) => item.path)
            .map((item) => {
              const active = location.pathname === item.path
              return (
                <button
                  key={item.label}
                  type="button"
                  className={active ? 'quick-nav-item active' : 'quick-nav-item'}
                  onClick={() => navigate(item.path!)}
                >
                  {item.label}
                </button>
              )
            })}
        </div>
        <main className="app-shell__main">
          <PageHeader title={title} description={description} />
          <section className="page-surface">{children}</section>
        </main>
      </div>
    </div>
  )
}
