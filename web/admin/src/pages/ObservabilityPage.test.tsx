import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken, setToken } from '../lib/auth'
import { ObservabilityPage } from './ObservabilityPage'

describe('ObservabilityPage', () => {
  beforeEach(() => {
    setToken('demo-admin-token')
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  it('renders summary, providers, and hotspots', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ requests: 12, cache_hit_rate: 0.5, provider_error_rate: 0.1, avg_latency_ms: 85.5 }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ object: 'list', data: [{ provider: 'openai', requests: 8, total_tokens: 240, provider_error_rate: 0.05 }] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ tenants: [{ key: 'tenant-a', requests: 8, total_tokens: 240, estimated_cost: 1.2 }], models: [{ key: 'gpt-4o-mini', requests: 8, total_tokens: 240, estimated_cost: 1.2 }] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    expect(await screen.findByRole('heading', { name: '可观测性', level: 1 })).toBeInTheDocument()
    expect(await screen.findByText('openai')).toBeInTheDocument()
    expect(await screen.findByText('tenant-a')).toBeInTheDocument()
    expect(await screen.findByText('gpt-4o-mini')).toBeInTheDocument()
  })

  it('applies filters to observability requests', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ object: 'list', data: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    const form = screen.getByRole('form', { name: '可观测性筛选' })
    await userEvent.type(within(form).getByLabelText('租户 ID'), 'tenant-a')
    await userEvent.click(within(form).getByRole('button', { name: '筛选' }))

    await waitFor(() => {
      const urls = fetchMock.mock.calls.map((call) => String(call[0]))
      expect(urls.some((url) => url.includes('tenant_id=tenant-a'))).toBe(true)
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
      <ObservabilityPage />
    </QueryClientProvider>,
  )
}
