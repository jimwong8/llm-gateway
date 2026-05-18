import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken, setToken } from '../lib/auth'
import { QuotaPage } from './QuotaPage'

describe('QuotaPage', () => {
  beforeEach(() => {
    setToken('demo-admin-token')
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  it('renders quota summary and trends', async () => {
    let callIndex = 0
    const fetchMock = vi.fn().mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/user/broadcasts')) {
        return new Response(JSON.stringify({ object: 'list', data: [], read_ids: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      const responses = [
        JSON.stringify({ tenant_id: 'tenant-a', used: 5, limit: 20, remaining: 15, rejected: 1, reject_rate: 0.2 }),
        JSON.stringify({ tenant_id: 'tenant-a', window_minutes: 15, points: [{ minute: '2026-03-25T00:00:00Z', used: 5, rejected: 1, remaining_estimate: 15 }] }),
      ]
      const body = responses[Math.min(callIndex, responses.length - 1)]
      callIndex++
      return new Response(body, {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    expect(await screen.findByRole('heading', { name: '配额管理', level: 1 })).toBeInTheDocument()
    expect(await screen.findByText('tenant-a')).toBeInTheDocument()
    expect(screen.getByText('剩余')).toBeInTheDocument()
    expect(screen.getByText('2026-03-25T00:00:00Z')).toBeInTheDocument()
  })

  it('applies tenant and window filters', async () => {
    const fetchMock = vi.fn().mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/user/broadcasts')) {
        return new Response(JSON.stringify({ object: 'list', data: [], read_ids: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(JSON.stringify({ tenant_id: 'tenant-a', window_minutes: 15, points: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    const form = screen.getByRole('form', { name: '配额筛选' })
    await userEvent.clear(within(form).getByLabelText('时间窗口（分钟）'))
    await userEvent.type(within(form).getByLabelText('时间窗口（分钟）'), '30')
    await userEvent.click(within(form).getByRole('button', { name: '筛选' }))

    await waitFor(() => {
      const urls = fetchMock.mock.calls.map((call) => String(call[0]))
      expect(urls.some((url) => url.includes('window_minutes=30'))).toBe(true)
    })
  })
})

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  })

  return render(
    <QueryClientProvider client={queryClient}>
      <QuotaPage />
    </QueryClientProvider>,
  )
}
