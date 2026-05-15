import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken, setToken } from '../lib/auth'
import { MemoryGovernancePage } from './MemoryGovernancePage'

describe('MemoryGovernancePage', () => {
  beforeEach(() => {
    setToken('demo-admin-token')
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  it('renders candidate facts, project facts and summary metrics', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [
              {
                id: 1,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'repo',
                fact_value: 'mono',
                source_text: 'we use monorepo',
                status: 'active',
                source_message_seq: 7,
                confirmation_count: 2,
                updated_at: '2026-04-19T10:00:00Z',
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
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [
              {
                id: 2,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'stack',
                fact_value: 'go',
                source_text: 'backend stack is go',
                status: 'active',
                source_message_seq: 3,
                last_verified_at: '2026-04-19T09:00:00Z',
                updated_at: '2026-04-19T09:30:00Z',
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
      expect(fetchMock).toHaveBeenCalledTimes(2)
    })

    expect(await screen.findByRole('heading', { name: 'Memory Governance', level: 1 })).toBeTruthy()
    expect(await screen.findByText('repo')).toBeTruthy()
    expect(screen.getByText('mono')).toBeTruthy()
    expect(screen.getByText('we use monorepo')).toBeTruthy()
    expect(screen.getByText('stack')).toBeTruthy()
    expect(screen.getByText('backend stack is go')).toBeTruthy()
    expect(screen.getByText('Candidate Facts')).toBeTruthy()
    expect(screen.getByText('Project Facts')).toBeTruthy()
    expect(screen.getByText('Active Project Facts')).toBeTruthy()
    expect(screen.getByText('Operator Notes')).toBeTruthy()
    expect(screen.getByText('选择一条事实查看详情')).toBeTruthy()
  })

  it('sorts candidate facts by status and updated time', async () => {
    const user = userEvent.setup()
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [
              {
                id: 1,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'zeta',
                fact_value: 'late',
                source_text: 'late update',
                status: 'promoted',
                source_message_seq: 7,
                confirmation_count: 2,
                updated_at: '2026-04-19T10:00:00Z',
              },
              {
                id: 2,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'alpha',
                fact_value: 'early',
                source_text: 'early update',
                status: 'pending',
                source_message_seq: 8,
                confirmation_count: 1,
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
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [],
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      )

    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    await screen.findByText('alpha')

    const candidateRegion = screen.getByRole('region', { name: 'Candidate Facts Table' })
    const candidateTable = within(candidateRegion).getByRole('table')
    let rows = within(candidateTable).getAllByRole('row')
    expect(within(rows[1]).getByText('alpha')).toBeTruthy()

    await user.click(within(candidateRegion).getByRole('button', { name: /状态/i }))
    rows = within(candidateTable).getAllByRole('row')
    expect(within(rows[1]).getByText('zeta')).toBeTruthy()

    await user.click(within(candidateRegion).getByRole('button', { name: /状态/i }))
    rows = within(candidateTable).getAllByRole('row')
    expect(within(rows[1]).getByText('alpha')).toBeTruthy()
  })

  it('asks for confirmation before running batch candidate actions', async () => {
    const user = userEvent.setup()
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [
              {
                id: 1,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'repo',
                fact_value: 'mono',
                source_text: 'we use monorepo',
                status: 'pending',
                source_message_seq: 7,
                confirmation_count: 1,
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
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [],
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
            action: 'confirm',
            success_count: 1,
            failure_count: 0,
            results: [
              {
                fact_key: 'repo',
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                status: 'confirmed',
                fact: {
                  id: 1,
                  tenant_id: 'tenant-a',
                  user_id: 'user-1',
                  fact_key: 'repo',
                  fact_value: 'mono',
                  source_text: 'we use monorepo',
                  status: 'confirmed',
                  source_message_seq: 7,
                  confirmation_count: 2,
                },
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
          JSON.stringify({ object: 'list', tenant_id: '', user_id: '', status: '', data: [{ id: 1, tenant_id: 'tenant-a', user_id: 'user-1', fact_key: 'repo', fact_value: 'mono', source_text: 'we use monorepo', status: 'confirmed', source_message_seq: 7, confirmation_count: 2 }] }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        ),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ object: 'list', tenant_id: '', user_id: '', status: '', data: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )

    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    await screen.findByText('repo')
    await user.click(screen.getByRole('checkbox', { name: '选择候选事实 repo' }))
    await user.click(screen.getByRole('button', { name: '批量确认' }))

    expect(screen.getByRole('dialog', { name: 'Batch Action Confirmation' })).toBeTruthy()
    expect(screen.getByRole('heading', { name: '确认批量确认' })).toBeTruthy()
    expect(fetchMock).toHaveBeenCalledTimes(2)

    await user.click(screen.getByRole('button', { name: '取消' }))
    expect(screen.queryByRole('dialog', { name: 'Batch Action Confirmation' })).toBeNull()
    expect(fetchMock).toHaveBeenCalledTimes(2)

    await user.click(screen.getByRole('button', { name: '批量确认' }))
    await user.click(screen.getByRole('button', { name: '确认批量确认' }))

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledTimes(5)
    })
    expect(await screen.findByText('批量确认完成：成功 1 条。repo→confirmed')).toBeTruthy()
  })

  it('shows selected fact details for candidate and project rows', async () => {
    const user = userEvent.setup()
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [
              {
                id: 1,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'repo',
                fact_value: 'mono',
                source_text: 'we use monorepo',
                status: 'confirmed',
                source_message_seq: 7,
                confirmation_count: 2,
                updated_at: '2026-04-19T10:00:00Z',
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
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [
              {
                id: 2,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'stack',
                fact_value: 'go',
                source_text: 'backend stack is go',
                status: 'superseded',
                source_message_seq: 3,
                superseded_by: 8,
                last_verified_at: '2026-04-19T09:00:00Z',
                updated_at: '2026-04-19T09:30:00Z',
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

    await screen.findByText('repo')

    await user.click(screen.getByText('repo'))
    expect(screen.getByText('事实详情')).toBeTruthy()
    expect(screen.getByText('Candidate Fact')).toBeTruthy()
    expect(screen.getByText('Allowed Actions')).toBeTruthy()
    expect(screen.getByText('拒绝 / 提升')).toBeTruthy()

    await user.click(screen.getByText('stack'))
    expect(screen.getByText('Project Fact')).toBeTruthy()
    expect(screen.getByText('由事实 #8 取代')).toBeTruthy()
  })

  it('submits filters and candidate fact action with current auth/api conventions', async () => {
    const user = userEvent.setup()
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [
              {
                id: 1,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'repo',
                fact_value: 'mono',
                source_text: 'we use monorepo',
                status: 'pending',
                source_message_seq: 7,
                confirmation_count: 1,
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
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [],
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
            tenant_id: 'tenant-a',
            user_id: 'user-1',
            status: 'pending',
            data: [
              {
                id: 1,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'repo',
                fact_value: 'mono',
                source_text: 'we use monorepo',
                status: 'pending',
                source_message_seq: 7,
                confirmation_count: 1,
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
            object: 'list',
            tenant_id: 'tenant-a',
            user_id: 'user-1',
            status: 'pending',
            data: [],
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
            id: 1,
            tenant_id: 'tenant-a',
            user_id: 'user-1',
            fact_key: 'repo',
            fact_value: 'mono',
            source_text: 'we use monorepo',
            status: 'confirmed',
            source_message_seq: 7,
            confirmation_count: 2,
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
            tenant_id: 'tenant-a',
            user_id: 'user-1',
            status: 'pending',
            data: [
              {
                id: 1,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'repo',
                fact_value: 'mono',
                source_text: 'we use monorepo',
                status: 'confirmed',
                source_message_seq: 7,
                confirmation_count: 2,
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
            object: 'list',
            tenant_id: 'tenant-a',
            user_id: 'user-1',
            status: 'pending',
            data: [
              {
                id: 2,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'repo',
                fact_value: 'mono',
                source_text: 'we use monorepo',
                status: 'active',
                source_message_seq: 7,
                last_verified_at: '2026-04-19T11:00:00Z',
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

    await screen.findByText('repo')

    const form = screen.getByRole('form', { name: 'Memory Governance Filters' })
    await user.type(within(form).getByLabelText('Tenant ID'), 'tenant-a')
    await user.type(within(form).getByLabelText('User ID'), 'user-1')
    await user.selectOptions(within(form).getByLabelText('Candidate Status'), 'pending')
    await user.selectOptions(within(form).getByLabelText('Project Status'), 'active')
    await user.click(within(form).getByRole('button', { name: '刷新 Memory Facts' }))

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledTimes(4)
    })

    expect(String(fetchMock.mock.calls[2][0])).toContain('/admin/memory/candidate-facts?tenant_id=tenant-a&user_id=user-1&status=pending')
    expect(String(fetchMock.mock.calls[3][0])).toContain('/admin/memory/project-facts?tenant_id=tenant-a&user_id=user-1&status=active')

    await user.click(screen.getByRole('button', { name: '确认' }))

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledTimes(7)
    })

    const [actionUrl, actionInit] = fetchMock.mock.calls[4]
    expect(String(actionUrl)).toBe('/admin/memory/candidate-facts/repo/confirm')
    expect(actionInit).toMatchObject({ method: 'POST' })
    const body = actionInit?.body ? JSON.parse(String(actionInit.body)) : {}
    expect(body).toEqual({
      tenant_id: 'tenant-a',
      user_id: 'user-1',
    })

    expect(await screen.findByText('已确认事实：repo（当前状态：confirmed）')).toBeTruthy()
    expect(screen.getByText('最近操作')).toBeTruthy()
  })

  it('supports batch confirm for selected visible candidate facts', async () => {
    const user = userEvent.setup()
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [
              {
                id: 1,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'repo',
                fact_value: 'mono',
                source_text: 'we use monorepo',
                status: 'pending',
                source_message_seq: 7,
                confirmation_count: 1,
              },
              {
                id: 2,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'stack',
                fact_value: 'go',
                source_text: 'backend stack is go',
                status: 'pending',
                source_message_seq: 8,
                confirmation_count: 1,
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
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [],
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
            action: 'confirm',
            success_count: 2,
            failure_count: 0,
            results: [
              {
                fact_key: 'repo',
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                status: 'confirmed',
                fact: {
                  id: 1,
                  tenant_id: 'tenant-a',
                  user_id: 'user-1',
                  fact_key: 'repo',
                  fact_value: 'mono',
                  source_text: 'we use monorepo',
                  status: 'confirmed',
                  source_message_seq: 7,
                  confirmation_count: 2,
                },
              },
              {
                fact_key: 'stack',
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                status: 'confirmed',
                fact: {
                  id: 2,
                  tenant_id: 'tenant-a',
                  user_id: 'user-1',
                  fact_key: 'stack',
                  fact_value: 'go',
                  source_text: 'backend stack is go',
                  status: 'confirmed',
                  source_message_seq: 8,
                  confirmation_count: 2,
                },
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
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [
              {
                id: 1,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'repo',
                fact_value: 'mono',
                source_text: 'we use monorepo',
                status: 'confirmed',
                source_message_seq: 7,
                confirmation_count: 2,
              },
              {
                id: 2,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'stack',
                fact_value: 'go',
                source_text: 'backend stack is go',
                status: 'confirmed',
                source_message_seq: 8,
                confirmation_count: 2,
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
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [],
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      )

    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    await screen.findByText('repo')

    await user.click(screen.getByRole('checkbox', { name: '选择当前可见候选事实' }))
    expect(screen.getByText('已选当前可见 2 / 2 · 本地命中 2 · 已拉取 2 · 可确认 2 · 可拒绝 2 · 可提升 0')).toBeTruthy()

    await user.click(screen.getByRole('button', { name: '批量确认' }))
    await user.click(screen.getByRole('button', { name: '确认批量确认' }))

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledTimes(5)
    })

    const [batchActionUrl, batchActionInit] = fetchMock.mock.calls[2]
    expect(String(batchActionUrl)).toBe('/admin/memory/candidate-facts/actions/confirm')
    expect(batchActionInit).toMatchObject({ method: 'POST' })
    const batchBody = batchActionInit?.body ? JSON.parse(String(batchActionInit.body)) : {}
    expect(batchBody.items).toEqual(expect.arrayContaining([
      { tenant_id: 'tenant-a', user_id: 'user-1', fact_key: 'repo' },
      { tenant_id: 'tenant-a', user_id: 'user-1', fact_key: 'stack' },
    ]))
    expect(batchBody.items).toHaveLength(2)
    expect(await screen.findByText('批量确认完成：成功 2 条。repo→confirmed；stack→confirmed')).toBeTruthy()
    expect(screen.getByLabelText('Candidate Fact Batch Actions').textContent).toContain('已选当前可见 0 / 2')
  })

  it('keeps failed selections and shows aggregated batch errors', async () => {
    const user = userEvent.setup()
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [
              {
                id: 1,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'repo',
                fact_value: 'mono',
                source_text: 'we use monorepo',
                status: 'pending',
                source_message_seq: 7,
                confirmation_count: 1,
              },
              {
                id: 2,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'stack',
                fact_value: 'go',
                source_text: 'backend stack is go',
                status: 'pending',
                source_message_seq: 8,
                confirmation_count: 1,
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
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [],
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
            action: 'confirm',
            success_count: 1,
            failure_count: 1,
            results: [
              {
                fact_key: 'repo',
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                status: 'confirmed',
                fact: {
                  id: 1,
                  tenant_id: 'tenant-a',
                  user_id: 'user-1',
                  fact_key: 'repo',
                  fact_value: 'mono',
                  source_text: 'we use monorepo',
                  status: 'confirmed',
                  source_message_seq: 7,
                  confirmation_count: 2,
                },
              },
              {
                fact_key: 'stack',
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                error: { message: 'fact locked', type: 'memory_governance_error' },
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
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [
              {
                id: 1,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'repo',
                fact_value: 'mono',
                source_text: 'we use monorepo',
                status: 'confirmed',
                source_message_seq: 7,
                confirmation_count: 2,
              },
              {
                id: 2,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'stack',
                fact_value: 'go',
                source_text: 'backend stack is go',
                status: 'pending',
                source_message_seq: 8,
                confirmation_count: 1,
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
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [],
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      )

    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    await screen.findByText('repo')

    await user.click(screen.getByRole('checkbox', { name: '选择当前可见候选事实' }))
    await user.click(screen.getByRole('button', { name: '批量确认' }))
    await user.click(screen.getByRole('button', { name: '确认批量确认' }))

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledTimes(5)
    })

    expect(await screen.findByText('批量确认完成：成功 1 条，失败 1 条。repo→confirmed')).toBeTruthy()
    expect(screen.getByLabelText('Candidate Fact Batch Actions').textContent).toContain('已选当前可见 1 / 2')
  })

  it('supports local search, page size control, pagination, and visible-scope selection', async () => {
    const user = userEvent.setup()
    const candidateData = Array.from({ length: 12 }, (_, index) => ({
      id: index + 1,
      tenant_id: 'tenant-a',
      user_id: `user-${index + 1}`,
      fact_key: `fact-${String(index + 1).padStart(2, '0')}`,
      fact_value: index === 10 ? 'special-match' : `value-${index + 1}`,
      source_text: index === 10 ? 'special source text' : `source text ${index + 1}`,
      status: 'pending',
      source_message_seq: index + 1,
      confirmation_count: 1,
      updated_at: `2026-04-${String(20 + index).padStart(2, '0')}T10:00:00Z`,
    }))

    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: candidateData,
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
            tenant_id: '',
            user_id: '',
            status: '',
            data: [],
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      )

    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    await screen.findByText('fact-12')

    const candidateRegion = screen.getByRole('region', { name: 'Candidate Facts Table' })
    const candidatePager = screen.getByRole('group', { name: 'Candidate Fact Pagination' })
    expect(within(candidateRegion).queryByText('fact-02')).toBeNull()
    expect(screen.getByText('Page 1 / 2 · 显示第 1-10 条，共 12 条')).toBeTruthy()

    await user.click(within(candidatePager).getByRole('button', { name: 'Next' }))
    expect(await within(candidateRegion).findByText('fact-02')).toBeTruthy()
    expect(within(candidateRegion).queryByText('fact-12')).toBeNull()
    expect(screen.getByText('Page 2 / 2 · 显示第 11-12 条，共 12 条')).toBeTruthy()

    await user.click(screen.getByRole('checkbox', { name: '选择当前可见候选事实' }))
    expect(screen.getByText('已选当前可见 2 / 2 · 本地命中 12 · 已拉取 12 · 可确认 2 · 可拒绝 2 · 可提升 0')).toBeTruthy()

    await user.click(within(candidatePager).getByRole('button', { name: 'Previous' }))
    expect(await within(candidateRegion).findByText('fact-12')).toBeTruthy()
    expect(screen.getByText('已选当前可见 0 / 10 · 本地命中 12 · 已拉取 12 · 可确认 0 · 可拒绝 0 · 可提升 0')).toBeTruthy()

    const localControls = screen.getByRole('region', { name: 'Candidate Fact Local Controls' })
    await user.clear(within(localControls).getByLabelText('本地搜索'))
    await user.type(within(localControls).getByLabelText('本地搜索'), 'special-match')

    expect(await within(candidateRegion).findByText('fact-11')).toBeTruthy()
    expect(within(candidateRegion).queryByText('fact-12')).toBeNull()
    expect(screen.getByText('Page 1 / 1 · 显示第 1-1 条，共 1 条')).toBeTruthy()
    expect(screen.getByText('显示第 1-1 条，共 1 条本地命中结果（后端已拉取 12 条）。')).toBeTruthy()

    await user.clear(within(localControls).getByLabelText('本地搜索'))
    await user.selectOptions(within(localControls).getByLabelText('Rows per page'), '25')

    expect(await within(candidateRegion).findByText('fact-02')).toBeTruthy()
    expect(within(candidateRegion).getByText('fact-12')).toBeTruthy()
    expect(screen.getByText('Page 1 / 1 · 显示第 1-12 条，共 12 条')).toBeTruthy()
  })

  it('supports batch reject for selected visible candidate facts', async () => {
    const user = userEvent.setup()
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [
              {
                id: 1,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'repo',
                fact_value: 'mono',
                source_text: 'we use monorepo',
                status: 'confirmed',
                source_message_seq: 7,
                confirmation_count: 2,
              },
              {
                id: 2,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'stack',
                fact_value: 'go',
                source_text: 'backend stack is go',
                status: 'pending',
                source_message_seq: 8,
                confirmation_count: 1,
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
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [],
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
            action: 'reject',
            success_count: 2,
            failure_count: 0,
            results: [
              {
                fact_key: 'repo',
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                status: 'rejected',
                fact: {
                  id: 1,
                  tenant_id: 'tenant-a',
                  user_id: 'user-1',
                  fact_key: 'repo',
                  fact_value: 'mono',
                  source_text: 'we use monorepo',
                  status: 'rejected',
                  source_message_seq: 7,
                  confirmation_count: 2,
                },
              },
              {
                fact_key: 'stack',
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                status: 'rejected',
                fact: {
                  id: 2,
                  tenant_id: 'tenant-a',
                  user_id: 'user-1',
                  fact_key: 'stack',
                  fact_value: 'go',
                  source_text: 'backend stack is go',
                  status: 'rejected',
                  source_message_seq: 8,
                  confirmation_count: 1,
                },
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
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [
              {
                id: 1,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'repo',
                fact_value: 'mono',
                source_text: 'we use monorepo',
                status: 'rejected',
                source_message_seq: 7,
                confirmation_count: 2,
              },
              {
                id: 2,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'stack',
                fact_value: 'go',
                source_text: 'backend stack is go',
                status: 'rejected',
                source_message_seq: 8,
                confirmation_count: 1,
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
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [],
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      )

    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    await screen.findByText('repo')

    await user.click(screen.getByRole('checkbox', { name: '选择当前可见候选事实' }))
    await user.click(screen.getByRole('button', { name: '批量拒绝' }))
    await user.click(screen.getByRole('button', { name: '确认批量拒绝' }))

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledTimes(5)
    })

    const [batchActionUrl, batchActionInit] = fetchMock.mock.calls[2]
    expect(String(batchActionUrl)).toBe('/admin/memory/candidate-facts/actions/reject')
    expect(batchActionInit).toMatchObject({ method: 'POST' })
    expect(await screen.findByText('批量拒绝完成：成功 2 条。repo→rejected；stack→rejected')).toBeTruthy()
  })

  it('supports batch promote for selected visible candidate facts', async () => {
    const user = userEvent.setup()
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [
              {
                id: 1,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'repo',
                fact_value: 'mono',
                source_text: 'we use monorepo',
                status: 'confirmed',
                source_message_seq: 7,
                confirmation_count: 2,
              },
              {
                id: 2,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'stack',
                fact_value: 'go',
                source_text: 'backend stack is go',
                status: 'confirmed',
                source_message_seq: 8,
                confirmation_count: 3,
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
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [],
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
            action: 'promote',
            success_count: 2,
            failure_count: 0,
            results: [
              {
                fact_key: 'repo',
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                status: 'promoted',
                fact: {
                  id: 1,
                  tenant_id: 'tenant-a',
                  user_id: 'user-1',
                  fact_key: 'repo',
                  fact_value: 'mono',
                  source_text: 'we use monorepo',
                  status: 'promoted',
                  source_message_seq: 7,
                  confirmation_count: 2,
                },
              },
              {
                fact_key: 'stack',
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                status: 'promoted',
                fact: {
                  id: 2,
                  tenant_id: 'tenant-a',
                  user_id: 'user-1',
                  fact_key: 'stack',
                  fact_value: 'go',
                  source_text: 'backend stack is go',
                  status: 'promoted',
                  source_message_seq: 8,
                  confirmation_count: 3,
                },
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
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [
              {
                id: 1,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'repo',
                fact_value: 'mono',
                source_text: 'we use monorepo',
                status: 'promoted',
                source_message_seq: 7,
                confirmation_count: 2,
              },
              {
                id: 2,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'stack',
                fact_value: 'go',
                source_text: 'backend stack is go',
                status: 'promoted',
                source_message_seq: 8,
                confirmation_count: 3,
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
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [
              {
                id: 11,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'repo',
                fact_value: 'mono',
                source_text: 'we use monorepo',
                status: 'active',
                source_message_seq: 7,
                last_verified_at: '2026-04-19T11:00:00Z',
              },
              {
                id: 12,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'stack',
                fact_value: 'go',
                source_text: 'backend stack is go',
                status: 'active',
                source_message_seq: 8,
                last_verified_at: '2026-04-19T11:00:00Z',
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

    await screen.findByText('repo')

    await user.click(screen.getByRole('checkbox', { name: '选择当前可见候选事实' }))
    await user.click(screen.getByRole('button', { name: '批量提升' }))
    await user.click(screen.getByRole('button', { name: '确认批量提升' }))

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledTimes(5)
    })

    const [batchActionUrl, batchActionInit] = fetchMock.mock.calls[2]
    expect(String(batchActionUrl)).toBe('/admin/memory/candidate-facts/actions/promote')
    expect(batchActionInit).toMatchObject({ method: 'POST' })
    expect(await screen.findByText('批量提升完成：成功 2 条。repo→promoted；stack→promoted')).toBeTruthy()
  })

  it('resets filters and disables invalid actions by candidate status', async () => {
    const user = userEvent.setup()
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [
              {
                id: 1,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'repo',
                fact_value: 'mono',
                source_text: 'we use monorepo',
                status: 'confirmed',
                source_message_seq: 7,
                confirmation_count: 2,
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
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [],
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
            tenant_id: '',
            user_id: '',
            status: '',
            data: [
              {
                id: 1,
                tenant_id: 'tenant-a',
                user_id: 'user-1',
                fact_key: 'repo',
                fact_value: 'mono',
                source_text: 'we use monorepo',
                status: 'confirmed',
                source_message_seq: 7,
                confirmation_count: 2,
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
            object: 'list',
            tenant_id: '',
            user_id: '',
            status: '',
            data: [],
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      )

    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    expect(await screen.findByText('repo')).toBeTruthy()
    expect(screen.getByRole('button', { name: '确认' }).hasAttribute('disabled')).toBe(true)
    expect(screen.getByRole('button', { name: '拒绝' }).hasAttribute('disabled')).toBe(false)
    expect(screen.getByRole('button', { name: '提升' }).hasAttribute('disabled')).toBe(false)

    const form = screen.getByRole('form', { name: 'Memory Governance Filters' })
    await user.type(within(form).getByLabelText('Tenant ID'), 'tenant-a')
    await user.type(within(form).getByLabelText('User ID'), 'user-1')
    await user.selectOptions(within(form).getByLabelText('Candidate Status'), 'confirmed')
    await user.selectOptions(within(form).getByLabelText('Project Status'), 'superseded')

    await user.click(screen.getByRole('checkbox', { name: '选择候选事实 repo' }))
    await user.click(within(form).getByRole('button', { name: '重置筛选' }))

    expect((within(form).getByLabelText('Tenant ID') as HTMLInputElement).value).toBe('')
    expect((within(form).getByLabelText('User ID') as HTMLInputElement).value).toBe('')
    expect((within(form).getByLabelText('Candidate Status') as HTMLSelectElement).value).toBe('')
    expect((within(form).getByLabelText('Project Status') as HTMLSelectElement).value).toBe('')
    expect(screen.getByText('已选当前可见 0 / 1 · 本地命中 1 · 已拉取 1 · 可确认 0 · 可拒绝 0 · 可提升 0')).toBeTruthy()
    expect(fetchMock).toHaveBeenCalledTimes(2)
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
    <MemoryRouter initialEntries={['/memory-governance']}>
      <QueryClientProvider client={queryClient}>
        <MemoryGovernancePage />
      </QueryClientProvider>
    </MemoryRouter>,
  )
}
