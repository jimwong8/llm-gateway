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
              <AppShell title="Config Center" description="查看配置版本列表、筛选结果，并在右侧详情抽屉里检查继承来源。">
                <div>Config Center Content</div>
              </AppShell>
            }
          />
          <Route
            path="/runtime-observer"
            element={
              <AppShell title="Runtime Observer" description="观察运行时策略状态。">
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

    expect(screen.getByRole('complementary', { name: 'Primary navigation' })).toBeInTheDocument()
    expect(screen.getByRole('banner')).toBeInTheDocument()
    expect(screen.getByRole('main')).toHaveTextContent('Dashboard Content')
    expect(screen.getAllByText('LLM Gateway').length).toBeGreaterThan(0)
  })

  it('toggles mobile navigation drawer from the topbar button', async () => {
    const user = userEvent.setup()
    renderShell()

    const toggleButton = screen.getByRole('button', { name: 'Toggle navigation' })
    const mobileDrawer = screen.getByTestId('mobile-drawer')

    expect(mobileDrawer).toHaveAttribute('data-open', 'false')

    await user.click(toggleButton)
    expect(mobileDrawer).toHaveAttribute('data-open', 'true')

    await user.click(screen.getByRole('button', { name: 'Close navigation' }))
    expect(mobileDrawer).toHaveAttribute('data-open', 'false')
  })

  it('navigates when clicking a sidebar module', async () => {
    const user = userEvent.setup()
    renderShell()

    await user.click(screen.getAllByRole('button', { name: 'Config Center' })[0])

    expect(await screen.findByRole('heading', { name: 'Config Center', level: 1 })).toBeInTheDocument()
    expect(screen.getByText('Config Center Content')).toBeInTheDocument()
  })

  it('includes runtime observer navigation entry', async () => {
    const user = userEvent.setup()
    renderShell()

    await user.click(screen.getAllByRole('button', { name: 'Runtime Observer' })[0])

    expect(await screen.findByRole('heading', { name: 'Runtime Observer', level: 1 })).toBeInTheDocument()
    expect(screen.getByText('Runtime Observer Content')).toBeInTheDocument()
  })
})
