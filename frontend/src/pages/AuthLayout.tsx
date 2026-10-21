import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { EyeOff, ThumbsUp } from 'lucide-react'

import { Avatar } from '@/components/Avatar'
import { LanguageSwitch } from '@/components/LanguageSwitch'
import { Logo } from '@/components/Logo'

// Giriş ve kayıt için ortak çerçeve: solda form, masaüstünde sağda ürünün kendisinden bir kesit.
export function AuthLayout({ title, lead, children }: { title: string; lead: string; children: ReactNode }) {
  return (
    <div className="min-h-dvh bg-ground">
      <div className="mx-auto grid min-h-dvh max-w-[1180px] grid-cols-1 gap-4 p-4 md:p-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.05fr)]">
        <div className="flex flex-col">
          <header className="flex h-12 items-center justify-between">
            <Logo />
            <LanguageSwitch />
          </header>
          <main className="flex flex-1 items-center py-10">
            <div className="w-full max-w-[400px] lg:ml-[clamp(0px,6vw,72px)]">
              <h1 className="font-display text-[34px] leading-[1.05] font-bold md:text-[44px]">{title}</h1>
              <p className="mt-3 text-[15px] leading-6 text-muted">{lead}</p>
              <div className="mt-8">{children}</div>
            </div>
          </main>
        </div>
        <ProductGlimpse />
      </div>
    </div>
  )
}

function ProductGlimpse() {
  const { t } = useTranslation()
  const ideas = t('auth.asideIdeas', { returnObjects: true })
  const people = ['Mustafa', 'Zeynep', 'Can', 'Elif']
  return (
    <aside className="hidden flex-col justify-between gap-10 rounded-[28px] bg-primary p-10 text-on-primary lg:flex">
      <div className="max-w-[440px]">
        <p className="font-display text-[30px] leading-[1.12] font-bold">{t('auth.asideTitle')}</p>
        <p className="mt-4 text-[16px] leading-6 text-[#e3e6ff]">{t('auth.asideLead')}</p>
      </div>

      <div data-ui className="rounded-[20px] bg-raised p-5 text-ink" aria-hidden="true">
        <div className="flex items-start justify-between gap-4">
          <div>
            <p className="text-[15px] font-semibold">{t('auth.asideGroup')}</p>
            <p className="num mt-1.5 inline-flex rounded-full bg-field px-2.5 py-0.5 text-xs font-semibold text-on-field">{t('auth.asideBudget')}</p>
          </div>
          <div className="flex -space-x-1.5">
            {people.map((p) => (
              <Avatar key={p} name={p} size="sm" className="ring-2 ring-raised" />
            ))}
          </div>
        </div>
        <ul className="mt-4 flex flex-col gap-2">
          {ideas.map((idea) => (
            <li key={idea.title} className="flex items-center gap-3 rounded-[12px] bg-surface px-3.5 py-3">
              <span className="min-w-0 flex-1 truncate text-sm">{idea.title}</span>
              <span className="num text-sm text-muted">{idea.price}</span>
              <span className="num inline-flex w-12 items-center justify-end gap-1 text-sm font-semibold text-accent">
                <ThumbsUp className="size-3.5" />
                {idea.score}
              </span>
            </li>
          ))}
        </ul>
        <p className="mt-4 inline-flex items-center gap-2 text-xs text-muted">
          <EyeOff className="size-3.5" />
          {t('auth.asideHidden')}
        </p>
      </div>
    </aside>
  )
}
