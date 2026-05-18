import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { setToken, clearToken } from '../../lib/auth'
import { BroadcastPage } from './BroadcastPage'

const listResponse = {
  object: 'list',
  data: [
    { id: 1, title: '通知一', content: '内容一', type: 'info', start_at: '2025-01-01T00:00:00Z', end_at: '2026-01-01T00:00:00Z', created_by: 'admin', created_at: '2025-01-01T00:00:00Z', updated_at: '2025-01-01T00:00:00Z' },
  ],
}

describe('debug', () => {
  beforeEach(() => {
    setToken('demo-admin-token')
    vi.spyOn(global, 'fetch').mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/user/broadcasts')) {
        return new Response(JSON.stringify({ object: 'list', data: [], read_ids: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } })
      }
      return new Response(JSON.stringify(listResponse), { status: 200, headers: { 'Content-Type': 'application/json' } })
    })
  })
  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  it('check fetch calls', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={qc}>
        <MemoryRouter>
          <BroadcastPage />
        </MemoryRouter>
      </QueryClientProvider>,
    )
    
    await new Promise(r => setTimeout(r, 2000))
    
    const body = document.body.innerHTML
    console.log('BODY_CONTAINS_加载失败:', body.includes('加载失败'))
    console.log('BODY_CONTAINS_通知一:', body.includes('通知一'))
    console.log('BODY_CONTAINS_加载中:', body.includes('加载中'))
    expect(true).toBe(true)
  })
})
