import { getUserToken } from './identity'
import type {
  ChatSession,
  ChatMessage,
  CreateSessionRequest,
  UpdateSessionRequest,
  SessionResponse,
  SSEEvent,
} from '../../types/chat'

function userAuthHeaders(): HeadersInit {
  const token = getUserToken()
  if (token) {
    return { Authorization: `Bearer ${token}` }
  }
  return {}
}

async function userFetch(input: string, init?: RequestInit): Promise<Response> {
  const res = await fetch(input, init)
  if (res.status === 401) {
    sessionStorage.removeItem('llm_gateway_user_token')
    window.location.href = '/login'
    throw new Error('Unauthorized')
  }
  return res
}

export async function createSession(data: CreateSessionRequest): Promise<ChatSession> {
  const res = await userFetch('/api/chat/sessions', {
    method: 'POST',
    headers: { ...userAuthHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error('Failed to create session')
  return res.json()
}

export async function listSessions(limit = 50, offset = 0): Promise<{ object: string; data: ChatSession[] }> {
  const res = await userFetch(`/api/chat/sessions?limit=${limit}&offset=${offset}`, {
    headers: { ...userAuthHeaders() },
  })
  if (!res.ok) throw new Error('Failed to list sessions')
  return res.json()
}

export async function getSession(id: number): Promise<SessionResponse> {
  const res = await userFetch(`/api/chat/sessions/${id}`, {
    headers: { ...userAuthHeaders() },
  })
  if (!res.ok) throw new Error('Failed to get session')
  return res.json()
}

export async function updateSessionTitle(id: number, data: UpdateSessionRequest): Promise<ChatSession> {
  const res = await userFetch(`/api/chat/sessions/${id}`, {
    method: 'PUT',
    headers: { ...userAuthHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error('Failed to update session')
  return res.json()
}

export async function deleteSession(id: number): Promise<void> {
  const res = await userFetch(`/api/chat/sessions/${id}`, {
    method: 'DELETE',
    headers: { ...userAuthHeaders() },
  })
  if (!res.ok) throw new Error('Failed to delete session')
}

export async function getSharedSession(hash: string): Promise<SessionResponse> {
  const res = await userFetch(`/api/chat/sessions/share/${hash}`)
  if (!res.ok) throw new Error('Failed to get shared session')
  return res.json()
}

export function streamChat(
  sessionId: number,
  content: string,
  onChunk: (text: string) => void,
  onDone: (event: { message_id: number; model: string; prompt_tokens: number; tokens: number }) => void,
  onError: (message: string) => void,
  signal?: AbortSignal,
): () => void {
  const controller = new AbortController()
  const cancel = () => controller.abort()
  const fetchSignal = signal ?? controller.signal

  const token = getUserToken()

  ;(async () => {
    try {
      const res = await fetch(`/api/chat/sessions/${sessionId}/messages:stream`, {
        method: 'POST',
        headers: {
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ content }),
        signal: fetchSignal,
      })

      if (!res.ok) {
        const body = await res.text()
        onError(body || `HTTP ${res.status}`)
        return
      }

      const reader = res.body?.getReader()
      if (!reader) {
        onError('No response body')
        return
      }

      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        for (const line of lines) {
          const trimmed = line.trim()
          if (!trimmed || !trimmed.startsWith('data: ')) continue
          const jsonStr = trimmed.slice(6)
          try {
            const event = JSON.parse(jsonStr) as SSEEvent
            if (event.type === 'chunk') {
              onChunk(event.content)
            } else if (event.type === 'done') {
              onDone({
                message_id: event.message_id,
                model: event.model,
                prompt_tokens: event.prompt_tokens,
                tokens: event.tokens,
              })
            } else if (event.type === 'error') {
              onError(event.message)
            }
          } catch {
            // skip unparseable lines
          }
        }
      }
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') return
      onError(err instanceof Error ? err.message : 'Stream error')
    }
  })()

  return cancel
}
