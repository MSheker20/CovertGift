import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import { Button } from '@/components/ui/button'

// Kapağı ara ara aralanan kutu: içinde ne olduğu görünmez, yalnızca iki kıvılcım kaçar.
// Logodaki kutunun çizimi; kapak (bant + fiyonk) ayrı grup olarak döner.
function GiftBox() {
  return (
    <svg viewBox="0 0 64 64" aria-hidden="true" className="size-24 overflow-visible text-primary">
      <g className="origin-[32px_14px] animate-[sparkle_3.2s_ease-out_infinite] fill-on-primary [transform-box:view-box]">
        <path d="M24 10l1.2 3 3 1.2-3 1.2L24 18.4l-1.2-3-3-1.2 3-1.2z" />
      </g>
      <g className="origin-[42px_10px] animate-[sparkle_3.2s_ease-out_0.15s_infinite] fill-primary [transform-box:view-box]">
        <path d="M42 6l.9 2.1 2.1.9-2.1.9L42 12l-.9-2.1-2.1-.9 2.1-.9z" />
      </g>
      <rect x="8" y="27" width="48" height="31" rx="8" fill="currentColor" />
      <rect x="29" y="27" width="6" height="31" className="fill-on-primary" />
      <g className="origin-[6px_26px] animate-[lid-peek_3.2s_ease-in-out_infinite] [transform-box:view-box]">
        <rect x="5" y="18" width="54" height="12" rx="6" fill="currentColor" />
        <rect x="29" y="18" width="6" height="12" className="fill-on-primary" />
        <path d="M32 18c-4-9-15-9-13.5-2.5C19.6 19 28 18 32 18Zm0 0c4-9 15-9 13.5-2.5C44.4 19 36 18 32 18Z" fill="currentColor" />
      </g>
    </svg>
  )
}

// Anti-Spoiler 403'ü: hedef kişi grubun içine (ya da davetine) hiç bakamaz.
export function SurpriseScreen() {
  const { t } = useTranslation()
  return (
    <div className="flex min-h-[calc(100dvh-88px)] items-center justify-center p-4 lg:p-8">
      <div className="flex w-full max-w-[640px] flex-col items-center rounded-[28px] bg-field px-6 py-14 text-center text-on-field md:py-20 grain">
        <GiftBox />
        <h1 className="font-display mt-8 text-[32px] leading-[1.05] font-bold md:text-[48px]">{t('group.surpriseTitle')}</h1>
        <p className="mt-4 max-w-[420px] text-[16px] leading-6 text-on-field-muted">{t('group.surpriseLead')}</p>
        <Button asChild size="lg" className="mt-8">
          <Link to="/">{t('group.surpriseBack')}</Link>
        </Button>
      </div>
    </div>
  )
}
