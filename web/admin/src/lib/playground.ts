import type { PlaygroundRequest, PlaygroundResponse } from '../types/playground'

export type PlaygroundResult = {
  body: PlaygroundResponse | Record<string, unknown>
  status: number
  headers: Record<string, string>
  elapsedMs: number
}

export async function sendPlaygroundRequest(payload: PlaygroundRequest): Promise<PlaygroundResult> {
  const startedAt = performance.now()
  const response = await fetch('/v1/chat/completions', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  })
  const elapsedMs = Math.round(performance.now() - startedAt)

  const headers: Record<string, string> = {}
  response.headers.forEach((value, key) => {
    headers[key] = value
  })

  const contentType = response.headers.get('Content-Type') ?? ''
  const body = contentType.includes('application/json')
    ? ((await response.json()) as PlaygroundResponse | Record<string, unknown>)
    : { message: await response.text() }

  return {
    body,
    status: response.status,
    headers,
    elapsedMs,
  }
}
