import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ADMIN_TOKEN_KEY, clearToken, setToken } from './auth'
import { ApiError, apiRequest } from './http'

describe('apiRequest', () => {
  beforeEach(() => {
    clearToken()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('adds bearer token to admin requests', async () => {
    setToken('demo-admin-token')

    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    await apiRequest('/admin/health')

    expect(fetchMock).toHaveBeenCalledTimes(1)
    const [, init] = fetchMock.mock.calls[0]
    const headers = new Headers(init?.headers)
    expect(headers.get('Authorization')).toBe('Bearer demo-admin-token')
  })

  it('clears token and throws ApiError on 401 response', async () => {
    setToken('demo-admin-token')

    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: { message: 'admin authentication required' } }), {
        status: 401,
        statusText: 'Unauthorized',
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    await expect(apiRequest('/admin/health')).rejects.toMatchObject({
      name: 'ApiError',
      status: 401,
      message: 'admin authentication required',
    })

    expect(window.sessionStorage.getItem(ADMIN_TOKEN_KEY)).toBeNull()
  })
})
