import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { LoginPage } from './LoginPage'
import { ADMIN_TOKEN_KEY } from '../lib/auth'

function mockOAuthConfig(enabled: boolean) {
  globalThis.fetch = vi.fn().mockImplementation(async (input: RequestInfo | URL) => {
    const url = String(input)
    if (url.includes('/api/user/broadcasts')) {
      return new Response(JSON.stringify({ object: 'list', data: [], read_ids: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } })
    }
    return {
      ok: true,
      headers: new Headers({ 'Content-Type': 'application/json' }),
      json: () => Promise.resolve({ github_enabled: enabled }),
    }
  })
}

describe('LoginPage', () => {
  beforeEach(() => {
    window.sessionStorage.clear()
    vi.restoreAllMocks()
  })

  it('stores token and navigates to dashboard after submit', async () => {
    const user = userEvent.setup()
    const router = createMemoryRouter(
      [
        { path: '/login', element: <LoginPage /> },
        { path: '/dashboard', element: <div>Dashboard</div> },
      ],
      {
        initialEntries: ['/login'],
      },
    )

    renderWithRouter(router)

    await user.type(await screen.findByLabelText('管理员 Token'), 'demo-admin-token')
    await user.click(screen.getByRole('button', { name: '进入控制台' }))

    expect(window.sessionStorage.getItem(ADMIN_TOKEN_KEY)).toBe('demo-admin-token')
    expect(await screen.findByText('Dashboard')).toBeInTheDocument()
  })

  it('shows validation error when token is blank', async () => {
    const user = userEvent.setup()
    const router = createMemoryRouter([{ path: '/login', element: <LoginPage /> }], {
      initialEntries: ['/login'],
    })

    renderWithRouter(router)

    await user.click(screen.getByRole('button', { name: '进入控制台' }))

    expect(screen.getByText('请输入管理员 Token')).toBeInTheDocument()
  })

  it('shows GitHub login button when OAuth is available', async () => {
    mockOAuthConfig(true)
    const user = userEvent.setup()
    const router = createMemoryRouter([{ path: '/login', element: <LoginPage /> }], {
      initialEntries: ['/login'],
    })
    renderWithRouter(router)

    await user.click(screen.getByText('用户登录'))

    await waitFor(() => {
      expect(screen.getByText('使用 GitHub 登录')).toBeInTheDocument()
    })

    const link = screen.getByText('使用 GitHub 登录').closest('a')
    expect(link?.getAttribute('href')).toBe('/api/auth/oauth/github')
  })

  it('hides GitHub login button when OAuth is unavailable', async () => {
    mockOAuthConfig(false)
    const user = userEvent.setup()
    const router = createMemoryRouter([{ path: '/login', element: <LoginPage /> }], {
      initialEntries: ['/login'],
    })
    renderWithRouter(router)

    await user.click(screen.getByText('用户登录'))
    await new Promise(r => setTimeout(r, 100))
    expect(screen.queryByText('使用 GitHub 登录')).not.toBeInTheDocument()
  })
})

function renderWithRouter(router: ReturnType<typeof createMemoryRouter>) {
  return render(<RouterProvider router={router} />)
}
