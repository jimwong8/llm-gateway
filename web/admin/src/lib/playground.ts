import type { PlaygroundRequest, PlaygroundResponse } from '../types/playground'

export type PlaygroundResult = {
  body: PlaygroundResponse | Record<string, unknown>
  status: number
  headers: Record<string, string>
  elapsedMs: number
}

export async function sendPlaygroundRequest(payload: PlaygroundRequest): Promise<PlaygroundResult> {
  const startedAt = performance.now()
  
  // Use bare fetch to get access to response headers
  const token = ''
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }
  
  const response = await fetch('/v1/chat/completions', {
    method: 'POST',
    headers,
    body: JSON.stringify(payload),
  })
  const elapsedMs = Math.round(performance.now() - startedAt)

  const respHeaders: Record<string, string> = {}
  response.headers.forEach((value, key) => {
    respHeaders[key] = value
  })

  const contentType = response.headers.get('Content-Type') ?? ''
  const body = contentType.includes('application/json')
    ? ((await response.json()) as PlaygroundResponse | Record<string, unknown>)
    : { message: await response.text() }

  if (!response.ok) {
    const errMsg = (body as Record<string, unknown>)?.message ?? response.statusText
    throw new Error(`Playground request failed: ${errMsg}`)
  }

  return { body, status: response.status, headers: respHeaders, elapsedMs }
}
