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

describe('debug2', () => {
  afterEach(() => { vi.restoreAllMocks() })

  it('check query error', async () => {
    setToken('demo-admin-token')
    
    // Track all fetch calls
    const fetchSpy = vi.spyOn(global, 'fetch').mockImplementation(async (url) => {
      console.log('FETCH CALLED:', url)
      return new Response(JSON.stringify(listResponse), { status: 200, headers: { 'Content-Type': 'application/json' } })
    })
    
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={qc}>
        <MemoryRouter>
          <BroadcastPage />
        </MemoryRouter>
      </QueryClientProvider>,
    )
    
    await new Promise(r => setTimeout(r, 3000))
    
    console.log('FETCH CALLS:', fetchSpy.mock.calls.length)
    console.log('BODY_MINI:', document.body.innerHTML.substring(0, 500))
    clearToken()
    expect(true).toBe(true)
  })
})
