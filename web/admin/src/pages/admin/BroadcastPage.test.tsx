import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { setToken, clearToken } from '../../lib/auth'
import { BroadcastPage } from './BroadcastPage'

const listResponse = {
  object: 'list',
  data: [
    { id: 1, title: '通知一', content: '内容一', type: 'info', start_at: '2025-01-01T00:00:00Z', end_at: '2026-01-01T00:00:00Z', created_by: 'admin', created_at: '2025-01-01T00:00:00Z', updated_at: '2025-01-01T00:00:00Z' },
    { id: 2, title: '告警', content: '内容二', type: 'warning', start_at: '2025-06-01T00:00:00Z', end_at: '2025-12-31T00:00:00Z', created_by: 'admin', created_at: '2025-06-01T00:00:00Z', updated_at: '2025-06-01T00:00:00Z' },
  ],
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <BroadcastPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('BroadcastPage', () => {
  beforeEach(() => {
    setToken('demo-admin-token')
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  it('renders page title', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify(listResponse), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    renderPage()
    expect(await screen.findByText('广播管理')).toBeInTheDocument()
  })

  it('displays broadcast list', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify(listResponse), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    renderPage()
    expect(await screen.findByText('通知一')).toBeInTheDocument()
    expect(await screen.findByText('告警')).toBeInTheDocument()
  })

  it('shows empty state when no broadcasts', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ object: 'list', data: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    renderPage()
    expect(await screen.findByText('暂无广播')).toBeInTheDocument()
  })

  it('can create a broadcast', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch')
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify(listResponse), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ id: 3, title: '新广播', content: '新内容', type: 'critical', start_at: '2025-01-01T00:00:00Z', end_at: '2026-01-01T00:00:00Z', created_by: 'admin', created_at: '2025-01-01T00:00:00Z', updated_at: '2025-01-01T00:00:00Z' }), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    // Subsequent GET refetches
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify(listResponse), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )

    renderPage()
    await screen.findByText('通知一')

    const user = userEvent.setup()
    await user.type(screen.getByPlaceholderText('广播标题'), '新广播')
    await user.type(screen.getByPlaceholderText('广播内容'), '新内容')
    await user.click(screen.getByRole('button', { name: '创建' }))

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith('/admin/broadcasts', expect.objectContaining({ method: 'POST' }))
    })
  })
})
