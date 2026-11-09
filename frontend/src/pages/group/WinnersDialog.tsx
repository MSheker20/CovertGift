import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { Idea } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { formatMoney } from '@/lib/format'
import { cn } from '@/lib/utils'

// Backend kuralıyla aynı üst sınır.
export const MAX_WINNERS = 20

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  // Seçilebilir (aktif) fikirler, havuzdaki sırasıyla.
  ideas: Idea[]
  // Pencere açılırken işaretli gelen fikir (kartın menüsünden seçilen).
  initialId?: string
  budget?: string
  pending: boolean
  onConfirm: (ids: string[]) => void
}

// Bir ya da daha fazla kazanan seçimi. Seçilenlerin toplamı bütçeyle birlikte gösterilir;
// bütçe aşılırsa uyarılır ama seçim engellenmez.
export function WinnersDialog({ open, onOpenChange, ideas, initialId, budget, pending, onConfirm }: Props) {
  const { t } = useTranslation()
  // Çağıran taraf pencereyi her açılışta yeniden bağlar (key), böylece seçim baştan kurulur.
  const [selected, setSelected] = useState<string[]>(initialId ? [initialId] : [])

  const total = ideas.filter((i) => selected.includes(i.id)).reduce((sum, i) => sum + (Number(i.estimated_price) || 0), 0)
  const budgetNum = Number(budget) || 0
  const over = budgetNum > 0 && total > budgetNum
  const full = selected.length >= MAX_WINNERS

  const toggle = (id: string) =>
    setSelected((s) => (s.includes(id) ? s.filter((x) => x !== id) : full ? s : [...s, id]))

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (pending) return
        onOpenChange(o)
      }}
    >
      <DialogContent closeLabel={t('common.close')} className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle className="text-[19px]">{t('pool.winnersTitle')}</DialogTitle>
          <DialogDescription className="text-[15px] leading-6">{t('pool.winnersBody')}</DialogDescription>
        </DialogHeader>

        <ul className="-mx-1 flex max-h-[45dvh] flex-col gap-1 overflow-y-auto px-1" aria-label={t('pool.winnersTitle')}>
          {ideas.map((idea) => {
            const checked = selected.includes(idea.id)
            const price = Number(idea.estimated_price)
            return (
              <li key={idea.id}>
                <label
                  className={cn(
                    'flex min-h-11 cursor-pointer items-center gap-3 rounded-[12px] border border-border px-3 py-2 transition-colors hover:border-control',
                    checked && 'border-primary bg-soft hover:border-primary',
                    !checked && full && 'cursor-not-allowed opacity-60',
                  )}
                >
                  <input
                    type="checkbox"
                    className="size-4 shrink-0 accent-[var(--color-primary)]"
                    checked={checked}
                    disabled={!checked && full}
                    onChange={() => toggle(idea.id)}
                  />
                  <span className="min-w-0 flex-1 truncate text-sm font-medium">{idea.title}</span>
                  <span className="num text-sm whitespace-nowrap text-muted">{price > 0 ? formatMoney(price) : t('pool.noPrice')}</span>
                </label>
              </li>
            )
          })}
        </ul>

        <div className="flex flex-col gap-1 text-sm" aria-live="polite">
          <p className="flex flex-wrap justify-between gap-x-3">
            <span className="num font-semibold">{t('pool.winnersTotal', { total: formatMoney(total) })}</span>
            {budgetNum > 0 && <span className="num text-muted">{t('pool.winnersBudget', { budget: formatMoney(budgetNum) })}</span>}
          </p>
          {over && <p className="text-xs font-medium text-pill-warn-ink">{t('pool.winnersOverBudget')}</p>}
          {full && <p className="text-xs text-muted">{t('pool.winnersMax', { max: MAX_WINNERS })}</p>}
        </div>

        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={pending}>
            {t('pool.cancel')}
          </Button>
          <Button onClick={() => onConfirm(selected)} disabled={pending || selected.length === 0}>
            {pending ? t('common.loading') : t('pool.winnersConfirm', { count: selected.length })}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
