import { useState, useEffect, useRef, useCallback } from 'react'
import { AppShell } from '../components/layout/AppShell'
import { WsClient, type WsMessage } from '../lib/api/websocket'

type ChatMessage = {
  id: number
  role: 'user' | 'assistant'
  content: string
}

export function WsChatPage() {
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [input, setInput] = useState('')
  const [connected, setConnected] = useState(false)
  const [sessionId, setSessionId] = useState<number | undefined>(undefined)
  const [streaming, setStreaming] = useState(false)
  const bottomRef = useRef<HTMLDivElement>(null)
  const clientRef = useRef<WsClient | null>(null)
  const streamRef = useRef('')
  const msgIdRef = useRef(0)

  useEffect(() => {
    const client = new WsClient({
      onMessage: (msg: WsMessage) => {
        if (msg.type === 'pong') return

        if (msg.type === 'session_created') {
          setSessionId(msg.session_id)
          return
        }

        if (msg.type === 'done') {
          setMessages((prev) => [
            ...prev,
            { id: msgIdRef.current++, role: 'assistant', content: msg.content },
          ])
          streamRef.current = ''
          setStreaming(false)
          return
        }

        if (msg.type === 'error') {
          setMessages((prev) => [
            ...prev,
            { id: msgIdRef.current++, role: 'assistant', content: `错误: ${msg.message}` },
          ])
          streamRef.current = ''
          setStreaming(false)
        }
      },
      onClose: () => setConnected(false),
      onError: () => setConnected(false),
    })

    client.connect()
    clientRef.current = client

    const checkConnected = setInterval(() => {
      if (client.isConnected) {
        setConnected(true)
        clearInterval(checkConnected)
      }
    }, 200)

    return () => {
      clearInterval(checkConnected)
      client.disconnect()
      clientRef.current = null
    }
  }, [])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, streaming])

  const handleSend = useCallback(() => {
    const content = input.trim()
    if (!content || streaming) return

    setMessages((prev) => [
      ...prev,
      { id: msgIdRef.current++, role: 'user', content },
    ])
    setInput('')
    setStreaming(true)
    streamRef.current = ''

    clientRef.current?.chat(content, sessionId)
  }, [input, streaming, sessionId])

  const handleKeyDown = useCallback((e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }, [handleSend])

  return (
    <AppShell title="WebSocket 聊天" description="通过 WebSocket 与 AI 实时对话">
      <div className="ws-chat-page">
        <div className="ws-chat-status">
          <span className={`status-dot ${connected ? 'status-dot--online' : 'status-dot--offline'}`} />
          {connected ? '已连接' : '连接中...'}
        </div>

        <div className="ws-chat-messages">
          {messages.length === 0 && (
            <div className="ws-chat-empty">发送消息开始对话</div>
          )}
          {messages.map((m) => (
            <div
              key={m.id}
              className={`ws-chat-message ws-chat-message--${m.role}`}
            >
              <div className="ws-chat-message__role">
                {m.role === 'user' ? '你' : 'AI'}
              </div>
              <div className="ws-chat-message__content">{m.content}</div>
            </div>
          ))}
          {streaming && (
            <div className="ws-chat-message ws-chat-message--assistant">
              <div className="ws-chat-message__role">AI</div>
              <div className="ws-chat-message__content">
                {streamRef.current || '思考中...'}
              </div>
            </div>
          )}
          <div ref={bottomRef} />
        </div>

        <div className="ws-chat-input">
          <textarea
            className="ws-chat-input__field"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="输入消息，Enter 发送，Shift+Enter 换行"
            rows={2}
            disabled={!connected || streaming}
          />
          <button
            type="button"
            className="ws-chat-input__send"
            onClick={handleSend}
            disabled={!connected || streaming || !input.trim()}
          >
            发送
          </button>
        </div>
      </div>
    </AppShell>
  )
}
