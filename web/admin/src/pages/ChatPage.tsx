import { useState, useEffect, useCallback, useRef } from 'react'
import type { ChatMessage } from '../types/chat'
import { listSessions, getSession, createSession, deleteSession, streamChat } from '../lib/api/chat'
import { ChatSidebar } from '../components/chat/ChatSidebar'
import { ChatMessageView } from '../components/chat/ChatMessage'
import { ChatInput } from '../components/chat/ChatInput'
import type { ChatSession } from '../types/chat'

export function ChatPage() {
  const [sessions, setSessions] = useState<ChatSession[]>([])
  const [activeId, setActiveId] = useState<number | null>(null)
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [streamingContent, setStreamingContent] = useState('')
  const [sending, setSending] = useState(false)
  const bottomRef = useRef<HTMLDivElement>(null)
  const accumulatedRef = useRef('')

  const fetchSessions = useCallback(async () => {
    try {
      const res = await listSessions()
      setSessions(res.data || [])
    } catch {
    }
  }, [])

  useEffect(() => {
    fetchSessions()
  }, [fetchSessions])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, streamingContent])

  const loadSession = useCallback(async (id: number) => {
    setActiveId(id)
    setStreamingContent('')
    try {
      const res = await getSession(id)
      setMessages(res.messages || [])
    } catch {
      setMessages([])
    }
  }, [])

  const handleNew = useCallback(async () => {
    try {
      const session = await createSession({ title: '新对话', model: 'gpt-4o-mini' })
      setSessions((prev) => [session, ...prev])
      loadSession(session.id)
    } catch {
    }
  }, [loadSession])

  const handleDelete = useCallback(async (id: number) => {
    try {
      await deleteSession(id)
      setSessions((prev) => prev.filter((s) => s.id !== id))
      if (activeId === id) {
        setActiveId(null)
        setMessages([])
      }
    } catch {
    }
  }, [activeId])

  const handleSend = useCallback((content: string) => {
    if (!activeId || sending) return

    const userMsg: ChatMessage = {
      id: Date.now(),
      session_id: activeId,
      role: 'user',
      content,
      model: '',
      tokens: 0,
      created_at: new Date().toISOString(),
    }
    setMessages((prev) => [...prev, userMsg])
    accumulatedRef.current = ''
    setStreamingContent('')
    setSending(true)

    streamChat(
      activeId,
      content,
      (chunk) => {
        accumulatedRef.current += chunk
        setStreamingContent(accumulatedRef.current)
      },
      (done) => {
        const assistantMsg: ChatMessage = {
          id: done.message_id,
          session_id: activeId,
          role: 'assistant',
          content: accumulatedRef.current,
          model: done.model,
          tokens: done.tokens,
          created_at: new Date().toISOString(),
        }
        setMessages((prev) => [...prev, assistantMsg])
        setStreamingContent('')
        setSending(false)
        fetchSessions()
      },
      () => {
        setSending(false)
      },
    )
  }, [activeId, sending, streamingContent, fetchSessions])

  const displayMessages = messages
  const hasStream = streamingContent.length > 0

  return (
    <div className="chat-page">
      <ChatSidebar
        sessions={sessions}
        activeId={activeId}
        onSelect={loadSession}
        onNew={handleNew}
        onDelete={handleDelete}
      />
      <div className="chat-main">
        {!activeId ? (
          <div className="chat-empty">选择或创建一个会话开始对话</div>
        ) : (
          <>
            <div className="chat-messages">
              {displayMessages.map((m) => (
                <ChatMessageView key={m.id} message={m} />
              ))}
              {hasStream && (
                <div className="chat-message chat-message--assistant">
                  <div className="chat-message-header">
                    <strong>AI</strong>
                    <span className="chat-message-model">流式输出</span>
                  </div>
                  <div className="chat-message-content">{streamingContent}</div>
                </div>
              )}
              <div ref={bottomRef} />
            </div>
            <ChatInput onSend={handleSend} disabled={sending} />
          </>
        )}
      </div>
    </div>
  )
}
