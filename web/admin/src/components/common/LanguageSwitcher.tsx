import { useTranslation } from 'react-i18next'
import { cn } from '../../lib/cn'

const LANGS = [
  { code: 'zh-CN', label: '中文' },
  { code: 'en-US', label: 'English' },
]

export function LanguageSwitcher() {
  const { i18n } = useTranslation()

  return (
    <div className="language-switcher" role="group" aria-label="语言切换">
      {LANGS.map((lang) => (
        <button
          key={lang.code}
          type="button"
          className={cn('language-switcher__btn', i18n.language === lang.code && 'active')}
          onClick={() => i18n.changeLanguage(lang.code)}
          aria-pressed={i18n.language === lang.code}
        >
          {lang.label}
        </button>
      ))}
    </div>
  )
}
