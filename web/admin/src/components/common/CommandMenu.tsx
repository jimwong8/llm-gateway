import { useEffect, useState, useCallback, useRef, type KeyboardEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { navItems } from '../layout/Sidebar'

interface CommandItem {
  id: string
  label: string
  path?: string
  icon?: string
  shortcut?: string
  action?: () => void
}

export function CommandMenu() {
  const navigate = useNavigate()
  const [isOpen, setIsOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)

  const commands: CommandItem[] = navItems
    .filter(item => item.path)
    .map(item => ({
      id: item.path!,
      label: item.label,
      path: item.path!,
      icon: item.icon,
    }))

  const filteredCommands = query
    ? commands.filter(cmd => cmd.label.toLowerCase().includes(query.toLowerCase()))
    : commands

  const executeCommand = useCallback((cmd: CommandItem) => {
    setIsOpen(false)
    setQuery('')
    if (cmd.action) cmd.action()
    else if (cmd.path) navigate(cmd.path)
  }, [navigate])

  useEffect(() => {
    const handleKeyDown = (e: globalThis.KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        setIsOpen(prev => !prev)
        setQuery('')
        setSelectedIndex(0)
      }
      if (e.key === 'Escape' && isOpen) setIsOpen(false)
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isOpen])

  useEffect(() => {
    if (isOpen && inputRef.current) inputRef.current.focus()
  }, [isOpen])

  useEffect(() => {
    setSelectedIndex(0)
  }, [query])

  if (!isOpen) return null

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault()
        setSelectedIndex(prev => Math.min(prev + 1, filteredCommands.length - 1))
        break
      case 'ArrowUp':
        e.preventDefault()
        setSelectedIndex(prev => Math.max(prev - 1, 0))
        break
      case 'Enter':
        e.preventDefault()
        if (filteredCommands[selectedIndex]) executeCommand(filteredCommands[selectedIndex])
        break
    }
  }

  return (
    <div className="cmdk-overlay" role="dialog" aria-label="命令面板" onClick={() => setIsOpen(false)}>
      <div className="cmdk-dialog" onClick={(e) => e.stopPropagation()}>
        <input
          ref={inputRef}
          type="text"
          className="cmdk-input"
          placeholder="输入命令或搜索..."
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={handleKeyDown}
          aria-label="搜索命令"
          autoComplete="off"
        />
        <div className="cmdk-list" role="listbox">
          {filteredCommands.length === 0 ? (
            <div className="cmdk-empty">
              <p>未找到匹配的命令</p>
            </div>
          ) : (
            filteredCommands.map((cmd, index) => (
              <div
                key={cmd.id}
                className="cmdk-item"
                role="option"
                aria-selected={index === selectedIndex}
                onClick={() => executeCommand(cmd)}
                onMouseEnter={() => setSelectedIndex(index)}
              >
                {cmd.icon && (
                  <span className="cmdk-item__icon" aria-hidden="true">{cmd.icon}</span>
                )}
                <div className="cmdk-item__content">
                  <div className="cmdk-item__title">{cmd.label}</div>
                </div>
                {cmd.shortcut && (
                  <kbd className="cmdk-item__shortcut">{cmd.shortcut}</kbd>
                )}
              </div>
            ))
          )}
        </div>
        <div className="cmdk-footer">
          <span><kbd>↑↓</kbd> 导航</span>
          <span><kbd>↵</kbd> 执行</span>
          <span><kbd>Esc</kbd> 关闭</span>
        </div>
      </div>
    </div>
  )
}
