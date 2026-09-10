import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { AppShell } from './AppShell'

describe('AppShell', () => {
  function renderShell(initialPath = '/dashboard') {
    return render(
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route
            path="/dashboard"
            element={
              <AppShell>
                <div>Dashboard Content</div>
              </AppShell>
            }
          />
          <Route
            path="/config-center"
            element={
              <AppShell title="配置中心" description="查看配置版本列表、筛选结果，并在右侧详情抽屉里检查继承来源。">
                <div>Config Center Content</div>
              </AppShell>
            }
          />
          <Route
            path="/runtime-observer"
            element={
              <AppShell title="运行时观测" description="观察运行时策略状态。">
                <div>Runtime Observer Content</div>
              </AppShell>
            }
          />
        </Routes>
      </MemoryRouter>,
    )
  }

  it('renders sidebar, topbar, and main content', () => {
    renderShell()

    expect(screen.getByRole('complementary', { name: '主导航' })).toBeInTheDocument()
    expect(screen.getByRole('banner')).toBeInTheDocument()
    expect(screen.getByRole('main')).toHaveTextContent('Dashboard Content')
    expect(screen.getAllByText('LLM Gateway').length).toBeGreaterThan(0)
  })

  it('toggles mobile navigation drawer from the topbar button', async () => {
    const user = userEvent.setup()
    renderShell()

    const toggleButton = screen.getByRole('button', { name: '切换导航' })
    const mobileDrawer = screen.getByTestId('mobile-drawer')

    expect(mobileDrawer).toHaveAttribute('data-open', 'false')

    await user.click(toggleButton)
    expect(mobileDrawer).toHaveAttribute('data-open', 'true')

    await user.click(screen.getByRole('button', { name: '关闭导航' }))
    expect(mobileDrawer).toHaveAttribute('data-open', 'false')
  })

  it('navigates when clicking a sidebar module', async () => {
    const user = userEvent.setup()
    renderShell()

    await user.click(screen.getAllByRole('button', { name: '配置中心' })[0])

    expect(await screen.findByRole('heading', { name: '配置中心', level: 1 })).toBeInTheDocument()
    expect(screen.getByText('Config Center Content')).toBeInTheDocument()
  })

  it('includes runtime observer navigation entry', async () => {
    const user = userEvent.setup()
    renderShell()

    await user.click(screen.getAllByRole('button', { name: '运行时观测' })[0])

    expect(await screen.findByRole('heading', { name: '运行时观测', level: 1 })).toBeInTheDocument()
    expect(screen.getByText('Runtime Observer Content')).toBeInTheDocument()
  })
})
