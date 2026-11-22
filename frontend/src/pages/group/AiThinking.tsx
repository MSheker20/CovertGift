import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Sparkles } from 'lucide-react'

import { Bar } from '@/components/Panel'

const STEPS = ['pool.aiStep1', 'pool.aiStep2', 'pool.aiStep3', 'pool.aiStep4'] as const

// Gemini fikir üretirken (1–2 dakika): durum şeridi, birkaç saniyede bir değişen kısa adım
// metni ve kenarı dönen üç hayalet kart. Ekran okuyucu yalnızca sabit açıklamayı bir kez duyar;
// dönen adım metni görseldir.
export function AiThinking() {
  const { t } = useTranslation()
  const [step, setStep] = useState(0)
  useEffect(() => {
    const timer = setInterval(() => setStep((s) => (s + 1) % STEPS.length), 3500)
    return () => clearInterval(timer)
  }, [])

  return (
    <div className="mb-2">
      <div className="mb-2 flex items-start gap-3 rounded-[14px] bg-soft px-4 py-3.5 text-sm text-on-soft">
        <Sparkles className="mt-0.5 size-4 shrink-0 animate-pulse motion-reduce:animate-none" aria-hidden />
        <div className="min-w-0">
          <p role="status">{t('pool.geminiWorkingLong')}</p>
          <p aria-hidden="true" className="mt-1 h-5 overflow-hidden font-medium">
            <span key={step} className="block animate-[text-in-up_320ms_cubic-bezier(0.2,0.8,0.2,1)]">
              {t(STEPS[step])}
            </span>
          </p>
        </div>
      </div>
      <ul className="flex flex-col gap-2" aria-hidden="true">
        {/* Başlık genişlikleri satır satır farklı: hayalet kartlar birbirinin kopyası gibi durmasın. */}
        {[62, 48, 55].map((width, i) => (
          <li
            key={width}
            className="ai-ghost flex gap-3 rounded-[18px] p-3 md:gap-4 md:p-3.5"
            style={{ animationDelay: `${i * -0.6}s` }}
          >
            <span className="inline-flex size-[72px] shrink-0 items-center justify-center rounded-[12px] bg-soft">
              <Sparkles className="size-5 text-on-soft opacity-70" />
            </span>
            <div className="flex-1 pt-1">
              <Bar className="h-4" style={{ width: `${width}%` }} />
              <Bar className="mt-2.5 h-3.5 w-1/4" />
              <Bar className="mt-4 h-3 w-1/2" />
            </div>
          </li>
        ))}
      </ul>
    </div>
  )
}
