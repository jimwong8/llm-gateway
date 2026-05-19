import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken, setToken } from './lib/auth'
import { router as appRouter } from './router'
import './i18n'

describe('router protection', () => {
  beforeEach(() => {
    clearToken()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('redirects unauthenticated users to login', async () => {
    const router = createMemoryRouter(appRouter.routes, {
      initialEntries: ['/dashboard'],
    })

    renderWithProviders(router)

    expect(await screen.findByRole('heading', { name: /登录/ })).toBeTruthy()
  })

  it('allows authenticated users to access dashboard', async () => {
    setToken('demo-admin-token')

    const router = createMemoryRouter(appRouter.routes, {
      initialEntries: ['/dashboard'],
    })

    renderWithProviders(router)

    expect(await screen.findByRole('heading', { name: '仪表盘', level: 1 })).toBeTruthy()
  })

  it('allows authenticated users to access memory governance page', async () => {
    setToken('demo-admin-token')

    const memoryResponse = JSON.stringify({
      object: 'list',
      tenant_id: '',
      user_id: '',
      status: '',
      data: [],
    })
    const fetchMock = vi.fn().mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/user/broadcasts')) {
        return new Response(JSON.stringify({ object: 'list', data: [], read_ids: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(memoryResponse, {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    const router = createMemoryRouter(appRouter.routes, {
      initialEntries: ['/memory-governance'],
    })

    renderWithProviders(router)

    expect(await screen.findByRole('heading', { name: '记忆治理', level: 1 })).toBeTruthy()
  })

  it('allows authenticated users to access runtime observer page', async () => {
    setToken('demo-admin-token')

    const runtimeResponse = JSON.stringify({
      environment: 'prod',
      active_policy: { version_id: '' },
      cache: { entry_count: 0, entries: [], invalidation_count: 0 },
      facts: { runtime_decisions: [], distribution_events: [] },
    })
    const fetchMock = vi.fn().mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/user/broadcasts')) {
        return new Response(JSON.stringify({ object: 'list', data: [], read_ids: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(runtimeResponse, {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    const router = createMemoryRouter(appRouter.routes, {
      initialEntries: ['/runtime-observer'],
    })

    renderWithProviders(router)

    expect(await screen.findByRole('heading', { name: '运行时观测', level: 1 })).toBeTruthy()
  })

  it('allows authenticated users to access runtime observer page', async () => {
    setToken('demo-admin-token')

    const runtimeResponse = JSON.stringify({
      environment: 'prod',
      active_policy: { version_id: '' },
      cache: { entry_count: 0, entries: [], invalidation_count: 0 },
      facts: { runtime_decisions: [], distribution_events: [] },
    })
    const fetchMock = vi.fn().mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/user/broadcasts')) {
        return new Response(JSON.stringify({ object: 'list', data: [], read_ids: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(runtimeResponse, {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    const router = createMemoryRouter(appRouter.routes, {
      initialEntries: ['/runtime-observer'],
    })

    renderWithProviders(router)

    expect(await screen.findByRole('heading', { name: '运行时观测', level: 1 })).toBeInTheDocument()
  })
})

function renderWithProviders(router: ReturnType<typeof createMemoryRouter>) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  })

  return render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
}
