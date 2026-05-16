import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken, setToken } from '../lib/auth'
import { ReleasesPage } from './ReleasesPage'

describe('ReleasesPage', () => {
  beforeEach(() => {
    setToken('demo-admin-token')
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  it('renders release and promotion workbench sections', async () => {
    render(<ReleasesPage />)

    expect(screen.getByRole('heading', { name: '发布管理', level: 1 })).toBeInTheDocument()
    expect(screen.getByRole('form', { name: '发布 Draft 表单' })).toBeInTheDocument()
    expect(screen.getByRole('form', { name: '推广表单' })).toBeInTheDocument()
  })

  it('shows latest result after a release action succeeds', async () => {
    const user = userEvent.setup()
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          version_id: 'cfg_201',
          status: 'released',
          environment: 'prod',
          source: {
            type: 'inheritance',
            source_environment: 'staging',
            source_version_id: 'cfg_200',
          },
        }),
        {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    render(<ReleasesPage />)

    const releaseForm = screen.getByRole('form', { name: '发布 Draft 表单' })
    await user.type(within(releaseForm).getByLabelText('模块'), 'router')
    await user.type(within(releaseForm).getByLabelText('租户 ID'), 'tenant-a')
    await user.type(within(releaseForm).getByLabelText('环境'), 'prod')
    await user.clear(within(releaseForm).getByLabelText('作用域'))
    await user.type(within(releaseForm).getByLabelText('作用域'), 'tenant')
    await user.type(within(releaseForm).getByLabelText('版本 ID'), 'cfg_201')
    await user.click(within(releaseForm).getByRole('button', { name: '发布 Draft' }))

    expect(await screen.findByText('已发布 cfg_201')).toBeInTheDocument()
    expect(screen.getByText('最近一次操作结果')).toBeInTheDocument()
    expect(screen.getByText('cfg_201')).toBeInTheDocument()
  })
})

import { within } from '@testing-library/react'
