import { useState, useEffect, useCallback, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import type { ChatMessage } from '../types/chat'
import { listSessions, getSession, createSession, deleteSession, streamChat } from '../lib/api/chat'
import { ChatSidebar } from '../components/chat/ChatSidebar'
import { ChatMessageView } from '../components/chat/ChatMessage'
import { ChatInput } from '../components/chat/ChatInput'
import type { ChatSession } from '../types/chat'

export function ChatPage() {
  const { t } = useTranslation()
  const [sessions, setSessions] = useState<ChatSession[]>([])
  const [activeId, setActiveId] = useState<number | null>(null)
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [streamingContent, setStreamingContent] = useState('')
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')
  const bottomRef = useRef<HTMLDivElement>(null)
  const accumulatedRef = useRef('')

  const fetchSessions = useCallback(async () => {
    try {
      const res = await listSessions()
      setSessions(res.data || [])
      setError('')
    } catch (e) {
      setError(t('chat.loadError'))
    }
  }, [t])

  useEffect(() => {
    fetchSessions()
  }, [fetchSessions])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, streamingContent])

  const loadSession = useCallback(async (id: number) => {
    setActiveId(id)
    setStreamingContent('')
    setError('')
    try {
      const res = await getSession(id)
      setMessages(res.messages || [])
    } catch {
      setMessages([])
      setError(t('chat.loadSessionError'))
    }
  }, [t])

  const handleNew = useCallback(async () => {
    try {
      const session = await createSession({ title: t('chat.newChat'), model: 'gpt-4o-mini' })
      setSessions((prev) => [session, ...prev])
      loadSession(session.id)
    } catch {
      setError(t('chat.createError'))
    }
  }, [loadSession, t])

  const handleDelete = useCallback(async (id: number) => {
    if (!window.confirm(t('chat.confirmDelete'))) return
    try {
      await deleteSession(id)
      setSessions((prev) => prev.filter((s) => s.id !== id))
      if (activeId === id) {
        setActiveId(null)
        setMessages([])
      }
    } catch {
      setError(t('chat.deleteError'))
    }
  }, [activeId, t])

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
        {error && (
          <div className="config-error" role="alert" style={{ margin: '8px 16px' }}>
            {error}
            <button type="button" onClick={fetchSessions} className="btn btn--sm" style={{ marginLeft: '8px' }}>
              {t('common.retry')}
            </button>
          </div>
        )}
        {!activeId ? (
          <div className="chat-empty">{t('chat.selectOrCreate')}</div>
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
                    <span className="chat-message-model">{t('chat.streaming')}</span>
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
