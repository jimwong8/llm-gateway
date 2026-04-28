import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken, setToken } from '../lib/auth'
import { PoliciesPage } from './PoliciesPage'

describe('PoliciesPage', () => {
  beforeEach(() => {
    setToken('demo-admin-token')
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  it('renders policy models page', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ tenant_id: '', models: ['gpt-4o-mini', 'claude-sonnet'] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    expect(await screen.findByRole('heading', { name: 'Policies', level: 1 })).toBeTruthy()
    expect(await screen.findByText('gpt-4o-mini')).toBeTruthy()
    expect(screen.getByText('claude-sonnet')).toBeTruthy()
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
    <MemoryRouter initialEntries={['/policies']}>
      <QueryClientProvider client={queryClient}>
        <PoliciesPage />
      </QueryClientProvider>
    </MemoryRouter>,
  )
}
