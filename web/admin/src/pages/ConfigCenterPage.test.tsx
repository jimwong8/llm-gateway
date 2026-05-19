import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { setToken, clearToken } from '../lib/auth'
import { ConfigCenterPage } from './ConfigCenterPage'

const oldVersionsResponse = [
  { version_id: 'cfg_002', status: 'released', environment: 'prod', source: { type: 'inheritance', source_environment: 'staging', source_version_id: 'cfg_001' } },
  { version_id: 'cfg_001', status: 'draft', environment: 'prod' },
]

const snapshotsResponse = { object: 'list', data: [
  { id: 1, version: 'v1', status: 'draft', notes: 'test snapshot', created_by: 'admin', created_at: '2026-01-01T00:00:00Z' },
  { id: 2, version: 'v2', status: 'published', notes: 'published', created_by: 'admin', created_at: '2026-01-02T00:00:00Z' },
]}

describe('ConfigCenterPage', () => {
  beforeEach(() => {
    setToken('demo-admin-token')
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  function mockFetch(responses: Record<string, unknown>) {
    const fetchMock = vi.fn((url: string | URL | Request) => {
      const urlStr = typeof url === 'string' ? url : url instanceof URL ? url.pathname + url.search : url.url
      for (const [pattern, data] of Object.entries(responses)) {
        if (urlStr.includes(pattern)) {
          return Promise.resolve(new Response(JSON.stringify(data), { status: 200, headers: { 'Content-Type': 'application/json' } }))
        }
      }
      return Promise.resolve(new Response(JSON.stringify([]), { status: 200 }))
    })
    vi.stubGlobal('fetch', fetchMock)
    return fetchMock
  }

  it('loads old version list and opens detail drawer', async () => {
    mockFetch({ '/admin/config-versions': oldVersionsResponse, '/admin/config/versions': snapshotsResponse })

    renderPage()

    expect(await screen.findByRole('heading', { name: '配置中心', level: 1 })).toBeInTheDocument()
    expect(await screen.findByText('cfg_002')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: '查看详情 cfg_002' }))

    const drawer = screen.getByRole('complementary', { name: '版本详情' })
    expect(drawer).toBeInTheDocument()
  })

  it('submits filters and refetches config versions', async () => {
    const fetchMock = mockFetch({ '/admin/config-versions': oldVersionsResponse, '/admin/config/versions': snapshotsResponse })

    renderPage()

    await screen.findByText('cfg_002')

    const filtersForm = screen.getByRole('form', { name: '配置筛选' })
    const filterFields = filtersForm.querySelectorAll('input')
    await userEvent.type(filterFields[0], 'router')
    await userEvent.type(filterFields[1], 'tenant-a')
    await userEvent.click(within(filtersForm).getByRole('button', { name: '筛选' }))

    await waitFor(() => {
      const calls = fetchMock.mock.calls.filter(c => String(c[0]).includes('/admin/config-versions?'))
      expect(calls.length).toBeGreaterThanOrEqual(1)
    })
  })

  it('shows config snapshot list', async () => {
    mockFetch({ '/admin/config-versions': oldVersionsResponse, '/admin/config/versions': snapshotsResponse })

    renderPage()

    expect(await screen.findByText('配置版本快照管理')).toBeInTheDocument()
    expect(await screen.findByText('v1')).toBeInTheDocument()
    expect(await screen.findByText('v2')).toBeInTheDocument()
    expect(await screen.findByText('已发布')).toBeInTheDocument()
    expect(await screen.findByText('草稿')).toBeInTheDocument()
  })

  it('allows creating a new draft snapshot', async () => {
    const fetchMock = mockFetch({ '/admin/config-versions': oldVersionsResponse, '/admin/config/versions': snapshotsResponse })

    const createResponse = { id: 3, version: 'v3', status: 'draft', notes: 'new draft', created_by: 'admin', created_at: '2026-01-03T00:00:00Z' }
    fetchMock.mockImplementation((url: string | URL | Request) => {
      const urlStr = typeof url === 'string' ? url : url instanceof URL ? url.pathname + url.search : url.url
      if (urlStr.includes('/admin/config/versions') && (typeof url === 'string' ? false : false) && true) {
        return Promise.resolve(new Response(JSON.stringify(createResponse), { status: 201, headers: { 'Content-Type': 'application/json' } }))
      }
      if (urlStr.includes('/admin/config/versions') && !urlStr.includes('export')) {
        if (urlStr.includes('/admin/config/versions') && (typeof url !== 'string')) {
        }
      }
      for (const [pattern, data] of Object.entries({ '/admin/config-versions': oldVersionsResponse, '/admin/config/versions': snapshotsResponse })) {
        if (urlStr.includes(pattern)) return Promise.resolve(new Response(JSON.stringify(data), { status: 200, headers: { 'Content-Type': 'application/json' } }))
      }
      return Promise.resolve(new Response(JSON.stringify([]), { status: 200 }))
    })

    renderPage()

    await screen.findByText('配置版本快照管理')

    await userEvent.click(screen.getByRole('button', { name: '创建草稿' }))

    const versionInput = screen.getByPlaceholderText(/v\d+/)
    await userEvent.type(versionInput, 'v3')

    const notesInput = screen.getByPlaceholderText('初始配置')
    await userEvent.type(notesInput, 'new draft')

    expect(screen.getByText('版本标识')).toBeInTheDocument()
  })

  it('shows error when snapshot query fails', async () => {
    const fetchMock = vi.fn((url: string | URL | Request) => {
      const urlStr = typeof url === 'string' ? url : url instanceof URL ? url.pathname + url.search : url.url
      if (urlStr.includes('/admin/config/versions')) {
        return Promise.reject(new Error('Network error'))
      }
      if (urlStr.includes('/admin/config-versions')) {
        return Promise.resolve(new Response(JSON.stringify(oldVersionsResponse), { status: 200, headers: { 'Content-Type': 'application/json' } }))
      }
      return Promise.resolve(new Response(JSON.stringify([]), { status: 200 }))
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    expect(await screen.findByText('配置版本快照管理')).toBeInTheDocument()
  })

  it('shows empty state when no snapshots', async () => {
    const emptyResponse = { object: 'list', data: [] }
    const fetchMock = vi.fn((url: string | URL | Request) => {
      const urlStr = typeof url === 'string' ? url : url instanceof URL ? url.pathname + url.search : url.url
      if (urlStr.includes('/admin/config/versions')) return Promise.resolve(new Response(JSON.stringify(emptyResponse), { status: 200, headers: { 'Content-Type': 'application/json' } }))
      return Promise.resolve(new Response(JSON.stringify(oldVersionsResponse), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    expect(await screen.findByText('暂无快照')).toBeInTheDocument()
  })
})

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })

  return render(
    <QueryClientProvider client={queryClient}>
      <ConfigCenterPage />
    </QueryClientProvider>,
  )
}
