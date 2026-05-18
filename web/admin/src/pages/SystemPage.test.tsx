import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken, setToken } from '../lib/auth'
import { SystemPage } from './SystemPage'

function renderWithQuery(ui: React.ReactElement) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>)
}

const defaultSiteConfig = {
  site_name: 'LLM Gateway',
  logo_url: '',
  jwt_secret_configured: false,
  smtp_host: '',
  smtp_port: 587,
  smtp_user: '',
  smtp_from: '',
  allow_registration: true,
  default_user_role: 'user',
  default_user_quota: 1000000,
  updated_at: '2026-01-01T00:00:00Z',
  updated_by: 'admin',
}

describe('SystemPage', () => {
  beforeEach(() => {
    setToken('demo-admin-token')
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  it('loads site config form', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(defaultSiteConfig), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    renderWithQuery(<SystemPage />)

    await screen.findByRole('heading', { name: '系统设置', level: 1 })
    await screen.findByDisplayValue('LLM Gateway')
  })

  it('updates site config on form submit', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(defaultSiteConfig), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    renderWithQuery(<SystemPage />)

    await screen.findByDisplayValue('LLM Gateway')

    const nameInput = screen.getByPlaceholderText('LLM Gateway')
    await userEvent.clear(nameInput)
    await userEvent.type(nameInput, 'My Gateway')

    const submitBtn = screen.getByRole('button', { name: '保存配置' })
    await userEvent.click(submitBtn)
  })

  it('rotates JWT secret', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(
      new Response(JSON.stringify(defaultSiteConfig), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    renderWithQuery(<SystemPage />)

    await screen.findByDisplayValue('LLM Gateway')

    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ jwt_secret: 'abc123', jwt_secret_rotated_at: '2026-01-01T00:00:00Z' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const rotateBtn = screen.getByRole('button', { name: '重新生成 JWT Secret' })
    await userEvent.click(rotateBtn)
  })
})
