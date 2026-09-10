import { useUIStore } from '../../stores/ui-store'

export function ThemeToggle() {
  const theme = useUIStore((s) => s.theme)
  const toggleTheme = useUIStore((s) => s.toggleTheme)
  return (
    <button
      onClick={toggleTheme}
      className="btn btn--sm btn--outline"
      aria-label={theme === 'light' ? '切换到暗色模式' : '切换到亮色模式'}
    >
      {theme === 'light' ? '🌙' : '☀️'}
    </button>
  )
}
