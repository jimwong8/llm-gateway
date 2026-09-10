import { useState, type FormEvent } from 'react'

type ChatInputProps = {
  onSend: (content: string) => void
  disabled?: boolean
}

export function ChatInput({ onSend, disabled }: ChatInputProps) {
  const [value, setValue] = useState('')

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault()
    const trimmed = value.trim()
    if (!trimmed || disabled) return
    onSend(trimmed)
    setValue('')
  }

  return (
    <form className="chat-input" onSubmit={handleSubmit}>
      <input
        type="text"
        className="chat-input-field"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        placeholder="输入消息..."
        disabled={disabled}
      />
      <button type="submit" className="chat-input-send" disabled={disabled || !value.trim()}>
        发送
      </button>
    </form>
  )
}
