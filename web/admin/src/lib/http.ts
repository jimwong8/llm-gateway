import { clearToken, getToken } from './auth'

export class ApiError extends Error {
  status: number
  payload: unknown

  constructor(status: number, message: string, payload: unknown) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.payload = payload
  }
}

type RequestOptions = {
  auth?: 'admin' | 'none'
  timeout?: number
  signal?: AbortSignal
}

async function parseResponseBody(response: Response): Promise<unknown> {
  const contentType = response.headers.get('Content-Type') ?? ''
  if (contentType.includes('application/json')) {
    return response.json()
  }
  const text = await response.text()
  return text ? { message: text } : null
}

async function toApiError(response: Response): Promise<ApiError> {
  const payload = await parseResponseBody(response)

  let message = response.statusText || 'Request failed'
  if (payload && typeof payload === 'object') {
    const errorPayload = (payload as { error?: { message?: string } }).error
    if (errorPayload?.message) {
      message = errorPayload.message
    } else {
      const topLevelMessage = (payload as { message?: string }).message
      if (topLevelMessage) {
        message = topLevelMessage
      }
    }
  }

  return new ApiError(response.status, message, payload)
}

export async function apiRequest<T>(
  input: RequestInfo | URL,
  init: RequestInit = {},
  options: RequestOptions = { auth: 'admin', timeout: 30000 },
): Promise<T> {
  const headers = new Headers(init.headers)
  const controller = new AbortController()
  const timeoutMs = options.timeout ?? 30000

  if (options.auth !== 'none') {
    const token = getToken()
    if (token) {
      headers.set('Authorization', `Bearer ${token}`)
    }
  }

  if (options.signal) {
    if (options.signal.aborted) {
      controller.abort()
    } else {
      options.signal.addEventListener('abort', () => controller.abort(), { once: true })
    }
  }

  const timeoutId = setTimeout(() => controller.abort(), timeoutMs)

  try {
    const response = await fetch(input, {
      ...init,
      headers,
      signal: controller.signal,
    })

    if (response.status === 401 || response.status === 403) {
      clearToken()
      window.dispatchEvent(new CustomEvent('auth:expired'))
      window.location.href = '/admin/ui/login'
    }

    if (!response.ok) {
      throw await toApiError(response)
    }

    return (await parseResponseBody(response)) as T
  } catch (err) {
    if (err instanceof ApiError) throw err
    if (err instanceof DOMException && err.name === 'AbortError') {
      throw new ApiError(408, 'Request timeout', { type: 'timeout_error' })
    }
    if (err instanceof TypeError && err.message === 'Failed to fetch') {
      throw new ApiError(0, 'Network error - please check your connection', { type: 'network_error' })
    }
    throw err
  } finally {
    clearTimeout(timeoutId)
  }
}

export function jsonRequest<T>(
  input: RequestInfo | URL,
  body?: unknown,
  init: RequestInit = {},
  options: RequestOptions = { auth: 'admin', timeout: 30000 },
) {
  const headers = new Headers(init.headers)
  headers.set('Content-Type', 'application/json')

  return apiRequest<T>(
    input,
    {
      ...init,
      method: init.method ?? 'POST',
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    },
    options,
  )
}
