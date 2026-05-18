import type { ChatSession } from '../../types/chat'

type ChatSidebarProps = {
  sessions: ChatSession[]
  activeId: number | null
  onSelect: (id: number) => void
  onNew: () => void
  onDelete: (id: number) => void
}

export function ChatSidebar({ sessions, activeId, onSelect, onNew, onDelete }: ChatSidebarProps) {
  return (
    <div className="chat-sidebar">
      <div className="chat-sidebar-header">
        <strong>历史会话</strong>
        <button type="button" onClick={onNew}>新建对话</button>
      </div>
      <div className="chat-sidebar-list">
        {sessions.length === 0 && (
          <div className="chat-sidebar-empty">暂无会话</div>
        )}
        {sessions.map((s) => (
          <div
            key={s.id}
            className={`chat-sidebar-item${s.id === activeId ? ' active' : ''}`}
            onClick={() => onSelect(s.id)}
          >
            <div className="chat-sidebar-item-title">{s.title}</div>
            <div className="chat-sidebar-item-meta">{s.model}</div>
            <button
              type="button"
              className="chat-sidebar-item-delete"
              onClick={(e) => { e.stopPropagation(); onDelete(s.id) }}
            >
              删除
            </button>
          </div>
        ))}
      </div>
    </div>
  )
}
