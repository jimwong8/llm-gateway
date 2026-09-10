import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken, setToken } from '../lib/auth'
import { PolicyVersionsPage } from './PolicyVersionsPage'

describe('PolicyVersionsPage', () => {
  beforeEach(() => {
    setToken('demo-admin-token')
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  it('renders policy versions and selected diff content', async () => {
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
        JSON.stringify({
          object: 'list',
          data: [
            {
              id: 'pv-3',
              environment: 'prod',
              status: 'active',
              created_by: 'ops',
              approved_by: 'owner',
              activated_at: '2026-04-19T10:00:00Z',
            },
            {
              id: 'pv-2',
              environment: 'prod',
              status: 'approved',
              created_by: 'ops',
            },
          ],
        }),
        JSON.stringify({
          from: 'pv-2',
          to: 'pv-3',
          changed_fields: ['routing.default_model'],
        }),
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

    expect(await screen.findByRole('heading', { name: '策略版本', level: 1 })).toBeInTheDocument()
    expect(await screen.findByRole('button', { name: 'pv-3' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'pv-2' })).toBeInTheDocument()

    await waitFor(() => {
      expect(fetchMock.mock.calls.length).toBeGreaterThanOrEqual(2)
    })

    const nonBroadcastCalls = fetchMock.mock.calls.filter(
      (call) => !String(call[0]).includes('/api/user/broadcasts'),
    )
    expect(String(nonBroadcastCalls[0][0])).toContain('/admin/governance/policy-versions?limit=50')
    expect(String(nonBroadcastCalls[1][0])).toContain('/admin/governance/policy-versions/pv-3/diff')

    expect(screen.getByTestId('policy-diff-content')).toHaveTextContent('routing.default_model')
  })

  it('supports approve and activate actions for straightforward transitions', async () => {
    const user = userEvent.setup()
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
        JSON.stringify({
          object: 'list',
          data: [
            {
              id: 'pv-draft',
              environment: 'prod',
              status: 'draft',
              created_by: 'ops',
            },
            {
              id: 'pv-approved',
              environment: 'prod',
              status: 'approved',
              created_by: 'ops',
            },
          ],
        }),
        JSON.stringify({
          diff: 'initial diff payload',
        }),
        JSON.stringify({
          id: 'pv-draft',
          environment: 'prod',
          status: 'approved',
        }),
        JSON.stringify({
          object: 'list',
          data: [
            {
              id: 'pv-draft',
              environment: 'prod',
              status: 'approved',
              created_by: 'ops',
              approved_by: 'admin-ui',
            },
            {
              id: 'pv-approved',
              environment: 'prod',
              status: 'approved',
              created_by: 'ops',
            },
          ],
        }),
        JSON.stringify({
          id: 'pv-approved',
          environment: 'prod',
          status: 'active',
        }),
        JSON.stringify({
          object: 'list',
          data: [
            {
              id: 'pv-approved',
              environment: 'prod',
              status: 'active',
              created_by: 'ops',
            },
            {
              id: 'pv-draft',
              environment: 'prod',
              status: 'approved',
              created_by: 'ops',
            },
          ],
        }),
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

    expect(await screen.findByRole('button', { name: 'pv-draft' })).toBeInTheDocument()

    await user.click(screen.getAllByRole('button', { name: '批准' })[0])

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        expect.stringContaining('/admin/governance/policy-versions/pv-draft/approve'),
        expect.objectContaining({ method: 'POST' }),
      )
    })

    const approveCall = fetchMock.mock.calls.find((call) => String(call[0]).includes('/admin/governance/policy-versions/pv-draft/approve'))
    const approveInit = approveCall?.[1]
    expect(approveInit).toMatchObject({ method: 'POST' })
    expect(JSON.parse(String(approveInit?.body ?? '{}'))).toMatchObject({ approved_by: 'admin-ui' })

    await user.click(screen.getAllByRole('button', { name: '激活' })[1])

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        expect.stringContaining('/admin/governance/policy-versions/pv-approved/activate'),
        expect.objectContaining({ method: 'POST' }),
      )
    })

    const activateCall = fetchMock.mock.calls.find((call) => String(call[0]).includes('/admin/governance/policy-versions/pv-approved/activate'))
    const activateInit = activateCall?.[1]

    expect(activateInit).toMatchObject({ method: 'POST' })

    expect(await screen.findByText('已激活策略版本 pv-approved')).toBeInTheDocument()
  })

  it('shows graceful message when diff API is unavailable', async () => {
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
        {
          body: JSON.stringify({
            object: 'list',
            data: [
              {
                id: 'pv-1',
                environment: 'prod',
                status: 'active',
              },
            ],
          }),
          status: 200,
        },
        {
          body: JSON.stringify({ error: { message: 'route not found' } }),
          status: 404,
        },
      ]
      const resp = responses[Math.min(callIndex, responses.length - 1)]
      callIndex++
      return new Response(resp.body, {
        status: resp.status,
        headers: { 'Content-Type': 'application/json' },
      })
    })

    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    expect(await screen.findByRole('button', { name: 'pv-1' })).toBeInTheDocument()
    expect(await screen.findByText('版本差异暂不可用（diff API 尚未就绪或返回异常）。')).toBeInTheDocument()
  })
  it('selects version from environment query and offers rollout deep link', async () => {
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
        JSON.stringify({
          object: 'list',
          data: [
            {
              id: 'pv-staging',
              environment: 'staging',
              status: 'approved',
              created_by: 'ops',
            },
            {
              id: 'pv-prod',
              environment: 'prod',
              status: 'active',
              created_by: 'ops',
            },
          ],
        }),
        JSON.stringify({ diff: 'staging diff payload' }),
      ]
      const body = responses[Math.min(callIndex, responses.length - 1)]
      callIndex++
      return new Response(body, {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    })

    vi.stubGlobal('fetch', fetchMock)

    renderPage('/policy-versions?environment=staging')

    expect(await screen.findByText('当前选中')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'pv-staging' })).toBeInTheDocument()
    expect(screen.getAllByRole('link', { name: '查看灰度发布' })[0]).toHaveAttribute('href', '/rollouts?policyVersionId=pv-staging&environment=staging')
  })
})

function renderPage(initialEntry = '/policy-versions') {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  })

  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <QueryClientProvider client={queryClient}>
        <PolicyVersionsPage />
      </QueryClientProvider>
    </MemoryRouter>,
  )
}
