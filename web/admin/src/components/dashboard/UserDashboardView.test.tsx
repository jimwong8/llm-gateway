import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const defaultMockData = {
  summary: {
    requests: 42,
    total_tokens: 56789,
    prompt_tokens: 30000,
    completion_tokens: 26789,
    estimated_cost: 0.056,
    avg_latency_ms: 123.4,
    provider_error_rate: 0.02,
    cache_hit_rate: 0.88,
  },
  model_distribution: [
    { key: 'gpt-4', requests: 30, total_tokens: 40000, estimated_cost: 0.04 },
    { key: 'gpt-3.5', requests: 12, total_tokens: 16789, estimated_cost: 0.016 },
  ],
  recent_api_keys: [
    { id: 1, name: 'My Key', key_prefix: 'sk-test', status: 'active', created_at: '2025-01-01T00:00:00Z' },
  ],
}

const usageMockData = {
  object: 'list',
  data: [
    { date: '01/01', requests: 5, prompt_tokens: 300, completion_tokens: 200, total_tokens: 500, estimated_cost: 0.001 },
    { date: '01/02', requests: 8, prompt_tokens: 600, completion_tokens: 400, total_tokens: 1000, estimated_cost: 0.002 },
  ],
}

function setupFetchMocks(dashboardData = defaultMockData, usageData = usageMockData) {
  return vi
    .fn()
    .mockResolvedValueOnce(
      new Response(JSON.stringify(dashboardData), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    .mockResolvedValueOnce(
      new Response(JSON.stringify(usageData), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
}

describe('UserDashboardView', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', setupFetchMocks())
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders summary cards with data', async () => {
    const { UserDashboardView } = await import('./UserDashboardView')
    renderWithQuery(<UserDashboardView />)
    expect(await screen.findByText('42')).toBeInTheDocument()
    expect(screen.getByText('56789')).toBeInTheDocument()
    expect(screen.getByText('30000')).toBeInTheDocument()
    expect(screen.getByText('26789')).toBeInTheDocument()
    expect(screen.getByText('0.0560')).toBeInTheDocument()
  })

  it('renders enhanced summary metrics', async () => {
    const { UserDashboardView } = await import('./UserDashboardView')
    renderWithQuery(<UserDashboardView />)
    expect(await screen.findByText('平均延迟 (ms)')).toBeInTheDocument()
    expect(screen.getByText('123.4')).toBeInTheDocument()
    expect(screen.getByText('缓存命中率')).toBeInTheDocument()
    expect(screen.getByText('88.0%')).toBeInTheDocument()
    expect(screen.getByText('错误率')).toBeInTheDocument()
    expect(screen.getByText('2.0%')).toBeInTheDocument()
  })

  it('renders API keys table', async () => {
    const { UserDashboardView } = await import('./UserDashboardView')
    renderWithQuery(<UserDashboardView />)
    expect(await screen.findByText('My Key')).toBeInTheDocument()
    expect(screen.getByText('sk-test...')).toBeInTheDocument()
    expect(screen.getByText('active')).toBeInTheDocument()
  })

  it('hides API keys section when none exist', async () => {
    const { UserDashboardView } = await import('./UserDashboardView')
    vi.restoreAllMocks()
    vi.stubGlobal(
      'fetch',
      setupFetchMocks({ ...defaultMockData, recent_api_keys: [] }),
    )
    renderWithQuery(<UserDashboardView />)
    expect(await screen.findByText('42')).toBeInTheDocument()
    expect(screen.queryByText('我的 API Keys')).not.toBeInTheDocument()
  })

  it('shows loading state', async () => {
    const { UserDashboardView } = await import('./UserDashboardView')
    vi.restoreAllMocks()
    vi.stubGlobal('fetch', vi.fn(() => new Promise(() => {})))
    renderWithQuery(<UserDashboardView />)
    expect(screen.getByText('正在加载用户面板…')).toBeInTheDocument()
  })

  it('shows error state on fetch failure', async () => {
    const { UserDashboardView } = await import('./UserDashboardView')
    vi.restoreAllMocks()
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('fetch error')))
    renderWithQuery(<UserDashboardView />)
    expect(await screen.findByText('用户面板加载失败')).toBeInTheDocument()
  })

  it('renders model distribution section when data exists', async () => {
    const { UserDashboardView } = await import('./UserDashboardView')
    vi.restoreAllMocks()
    vi.stubGlobal('fetch', setupFetchMocks())
    renderWithQuery(<UserDashboardView />)
    expect(await screen.findByText('模型分布')).toBeInTheDocument()
  })

  it('hides model distribution when no data', async () => {
    const { UserDashboardView } = await import('./UserDashboardView')
    vi.restoreAllMocks()
    vi.stubGlobal(
      'fetch',
      setupFetchMocks({ ...defaultMockData, model_distribution: [] }),
    )
    renderWithQuery(<UserDashboardView />)
    expect(await screen.findByText('42')).toBeInTheDocument()
    expect(screen.queryByText('模型分布')).not.toBeInTheDocument()
  })
})

function renderWithQuery(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>)
}
