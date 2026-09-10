import { useEffect } from 'react'
import { useUIStore } from '../stores/ui-store'

/**
 * 应用全局主题类到 <html>。
 *
 * 之前有 ThemeProvider (React Context) 和 ui-store (Zustand) 两套状态，
 * 切换其中一个不影响另一个。这里把 ui-store 作为唯一真源，
 * useEffect 只负责把 theme 同步到 documentElement.classList。
 */
export function useApplyTheme() {
  const theme = useUIStore((s) => s.theme)

  useEffect(() => {
    const root = document.documentElement
    root.classList.remove('light', 'dark')
    root.classList.add(theme)
    root.style.colorScheme = theme
  }, [theme])
}
