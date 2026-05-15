import { PropsWithChildren, useState } from 'react'
import { useInRouterContext, useLocation, useNavigate } from 'react-router-dom'
import { PageHeader } from '../common/PageHeader'
import { navItems, Sidebar } from './Sidebar'
import { Topbar } from './Topbar'

type AppShellProps = PropsWithChildren<{
  title?: string
  description?: string
}>

export function AppShell(props: AppShellProps) {
  const inRouter = useInRouterContext()
  if (!inRouter) {
    return <AppShellLayout {...props} currentPath="" onNavigate={() => undefined} />
  }
  return <RoutedAppShell {...props} />
}

function RoutedAppShell(props: AppShellProps) {
  const navigate = useNavigate()
  const location = useLocation()
  return <AppShellLayout {...props} currentPath={location.pathname} onNavigate={(path) => navigate(path)} />
}

function AppShellLayout({
  children,
  title = 'Dashboard',
  description = '第一期先建立完整响应式后台布局骨架。',
  currentPath,
  onNavigate,
}: AppShellProps & { currentPath: string; onNavigate: (path: string) => void }) {
  const [mobileOpen, setMobileOpen] = useState(false)

  return (
    <div className="app-shell">
      <Sidebar currentPath={currentPath} onNavigate={onNavigate} />
      <Sidebar mobile open={mobileOpen} onClose={() => setMobileOpen(false)} currentPath={currentPath} onNavigate={onNavigate} />
      <div className="app-shell__content">
        <Topbar onToggleNavigation={() => setMobileOpen((value) => !value)} />
        <div className="app-quick-nav" aria-label="Quick navigation">
          {navItems
            .filter((item) => item.path)
            .map((item) => {
              const active = currentPath === item.path
              return (
                <button
                  key={item.label}
                  type="button"
                  className={active ? 'quick-nav-item active' : 'quick-nav-item'}
                  onClick={() => onNavigate(item.path!)}
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
