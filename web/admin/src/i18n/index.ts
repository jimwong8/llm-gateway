import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import zhCN from './locales/zh-CN.json'

const loadedLanguages = new Set(['zh-CN'])

i18n.use(initReactI18next).init({
  resources: {
    'zh-CN': { translation: zhCN },
  },
  lng: 'zh-CN',
  fallbackLng: 'zh-CN',
  interpolation: { escapeValue: false },
})

export async function loadLanguage(lang: string): Promise<void> {
  if (loadedLanguages.has(lang)) return
  if (lang === 'en-US') {
    const enUS = await import('./locales/en-US.json')
    i18n.addResourceBundle('en-US', 'translation', enUS.default)
    loadedLanguages.add('en-US')
  }
}

export default i18n
