import { useId, useLayoutEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Plus, ShieldAlert, ShieldCheck, UserCheck } from 'lucide-react'

import type { AccountSummary } from '@/api/types'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { DescriptionDialog, LinkTargetDialog } from './GroupManage'

// Başlığın altındaki açıklama: en çok 3 satır görünür; taşıyorsa "Devamını gör / Daha az".
// Düğme yalnızca metin gerçekten taşıyorsa çizilir (ölçülür). Açıklama yoksa yöneticiye
// "Açıklama ekle" ipucu, üyeye hiçbir şey gösterilir.
export function GroupDescription({
  groupId,
  text,
  isAdmin,
  multi = false,
}: {
  groupId: string
  text?: string
  isAdmin: boolean
  multi?: boolean
}) {
  const { t } = useTranslation()
  const id = useId()
  const ref = useRef<HTMLParagraphElement>(null)
  const [expanded, setExpanded] = useState(false)
  const [overflows, setOverflows] = useState(false)
  const [editing, setEditing] = useState(false)
  // Açıklama değişince (düzenleme ya da başka üyenin değişikliği) yeniden kısaltılmış başlar.
  const [shown, setShown] = useState(text)
  if (text !== shown) {
    setShown(text)
    setExpanded(false)
  }

  useLayoutEffect(() => {
    const el = ref.current
    // Açıkken ölçülmez: düğme "Daha az" olarak kalır.
    if (!el || expanded) return
    const measure = () => setOverflows(el.scrollHeight - el.clientHeight > 1)
    measure()
    // Yazı tipi geç yüklenince satır sayısı değişebilir.
    void document.fonts?.ready.then(measure)
    if (typeof ResizeObserver === 'undefined') return
    const observer = new ResizeObserver(measure)
    observer.observe(el)
    return () => observer.disconnect()
  }, [text, expanded])

  if (!text) {
    if (!isAdmin) return null
    return (
      <>
        <button
          type="button"
          onClick={() => setEditing(true)}
          className="inline-flex h-9 w-fit cursor-pointer items-center gap-1 rounded-full border border-dashed border-control px-3 text-sm text-muted transition-colors hover:border-accent hover:text-ink"
        >
          <Plus className="size-3.5" aria-hidden />
          {t('group.descriptionAdd')}
        </button>
        <DescriptionDialog groupId={groupId} open={editing} onOpenChange={setEditing} multi={multi} />
      </>
    )
  }

  return (
    <div className="max-w-[68ch]">
      <p
        id={id}
        ref={ref}
        data-testid="group-description"
        className={cn('text-[15px] leading-6 break-words whitespace-pre-line text-ink', !expanded && 'line-clamp-3')}
      >
        {text}
      </p>
      {(overflows || expanded) && (
        <button
          type="button"
          aria-expanded={expanded}
          aria-controls={id}
          onClick={() => setExpanded((v) => !v)}
          className="-ml-1 inline-flex h-9 cursor-pointer items-center rounded-full px-1 text-sm font-medium text-accent hover:underline"
        >
          {expanded ? t('group.descriptionLess') : t('group.descriptionMore')}
        </button>
      )}
    </div>
  )
}

// Hedef kişinin hesap durumu. Renk tek başına anlam taşımaz: durum metni her zaman yazılıdır,
// ikon aria-hidden'dır. Bağlıysa kalkan kobalt ve bağlı hesabın adı; değilse turuncu uyarı tonu
// ve (yalnızca yöneticiye) "Hesabı bağla" eylemi.
export function TargetStatus({
  groupId,
  targetName,
  linked,
  account,
  isAdmin,
}: {
  groupId: string
  targetName: string
  linked: boolean
  account?: AccountSummary
  isAdmin: boolean
}) {
  const { t } = useTranslation()
  const [linking, setLinking] = useState(false)

  if (linked) {
    const name = account?.name || targetName
    const hint = t('group.targetLinkedHint', { name })
    return (
      <div className="flex">
        <span
          title={hint}
          data-state="linked"
          className="inline-flex max-w-full min-h-8 items-center gap-1.5 rounded-full bg-soft px-3 py-1 text-[13px] leading-5 font-medium text-on-soft"
        >
          <ShieldCheck className="size-4 shrink-0 text-accent" aria-hidden />
          <span className="min-w-0 break-words">{t('group.targetLinked', { name })}</span>
          <span className="sr-only">. {hint}</span>
        </span>
      </div>
    )
  }

  const hint = t('group.targetNotLinkedHint')
  return (
    <div className="flex flex-wrap items-center gap-2">
      <span
        title={hint}
        data-state="not-linked"
        className="inline-flex max-w-full min-h-8 items-center gap-1.5 rounded-full bg-pill-high-bg px-3 py-1 text-[13px] leading-5 font-medium text-pill-high-ink"
      >
        <ShieldAlert className="size-4 shrink-0" aria-hidden />
        <span className="min-w-0 break-words">{t('group.targetNotLinked')}</span>
        <span className="sr-only">. {hint}</span>
      </span>
      {isAdmin && (
        <>
          <Button type="button" variant="outline" size="sm" onClick={() => setLinking(true)}>
            <UserCheck aria-hidden />
            {t('group.targetLinkAction')}
          </Button>
          <LinkTargetDialog groupId={groupId} targetName={targetName} open={linking} onOpenChange={setLinking} />
        </>
      )}
    </div>
  )
}
