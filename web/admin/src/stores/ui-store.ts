import { create } from 'zustand'

export type Theme = 'light' | 'dark'
export type Language = 'zh-CN' | 'en-US'

interface UIState {
  sidebarCollapsed: boolean
  theme: Theme
  language: Language
  setSidebarCollapsed: (collapsed: boolean) => void
  toggleSidebar: () => void
  setTheme: (theme: Theme) => void
  toggleTheme: () => void
  setLanguage: (language: Language) => void
}

function loadInitialState(): Pick<UIState, 'sidebarCollapsed' | 'theme' | 'language'> {
  const result: Pick<UIState, 'sidebarCollapsed' | 'theme' | 'language'> = {
    sidebarCollapsed: false,
    theme: 'dark',
    language: 'zh-CN',
  }
  try {
    const raw = window.localStorage.getItem('llm-gateway-ui')
    if (!raw) return result
    const parsed = JSON.parse(raw)
    if (typeof parsed.sidebarCollapsed === 'boolean') result.sidebarCollapsed = parsed.sidebarCollapsed
    if (parsed.theme === 'light' || parsed.theme === 'dark') result.theme = parsed.theme
    if (parsed.language === 'zh-CN' || parsed.language === 'en-US') result.language = parsed.language
  } catch {
    /* localStorage 不可用时静默降级到默认值 */
  }
  return result
}

function persistState(s: UIState) {
  try {
    window.localStorage.setItem(
      'llm-gateway-ui',
      JSON.stringify({
        sidebarCollapsed: s.sidebarCollapsed,
        theme: s.theme,
        language: s.language,
      }),
    )
  } catch {
    /* noop */
  }
}

export const useUIStore = create<UIState>((set) => ({
  ...loadInitialState(),
  setSidebarCollapsed: (collapsed) => set((s) => { persistState({ ...s, sidebarCollapsed: collapsed }); return { sidebarCollapsed: collapsed } }),
  toggleSidebar: () => set((s) => { const next = !s.sidebarCollapsed; persistState({ ...s, sidebarCollapsed: next }); return { sidebarCollapsed: next } }),
  setTheme: (theme) => set((s) => { persistState({ ...s, theme }); return { theme } }),
  toggleTheme: () => set((s) => { const next: Theme = s.theme === 'light' ? 'dark' : 'light'; persistState({ ...s, theme: next }); return { theme: next } }),
  setLanguage: (language) => set((s) => { persistState({ ...s, language }); return { language } }),
}))
