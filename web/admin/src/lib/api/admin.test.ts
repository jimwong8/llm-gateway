import { ApiError, apiRequest, jsonRequest } from '../http'

describe('ApiError', () => {
  it('creates error with status, message, and payload', () => {
    const err = new ApiError(400, 'Bad Request', { error: 'invalid' })
    expect(err.status).toBe(400)
    expect(err.message).toBe('Bad Request')
    expect(err.payload).toEqual({ error: 'invalid' })
    expect(err.name).toBe('ApiError')
  })
})

describe('apiRequest', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('sends Authorization header when token exists', async () => {
    const fetchSpy = vi.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    window.sessionStorage.setItem('llm_gateway_admin_token', 'test-token')
    await apiRequest('/api/test')

    expect(fetchSpy).toHaveBeenCalledWith(
      '/api/test',
      expect.objectContaining({
        headers: expect.any(Headers),
      }),
    )

    const callArgs = fetchSpy.mock.calls[0]
    const headers = callArgs[1]?.headers as Headers
    expect(headers.get('Authorization')).toBe('Bearer test-token')
  })

  it('does not send Authorization header when auth is none', async () => {
    const fetchSpy = vi.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    window.sessionStorage.setItem('llm_gateway_admin_token', 'test-token')
    await apiRequest('/api/test', {}, { auth: 'none' })

    const callArgs = fetchSpy.mock.calls[0]
    const headers = callArgs[1]?.headers as Headers
    expect(headers.get('Authorization')).toBeNull()
  })

  it('throws ApiError on non-ok response', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ error: { message: 'Not Found' } }), {
        status: 404,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    window.sessionStorage.setItem('llm_gateway_admin_token', 'test-token')
    await expect(apiRequest('/api/missing')).rejects.toThrow(ApiError)
  })

  it('clears token on 401 response', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ error: 'Unauthorized' }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    window.sessionStorage.setItem('llm_gateway_admin_token', 'test-token')
    try {
      await apiRequest('/api/protected')
    } catch {
      // expected
    }

    expect(window.sessionStorage.getItem('llm_gateway_admin_token')).toBeNull()
  })

  it('clears token on 403 response', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ error: 'Forbidden' }), {
        status: 403,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    window.sessionStorage.setItem('llm_gateway_admin_token', 'test-token')
    try {
      await apiRequest('/api/protected')
    } catch {
      // expected
    }

    expect(window.sessionStorage.getItem('llm_gateway_admin_token')).toBeNull()
  })

  it('parses JSON response', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ data: 'value' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    window.sessionStorage.setItem('llm_gateway_admin_token', 'test-token')
    const result = await apiRequest<{ data: string }>('/api/data')
    expect(result).toEqual({ data: 'value' })
  })

  it('parses text response', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(
      new Response('plain text', {
        status: 200,
        headers: { 'Content-Type': 'text/plain' },
      }),
    )

    window.sessionStorage.setItem('llm_gateway_admin_token', 'test-token')
    const result = await apiRequest('/api/text')
    expect(result).toEqual({ message: 'plain text' })
  })
})

describe('jsonRequest', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('sends JSON body and Content-Type header', async () => {
    const fetchSpy = vi.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    window.sessionStorage.setItem('llm_gateway_admin_token', 'test-token')
    await jsonRequest('/api/create', { name: 'test' })

    const callArgs = fetchSpy.mock.calls[0]
    const init = callArgs[1] as RequestInit
    expect(init.method).toBe('POST')
    expect(init.body).toBe(JSON.stringify({ name: 'test' }))

    const headers = init.headers as Headers
    expect(headers.get('Content-Type')).toBe('application/json')
  })

  it('sends POST by default', async () => {
    const fetchSpy = vi.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    window.sessionStorage.setItem('llm_gateway_admin_token', 'test-token')
    await jsonRequest('/api/action')

    const callArgs = fetchSpy.mock.calls[0]
    expect((callArgs[1] as RequestInit).method).toBe('POST')
  })

  it('allows overriding method', async () => {
    const fetchSpy = vi.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    window.sessionStorage.setItem('llm_gateway_admin_token', 'test-token')
    await jsonRequest('/api/update', { id: 1 }, { method: 'PUT' })

    const callArgs = fetchSpy.mock.calls[0]
    expect((callArgs[1] as RequestInit).method).toBe('PUT')
  })

  it('sends undefined body when body is undefined', async () => {
    const fetchSpy = vi.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    window.sessionStorage.setItem('llm_gateway_admin_token', 'test-token')
    await jsonRequest('/api/action')

    const callArgs = fetchSpy.mock.calls[0]
    expect((callArgs[1] as RequestInit).body).toBeUndefined()
  })
})
