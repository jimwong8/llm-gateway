import type { ChatMessage } from '../../types/chat'

type ChatMessageProps = {
  message: ChatMessage
}

export function ChatMessageView({ message }: ChatMessageProps) {
  const isUser = message.role === 'user'
  return (
    <div className={`chat-message ${isUser ? 'chat-message--user' : 'chat-message--assistant'}`}>
      <div className="chat-message-header">
        <strong>{isUser ? '你' : 'AI'}</strong>
        <span className="chat-message-model">{message.model}</span>
        {message.tokens > 0 && <span className="chat-message-tokens">{message.tokens} tokens</span>}
      </div>
      <div className="chat-message-content">{message.content}</div>
    </div>
  )
}
