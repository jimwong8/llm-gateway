import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken, setToken } from '../lib/auth'
import { ApprovalsPage } from './ApprovalsPage'

describe('ApprovalsPage', () => {
  beforeEach(() => {
    setToken('demo-admin-token')
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  it('submits approve, override, reject actions', async () => {
    const user = userEvent.setup()
    const responses = [
      new Response(
        JSON.stringify({
          object: 'list',
          data: [
            {
              id: 'rec-1',
              agent_id: 'agent-a',
              task_type: 'chat',
              environment: 'prod',
              recommended_model: 'gpt-4o-mini',
              status: 'pending',
              updated_at: '2026-04-19T10:00:00Z',
            },
          ],
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
      new Response(
        JSON.stringify({ id: 'ap-1', recommendation_id: 'rec-1', decision: 'approved' }),
        { status: 201, headers: { 'Content-Type': 'application/json' } },
      ),
      new Response(
        JSON.stringify({ object: 'list', data: [] }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
      new Response(
        JSON.stringify({ id: 'ap-2', recommendation_id: 'rec-1', decision: 'overridden' }),
        { status: 201, headers: { 'Content-Type': 'application/json' } },
      ),
      new Response(
        JSON.stringify({ object: 'list', data: [] }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
      new Response(
        JSON.stringify({ id: 'ap-3', recommendation_id: 'rec-1', decision: 'rejected' }),
        { status: 201, headers: { 'Content-Type': 'application/json' } },
      ),
      new Response(
        JSON.stringify({ object: 'list', data: [] }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    ]
    let callIndex = 0
    const fetchMock = vi.fn().mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/user/broadcasts')) {
        return new Response(JSON.stringify({ object: 'list', data: [], read_ids: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      const response = responses[callIndex]
      callIndex++
      return response
    })

    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    await screen.findByText('rec-1')
    await user.click(screen.getByRole('button', { name: '选择' }))

    const form = screen.getByRole('form', { name: 'Governance Approval Form' })

    await user.click(within(form).getByRole('button', { name: '提交审批' }))
    await screen.findByText('审批成功：ap-1（approved）')

    await user.selectOptions(within(form).getByLabelText('决策'), 'overridden')
    await user.type(within(form).getByLabelText('最终模型'), 'gpt-4.1-mini')
    await user.click(within(form).getByRole('button', { name: '提交审批' }))
    await screen.findByText('审批成功：ap-2（overridden）')

    await user.selectOptions(within(form).getByLabelText('决策'), 'rejected')
    await user.clear(within(form).getByLabelText('审批原因'))
    await user.type(within(form).getByLabelText('审批原因'), 'manual reject')
    await user.click(within(form).getByRole('button', { name: '提交审批' }))
    await screen.findByText('审批成功：ap-3（rejected）')

    const approvalCalls = fetchMock.mock.calls.filter((call) => String(call[0]).includes('/admin/governance/approvals'))
    const approveBody = approvalCalls[0]?.[1]?.body ? JSON.parse(String(approvalCalls[0][1].body)) : {}
    expect(approveBody).toMatchObject({
      recommendation_id: 'rec-1',
      decision: 'approved',
      approved_by: 'ops-bot',
      effective_scope: {
        scope: 'agent',
        environment: 'prod',
      },
    })
    const overrideBody = approvalCalls[1]?.[1]?.body ? JSON.parse(String(approvalCalls[1][1].body)) : {}
    expect(overrideBody).toMatchObject({
      recommendation_id: 'rec-1',
      decision: 'overridden',
      final_model: 'gpt-4.1-mini',
    })
    const rejectBody = approvalCalls[2]?.[1]?.body ? JSON.parse(String(approvalCalls[2][1].body)) : {}
    expect(rejectBody).toMatchObject({
      recommendation_id: 'rec-1',
      decision: 'rejected',
      approval_reason: 'manual reject',
    })
  })

  it('shows validation and backend error messages', async () => {
    const user = userEvent.setup()
    const responses = [
      new Response(
        JSON.stringify({
          object: 'list',
          data: [
            {
              id: 'rec-1',
              agent_id: 'agent-a',
              task_type: 'chat',
              environment: 'prod',
            },
          ],
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
      new Response(
        JSON.stringify({ error: { message: 'decision invalid' } }),
        { status: 400, headers: { 'Content-Type': 'application/json' } },
      ),
    ]
    let callIndex = 0
    const fetchMock = vi.fn().mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/user/broadcasts')) {
        return new Response(JSON.stringify({ object: 'list', data: [], read_ids: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      const response = responses[callIndex]
      callIndex++
      return response
    })

    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    await screen.findByText('rec-1')
    await user.click(screen.getByRole('button', { name: '选择' }))

    const form = screen.getByRole('form', { name: 'Governance Approval Form' })
    await user.selectOptions(within(form).getByLabelText('决策'), 'overridden')
    await user.click(within(form).getByRole('button', { name: '提交审批' }))

    expect(await screen.findByText('覆盖决策必须填写最终模型。')).toBeInTheDocument()

    await user.type(within(form).getByLabelText('最终模型'), 'gpt-4o-mini')
    await user.click(within(form).getByRole('button', { name: '提交审批' }))

    await waitFor(() => {
      expect(fetchMock.mock.calls.length).toBeGreaterThanOrEqual(3)
    })

    expect(await screen.findByText('审批失败：decision invalid')).toBeInTheDocument()
  })
  it('prefills recommendation context from query params', async () => {
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
              agent_id: 'agent-a',
              task_type: 'chat',
              environment: 'prod',
            },
          ],
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      )
    })

    vi.stubGlobal('fetch', fetchMock)

    renderPage('/approvals?recommendationId=rec-1&environment=staging')

    expect(await screen.findByDisplayValue('rec-1')).toBeInTheDocument()
    expect(screen.getByDisplayValue('staging')).toBeInTheDocument()
  })
})

function renderPage(initialEntry = '/approvals') {
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
        <ApprovalsPage />
      </QueryClientProvider>
    </MemoryRouter>,
  )
}
