import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken, setToken } from '../lib/auth'
import { DriftDashboardPage } from './DriftDashboardPage'

describe('DriftDashboardPage', () => {
  beforeEach(() => {
    setToken('demo-admin-token')
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  it('renders drift rows and summary metrics', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          object: 'list',
          data: [
            {
              id: 'drift-1',
              environment: 'prod',
              agent_id: 'agent-1',
              active_model: 'gpt-4o-mini',
              recommended_model: 'claude-3-7-sonnet',
              status: 'detected',
              detected_at: '2026-04-19T10:00:00Z',
            },
            {
              id: 'drift-2',
              environment: 'prod',
              agent_id: 'agent-2',
              active_model: 'claude-3-haiku',
              recommended_model: 'gpt-4.1-mini',
              status: 'resolved',
              detected_at: '2026-04-19T11:00:00Z',
            },
          ],
        }),
        {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledTimes(1)
    })

    expect(await screen.findByRole('heading', { name: '漂移仪表盘', level: 1 })).toBeInTheDocument()
    expect(await screen.findByText('drift-1')).toBeInTheDocument()
    expect(screen.getByText('drift-2')).toBeInTheDocument()
    expect(screen.getByText('漂移总数')).toBeInTheDocument()
    expect(screen.getByText('已检测')).toBeInTheDocument()
    expect(screen.getByText('已解决')).toBeInTheDocument()
  })

  it('shows empty state when no drift rows', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          object: 'list',
          data: [],
        }),
        {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    expect(await screen.findByText('当前没有漂移数据。')).toBeInTheDocument()
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
    <MemoryRouter initialEntries={['/drifts']}>
      <QueryClientProvider client={queryClient}>
        <DriftDashboardPage />
      </QueryClientProvider>
    </MemoryRouter>,
  )
}
