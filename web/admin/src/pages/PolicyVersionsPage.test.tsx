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
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
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
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            from: 'pv-2',
            to: 'pv-3',
            changed_fields: ['routing.default_model'],
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      )

    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    expect(await screen.findByRole('heading', { name: 'Policy Version Center', level: 1 })).toBeInTheDocument()
    expect(await screen.findByRole('button', { name: 'pv-3' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'pv-2' })).toBeInTheDocument()

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledTimes(2)
    })

    expect(String(fetchMock.mock.calls[0][0])).toContain('/admin/governance/policy-versions?limit=50')
    expect(String(fetchMock.mock.calls[1][0])).toContain('/admin/governance/policy-versions/pv-3/diff')

    expect(screen.getByTestId('policy-diff-content')).toHaveTextContent('routing.default_model')
  })

  it('supports approve and activate actions for straightforward transitions', async () => {
    const user = userEvent.setup()
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
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
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            diff: 'initial diff payload',
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            id: 'pv-draft',
            environment: 'prod',
            status: 'approved',
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
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
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            id: 'pv-approved',
            environment: 'prod',
            status: 'active',
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
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
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      )

    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    expect(await screen.findByRole('button', { name: 'pv-draft' })).toBeInTheDocument()

    await user.click(screen.getAllByRole('button', { name: 'Approve' })[0])

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

    await user.click(screen.getAllByRole('button', { name: 'Activate' })[1])

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
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            object: 'list',
            data: [
              {
                id: 'pv-1',
                environment: 'prod',
                status: 'active',
              },
            ],
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ error: { message: 'route not found' } }), {
          status: 404,
          headers: { 'Content-Type': 'application/json' },
        }),
      )

    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    expect(await screen.findByRole('button', { name: 'pv-1' })).toBeInTheDocument()
    expect(await screen.findByText('版本差异暂不可用（diff API 尚未就绪或返回异常）。')).toBeInTheDocument()
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
    <MemoryRouter initialEntries={['/policy-versions']}>
      <QueryClientProvider client={queryClient}>
        <PolicyVersionsPage />
      </QueryClientProvider>
    </MemoryRouter>,
  )
}
