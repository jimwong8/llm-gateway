import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { setToken, clearToken } from '../../lib/auth'
import { CreateDraftForm } from './CreateDraftForm'

describe('CreateDraftForm', () => {
  beforeEach(() => {
    setToken('demo-admin-token')
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  it('validates required fields before submit', async () => {
    const user = userEvent.setup()

    render(<CreateDraftForm />)

    await user.click(screen.getByRole('button', { name: '创建 Draft' }))

    expect(screen.getByText('请填写必填字段')).toBeInTheDocument()
  })

  it('submits create draft request and reports success', async () => {
    const user = userEvent.setup()
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
          version_id: 'cfg_101',
          status: 'draft',
          environment: 'prod',
          source: {
            type: 'inheritance',
            source_environment: 'staging',
            source_version_id: 'cfg_100',
          },
        }),
        {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        },
      )
    })
    vi.stubGlobal('fetch', fetchMock)

    const onCreated = vi.fn()
    render(<CreateDraftForm onCreated={onCreated} />)

    await user.type(screen.getByLabelText('模块'), 'router')
    await user.type(screen.getByLabelText('租户 ID'), 'tenant-a')
    await user.clear(screen.getByLabelText('作用域'))
    await user.type(screen.getByLabelText('作用域'), 'tenant')
    await user.type(screen.getByLabelText('来源环境'), 'staging')
    await user.type(screen.getByLabelText('目标环境'), 'prod')
    await user.type(screen.getByLabelText('执行人'), 'release-bot')
    await user.type(screen.getByLabelText('原因'), 'prepare prod draft')

    await user.click(screen.getByRole('button', { name: '创建 Draft' }))

    expect(await screen.findByText('已创建 Draft cfg_101')).toBeInTheDocument()
    expect(fetchMock.mock.calls.length).toBeGreaterThanOrEqual(1)
    expect(String(fetchMock.mock.calls[0][0])).toBe('/admin/inheritance-drafts')
    expect(onCreated).toHaveBeenCalledWith(
      expect.objectContaining({
        version_id: 'cfg_101',
        status: 'draft',
      }),
    )
  })
})
