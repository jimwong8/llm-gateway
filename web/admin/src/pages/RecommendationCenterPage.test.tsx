import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken, setToken } from '../lib/auth'
import { RecommendationCenterPage } from './RecommendationCenterPage'

describe('RecommendationCenterPage', () => {
  beforeEach(() => {
    setToken('demo-admin-token')
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  it('renders recommendation rows and summary metrics', async () => {
    const fetchMock = vi.fn().mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/user/broadcasts')) {
        return new Response(JSON.stringify({ object: 'list', data: [], read_ids: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(
        JSON.stringify({
          object: 'list',
          data: [
            {
              id: 'rec-1',
              agent_id: 'agent-1',
              task_type: 'chat',
              environment: 'prod',
              recommended_model: 'gpt-4o-mini',
              status: 'pending',
              updated_at: '2026-04-19T10:00:00Z',
            },
            {
              id: 'rec-2',
              agent_id: 'agent-2',
              task_type: 'chat',
              environment: 'prod',
              recommended_model: 'claude-sonnet',
              status: 'approved',
              updated_at: '2026-04-19T11:00:00Z',
            },
          ],
        }),
        {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        },
      )
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    await waitFor(() => {
      expect(fetchMock.mock.calls.length).toBeGreaterThanOrEqual(1)
    })

    expect(await screen.findByRole('heading', { name: '推荐管理', level: 1 })).toBeInTheDocument()
    expect(await screen.findByText('rec-1')).toBeInTheDocument()
    expect(screen.getByText('rec-2')).toBeInTheDocument()
    expect(screen.getByText('推荐总数')).toBeInTheDocument()
    expect(screen.getByText('智能体数')).toBeInTheDocument()
  })

  it('opens approval dialog and submits approval request', async () => {
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
      callIndex++
      if (callIndex === 1) {
        return new Response(
          JSON.stringify({
            object: 'list',
            data: [
              {
                id: 'rec-1',
                agent_id: 'agent-1',
                task_type: 'chat',
                environment: 'prod',
                recommended_model: 'gpt-4o-mini',
                status: 'pending',
              },
            ],
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        )
      }
      if (callIndex === 2) {
        return new Response(
          JSON.stringify({
            id: 'ap-1',
            recommendation_id: 'rec-1',
            status: 'approved',
          }),
          {
            status: 201,
            headers: { 'Content-Type': 'application/json' },
          },
        )
      }
      return new Response(
        JSON.stringify({
          object: 'list',
          data: [
            {
              id: 'rec-1',
              agent_id: 'agent-1',
              task_type: 'chat',
              environment: 'prod',
              recommended_model: 'gpt-4o-mini',
              status: 'approved',
            },
          ],
        }),
        {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        },
      )
    })

    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    await screen.findByText('rec-1')
    expect(screen.getByRole('link', { name: '去审批页' })).toHaveAttribute('href', '/approvals?recommendationId=rec-1&environment=prod')

    await user.click(screen.getByRole('button', { name: '审批' }))

    const dialog = screen.getByRole('dialog', { name: '审批推荐' })
    const form = within(dialog).getByRole('form', { name: 'Governance Approval Form' })

    await user.clear(within(form).getByLabelText('审批人'))
    await user.type(within(form).getByLabelText('审批人'), 'ops-reviewer')
    await user.click(within(form).getByRole('button', { name: '确认审批' }))

    await waitFor(() => {
      expect(fetchMock.mock.calls.length).toBeGreaterThanOrEqual(3)
    })

    const approvalCall = fetchMock.mock.calls.find(
      (call) => String(call[0]) === '/admin/governance/approvals',
    )
    expect(approvalCall).toBeDefined()
    const [approvalUrl, approvalInit] = approvalCall!
    expect(String(approvalUrl)).toBe('/admin/governance/approvals')
    expect(approvalInit).toMatchObject({ method: 'POST' })
    const body = approvalInit?.body ? JSON.parse(String(approvalInit.body)) : {}
    expect(body).toMatchObject({
      recommendation_id: 'rec-1',
      decision: 'approved',
      approved_by: 'ops-reviewer',
      effective_scope: {
        scope: 'agent',
        environment: 'prod',
      },
    })

    expect(await screen.findByText('已创建审批：ap-1')).toBeInTheDocument()
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
    <MemoryRouter initialEntries={['/recommendations']}>
      <QueryClientProvider client={queryClient}>
        <RecommendationCenterPage />
      </QueryClientProvider>
    </MemoryRouter>,
  )
}
