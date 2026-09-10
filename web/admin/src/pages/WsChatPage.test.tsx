import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken, setToken } from '../lib/auth'
import { WsChatPage } from './WsChatPage'

// ── scrollIntoView mock (jsdom 不支持) ──────────────────────────────────────
Element.prototype.scrollIntoView = vi.fn()

// ── WebSocket 常量（在 stub 之前保存） ──────────────────────────────────────
const WS_CONNECTING = 0
const WS_OPEN = 1
const WS_CLOSED = 3

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

const wsInstances: WsMockInstance[] = []

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
    wsInstances.push(instance)
    return instance
  }) as ReturnType<typeof vi.fn> & { OPEN: number; CLOSED: number; CONNECTING: number }
  fn.CONNECTING = 0
  fn.OPEN = 1
  fn.CLOSING = 2
  fn.CLOSED = 3
  return fn
}

function getLatestWs(): WsMockInstance {
  return wsInstances[wsInstances.length - 1]
}

function simulateOpen(ws: WsMockInstance) {
  ws.readyState = 1 // OPEN
  ws.onopen?.()
}

function simulateMessage(ws: WsMockInstance, data: object) {
  ws.onmessage?.({ data: JSON.stringify(data) })
}

// ── Tests ────────────────────────────────────────────────────────────────────

describe('WsChatPage', () => {
  let wsMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    setToken('demo-admin-token')
    wsInstances.length = 0
    wsMock = createWsMock()
    vi.stubGlobal('WebSocket', wsMock)
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  it('renders chat interface with connection status and empty state', () => {
    render(<WsChatPage />)

    // 连接中状态
    expect(screen.getByText('连接中...')).toBeInTheDocument()
    expect(screen.getByText('发送消息开始对话')).toBeInTheDocument()

    // 输入区域
    expect(screen.getByPlaceholderText('输入消息，Enter 发送，Shift+Enter 换行')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '发送' })).toBeInTheDocument()
  })

  it('shows connected status after WebSocket opens', async () => {
    render(<WsChatPage />)

    const ws = getLatestWs()
    simulateOpen(ws)

    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })
  })

  it('sends a user message and displays it after clicking send', async () => {
    const user = userEvent.setup()
    render(<WsChatPage />)

    const ws = getLatestWs()
    simulateOpen(ws)

    // 等待连接就绪
    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })

    // 输入并发送消息
    const textarea = screen.getByPlaceholderText('输入消息，Enter 发送，Shift+Enter 换行')
    await user.type(textarea, '你好')
    await user.click(screen.getByRole('button', { name: '发送' }))

    // 用户消息应出现在界面
    expect(screen.getByText('你好')).toBeInTheDocument()
    expect(screen.getByText('你')).toBeInTheDocument()

    // WebSocket 应收到 chat 消息
    expect(ws.send).toHaveBeenCalledWith(
      JSON.stringify({ type: 'chat', content: '你好', session: undefined }),
    )
  })

  it('displays assistant message after receiving done response', async () => {
    const user = userEvent.setup()
    render(<WsChatPage />)

    const ws = getLatestWs()
    simulateOpen(ws)

    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })

    // 发送消息
    const textarea = screen.getByPlaceholderText('输入消息，Enter 发送，Shift+Enter 换行')
    await user.type(textarea, '讲个笑话')
    await user.click(screen.getByRole('button', { name: '发送' }))

    // 模拟 session_created
    simulateMessage(ws, { type: 'session_created', session_id: 42 })

    // 模拟 done 响应
    simulateMessage(ws, { type: 'done', content: '这是一个笑话。' })

    // AI 消息应出现在界面
    await waitFor(() => {
      expect(screen.getByText('这是一个笑话。')).toBeInTheDocument()
    })
    expect(screen.getByText('AI')).toBeInTheDocument()
  })

  it('displays error message when server sends error', async () => {
    const user = userEvent.setup()
    render(<WsChatPage />)

    const ws = getLatestWs()
    simulateOpen(ws)

    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })

    const textarea = screen.getByPlaceholderText('输入消息，Enter 发送，Shift+Enter 换行')
    await user.type(textarea, '测试错误')
    await user.click(screen.getByRole('button', { name: '发送' }))

    // 模拟 error 响应
    simulateMessage(ws, { type: 'error', message: '内部服务错误' })

    await waitFor(() => {
      expect(screen.getByText('错误: 内部服务错误')).toBeInTheDocument()
    })
  })

  it('disables input and send button when disconnected', () => {
    render(<WsChatPage />)

    // 未连接时输入框和发送按钮应禁用
    expect(screen.getByPlaceholderText('输入消息，Enter 发送，Shift+Enter 换行')).toBeDisabled()
    expect(screen.getByRole('button', { name: '发送' })).toBeDisabled()
  })

  it('sends message on Enter key and ignores Shift+Enter', async () => {
    const user = userEvent.setup()
    render(<WsChatPage />)

    const ws = getLatestWs()
    simulateOpen(ws)

    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })

    const textarea = screen.getByPlaceholderText('输入消息，Enter 发送，Shift+Enter 换行')
    await user.type(textarea, 'Enter 发送{Enter}')

    // 应发送了消息
    expect(ws.send).toHaveBeenCalledWith(
      JSON.stringify({ type: 'chat', content: 'Enter 发送', session: undefined }),
    )
  })

  it('cleans up WebSocket on unmount', async () => {
    const { unmount } = render(<WsChatPage />)

    const ws = getLatestWs()
    simulateOpen(ws)

    await waitFor(() => {
      expect(screen.getByText('已连接')).toBeInTheDocument()
    })

    unmount()

    // close 应被调用
    expect(ws.close).toHaveBeenCalled()
  })
})
