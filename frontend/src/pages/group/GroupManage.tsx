import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { MoreHorizontal, Pencil, Plus, Trash2, X } from 'lucide-react'
import { toast } from 'sonner'

import { ApiError } from '@/api/client'
import { groupsApi, queryKeys } from '@/api/endpoints'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { Bar } from '@/components/Panel'
import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { errorMessage } from '@/lib/errors'
import { formatMoney } from '@/lib/format'

// Backend kuralıyla aynı: tek öğe, 1–100 karakter, virgül/noktalı virgül/satır sonu yok.
const INTEREST = /^[^,;\r\n]{1,100}$/
// 0–99999999.99, en fazla iki ondalık; virgül de kabul edilir.
const MONEY = /^\d{1,8}([.,]\d{1,2})?$/

// İlgi alanları: her üye ekleyebilir, yalnızca admin silebilir (yetki API'de ayrıca denetlenir).
export function InterestsTile({ groupId, interests, isAdmin }: { groupId: string; interests?: string[]; isAdmin: boolean }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [adding, setAdding] = useState(false)
  const [value, setValue] = useState('')
  const [error, setError] = useState<string | null>(null)

  const refresh = () => queryClient.invalidateQueries({ queryKey: queryKeys.group(groupId), exact: true })

  const add = useMutation({
    mutationFn: (interest: string) => groupsApi.addInterest(groupId, interest),
    onSuccess: () => {
      setValue('')
      setAdding(false)
      void refresh()
    },
    onError: (err) =>
      setError(err instanceof ApiError && err.status === 409 ? t('manage.interestDuplicate') : errorMessage(err)),
  })

  const remove = useMutation({
    mutationFn: (interest: string) => groupsApi.removeInterest(groupId, interest),
    onSuccess: () => void refresh(),
    onError: (err) => toast.error(errorMessage(err)),
  })

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    const interest = value.trim()
    if (!INTEREST.test(interest)) {
      setError(t('manage.interestInvalid'))
      return
    }
    setError(null)
    add.mutate(interest)
  }

  return (
    <div className="col-span-2 rounded-[22px] bg-surface p-4 md:col-span-4 md:p-5">
      <dt className="text-sm font-medium text-muted">{t('group.interestsPanel')}</dt>
      <dd className="mt-3 flex flex-wrap items-center gap-1.5">
        {interests === undefined && <Bar className="h-7 w-40" />}
        {interests?.length === 0 && !adding && <span className="text-sm text-muted">{t('group.interestsEmpty')}</span>}
        {interests?.map((i) => (
          <span
            key={i}
            className="inline-flex h-8 items-center gap-1 rounded-full bg-soft pr-1 pl-3 text-sm font-medium text-on-soft"
          >
            {i}
            {isAdmin ? (
              <button
                type="button"
                aria-label={t('manage.removeInterest', { interest: i })}
                disabled={remove.isPending}
                onClick={() => remove.mutate(i)}
                className="inline-flex size-6 cursor-pointer items-center justify-center rounded-full transition-colors hover:bg-active"
              >
                <X className="size-3.5" />
              </button>
            ) : (
              <span className="w-2" />
            )}
          </span>
        ))}
        {interests !== undefined && !adding && (
          <button
            type="button"
            aria-label={t('manage.addInterestAria')}
            onClick={() => setAdding(true)}
            className="inline-flex h-8 cursor-pointer items-center gap-1 rounded-full border border-dashed border-control px-3 text-sm text-muted transition-colors hover:border-accent hover:text-ink"
          >
            <Plus className="size-3.5" />
            {t('manage.addInterest')}
          </button>
        )}
      </dd>
      {adding && (
        <form onSubmit={onSubmit} className="mt-3">
          <div className="flex gap-2">
            <Input
              autoFocus
              aria-label={t('manage.interestLabel')}
              placeholder={t('manage.interestPlaceholder')}
              maxLength={100}
              aria-invalid={error ? true : undefined}
              value={value}
              onChange={(e) => {
                setValue(e.target.value)
                setError(null)
              }}
              onKeyDown={(e) => e.key === 'Escape' && setAdding(false)}
            />
            <Button
              type="submit"
              variant="outline"
              className="h-11 md:h-10"
              aria-label={t('manage.addInterestAria')}
              disabled={add.isPending}
            >
              {t('manage.addInterest')}
            </Button>
          </div>
          {error && (
            <p role="alert" className="mt-2 text-xs text-negative">
              {error}
            </p>
          )}
        </form>
      )}
    </div>
  )
}

// Toplam bütçe: admin yerinde düzenler. Kişi başı tutar ve bütçe üstü etiketleri yenilenir.
export function BudgetTile({ groupId, total, isAdmin }: { groupId: string; total?: string; isAdmin: boolean }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState(false)
  const [value, setValue] = useState('')
  const [error, setError] = useState<string | null>(null)

  const save = useMutation({
    mutationFn: (amount: string) => groupsApi.updateBudget(groupId, amount),
    onSuccess: () => {
      setEditing(false)
      toast.success(t('manage.budgetSaved'))
      // Grup detayı, grup listesi ve fikirlerin bütçe üstü etiketleri birlikte değişir.
      void queryClient.invalidateQueries({ queryKey: queryKeys.groups })
    },
    onError: (err) => setError(errorMessage(err)),
  })

  const start = () => {
    setValue(total ? String(Number(total)) : '')
    setError(null)
    setEditing(true)
  }

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    const amount = value.trim()
    if (!MONEY.test(amount)) {
      setError(t('manage.budgetInvalid'))
      return
    }
    save.mutate(amount.replace(',', '.'))
  }

  return (
    <div className="rounded-[22px] bg-surface p-4 md:col-span-4 md:p-5">
      <dt className="flex items-center justify-between gap-2 text-sm font-medium text-muted">
        {t('group.totalBudget')}
        {isAdmin && !editing && total !== undefined && (
          <button
            type="button"
            aria-label={t('manage.editBudget')}
            title={t('manage.editBudget')}
            onClick={start}
            className="-my-2 -mr-2 inline-flex size-9 cursor-pointer items-center justify-center rounded-full text-muted transition-colors hover:bg-active hover:text-ink"
          >
            <Pencil className="size-4" />
          </button>
        )}
      </dt>
      {editing ? (
        <dd>
          <form onSubmit={onSubmit} className="mt-2 flex flex-col gap-2">
            <Input
              autoFocus
              inputMode="decimal"
              aria-label={t('manage.budgetLabel')}
              aria-invalid={error ? true : undefined}
              className="num"
              value={value}
              onChange={(e) => {
                setValue(e.target.value)
                setError(null)
              }}
              onKeyDown={(e) => e.key === 'Escape' && setEditing(false)}
            />
            {error && (
              <p role="alert" className="text-xs text-negative">
                {error}
              </p>
            )}
            <div className="flex gap-2">
              <Button type="submit" size="sm" disabled={save.isPending}>
                {save.isPending ? t('common.loading') : t('manage.save')}
              </Button>
              <Button type="button" size="sm" variant="ghost" onClick={() => setEditing(false)}>
                {t('manage.cancel')}
              </Button>
            </div>
          </form>
        </dd>
      ) : (
        <dd className="font-display num mt-2 text-[22px] leading-8 font-semibold md:text-[32px] md:leading-10">
          {total !== undefined ? formatMoney(total) : <Bar className="h-8 w-28" />}
        </dd>
      )}
    </div>
  )
}

// Başlıktaki admin menüsü. Silme geri alınamaz; onay grubun adını anar.
export function GroupMenu({ groupId, title }: { groupId: string; title: string }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [confirm, setConfirm] = useState(false)

  const remove = useMutation({
    mutationFn: () => groupsApi.remove(groupId),
    onSuccess: () => {
      toast.success(t('manage.deleted', { title }))
      setConfirm(false)
      queryClient.removeQueries({ queryKey: queryKeys.group(groupId) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.groups })
      navigate('/', { replace: true })
    },
    onError: (err) => toast.error(errorMessage(err)),
  })

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" size="icon" aria-label={t('manage.groupActions')}>
            <MoreHorizontal />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem variant="destructive" onSelect={() => setConfirm(true)}>
            <Trash2 />
            {t('manage.deleteGroup')}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <ConfirmDialog
        open={confirm}
        onOpenChange={(o) => !remove.isPending && setConfirm(o)}
        title={t('manage.deleteTitle')}
        body={t('manage.deleteBody', { title })}
        confirmLabel={t('manage.deleteConfirm')}
        cancelLabel={t('manage.cancel')}
        pending={remove.isPending}
        onConfirm={() => remove.mutate()}
      />
    </>
  )
}
