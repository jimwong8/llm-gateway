import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken, setToken } from '../auth'
import { WsClient } from './websocket'

// ── WebSocket mock ──────────────────────────────────────────────────────────

type WsMockInstance = {
  url: string
  onopen: (() => void) | null
  onmessage: ((event: { data: string }) => void) | null
  onerror: ((event: Event) => void) | null
  onclose: (() => void) | null
  readyState: number
  send: ReturnType<typeof vi.fn>
  close: ReturnType<typeof vi.fn>
}

let currentInstance: WsMockInstance | null = null

function createWsMock(): ReturnType<typeof vi.fn> & { OPEN: number; CLOSED: number; CONNECTING: number } {
  const fn = vi.fn((url: string) => {
    const instance: WsMockInstance = {
      url,
      onopen: null,
      onmessage: null,
      onerror: null,
      onclose: null,
      readyState: 0, // CONNECTING
      send: vi.fn(),
      close: vi.fn(() => {
        instance.readyState = 3 // CLOSED
        instance.onclose?.()
      }),
    }
    currentInstance = instance
    return instance
  }) as ReturnType<typeof vi.fn> & { OPEN: number; CLOSED: number; CONNECTING: number }

  fn.CONNECTING = 0
  fn.OPEN = 1
  fn.CLOSING = 2
  fn.CLOSED = 3
  return fn
}

// ── Tests ────────────────────────────────────────────────────────────────────

describe('WsClient', () => {
  let wsMock: ReturnType<ReturnType<typeof createWsMock>>

  beforeEach(() => {
    currentInstance = null
    wsMock = createWsMock()
    vi.stubGlobal('WebSocket', wsMock)
    vi.useFakeTimers()
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  describe('connect', () => {
    it('creates WebSocket with correct URL', () => {
      const client = new WsClient({ onMessage: vi.fn() })
      client.connect()

      expect(wsMock).toHaveBeenCalledTimes(1)
      const url = wsMock.mock.calls[0][0] as string
      expect(url).toContain('/api/ws/chat')
    })

    it('appends token as query parameter when token exists', () => {
      setToken('test-token-123')
      const client = new WsClient({ onMessage: vi.fn() })
      client.connect()

      const url = wsMock.mock.calls[0][0] as string
      expect(url).toContain('token=test-token-123')
    })

    it('does not append token when token is empty', () => {
      clearToken()
      const client = new WsClient({ onMessage: vi.fn() })
      client.connect()

      const url = wsMock.mock.calls[0][0] as string
      expect(url).not.toContain('token=')
    })

    it('uses custom URL when provided', () => {
      const client = new WsClient({
        onMessage: vi.fn(),
        url: 'wss://custom.example.com/ws',
      })
      client.connect()

      const url = wsMock.mock.calls[0][0] as string
      expect(url).toBe('wss://custom.example.com/ws')
    })

    it('resets reconnect attempts on open', () => {
      const client = new WsClient({ onMessage: vi.fn() })
      client.connect()

      // 模拟断线
      currentInstance!.readyState = 3 // CLOSED
      currentInstance!.onclose?.()

      vi.advanceTimersByTime(3000)
      expect(wsMock).toHaveBeenCalledTimes(2)

      // 模拟重连成功
      currentInstance!.readyState = 1 // OPEN
      currentInstance!.onopen?.()
      expect(client.isConnected).toBe(true)
    })
  })

  describe('send / ping / chat', () => {
    it('sends JSON message when connected', () => {
      setToken('tok')
      const client = new WsClient({ onMessage: vi.fn() })
      client.connect()

      currentInstance!.readyState = 1 // OPEN
      currentInstance!.onopen?.()

      client.send({ type: 'ping' })

      expect(currentInstance!.send).toHaveBeenCalledWith('{"type":"ping"}')
    })

    it('does not send when not connected', () => {
      const client = new WsClient({ onMessage: vi.fn() })
      client.connect()

      // readyState 为 CONNECTING (0)，不等于 OPEN (1)
      client.send({ type: 'ping' })

      expect(currentInstance!.send).not.toHaveBeenCalled()
    })

    it('ping sends ping message', () => {
      setToken('tok')
      const client = new WsClient({ onMessage: vi.fn() })
      client.connect()

      currentInstance!.readyState = 1 // OPEN
      currentInstance!.onopen?.()

      client.ping()

      expect(currentInstance!.send).toHaveBeenCalledWith('{"type":"ping"}')
    })

    it('chat sends chat message with content and session', () => {
      setToken('tok')
      const client = new WsClient({ onMessage: vi.fn() })
      client.connect()

      currentInstance!.readyState = 1 // OPEN
      currentInstance!.onopen?.()

      client.chat('你好世界', 42)

      expect(currentInstance!.send).toHaveBeenCalledWith(
        JSON.stringify({ type: 'chat', content: '你好世界', session: 42 }),
      )
    })

    it('chat sends chat message without session', () => {
      setToken('tok')
      const client = new WsClient({ onMessage: vi.fn() })
      client.connect()

      currentInstance!.readyState = 1 // OPEN
      currentInstance!.onopen?.()

      client.chat('测试')

      expect(currentInstance!.send).toHaveBeenCalledWith(
        JSON.stringify({ type: 'chat', content: '测试', session: undefined }),
      )
    })
  })

  describe('message handling', () => {
    it('calls onMessage when receiving valid JSON', () => {
      const onMessage = vi.fn()
      const client = new WsClient({ onMessage })
      client.connect()

      currentInstance!.onmessage?.({
        data: JSON.stringify({ type: 'done', content: '完成' }),
      })

      expect(onMessage).toHaveBeenCalledWith({ type: 'done', content: '完成' })
    })

    it('ignores malformed JSON messages', () => {
      const onMessage = vi.fn()
      const client = new WsClient({ onMessage })
      client.connect()

      currentInstance!.onmessage?.({ data: 'not json {{{' })

      expect(onMessage).not.toHaveBeenCalled()
    })

    it('calls onError on WebSocket error', () => {
      const onError = vi.fn()
      const client = new WsClient({ onMessage: vi.fn(), onError })
      client.connect()

      const fakeEvent = new Event('error')
      currentInstance!.onerror?.(fakeEvent)

      expect(onError).toHaveBeenCalledWith(fakeEvent)
    })

    it('calls onClose on WebSocket close', () => {
      const onClose = vi.fn()
      const client = new WsClient({ onMessage: vi.fn(), onClose })
      client.connect()

      currentInstance!.onclose?.()

      expect(onClose).toHaveBeenCalled()
    })
  })

  describe('disconnect', () => {
    it('closes WebSocket and prevents reconnect', () => {
      const client = new WsClient({ onMessage: vi.fn() })
      client.connect()

      currentInstance!.readyState = 1 // OPEN
      currentInstance!.onopen?.()

      client.disconnect()

      expect(currentInstance!.close).toHaveBeenCalled()
    })

    it('isConnected returns false after disconnect', () => {
      const client = new WsClient({ onMessage: vi.fn() })
      client.connect()

      currentInstance!.readyState = 1 // OPEN
      currentInstance!.onopen?.()

      expect(client.isConnected).toBe(true)

      client.disconnect()

      expect(client.isConnected).toBe(false)
    })

    it('isConnected returns false when not yet opened', () => {
      const client = new WsClient({ onMessage: vi.fn() })
      client.connect()

      // readyState 为 CONNECTING (0)，不等于 OPEN (1)
      expect(client.isConnected).toBe(false)
    })
  })

  describe('reconnect', () => {
    it('schedules reconnect on unexpected close', () => {
      const client = new WsClient({
        onMessage: vi.fn(),
        reconnectInterval: 1000,
        maxReconnectAttempts: 3,
      })
      client.connect()

      currentInstance!.readyState = 1 // OPEN
      currentInstance!.onopen?.()
      currentInstance!.readyState = 3 // CLOSED
      currentInstance!.onclose?.()

      expect(wsMock).toHaveBeenCalledTimes(1)

      vi.advanceTimersByTime(1000)

      expect(wsMock).toHaveBeenCalledTimes(2)
    })

    it('does not reconnect after max attempts reached', () => {
      const client = new WsClient({
        onMessage: vi.fn(),
        reconnectInterval: 100,
        maxReconnectAttempts: 2,
      })
      client.connect()

      // 第一次断线
      currentInstance!.readyState = 1 // OPEN
      currentInstance!.onopen?.()
      currentInstance!.readyState = 3 // CLOSED
      currentInstance!.onclose?.()

      vi.advanceTimersByTime(100)
      expect(wsMock).toHaveBeenCalledTimes(2)

      // 第二次断线
      currentInstance!.readyState = 3 // CLOSED
      currentInstance!.onclose?.()

      vi.advanceTimersByTime(200)
      expect(wsMock).toHaveBeenCalledTimes(3)

      // 第三次断线 — 已达 maxReconnectAttempts=2，不应再重连
      currentInstance!.readyState = 3 // CLOSED
      currentInstance!.onclose?.()

      vi.advanceTimersByTime(300)
      expect(wsMock).toHaveBeenCalledTimes(3)
    })

    it('does not reconnect on intentional close (disconnect)', () => {
      const client = new WsClient({
        onMessage: vi.fn(),
        reconnectInterval: 100,
      })
      client.connect()

      currentInstance!.readyState = 1 // OPEN
      currentInstance!.onopen?.()

      client.disconnect()

      vi.advanceTimersByTime(5000)

      expect(wsMock).toHaveBeenCalledTimes(1)
    })

    it('uses increasing backoff interval', () => {
      const client = new WsClient({
        onMessage: vi.fn(),
        reconnectInterval: 1000,
        maxReconnectAttempts: 3,
      })
      client.connect()

      // 第一次连接 + 断线
      currentInstance!.readyState = 1 // OPEN
      currentInstance!.onopen?.()
      currentInstance!.readyState = 3 // CLOSED
      currentInstance!.onclose?.()

      // 首次重连间隔 = 1000 * (0 + 1) = 1000ms
      vi.advanceTimersByTime(999)
      expect(wsMock).toHaveBeenCalledTimes(1)
      vi.advanceTimersByTime(1)
      expect(wsMock).toHaveBeenCalledTimes(2)

      // 第二次断线
      currentInstance!.readyState = 3 // CLOSED
      currentInstance!.onclose?.()

      // 第二次重连间隔 = 1000 * (1 + 1) = 2000ms
      vi.advanceTimersByTime(1999)
      expect(wsMock).toHaveBeenCalledTimes(2)
      vi.advanceTimersByTime(1)
      expect(wsMock).toHaveBeenCalledTimes(3)
    })
  })
})
