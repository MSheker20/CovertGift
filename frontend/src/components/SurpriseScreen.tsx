import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import { LogoMark } from '@/components/Logo'
import { Button } from '@/components/ui/button'

// Anti-Spoiler 403'ü: hedef kişi grubun içine (ya da davetine) hiç bakamaz.
export function SurpriseScreen() {
  const { t } = useTranslation()
  return (
    <div className="flex min-h-[calc(100dvh-88px)] items-center justify-center p-4 lg:p-8">
      <div className="flex w-full max-w-[640px] flex-col items-center rounded-[28px] bg-field px-6 py-14 text-center text-on-field md:py-20">
        <LogoMark className="size-20 origin-bottom animate-[wiggle_2.4s_ease-in-out_infinite] text-primary" />
        <h1 className="font-display mt-8 text-[32px] leading-[1.05] font-bold md:text-[48px]">{t('group.surpriseTitle')}</h1>
        <p className="mt-4 max-w-[420px] text-[16px] leading-6 text-on-field-muted">{t('group.surpriseLead')}</p>
        <Button asChild size="lg" className="mt-8">
          <Link to="/">{t('group.surpriseBack')}</Link>
        </Button>
      </div>
    </div>
  )
}
