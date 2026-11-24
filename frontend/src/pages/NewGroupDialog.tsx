import { useState, type FormEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, ImagePlus } from 'lucide-react'
import { toast } from 'sonner'

import { ApiError } from '@/api/client'
import { friendsApi, groupsApi, queryKeys } from '@/api/endpoints'
import { useObjectUrl } from '@/api/useGroupCover'
import { useAuth } from '@/auth/AuthProvider'
import { Avatar } from '@/components/Avatar'
import { CoverCropDialog } from '@/components/CoverCropDialog'
import { FormError, FormField } from '@/components/FormField'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Input, Textarea } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import type { Friend, GroupMode } from '@/api/types'
import { cn } from '@/lib/utils'
import { MAX_GROUP_DESCRIPTION } from '@/lib/description'
import { errorMessage } from '@/lib/errors'
import { eventDateBounds } from '@/lib/eventDate'

const MONEY = /^\d{1,8}([.,]\d{1,2})?$/

const EMPTY = { title: '', target: '', targetEmail: '', budget: '', interests: '', notes: '', eventDate: '' }

export function NewGroupDialog({ trigger }: { trigger: ReactNode }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { user } = useAuth()
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState(EMPTY)
  const [mode, setMode] = useState<GroupMode>('SINGLE')
  const multi = mode === 'MULTI'
  const [localError, setLocalError] = useState<string | null>(null)
  // İsteğe bağlı kapak: tarayıcıda kırpılmış JPEG; grup kurulduktan sonra yüklenir.
  const [cover, setCover] = useState<Blob | null>(null)
  const [pickingCover, setPickingCover] = useState(false)
  const coverPreview = useObjectUrl(cover)
  const bounds = eventDateBounds()
  // Arkadaşlar: hedef kişi listeden seçilebilir, seçilenlere grup daveti gider.
  const friends = useQuery({
    queryKey: queryKeys.friends,
    queryFn: ({ signal }) => friendsApi.list(signal),
    enabled: open,
  })
  const friendList: Friend[] = friends.data?.friends ?? []
  const [targetFriend, setTargetFriend] = useState('')
  const [memberIds, setMemberIds] = useState<string[]>([])
  const useFriendTarget = !multi && targetFriend !== ''
  const invitees = memberIds.filter((id) => id !== targetFriend || multi)

  const set = (key: keyof typeof EMPTY) => (e: { target: { value: string } }) =>
    setForm((f) => ({ ...f, [key]: e.target.value }))

  const mutation = useMutation({
    mutationFn: async () => {
      // MULTI'de hedef, bütçe, ilgi alanı ve tarih gönderilmez (sunucu 400 döner); bunlar her kişinin
      // havuzunda, alıcılar seçildikten sonra ayarlanır.
      const group = await groupsApi.create(
        multi
          ? {
              title: form.title.trim(),
              mode: 'MULTI',
              description: form.notes.trim() || undefined,
              member_ids: invitees.length > 0 ? invitees : undefined,
            }
          : {
              member_ids: invitees.length > 0 ? invitees : undefined,
              title: form.title.trim(),
              target_person_name: form.target.trim(),
              target_email: useFriendTarget ? undefined : form.targetEmail.trim() || undefined,
              target_user_id: useFriendTarget ? targetFriend : undefined,
              total_budget: (form.budget.trim() || '0').replace(',', '.'),
              description: form.notes.trim() || undefined,
              // Backend virgül ve satır sonu ayrımlı metni listeye ayrıştırır.
              target_interests: form.interests.trim() || undefined,
              event_date: form.eventDate || undefined,
            },
      )
      // Fotoğraf yüklenemese de grup kurulmuştur; hata ayrıca bildirilir, grup geri alınmaz.
      let coverFailed = false
      if (cover) {
        try {
          await groupsApi.uploadCover(group.id, cover)
        } catch {
          coverFailed = true
        }
      }
      return { group, coverFailed, invited: invitees.length }
    },
    onSuccess: ({ group, coverFailed, invited }) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.groups })
      toast.success(t('newGroup.created', { title: group.title }))
      if (invited > 0) toast.success(t('newGroup.friends.invited', { count: invited }))
      if (coverFailed) toast.error(t('newGroup.coverFailed'))
      setOpen(false)
      setForm(EMPTY)
      setMode('SINGLE')
      setCover(null)
      setTargetFriend('')
      setMemberIds([])
      navigate(`/groups/${group.id}`)
    },
  })

  // Kayıtlı olmayan e-posta alanın altında gösterilir; grup kurulmaz, isim yedeği için alan boşaltılabilir.
  const emailNotFound = mutation.error instanceof ApiError && mutation.error.status === 404
  const emailSelf = !useFriendTarget && !!user && form.targetEmail.trim().toLowerCase() === user.email.toLowerCase()

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    if (!multi && form.budget.trim() && !MONEY.test(form.budget.trim())) {
      setLocalError(t('newGroup.invalidBudget'))
      return
    }
    if (!multi && form.eventDate && (form.eventDate < bounds.min || form.eventDate > bounds.max)) {
      setLocalError(t('manage.eventDateInvalid'))
      return
    }
    if (!multi && emailSelf) return
    setLocalError(null)
    mutation.mutate()
  }

  const emailError = emailSelf ? t('newGroup.targetEmailSelf') : emailNotFound ? t('newGroup.targetEmailNotFound') : null

  const onOpenChange = (next: boolean) => {
    setOpen(next)
    if (!next) {
      mutation.reset()
      setLocalError(null)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>{trigger}</DialogTrigger>
      <DialogContent closeLabel={t('common.close')}>
        <DialogHeader>
          <DialogTitle>{t('newGroup.title')}</DialogTitle>
          <DialogDescription>{t('newGroup.description')}</DialogDescription>
        </DialogHeader>
        <form onSubmit={onSubmit} className="flex flex-col gap-4">
          <FormError message={localError ?? (mutation.error && !emailNotFound ? errorMessage(mutation.error) : null)} />
          <fieldset className="flex flex-col gap-2" aria-describedby="g-mode-hint">
            <legend className="mb-2 text-sm leading-none font-medium">{t('newGroup.mode.label')}</legend>
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
              {(['SINGLE', 'MULTI'] as const).map((m) => (
                <label
                  key={m}
                  className={cn(
                    'flex min-h-11 cursor-pointer items-center gap-3 rounded-[12px] border px-3.5 py-2.5 text-sm font-medium transition-colors focus-within:ring-2 focus-within:ring-accent/40',
                    mode === m ? 'border-primary bg-soft text-on-soft' : 'border-control bg-raised hover:bg-surface',
                  )}
                >
                  <input
                    type="radio"
                    name="g-mode"
                    value={m}
                    checked={mode === m}
                    onChange={() => {
                      setMode(m)
                      setLocalError(null)
                      mutation.reset()
                    }}
                    className="size-4 shrink-0 accent-[var(--color-primary)]"
                  />
                  {t(m === 'SINGLE' ? 'newGroup.mode.single' : 'newGroup.mode.multi')}
                </label>
              ))}
            </div>
            <p id="g-mode-hint" className="text-xs text-muted">
              {t(multi ? 'newGroup.mode.multiHint' : 'newGroup.mode.singleHint')}
            </p>
          </fieldset>
          <FormField id="g-title" label={t('newGroup.groupTitle')}>
            <Input
              id="g-title"
              required
              minLength={2}
              maxLength={150}
              placeholder={t('newGroup.groupTitlePlaceholder')}
              value={form.title}
              onChange={set('title')}
            />
          </FormField>
          {!multi && (
            <>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <FormField id="g-target" label={t('newGroup.target')}>
                <Input
                  id="g-target"
                  required
                  minLength={2}
                  maxLength={100}
                  placeholder={t('newGroup.targetPlaceholder')}
                  value={form.target}
                  onChange={set('target')}
                />
              </FormField>
              <FormField id="g-budget" label={t('newGroup.budget')} hint={t('newGroup.budgetHint')}>
                <Input
                  id="g-budget"
                  inputMode="decimal"
                  placeholder="2500"
                  aria-describedby="g-budget-hint"
                  className="num"
                  value={form.budget}
                  onChange={set('budget')}
                />
              </FormField>
            </div>
            {friendList.length > 0 && (
              <FormField id="g-target-friend" label={t('newGroup.friends.targetLabel')} hint={t('newGroup.friends.targetHint')}>
                <Select
                  id="g-target-friend"
                  aria-describedby="g-target-friend-hint"
                  value={targetFriend}
                  onChange={(e) => {
                    const id = e.target.value
                    setTargetFriend(id)
                    setMemberIds((ids) => ids.filter((m) => m !== id))
                    const picked = friendList.find((f) => f.id === id)
                    if (picked && !form.target.trim()) setForm((f) => ({ ...f, target: picked.name.split(' ')[0] }))
                    if (emailNotFound) mutation.reset()
                  }}
                >
                  <option value="">{t('newGroup.friends.targetNone')}</option>
                  {friendList.map((f) => (
                    <option key={f.id} value={f.id}>
                      {f.name}
                    </option>
                  ))}
                </Select>
              </FormField>
            )}
            {!useFriendTarget && (
            <FormField
              id="g-target-email"
              label={t('newGroup.targetEmail')}
              optional={t('newGroup.optional')}
              hint={t('newGroup.targetEmailHint')}
            >
              <Input
                id="g-target-email"
                type="email"
                autoComplete="off"
                maxLength={150}
                placeholder={t('newGroup.targetEmailPlaceholder')}
                aria-describedby={emailError ? 'g-target-email-error g-target-email-hint' : 'g-target-email-hint'}
                aria-invalid={emailError ? true : undefined}
                value={form.targetEmail}
                onChange={(e) => {
                  set('targetEmail')(e)
                  if (emailNotFound) mutation.reset()
                }}
              />
              {emailError && (
                <p id="g-target-email-error" role="alert" className="text-xs text-negative">
                  {emailError}
                </p>
              )}
            </FormField>
            )}
            <FormField id="g-event-date" label={t('newGroup.eventDate')} optional={t('newGroup.optional')} hint={t('newGroup.eventDateHint')}>
              <Input
                id="g-event-date"
                type="date"
                min={bounds.min}
                max={bounds.max}
                aria-describedby="g-event-date-hint"
                value={form.eventDate}
                onChange={set('eventDate')}
              />
            </FormField>
            <FormField
              id="g-interests"
              label={t('newGroup.interests')}
              optional={t('newGroup.optional')}
              hint={t('newGroup.interestsHint')}
            >
              <Input
                id="g-interests"
                placeholder={t('newGroup.interestsPlaceholder')}
                aria-describedby="g-interests-hint"
                value={form.interests}
                onChange={set('interests')}
              />
            </FormField>
            </>
          )}
          <FriendPicker
            friends={friendList.filter((f) => multi || f.id !== targetFriend)}
            loading={friends.isPending}
            selected={memberIds}
            onChange={setMemberIds}
          />
          <FormField id="g-notes" label={t('newGroup.notes')} optional={t('newGroup.optional')}>
            <Textarea
              id="g-notes"
              rows={2}
              maxLength={MAX_GROUP_DESCRIPTION}
              placeholder={t('newGroup.notesPlaceholder')}
              value={form.notes}
              onChange={set('notes')}
            />
          </FormField>
          <div className="flex flex-col gap-2" role="group" aria-labelledby="g-cover-label">
            <p id="g-cover-label" className="flex items-center gap-2 text-sm leading-none font-medium">
              {t('newGroup.cover')}
              <span className="font-normal text-muted">({t('newGroup.optional')})</span>
            </p>
            <div className="flex items-center gap-3">
              {coverPreview && (
                <img
                  src={coverPreview}
                  alt={t('newGroup.coverPreviewAlt')}
                  className="aspect-[4/3] w-24 shrink-0 rounded-[12px] object-cover"
                />
              )}
              <div className="flex flex-wrap gap-2">
                <Button type="button" variant="outline" onClick={() => setPickingCover(true)} disabled={mutation.isPending}>
                  <ImagePlus />
                  {cover ? t('cover.change') : t('cover.pick')}
                </Button>
                {cover && (
                  <Button type="button" variant="ghost" onClick={() => setCover(null)} disabled={mutation.isPending}>
                    {t('cover.remove')}
                  </Button>
                )}
              </div>
            </div>
            <p className="text-xs text-muted">{t('newGroup.coverHint')}</p>
          </div>
          <Button type="submit" size="lg" className="mt-1 w-full sm:ml-auto sm:w-auto" disabled={mutation.isPending}>
            {mutation.isPending ? t('common.loading') : t('newGroup.submit')}
          </Button>
        </form>
        <CoverCropDialog
          open={pickingCover}
          onOpenChange={setPickingCover}
          onCropped={(image) => {
            setCover(image)
            setPickingCover(false)
          }}
          confirmLabel={t('cover.use')}
          hint={multi ? t('cover.visibleToRecipients') : undefined}
        />
      </DialogContent>
    </Dialog>
  )
}

// "Kimlerle?" — arkadaş çoklu seçici. Seçilenlere grup daveti gider; kabul edene kadar üye
// sayılmazlar. Hedef kişi listede yer almaz.
function FriendPicker({
  friends,
  loading,
  selected,
  onChange,
}: {
  friends: Friend[]
  loading: boolean
  selected: string[]
  onChange: (ids: string[]) => void
}) {
  const { t } = useTranslation()
  const toggle = (id: string) => onChange(selected.includes(id) ? selected.filter((x) => x !== id) : [...selected, id])
  return (
    <fieldset className="flex flex-col gap-2" aria-describedby="g-friends-hint">
      <legend className="mb-2 flex items-center gap-2 text-sm leading-none font-medium">
        {t('newGroup.friends.label')}
        <span className="font-normal text-muted">({t('newGroup.optional')})</span>
      </legend>
      {loading ? null : friends.length === 0 ? (
        <p id="g-friends-hint" className="text-xs text-muted">
          {t('newGroup.friends.none')}{' '}
          <Link to="/friends" className="font-medium text-ink underline underline-offset-4">
            {t('newGroup.friends.addFriends')}
          </Link>
        </p>
      ) : (
        <>
          <div className="flex max-h-[168px] flex-wrap gap-2 overflow-y-auto">
            {friends.map((f) => {
              const on = selected.includes(f.id)
              return (
                <label
                  key={f.id}
                  className={cn(
                    'inline-flex h-10 cursor-pointer items-center gap-2 rounded-full border pr-3 pl-1.5 text-sm transition-colors focus-within:ring-2 focus-within:ring-accent/40',
                    on ? 'border-primary bg-soft text-on-soft' : 'border-control bg-raised hover:bg-surface',
                  )}
                >
                  <input type="checkbox" className="sr-only" checked={on} onChange={() => toggle(f.id)} />
                  <Avatar name={f.name} seed={f.id} userId={f.id} version={f.avatar_version} size="sm" />
                  <span className="max-w-[140px] truncate">{f.name}</span>
                  {on && <Check className="size-4" aria-hidden />}
                </label>
              )
            })}
          </div>
          <p id="g-friends-hint" className="text-xs text-muted">
            {t('newGroup.friends.hint')}
          </p>
        </>
      )}
    </fieldset>
  )
}
