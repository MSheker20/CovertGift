import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'

import en from './locales/en'
import tr, { type Messages } from './locales/tr'

export const LANGUAGES = ['tr', 'en'] as const
export type Language = (typeof LANGUAGES)[number]

const STORAGE_KEY = 'covertgift.lang'

function initialLanguage(): Language {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved === 'tr' || saved === 'en') return saved
  } catch {
    // yok say
  }
  return navigator.language?.toLowerCase().startsWith('en') ? 'en' : 'tr'
}

declare module 'i18next' {
  interface CustomTypeOptions {
    defaultNS: 'translation'
    resources: { translation: Messages }
  }
}

void i18n.use(initReactI18next).init({
  resources: { tr: { translation: tr }, en: { translation: en } },
  lng: initialLanguage(),
  fallbackLng: 'tr',
  interpolation: { escapeValue: false },
  returnObjects: true,
})

i18n.on('languageChanged', (lng) => {
  document.documentElement.lang = lng
  try {
    localStorage.setItem(STORAGE_KEY, lng)
  } catch {
    // yok say
  }
})
document.documentElement.lang = i18n.language

export default i18n
