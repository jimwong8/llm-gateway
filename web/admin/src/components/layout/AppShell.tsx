import { PropsWithChildren, useState } from 'react'
import { useInRouterContext, useLocation, useNavigate } from 'react-router-dom'
import { PageHeader } from '../common/PageHeader'
import { Breadcrumb } from '../common/Breadcrumb'
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
  title = '仪表盘',
  description = '管理控制台仪表盘，聚合展示服务状态与关键指标。',
  currentPath,
  onNavigate,
}: AppShellProps & { currentPath: string; onNavigate: (path: string) => void }) {
  const [mobileOpen, setMobileOpen] = useState(false)

  return (
    <div className="app-shell">
      {/* 跳过链接 (WCAG 2.4.1) */}
      <a href="#main-content" className="skip-link">
        跳转到主要内容
      </a>
      
      <Sidebar currentPath={currentPath} onNavigate={onNavigate} />
      <Sidebar mobile open={mobileOpen} onClose={() => setMobileOpen(false)} currentPath={currentPath} onNavigate={onNavigate} />
      <div className="app-shell__content">
        <Topbar onToggleNavigation={() => setMobileOpen((value) => !value)} />
        
        {/* 面包屑导航 */}
        <Breadcrumb />
        
        {/* 快捷导航 */}
        <div className="app-quick-nav" aria-label="快速导航" role="navigation">
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
                  aria-current={active ? 'page' : undefined}
                >
                  {item.label}
                </button>
              )
            })}
        </div>
        
        <main className="app-shell__main" id="main-content" role="main">
          <PageHeader title={title} description={description} />
          <section className="page-surface">{children}</section>
        </main>
      </div>
    </div>
  )
}
