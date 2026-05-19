import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { cn } from '../../lib/cn'
import { loadLanguage } from '../../i18n'

const LANGS = [
  { code: 'zh-CN', label: '中文' },
  { code: 'en-US', label: 'English' },
]

export function LanguageSwitcher() {
  const { i18n } = useTranslation()
  const [loading, setLoading] = useState(false)

  const handleSwitch = async (lang: string) => {
    if (loading || i18n.language === lang) return
    setLoading(true)
    try {
      await loadLanguage(lang)
      await i18n.changeLanguage(lang)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="language-switcher" role="group" aria-label="语言切换">
      {LANGS.map((lang) => (
        <button
          key={lang.code}
          type="button"
          className={cn('language-switcher__btn', i18n.language === lang.code && 'active')}
          onClick={() => handleSwitch(lang.code)}
          disabled={loading}
          aria-pressed={i18n.language === lang.code}
        >
          {lang.label}
        </button>
      ))}
    </div>
  )
}
