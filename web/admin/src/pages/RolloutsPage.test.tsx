import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken, setToken } from '../lib/auth'
import { RolloutsPage } from './RolloutsPage'

describe('RolloutsPage', () => {
  beforeEach(() => {
    setToken('demo-admin-token')
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  it('renders rollout rows and summary metrics', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          object: 'list',
          data: [
            {
              id: 'ro-1',
              policy_version_id: 'pv-1',
              environment: 'prod',
              rollout_mode: 'progressive',
              rollout_percent: 50,
              status: 'running',
              triggered_by: 'ops-bot',
              error_rate: 0.012,
              p95_latency: 640,
              fallback_rate: 0.008,
              sample_count: 1200,
              updated_at: '2026-04-19T10:00:00Z',
            },
            {
              id: 'ro-2',
              policy_version_id: 'pv-2',
              environment: 'prod',
              rollout_mode: 'progressive',
              rollout_percent: 100,
              status: 'promoted',
              triggered_by: 'ops-bot',
              error_rate: 0.062,
              p95_latency: 1620,
              fallback_rate: 0.041,
              sample_count: 2300,
              updated_at: '2026-04-19T11:00:00Z',
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

    expect(await screen.findByRole('heading', { name: 'Governance Rollouts', level: 1 })).toBeInTheDocument()
    expect(await screen.findByText('ro-1')).toBeInTheDocument()
    expect(screen.getByText('ro-2')).toBeInTheDocument()
    expect(screen.getByText('Total Rollouts')).toBeInTheDocument()
    expect(screen.getByText('Average Percent')).toBeInTheDocument()
    expect(screen.getByText('Dashboard Error Rate')).toBeInTheDocument()
    expect(screen.getByText('Dashboard P95 Latency')).toBeInTheDocument()
    expect(screen.getByText('Dashboard Fallback Rate')).toBeInTheDocument()
    expect(screen.getByText('Dashboard Samples')).toBeInTheDocument()
    expect(screen.getByText('Healthy')).toBeInTheDocument()
    expect(screen.getByText('Watch')).toBeInTheDocument()
    expect(screen.getByText('Critical')).toBeInTheDocument()
    expect(screen.getByRole('cell', { name: '1.2%' })).toBeInTheDocument()
    expect(screen.getByRole('cell', { name: '640 ms' })).toBeInTheDocument()
    expect(screen.getByRole('cell', { name: '0.8%' })).toBeInTheDocument()
    expect(screen.getByRole('cell', { name: '1200' })).toBeInTheDocument()
  })

  it('opens rollback dialog and submits rollback request', async () => {
    const user = userEvent.setup()
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            object: 'list',
            data: [
              {
                id: 'ro-1',
                policy_version_id: 'pv-1',
                environment: 'prod',
                rollout_mode: 'progressive',
                rollout_percent: 60,
                status: 'running',
                triggered_by: 'ops-bot',
                error_rate: 0.018,
                p95_latency: 720,
                fallback_rate: 0.006,
                sample_count: 900,
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
            rollback: {
              id: 'rb-1',
              rollout_id: 'ro-1',
              actor: 'ops-reviewer',
              environment: 'prod',
              restored_policy_version_id: 'pv-prev',
              reverted_policy_version_id: 'pv-cur',
              reason: 'manual rollback',
              status: 'completed',
            },
            result: {},
          }),
          {
            status: 201,
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
                id: 'ro-1',
                policy_version_id: 'pv-1',
                environment: 'prod',
                rollout_mode: 'progressive',
                rollout_percent: 60,
                status: 'running',
                triggered_by: 'ops-bot',
                error_rate: 0.018,
                p95_latency: 720,
                fallback_rate: 0.006,
                sample_count: 900,
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

    await screen.findByText('ro-1')
    await user.click(screen.getByRole('button', { name: '回滚' }))

    const dialog = screen.getByRole('dialog', { name: '回滚 Rollout' })
    const form = within(dialog).getByRole('form', { name: 'Rollback Release Form' })

    await user.clear(within(form).getByLabelText('Actor'))
    await user.type(within(form).getByLabelText('Actor'), 'ops-reviewer')
    await user.clear(within(form).getByLabelText('Reason'))
    await user.type(within(form).getByLabelText('Reason'), 'manual rollback')
    await user.click(within(form).getByRole('button', { name: '确认回滚' }))

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledTimes(3)
    })

    const [rollbackUrl, rollbackInit] = fetchMock.mock.calls[1]
    expect(String(rollbackUrl)).toBe('/admin/governance/rollbacks')
    expect(rollbackInit).toMatchObject({ method: 'POST' })
    const body = rollbackInit?.body ? JSON.parse(String(rollbackInit.body)) : {}
    expect(body).toMatchObject({
      rollout_id: 'ro-1',
      actor: 'ops-reviewer',
      reason: 'manual rollback',
    })

    expect(await screen.findByText('已触发回滚：rb-1')).toBeInTheDocument()
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
    <MemoryRouter initialEntries={['/rollouts']}>
      <QueryClientProvider client={queryClient}>
        <RolloutsPage />
      </QueryClientProvider>
    </MemoryRouter>,
  )
}
