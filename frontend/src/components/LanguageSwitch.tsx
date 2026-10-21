import { useTranslation } from 'react-i18next'

import { LANGUAGES } from '@/i18n'
import { cn } from '@/lib/utils'

export function LanguageSwitch({ className }: { className?: string }) {
  const { t, i18n } = useTranslation()
  return (
    <div role="group" aria-label={t('common.language')} className={cn('inline-flex rounded-full bg-active p-0.5', className)}>
      {LANGUAGES.map((lng) => {
        const active = i18n.language === lng
        return (
          <button
            key={lng}
            type="button"
            aria-pressed={active}
            onClick={() => void i18n.changeLanguage(lng)}
            className={cn(
              'h-8 min-w-11 cursor-pointer rounded-full px-3 text-xs font-medium transition-colors',
              active ? 'bg-raised text-ink' : 'text-muted hover:text-ink',
            )}
          >
            {lng.toUpperCase()}
          </button>
        )
      })}
    </div>
  )
}
