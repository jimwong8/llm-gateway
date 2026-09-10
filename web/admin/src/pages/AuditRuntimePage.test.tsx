import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken, setToken } from '../lib/auth'
import { AuditRuntimePage } from './AuditRuntimePage'

const auditEvents = [
  {
    type: 'release',
    tenant_id: 'tenant-a',
    environment: 'prod',
    version_id: 'cfg_201',
    actor: 'release-bot',
    created_at: '2026-03-25T00:00:00Z',
  },
]

const runtimeEvents = [
  {
    version: {
      module: 'router',
      tenant_id: 'tenant-a',
      environment: 'prod',
      scope: 'tenant',
      project_id: '',
      version: 'cfg_201',
      actor: 'release-bot',
      source: 'released',
      summary: 'promote',
      created_at: '2026-03-25T00:00:00Z',
      source_environment: 'staging',
      source_version: 'cfg_200',
    },
  },
]

describe('AuditRuntimePage', () => {
  beforeEach(() => {
    setToken('demo-admin-token')
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  it('switches between audit and runtime tabs', async () => {
    const fetchMock = vi.fn().mockImplementation(async (input: RequestInfo | URL) => {
      const url = typeof input === 'string' ? input : input.toString()
      if (url.includes('/api/user/broadcasts')) {
        return new Response(JSON.stringify({ object: 'list', data: [], read_ids: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      if (url.includes('/admin/audit-events')) {
        return new Response(JSON.stringify(auditEvents), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      if (url.includes('/admin/runtime-events')) {
        return new Response(JSON.stringify(runtimeEvents), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(JSON.stringify([]), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    expect(await screen.findByText('cfg_201', { exact: false })).toBeInTheDocument()

    await userEvent.click(screen.getByRole('tab', { name: '运行时事件' }))

    expect(await screen.findByText('router')).toBeInTheDocument()
    expect(screen.getByText('cfg_200', { exact: false })).toBeInTheDocument()
  })

  it('requests summary mode when selected', async () => {
    const fetchMock = vi.fn().mockImplementation(async (input: RequestInfo | URL) => {
      const url = typeof input === 'string' ? input : input.toString()
      if (url.includes('/api/user/broadcasts')) {
        return new Response(JSON.stringify({ object: 'list', data: [], read_ids: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      if (url.includes('summary=true')) {
        return new Response(
          JSON.stringify({
            total: 0,
            by_type: {},
            by_environment: {},
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        )
      }
      if (url.includes('/admin/audit-events') || url.includes('/admin/runtime-events')) {
        return new Response(
          JSON.stringify({
            total: 1,
            by_type: { release: 1 },
            by_environment: { prod: 1 },
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        )
      }
      return new Response(JSON.stringify(auditEvents), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    // Wait for data to load
    await screen.findByText('release', { exact: false })
    await userEvent.click(screen.getByRole('checkbox'))
    await userEvent.click(screen.getByRole('button', { name: '筛选' }))

    expect(await screen.findByText('总数')).toBeInTheDocument()
    const requestedUrls = fetchMock.mock.calls.map((call) => String(call[0]))
    expect(requestedUrls.some((url) => url.includes('summary=true'))).toBe(true)
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
      <AuditRuntimePage />
    </QueryClientProvider>,
  )
}
