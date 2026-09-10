import { PropsWithChildren, useState } from 'react'
import { useInRouterContext, useLocation, useNavigate } from 'react-router-dom'
import { PageHeader } from '../common/PageHeader'
import { navItems, Sidebar } from './Sidebar'
import { Topbar } from './Topbar'
import { useUIStore } from '../../stores/ui-store'
import { useApplyTheme } from '../../hooks/use-apply-theme'

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
  title = '仪表盘',
  description = '管理控制台仪表盘，聚合展示服务状态与关键指标。',
  currentPath,
  onNavigate,
}: AppShellProps & { currentPath: string; onNavigate: (path: string) => void }) {
  // 应用全局主题类到 <html>（ui-store 是唯一真源）
  useApplyTheme()

  const sidebarCollapsed = useUIStore((s) => s.sidebarCollapsed)
  const toggleSidebar = useUIStore((s) => s.toggleSidebar)
  const [mobileOpen, setMobileOpen] = useState(false)

  return (
    <div className={`app-shell${sidebarCollapsed ? ' app-shell--collapsed' : ''}`}>
      <Sidebar
        currentPath={currentPath}
        onNavigate={onNavigate}
        collapsed={sidebarCollapsed}
        onToggleCollapse={toggleSidebar}
      />
      <Sidebar
        mobile
        open={mobileOpen}
        onClose={() => setMobileOpen(false)}
        currentPath={currentPath}
        onNavigate={onNavigate}
      />
      <div className="app-shell__content">
        <Topbar onToggleNavigation={() => setMobileOpen((value) => !value)} />
        <div className="app-quick-nav" aria-label="快速导航">
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
