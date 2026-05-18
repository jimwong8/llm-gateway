import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken, setToken } from '../lib/auth'
import { RuntimeObserverPage } from './RuntimeObserverPage'

describe('RuntimeObserverPage', () => {
  beforeEach(() => {
    setToken('demo-admin-token')
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  it('renders active policy, cache state and recent facts', async () => {
    const fetchMock = vi.fn().mockImplementation(async (input: RequestInfo | URL) => {
      const url = typeof input === 'string' ? input : input.toString()
      if (url.includes('/api/user/broadcasts')) {
        return new Response(JSON.stringify({ object: 'list', data: [], read_ids: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(
        JSON.stringify({
          environment: 'prod',
          observed_at: '2026-04-19T12:00:00Z',
          active_policy: {
            version_id: 'pv-1',
            updated_at: '2026-04-19T11:59:00Z',
          },
          cache: {
            entry_count: 1,
            entries: [{ environment: 'prod', policy_version_id: 'pv-1', cached_at: '2026-04-19T11:58:00Z' }],
            invalidation_count: 2,
            last_invalidated_at: '2026-04-19T11:57:00Z',
            last_invalidated_environment: 'prod',
          },
          facts: {
            runtime_decisions: [{ request_id: 'req-1', resolved_model: 'gpt-4o-mini', matched_scope_type: 'agent', created_at: '2026-04-19T11:56:00Z' }],
            distribution_events: [{ event_id: 'ev-1', event_type: 'policy_distribution.activated', rollout_id: 'ro-1', created_at: '2026-04-19T11:55:00Z' }],
          },
        }),
        {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        },
      )
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    expect(await screen.findByRole('heading', { name: '运行时观测', level: 1 })).toBeInTheDocument()
    expect(await screen.findByText('活跃策略')).toBeInTheDocument()
    expect(await screen.findByText('req-1')).toBeInTheDocument()
    expect(await screen.findByText('ev-1')).toBeInTheDocument()
    expect(screen.getByText('失效次数')).toBeInTheDocument()
  })

  it('submits selected environment in request', async () => {
    const fetchMock = vi.fn().mockImplementation(async (input: RequestInfo | URL) => {
      const url = typeof input === 'string' ? input : input.toString()
      if (url.includes('/api/user/broadcasts')) {
        return new Response(JSON.stringify({ object: 'list', data: [], read_ids: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(
        JSON.stringify({
          environment: 'staging',
          active_policy: { version_id: '' },
          cache: { entry_count: 0, entries: [], invalidation_count: 0 },
          facts: { runtime_decisions: [], distribution_events: [] },
        }),
        {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        },
      )
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    const form = screen.getByRole('form', { name: '运行时观测筛选' })
    await userEvent.clear(within(form).getByLabelText('环境'))
    await userEvent.type(within(form).getByLabelText('环境'), 'staging')
    await userEvent.click(within(form).getByRole('button', { name: '刷新观察数据' }))

    await waitFor(() => {
      const urls = fetchMock.mock.calls.map((call) => String(call[0]))
      expect(urls.some((url) => url.includes('environment=staging'))).toBe(true)
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
    <MemoryRouter initialEntries={['/runtime-observer']}>
      <QueryClientProvider client={queryClient}>
        <RuntimeObserverPage />
      </QueryClientProvider>
    </MemoryRouter>,
  )
}
