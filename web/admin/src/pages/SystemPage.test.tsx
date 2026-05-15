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

describe('SystemPage', () => {
  beforeEach(() => {
    setToken('demo-admin-token')
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  it('loads health, usage, and audit summaries', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            service: 'llm-gateway',
            admin_auth: 'enabled',
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        ),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ object: 'list', data: [{ tenant_id: 'tenant-a' }] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ object: 'list', data: [{ request_id: 'req-1' }] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    vi.stubGlobal('fetch', fetchMock)

    renderWithQuery(<SystemPage />)

    expect(await screen.findByText('llm-gateway')).toBeInTheDocument()
    expect(screen.getByText('enabled')).toBeInTheDocument()
    expect(screen.getByText('Usage Items')).toBeInTheDocument()
    expect(screen.getByText('Audit Items')).toBeInTheDocument()
    expect(screen.getAllByText('1').length).toBeGreaterThanOrEqual(2)
  })
})
