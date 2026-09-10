import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken, setToken } from '../lib/auth'
import { clearUserToken, setUserToken } from '../lib/api/identity'
import { DashboardPage } from './DashboardPage'
import '../i18n'

function makeJWT(role: string): string {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }))
  const payload = btoa(JSON.stringify({ uid: 1, email: 't@t.com', role, exp: 9999999999 }))
  const signature = 'test-sig'
  return `${header}.${payload}.${signature}`
}

const adminFetchMocks = () =>
  vi.fn(async (input: RequestInfo | URL) => {
    const url = typeof input === 'string' ? input : input.toString()
    if (url.includes('/api/user/broadcasts')) {
      return new Response(JSON.stringify({ object: 'list', data: [], read_ids: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }
    if (url.includes('/admin/health')) {
      return new Response(JSON.stringify({ service: 'llm-gateway', admin_auth: 'enabled' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }
    if (url.includes('/admin/observability/summary')) {
      return new Response(
        JSON.stringify({
          requests: 12,
          total_tokens: 345,
          cache_hit_rate: 0.88,
          provider_error_rate: 0.125,
          avg_latency_ms: 123.4,
        }),
        {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        },
      )
    }
    return new Response(JSON.stringify({ data: [] }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    })
  })

const userFetchMocks = () =>
  vi.fn(async (input: RequestInfo | URL) => {
    const url = typeof input === 'string' ? input : input.toString()
    if (url.includes('/api/user/broadcasts')) {
      return new Response(JSON.stringify({ object: 'list', data: [], read_ids: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }
    return new Response(
      JSON.stringify({
        summary: { requests: 5, total_tokens: 1000, prompt_tokens: 600, completion_tokens: 400, estimated_cost: 0.002, avg_latency_ms: 50, provider_error_rate: 0.01, cache_hit_rate: 0.9 },
        model_distribution: [],
        recent_api_keys: [],
      }),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    )
  })

describe('DashboardPage - admin view', () => {
  beforeEach(() => {
    setToken('demo-admin-token')
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  it('renders service and summary cards from admin endpoints', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = typeof input === 'string' ? input : input.toString()
      if (url.includes('/api/user/broadcasts')) {
        return new Response(JSON.stringify({ object: 'list', data: [], read_ids: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      if (url.includes('/admin/health')) {
        return new Response(JSON.stringify({ service: 'llm-gateway', admin_auth: 'enabled' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(
        JSON.stringify({
          requests: 12,
          total_tokens: 345,
          cache_hit_rate: 0.88,
          provider_error_rate: 0.125,
          avg_latency_ms: 123.4,
        }),
        {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        },
      )
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    expect(await screen.findByRole('heading', { name: '仪表盘', level: 1 })).toBeInTheDocument()
    expect(screen.getAllByText('LLM Gateway').length).toBeGreaterThan(0)
    // Terminal-style dashboard renders polaris metrics from admin endpoints
    expect(await screen.findByText('RPS')).toBeInTheDocument()
  })

  it('renders terminal homepage sections', async () => {
    const fetchMock = adminFetchMocks()
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    // 北极星状态带 — RPS metric label
    expect(await screen.findByText('RPS')).toBeInTheDocument()
    // 渠道健康矩阵
    expect(await screen.findByText('渠道健康')).toBeInTheDocument()
    // 安全事件面板
    expect(await screen.findByText('安全事件')).toBeInTheDocument()
    // 快捷动作面板
    expect(await screen.findByText('快捷操作')).toBeInTheDocument()
  })

  it('renders loading state while queries are pending', async () => {
    // Deferred promise so fetch never resolves during the assertion
    let resolveFetch!: (value: Response) => void
    const deferred = new Promise<Response>((resolve) => {
      resolveFetch = resolve
    })
    const fetchMock = vi.fn().mockReturnValue(deferred)
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    // Loading text should appear when queries are in-flight
    expect(await screen.findByText('正在加载首页概览…')).toBeInTheDocument()

    // Resolve pending fetches so the test doesn't hang / produce act warnings
    resolveFetch(
      new Response(JSON.stringify({}), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    await vi.waitFor(() => {
      expect(screen.queryByText('正在加载首页概览…')).not.toBeInTheDocument()
    })
  })
})

describe('DashboardPage - user view with JWT role', () => {
  afterEach(() => {
    clearUserToken()
    vi.restoreAllMocks()
  })

  it('renders user dashboard when JWT role is user', async () => {
    setUserToken(makeJWT('user'))
    const fetchMock = userFetchMocks()
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    expect(await screen.findByRole('heading', { name: '我的面板', level: 1 })).toBeInTheDocument()
    expect(await screen.findByText('总请求数')).toBeInTheDocument()
    expect(screen.getByText('5')).toBeInTheDocument()
  })

  it('renders admin dashboard when JWT role is admin', async () => {
    setUserToken(makeJWT('admin'))
    const fetchMock = adminFetchMocks()
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    expect(await screen.findByRole('heading', { name: '仪表盘', level: 1 })).toBeInTheDocument()
    expect(screen.getAllByText('LLM Gateway').length).toBeGreaterThan(0)
  })

  it('renders admin dashboard when JWT role is operator', async () => {
    setUserToken(makeJWT('operator'))
    const fetchMock = adminFetchMocks()
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    expect(await screen.findByRole('heading', { name: '仪表盘', level: 1 })).toBeInTheDocument()
  })

  it('renders admin dashboard when JWT role is readonly', async () => {
    setUserToken(makeJWT('readonly'))
    const fetchMock = adminFetchMocks()
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    expect(await screen.findByRole('heading', { name: '仪表盘', level: 1 })).toBeInTheDocument()
  })

  it('shows user dashboard error state on fetch failure', async () => {
    setUserToken(makeJWT('user'))
    const fetchMock = vi.fn().mockRejectedValue(new Error('fetch failed'))
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    expect(await screen.findByText('用户面板加载失败')).toBeInTheDocument()
  })
})

describe('DashboardPage - no token', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders admin view when no user token exists', async () => {
    const fetchMock = adminFetchMocks()
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    expect(await screen.findByRole('heading', { name: '仪表盘', level: 1 })).toBeInTheDocument()
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
    <MemoryRouter initialEntries={['/dashboard']}>
      <QueryClientProvider client={queryClient}>
        <DashboardPage />
      </QueryClientProvider>
    </MemoryRouter>,
  )
}
