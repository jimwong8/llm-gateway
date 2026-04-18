export type PlaygroundMessage = {
  role: string
  content: string
}

export type PlaygroundRequest = {
  model: string
  tenant_id: string
  task_hint?: string
  messages: PlaygroundMessage[]
}

export type PlaygroundResponse = {
  id?: string
  object?: string
  created?: number
  model?: string
  choices?: Array<{
    index: number
    finish_reason?: string
    message?: {
      role: string
      content: string
    }
  }>
  usage?: {
    prompt_tokens?: number
    completion_tokens?: number
    total_tokens?: number
  }
}
