export type ChatSession = {
  id: number
  user_id: number
  title: string
  model: string
  created_at: string
  updated_at: string
}

export type ChatMessage = {
  id: number
  session_id: number
  role: 'user' | 'assistant' | 'system'
  content: string
  model: string
  tokens: number
  created_at: string
}

export type CreateSessionRequest = {
  title: string
  model: string
}

export type UpdateSessionRequest = {
  title: string
}

export type SendMessageRequest = {
  content: string
}

export type SessionResponse = {
  session: ChatSession
  messages: ChatMessage[]
}

export type SSEChunk = {
  content: string
  type: 'chunk'
}

export type SSEDone = {
  type: 'done'
  message_id: number
  model: string
  prompt_tokens: number
  tokens: number
}

export type SSEError = {
  type: 'error'
  message: string
}

export type SSEEvent = SSEChunk | SSEDone | SSEError
