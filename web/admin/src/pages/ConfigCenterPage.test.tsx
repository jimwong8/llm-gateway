import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { setToken, clearToken } from '../lib/auth'
import { ConfigCenterPage } from './ConfigCenterPage'

const versionsResponse = [
  {
    version_id: 'cfg_002',
    status: 'released',
    environment: 'prod',
    source: {
      type: 'inheritance',
      source_environment: 'staging',
      source_version_id: 'cfg_001',
    },
  },
  {
    version_id: 'cfg_001',
    status: 'draft',
    environment: 'prod',
  },
]

describe('ConfigCenterPage', () => {
  beforeEach(() => {
    setToken('demo-admin-token')
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  it('loads version list and opens detail drawer', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(versionsResponse), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    expect(await screen.findByRole('heading', { name: 'Config Center', level: 1 })).toBeInTheDocument()
    expect(await screen.findByText('cfg_002')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: '查看详情 cfg_002' }))

    const drawer = screen.getByRole('complementary', { name: 'Version details' })
    expect(within(drawer).getByText('Source Environment')).toBeInTheDocument()
    expect(within(drawer).getByText('staging')).toBeInTheDocument()
    expect(within(drawer).getByText('Source Version')).toBeInTheDocument()
    expect(within(drawer).getByText('cfg_001')).toBeInTheDocument()
  })

  it('submits filters and refetches config versions with query params', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(versionsResponse), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    await screen.findByText('cfg_002')

    const filtersForm = screen.getByRole('form', { name: 'Config Filters' })
    await userEvent.type(within(filtersForm).getByLabelText('Module'), 'router')
    await userEvent.type(within(filtersForm).getByLabelText('Tenant ID'), 'tenant-a')
    await userEvent.click(within(filtersForm).getByRole('button', { name: '应用筛选' }))

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledTimes(2)
    })

    expect(String(fetchMock.mock.calls[1][0])).toContain('/admin/config-versions?module=router&tenant_id=tenant-a')
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
      <ConfigCenterPage />
    </QueryClientProvider>,
  )
}
