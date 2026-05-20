import { useUIStore } from '../../stores/ui-store'

export function ThemeToggle() {
  const { theme, setTheme } = useUIStore()
  return (
    <button
      onClick={() => setTheme(theme === 'light' ? 'dark' : 'light')}
      className="btn btn--sm btn--outline"
      aria-label="切换主题"
    >
      {theme === 'light' ? '🌙' : '☀️'}
    </button>
  )
}
