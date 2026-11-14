import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { CalendarDays, Eye, FileText, ImageOff, ImagePlus, MoreHorizontal, Pencil, Plus, RotateCcw, Trash2, UserCheck, X } from 'lucide-react'
import { toast } from 'sonner'

import { ApiError } from '@/api/client'
import { groupsApi, queryKeys } from '@/api/endpoints'
import type { GroupMode } from '@/api/types'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { CoverCropDialog } from '@/components/CoverCropDialog'
import { FormField } from '@/components/FormField'
import { Bar } from '@/components/Panel'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input, Textarea } from '@/components/ui/input'
import { cn } from '@/lib/utils'
import { descriptionLength, MAX_GROUP_DESCRIPTION } from '@/lib/description'
import { errorMessage } from '@/lib/errors'
import { eventDateBounds } from '@/lib/eventDate'
import { formatMoney } from '@/lib/format'
import { groupScope, type Scope } from './scope'

// Backend kuralıyla aynı: tek öğe, 1–100 karakter, virgül/noktalı virgül/satır sonu yok.
const INTEREST = /^[^,;\r\n]{1,100}$/
// 0–99999999.99, en fazla iki ondalık; virgül de kabul edilir.
const MONEY = /^\d{1,8}([.,]\d{1,2})?$/

// İlgi alanları: her üye ekleyebilir; SINGLE'da yalnızca admin, havuzda yalnızca havuz yöneticisi
// silebilir (canManage; yetki API'de ayrıca denetlenir).
export function InterestsTile({ scope, interests, canManage }: { scope: Scope; interests?: string[]; canManage: boolean }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [adding, setAdding] = useState(false)
  const [value, setValue] = useState('')
  const [error, setError] = useState<string | null>(null)

  const refresh = () => queryClient.invalidateQueries({ queryKey: scope.detailKey, exact: true })

  const add = useMutation({
    mutationFn: (interest: string) => scope.addInterest(interest),
    onSuccess: () => {
      setValue('')
      setAdding(false)
      void refresh()
    },
    onError: (err) =>
      setError(err instanceof ApiError && err.status === 409 ? t('manage.interestDuplicate') : errorMessage(err)),
  })

  const remove = useMutation({
    mutationFn: (interest: string) => scope.removeInterest(interest),
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
            {canManage ? (
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
export function BudgetTile({ scope, total, canManage }: { scope: Scope; total?: string; canManage: boolean }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState(false)
  const [value, setValue] = useState('')
  const [error, setError] = useState<string | null>(null)

  const save = useMutation({
    mutationFn: (amount: string) => scope.updateBudget(amount),
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
        {canManage && !editing && total !== undefined && (
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

// Grup açıklaması: yalnızca yönetici düzenler. Boş bırakmak açıklamayı kaldırır; sunucu metni kırpar.
export function DescriptionDialog({
  groupId,
  current,
  open,
  onOpenChange,
  multi = false,
}: {
  groupId: string
  current?: string
  // MULTI grupta açıklamayı hediye alacak kişiler de görür; pencere bunu söyler.
  multi?: boolean
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [value, setValue] = useState(current ?? '')
  const [error, setError] = useState<string | null>(null)
  // Pencere dışarıdan (menü ya da başlıktaki ipucu) açılır; her açılışta güncel açıklamayla başlar.
  const [wasOpen, setWasOpen] = useState(open)
  if (open !== wasOpen) {
    setWasOpen(open)
    if (open) {
      setValue(current ?? '')
      setError(null)
    }
  }
  const count = descriptionLength(value)
  const tooLong = count > MAX_GROUP_DESCRIPTION

  const save = useMutation({
    mutationFn: (text: string) => groupsApi.updateDescription(groupId, text),
    onSuccess: (_g, text) => {
      toast.success(t(text.trim() ? 'manage.descriptionSaved' : 'manage.descriptionRemoved'))
      onOpenChange(false)
      void queryClient.invalidateQueries({ queryKey: queryKeys.groups })
    },
    onError: (err) => setError(errorMessage(err)),
  })

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    if (tooLong) {
      setError(t('manage.descriptionTooLong', { max: MAX_GROUP_DESCRIPTION }))
      return
    }
    save.mutate(value)
  }

  const describedBy = ['description-count', error ? 'description-error' : ''].filter(Boolean).join(' ')

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (save.isPending) return
        onOpenChange(o)
      }}
    >
      <DialogContent closeLabel={t('common.close')} className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle className="text-[19px]">{t('manage.descriptionTitle')}</DialogTitle>
          <DialogDescription className="text-[15px] leading-6">{t('manage.descriptionBody')}</DialogDescription>
          {multi && (
            <p className="flex items-start gap-2 text-sm font-medium text-ink">
              <Eye className="mt-0.5 size-4 shrink-0 text-muted" aria-hidden />
              {t('manage.descriptionVisibleToRecipients')}
            </p>
          )}
        </DialogHeader>
        <form onSubmit={onSubmit} className="flex flex-col gap-4">
          <FormField id="group-description" label={t('manage.descriptionLabel')}>
            <Textarea
              id="group-description"
              rows={6}
              autoFocus
              className="max-h-[50dvh] resize-y"
              placeholder={t('newGroup.notesPlaceholder')}
              aria-invalid={error || tooLong ? true : undefined}
              aria-describedby={describedBy}
              value={value}
              onChange={(e) => {
                setValue(e.target.value)
                setError(null)
              }}
            />
            <p
              id="description-count"
              className={cn('num text-right text-xs', tooLong ? 'font-medium text-negative' : 'text-muted')}
            >
              {t('manage.descriptionCount', { count, max: MAX_GROUP_DESCRIPTION })}
            </p>
            {error && (
              <p id="description-error" role="alert" className="text-xs text-negative">
                {error}
              </p>
            )}
          </FormField>
          <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={save.isPending}>
              {t('manage.cancel')}
            </Button>
            <Button type="submit" disabled={save.isPending}>
              {save.isPending ? t('common.loading') : t('manage.save')}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// Hedef kişiyi e-postayla bir kez bağlar. Sonrasında Anti-Spoiler o kişiyi gruptan dışlar.
// Hem yönetici menüsünden hem de başlıktaki "Hesap bağlı değil" durumundan açılır.
export function LinkTargetDialog({
  groupId,
  targetName,
  open,
  onOpenChange,
}: {
  groupId: string
  targetName: string
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [email, setEmail] = useState('')

  const link = useMutation({
    mutationFn: () => groupsApi.linkTarget(groupId, email.trim()),
    onSuccess: () => {
      toast.success(t('manage.linked', { name: targetName }))
      onOpenChange(false)
      setEmail('')
      void queryClient.invalidateQueries({ queryKey: queryKeys.groups })
    },
  })

  const error = link.error
    ? link.error instanceof ApiError && link.error.status === 404
      ? t('manage.linkNotFound')
      : link.error instanceof ApiError && link.error.status === 400 && /kendi/i.test(link.error.message)
        ? t('manage.linkSelf')
        : errorMessage(link.error)
    : null

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    link.mutate()
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (link.isPending) return
        onOpenChange(o)
        if (!o) link.reset()
      }}
    >
      <DialogContent closeLabel={t('common.close')} className="sm:max-w-[440px]">
        <DialogHeader>
          <DialogTitle className="text-[19px]">{t('manage.linkTitle')}</DialogTitle>
          <DialogDescription className="text-[15px] leading-6">{t('manage.linkBody', { name: targetName })}</DialogDescription>
        </DialogHeader>
        <form onSubmit={onSubmit} className="flex flex-col gap-4">
          <FormField id="link-target-email" label={t('manage.linkEmail')}>
            <Input
              id="link-target-email"
              type="email"
              required
              autoFocus
              autoComplete="off"
              maxLength={150}
              placeholder={t('newGroup.targetEmailPlaceholder')}
              aria-describedby={error ? 'link-target-error' : undefined}
              aria-invalid={error ? true : undefined}
              value={email}
              onChange={(e) => {
                setEmail(e.target.value)
                if (link.error) link.reset()
              }}
            />
            {error && (
              <p id="link-target-error" role="alert" className="text-xs text-negative">
                {error}
              </p>
            )}
          </FormField>
          <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={link.isPending}>
              {t('manage.cancel')}
            </Button>
            <Button type="submit" disabled={link.isPending}>
              {link.isPending ? t('common.loading') : t('manage.linkSubmit')}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// Hediye tarihi: doğum günü ya da hediyenin verileceği gün. Boş bırakmak tarihi kaldırır.
export function EventDateDialog({
  scope,
  current,
  open,
  onOpenChange,
}: {
  scope: Scope
  current?: string
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [value, setValue] = useState(current ?? '')
  const [error, setError] = useState<string | null>(null)
  const bounds = eventDateBounds()

  const save = useMutation({
    mutationFn: (date: string | null) => scope.updateEventDate(date),
    onSuccess: (_d, date) => {
      toast.success(t(date ? 'manage.eventDateSaved' : 'manage.eventDateRemoved'))
      onOpenChange(false)
      void queryClient.invalidateQueries({ queryKey: queryKeys.groups })
    },
    onError: (err) => setError(errorMessage(err)),
  })

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    if (!value) {
      save.mutate(null)
      return
    }
    // "YYYY-MM-DD" metinleri sözlük sırasıyla karşılaştırılabilir.
    if (value < bounds.min || value > bounds.max) {
      setError(t('manage.eventDateInvalid'))
      return
    }
    save.mutate(value)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (save.isPending) return
        if (o) {
          setValue(current ?? '')
          setError(null)
        }
        onOpenChange(o)
      }}
    >
      <DialogContent closeLabel={t('common.close')} className="sm:max-w-[400px]">
        <DialogHeader>
          <DialogTitle className="text-[19px]">{t('manage.eventDateTitle')}</DialogTitle>
          <DialogDescription className="text-[15px] leading-6">{t('manage.eventDateBody')}</DialogDescription>
        </DialogHeader>
        <form onSubmit={onSubmit} className="flex flex-col gap-4">
          <FormField id="event-date" label={t('manage.eventDateLabel')}>
            <Input
              id="event-date"
              type="date"
              min={bounds.min}
              max={bounds.max}
              aria-invalid={error ? true : undefined}
              aria-describedby={error ? 'event-date-error' : undefined}
              value={value}
              onChange={(e) => {
                setValue(e.target.value)
                setError(null)
              }}
            />
            {error && (
              <p id="event-date-error" role="alert" className="text-xs text-negative">
                {error}
              </p>
            )}
          </FormField>
          <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            {current && (
              <Button type="button" variant="ghost" className="sm:mr-auto" onClick={() => save.mutate(null)} disabled={save.isPending}>
                {t('manage.eventDateRemove')}
              </Button>
            )}
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={save.isPending}>
              {t('manage.cancel')}
            </Button>
            <Button type="submit" disabled={save.isPending}>
              {save.isPending ? t('common.loading') : t('manage.save')}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// Başlıktaki admin menüsü. Silme geri alınamaz; onay grubun adını anar.
// Hedef kişi yalnızca bağlı değilse bağlanabilir; bağlantı değiştirilemez.
// Grup menüsü. Yönetici tüm işlemleri "Grup işlemleri" menüsünde görür. Üye, izin varsa yalnızca
// kapak fotoğrafı işlemlerini içeren "Kapak fotoğrafı" düğmesini görür; izin yoksa hiçbir şey çizilmez.
export function GroupMenu({
  groupId,
  title,
  targetName,
  targetLinked,
  description,
  eventDate,
  completed,
  isAdmin,
  coverVersion,
  coverAdminOnly,
  mode = 'SINGLE',
}: {
  groupId: string
  title: string
  targetName: string
  targetLinked: boolean
  description?: string
  eventDate?: string
  completed: boolean
  isAdmin: boolean
  coverVersion?: string
  coverAdminOnly: boolean
  // MULTI'de tarih, kazanan ve hedef kişi havuz düzeyindedir; grup menüsünde yer almaz.
  mode?: GroupMode
}) {
  const { t } = useTranslation()
  const multi = mode === 'MULTI'
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [confirm, setConfirm] = useState(false)
  const [linking, setLinking] = useState(false)
  const [dating, setDating] = useState(false)
  const [describing, setDescribing] = useState(false)
  const [reopening, setReopening] = useState(false)
  const [covering, setCovering] = useState(false)
  const [removingCover, setRemovingCover] = useState(false)
  // Yönetici her zaman; üyeler yalnızca kısıt kapalıyken. Yetki API'de ayrıca denetlenir.
  const coverEditable = isAdmin || !coverAdminOnly

  const refreshGroups = () => queryClient.invalidateQueries({ queryKey: queryKeys.groups })

  const uploadCover = useMutation({
    mutationFn: (image: Blob) => groupsApi.uploadCover(groupId, image),
    onSuccess: () => {
      toast.success(t('cover.saved'))
      setCovering(false)
      void refreshGroups()
    },
  })

  const removeCover = useMutation({
    mutationFn: () => groupsApi.removeCover(groupId),
    onSuccess: () => {
      toast.success(t('cover.removed'))
      setRemovingCover(false)
      void refreshGroups()
    },
    onError: (err) => toast.error(errorMessage(err)),
  })

  const coverPolicy = useMutation({
    mutationFn: (adminOnly: boolean) => groupsApi.setCoverPolicy(groupId, adminOnly),
    onSuccess: (res) => {
      toast.success(t(res.cover_admin_only ? 'cover.adminOnlyOn' : 'cover.adminOnlyOff'))
      void refreshGroups()
    },
    onError: (err) => toast.error(errorMessage(err)),
  })

  const reopen = useMutation({
    mutationFn: () => groupsApi.reopen(groupId),
    onSuccess: () => {
      toast.success(t('manage.reopened'))
      setReopening(false)
      void queryClient.invalidateQueries({ queryKey: queryKeys.groups })
    },
    onError: (err) => toast.error(errorMessage(err)),
  })

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

  if (!isAdmin && !coverEditable) return null

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          {/* Üyenin menüsünde yalnızca fotoğraf işlemleri vardır; düğme de bunu söyler. */}
          <Button variant="outline" size="icon" aria-label={isAdmin ? t('manage.groupActions') : t('cover.actions')}>
            {isAdmin ? <MoreHorizontal /> : <ImagePlus />}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-[min(288px,calc(100vw-24px))]">
          {coverEditable && (
            <DropdownMenuItem
              onSelect={() => {
                uploadCover.reset()
                setCovering(true)
              }}
            >
              <ImagePlus />
              {coverVersion ? t('cover.change') : t('cover.add')}
            </DropdownMenuItem>
          )}
          {coverEditable && coverVersion && (
            <DropdownMenuItem onSelect={() => setRemovingCover(true)}>
              <ImageOff />
              {t('cover.remove')}
            </DropdownMenuItem>
          )}
          {isAdmin && (
            <>
              {/* Ayar değişirken menü açık kalır; işaret kutusu yeni durumu gösterir. */}
              <DropdownMenuCheckboxItem
                checked={coverAdminOnly}
                disabled={coverPolicy.isPending}
                onSelect={(e) => e.preventDefault()}
                onCheckedChange={(checked) => coverPolicy.mutate(checked === true)}
              >
                {t('cover.adminOnly')}
              </DropdownMenuCheckboxItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onSelect={() => setDescribing(true)}>
                <FileText />
                {t('manage.editDescription')}
              </DropdownMenuItem>
              {!multi && (
                <DropdownMenuItem onSelect={() => setDating(true)}>
                  <CalendarDays />
                  {t('manage.eventDate')}
                </DropdownMenuItem>
              )}
              {!multi && completed && (
                <DropdownMenuItem onSelect={() => setReopening(true)}>
                  <RotateCcw />
                  {t('manage.reopen')}
                </DropdownMenuItem>
              )}
              {!multi && !targetLinked && (
                <DropdownMenuItem onSelect={() => setLinking(true)}>
                  <UserCheck />
                  {t('manage.linkTarget')}
                </DropdownMenuItem>
              )}
              <DropdownMenuItem variant="destructive" onSelect={() => setConfirm(true)}>
                <Trash2 />
                {t('manage.deleteGroup')}
              </DropdownMenuItem>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      <CoverCropDialog
        open={covering}
        onOpenChange={setCovering}
        onCropped={(image) => uploadCover.mutate(image)}
        confirmLabel={t('cover.save')}
        pending={uploadCover.isPending}
        error={uploadCover.error ? errorMessage(uploadCover.error) : null}
        hint={multi ? t('cover.visibleToRecipients') : undefined}
      />
      <ConfirmDialog
        open={removingCover}
        onOpenChange={(o) => !removeCover.isPending && setRemovingCover(o)}
        title={t('cover.removeTitle')}
        body={t('cover.removeBody', { title })}
        confirmLabel={t('cover.removeConfirm')}
        cancelLabel={t('manage.cancel')}
        pending={removeCover.isPending}
        onConfirm={() => removeCover.mutate()}
      />
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
      <LinkTargetDialog groupId={groupId} targetName={targetName} open={linking} onOpenChange={setLinking} />
      <EventDateDialog scope={groupScope(groupId)} current={eventDate} open={dating} onOpenChange={setDating} />
      <DescriptionDialog groupId={groupId} current={description} open={describing} onOpenChange={setDescribing} multi={multi} />
      <ConfirmDialog
        open={reopening}
        onOpenChange={(o) => !reopen.isPending && setReopening(o)}
        title={t('manage.reopenTitle')}
        body={t('manage.reopenBody')}
        confirmLabel={t('manage.reopenConfirm')}
        cancelLabel={t('manage.cancel')}
        pending={reopen.isPending}
        onConfirm={() => reopen.mutate()}
      />
    </>
  )
}
