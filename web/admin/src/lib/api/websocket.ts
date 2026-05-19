import { getToken } from '../auth'

export type WsMessage =
  | { type: 'ping' }
  | { type: 'pong' }
  | { type: 'chat'; content: string; session?: number }
  | { type: 'session_created'; session_id: number }
  | { type: 'done'; content: string }
  | { type: 'error'; message: string }

type MessageHandler = (message: WsMessage) => void
type ErrorHandler = (error: Event) => void
type CloseHandler = (event: CloseEvent) => void

export type WsClientOptions = {
  url?: string
  onMessage: MessageHandler
  onError?: ErrorHandler
  onClose?: CloseHandler
  reconnectInterval?: number
  maxReconnectAttempts?: number
}

export class WsClient {
  private ws: WebSocket | null = null
  private readonly options: WsClientOptions
  private reconnectAttempts = 0
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private intentionalClose = false

  constructor(options: WsClientOptions) {
    this.options = options
  }

  connect(): void {
    this.intentionalClose = false
    const token = getToken()
    const baseUrl = this.options.url ?? this.buildWsUrl()
    const url = token ? `${baseUrl}?token=${encodeURIComponent(token)}` : baseUrl

    this.ws = new WebSocket(url)
    this.ws.onmessage = this.handleMessage
    this.ws.onerror = this.handleError
    this.ws.onclose = this.handleClose
    this.ws.onopen = () => {
      this.reconnectAttempts = 0
    }
  }

  send(message: WsMessage): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message))
    }
  }

  ping(): void {
    this.send({ type: 'ping' })
  }

  chat(content: string, session?: number): void {
    this.send({ type: 'chat', content, session })
  }

  disconnect(): void {
    this.intentionalClose = true
    this.clearReconnectTimer()
    if (this.ws) {
      this.ws.onclose = null
      this.ws.close()
      this.ws = null
    }
  }

  get isConnected(): boolean {
    return this.ws !== null && this.ws.readyState === WebSocket.OPEN
  }

  private buildWsUrl(): string {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    return `${protocol}//${host}/api/ws/chat`
  }

  private handleMessage = (event: MessageEvent): void => {
    try {
      const data = JSON.parse(event.data) as WsMessage
      this.options.onMessage(data)
    } catch {
      // ignore malformed messages
    }
  }

  private handleError = (error: Event): void => {
    this.options.onError?.(error)
  }

  private handleClose = (event: CloseEvent): void => {
    this.options.onClose?.(event)
    if (!this.intentionalClose) {
      this.scheduleReconnect()
    }
  }

  private scheduleReconnect(): void {
    const maxAttempts = this.options.maxReconnectAttempts ?? 5
    if (this.reconnectAttempts >= maxAttempts) return

    const interval = this.options.reconnectInterval ?? 3000
    this.reconnectTimer = setTimeout(() => {
      this.reconnectAttempts++
      this.connect()
    }, interval * (this.reconnectAttempts + 1))
  }

  private clearReconnectTimer(): void {
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
  }
}
