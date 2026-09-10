import { useUIStore } from '../../stores/ui-store'

export function LanguageSwitcher() {
  const { language, setLanguage } = useUIStore()
  return (
    <select
      value={language}
      onChange={(e) => setLanguage(e.target.value as 'zh-CN' | 'en-US')}
      className="btn btn--sm btn--outline"
      style={{ padding: '4px 8px', fontSize: '13px' }}
    >
      <option value="zh-CN">中文</option>
      <option value="en-US">English</option>
    </select>
  )
}
