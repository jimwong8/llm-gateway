import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { LoginPage } from './LoginPage'
import { ADMIN_TOKEN_KEY } from '../lib/auth'

describe('LoginPage', () => {
  beforeEach(() => {
    window.sessionStorage.clear()
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
})

function renderWithRouter(router: ReturnType<typeof createMemoryRouter>) {
  return render(<RouterProvider router={router} />)
}
