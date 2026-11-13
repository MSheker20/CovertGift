import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Gift, MoreHorizontal, Trash2, UserCog } from 'lucide-react'
import { toast } from 'sonner'

import { ApiError } from '@/api/client'
import { queryKeys, recipientsApi } from '@/api/endpoints'
import type { GroupDetail, GroupMember } from '@/api/types'
import { Avatar } from '@/components/Avatar'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { FormError, FormField } from '@/components/FormField'
import { Panel } from '@/components/Panel'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Select } from '@/components/ui/select'
import { errorMessage } from '@/lib/errors'
import { AddMemberForm } from './AddMemberForm'
import { InviteLink } from './InviteLink'

// Üyeler paneli. SINGLE'da eskisiyle aynıdır. MULTI'de hediye alacak kişiler rozetle işaretlenir;
// yönetici her üyede "hediye alınacak kişi yap" gibi işlemleri görür.
export function MembersPanel({
  groupId,
  d,
  meId,
  onPoolCreated,
}: {
  groupId: string
  d?: GroupDetail
  meId?: string
  // Yeni havuz kurulunca (kendi havuzum değilse) sekmesini seçmek için.
  onPoolCreated?: (poolId: string) => void
}) {
  const { t } = useTranslation()
  const multi = d?.group.mode === 'MULTI'
  const isAdmin = d?.my_role === 'admin'
  const recipientIds = new Set((d?.pools ?? []).map((p) => p.recipient.id))
  return (
    <Panel title={t('group.membersPanel')} count={d?.member_count} className="md:col-span-4">
      <ul className="flex flex-col gap-1">
        {(d?.members ?? []).map((m) => (
          <li key={m.id} className="flex min-h-11 items-center gap-3">
            <Avatar name={m.user?.name ?? '?'} seed={m.user_id} size="sm" />
            {multi ? (
              // MULTI'de rozetler adın altına iner: dar ekranda ad "D…" diye kısalmasın.
              <span className="min-w-0 flex-1 py-1">
                <span className="block truncate text-sm">
                  {m.user?.name ?? '—'}
                  {m.user_id === meId && <span className="text-muted"> ({t('group.you')})</span>}
                </span>
                {(recipientIds.has(m.user_id) || m.role === 'admin') && (
                  <span className="mt-1 flex flex-wrap gap-1">
                    {recipientIds.has(m.user_id) && (
                      <span className="inline-flex items-center gap-1 rounded-full bg-field px-2 py-0.5 text-xs font-medium whitespace-nowrap text-on-field">
                        <Gift className="size-3" aria-hidden />
                        {t('pools.recipientBadge')}
                      </span>
                    )}
                    {m.role === 'admin' && (
                      <span className="rounded-full bg-soft px-2 py-0.5 text-xs font-medium whitespace-nowrap text-on-soft">
                        {t('home.admin')}
                      </span>
                    )}
                  </span>
                )}
              </span>
            ) : (
              <>
                <span className="min-w-0 flex-1 truncate text-sm">
                  {m.user?.name ?? '—'}
                  {m.user_id === meId && <span className="text-muted"> ({t('group.you')})</span>}
                </span>
                {m.role === 'admin' && (
                  <span className="rounded-full bg-soft px-2 py-0.5 text-xs font-medium whitespace-nowrap text-on-soft">
                    {t('home.admin')}
                  </span>
                )}
              </>
            )}
            {multi && isAdmin && d && (
              <MemberRecipientMenu
                groupId={groupId}
                member={m}
                detail={d}
                meId={meId}
                isRecipient={recipientIds.has(m.user_id)}
                onPoolCreated={onPoolCreated}
              />
            )}
          </li>
        ))}
      </ul>
      {d && <AddMemberForm groupId={groupId} multi={multi} />}
      {/* MULTI'de hedef kişi yoktur: "hedef bağlı değil" uyarısı gösterilmez. */}
      {isAdmin && d && (
        <InviteLink key={groupId} groupId={groupId} targetLinked={multi || d.target_account !== undefined} multi={multi} />
      )}
    </Panel>
  )
}

function MemberRecipientMenu({
  groupId,
  member,
  detail,
  meId,
  isRecipient,
  onPoolCreated,
}: {
  groupId: string
  member: GroupMember
  detail: GroupDetail
  meId?: string
  isRecipient: boolean
  onPoolCreated?: (poolId: string) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const name = member.user?.name ?? '—'
  const self = member.user_id === meId
  const [dialog, setDialog] = useState<'make' | 'manager' | null>(null)
  // 'ideas': sunucu havuzda fikir olduğunu söyledi. 'own': yönetici kendi havuzunu kaldırıyor; fikir
  // olup olmadığı asla belli edilmez, her zaman aynı genel onay istenir.
  const [confirming, setConfirming] = useState<'ideas' | 'own' | null>(null)

  const refresh = () => queryClient.invalidateQueries({ queryKey: queryKeys.groups })

  const remove = useMutation({
    mutationFn: ({ confirm }: { confirm: boolean }) => recipientsApi.remove(groupId, member.user_id, confirm),
    onSuccess: () => {
      toast.success(t('pools.removed', { name }))
      setConfirming(null)
      void refresh()
    },
    onError: (err, vars) => {
      if (err instanceof ApiError && err.status === 409 && !vars.confirm) {
        setConfirming('ideas')
        return
      }
      setConfirming(null)
      toast.error(errorMessage(err))
    },
  })

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon" aria-label={t('pools.memberActions', { name })} disabled={remove.isPending}>
            <MoreHorizontal />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-[min(288px,calc(100vw-24px))]">
          {!isRecipient && (
            <DropdownMenuItem onSelect={() => setDialog('make')}>
              <Gift />
              {t('pools.makeRecipient')}
            </DropdownMenuItem>
          )}
          {isRecipient && self && (
            <DropdownMenuItem onSelect={() => setDialog('manager')}>
              <UserCog />
              {t('pools.changeManager')}
            </DropdownMenuItem>
          )}
          {isRecipient && (
            <DropdownMenuItem
              variant="destructive"
              onSelect={() => (self ? setConfirming('own') : remove.mutate({ confirm: false }))}
            >
              <Trash2 />
              {t('pools.removeRecipient')}
            </DropdownMenuItem>
          )}
        </DropdownMenuContent>
      </DropdownMenu>

      {dialog === 'make' && (
        <RecipientDialog
          mode="make"
          groupId={groupId}
          member={member}
          detail={detail}
          meId={meId}
          onPoolCreated={onPoolCreated}
          onOpenChange={(o) => !o && setDialog(null)}
        />
      )}
      {dialog === 'manager' && (
        <RecipientDialog
          mode="manager"
          groupId={groupId}
          member={member}
          detail={detail}
          meId={meId}
          onPoolCreated={onPoolCreated}
          onOpenChange={(o) => !o && setDialog(null)}
        />
      )}
      <ConfirmDialog
        open={confirming !== null}
        onOpenChange={(o) => !o && !remove.isPending && setConfirming(null)}
        title={confirming === 'own' ? t('pools.removeOwnTitle') : t('pools.removeConfirmTitle', { name })}
        body={confirming === 'own' ? t('pools.removeOwnLead') : t('pools.removeConfirmLead', { name })}
        confirmLabel={t('pools.removeConfirm')}
        cancelLabel={t('manage.cancel')}
        pending={remove.isPending}
        onConfirm={() => remove.mutate({ confirm: true })}
      />
    </>
  )
}

// "Hediye alınacak kişi yap" ve "Sorumluyu değiştir" pencereleri. Yönetici kendini alıcı yaparken
// ya da sorumluyu değiştirirken havuzu yönetecek başka bir üye seçmek zorundadır; seçilmeden
// istek gönderilmez.
function RecipientDialog({
  mode,
  groupId,
  member,
  detail,
  meId,
  onPoolCreated,
  onOpenChange,
}: {
  mode: 'make' | 'manager'
  groupId: string
  member: GroupMember
  detail: GroupDetail
  meId?: string
  onPoolCreated?: (poolId: string) => void
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const name = member.user?.name ?? '—'
  const self = member.user_id === meId
  const needsManager = mode === 'manager' || self
  const candidates = (detail.members ?? []).filter((m) => m.user_id !== member.user_id)
  const [manager, setManager] = useState('')
  const [error, setError] = useState<string | null>(null)

  const save = useMutation({
    mutationFn: async () => {
      if (mode === 'make') return (await recipientsApi.add(groupId, member.user_id, needsManager ? manager : undefined)).pool
      await recipientsApi.setManager(groupId, member.user_id, manager)
      return null
    },
    onSuccess: (pool) => {
      // Kendi havuzum kilitlidir; yalnızca başkasının yeni havuzunun sekmesi seçilir.
      if (pool && !pool.locked) onPoolCreated?.(pool.id)
      toast.success(mode === 'make' ? t('pools.recipientAdded', { name }) : t('pools.managerChanged'))
      onOpenChange(false)
      void queryClient.invalidateQueries({ queryKey: queryKeys.groups })
    },
    onError: (err) => setError(errorMessage(err)),
  })

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    if (needsManager && !manager) {
      setError(t('pools.managerRequired'))
      return
    }
    setError(null)
    save.mutate()
  }

  const title =
    mode === 'manager' ? t('pools.changeManagerTitle') : t('pools.makeRecipientTitle', { name: self ? t('pools.you') : name })
  const body = mode === 'manager' || self ? t('pools.makeSelfBody') : t('pools.makeBody', { name })

  return (
    <Dialog open onOpenChange={(o) => !save.isPending && onOpenChange(o)}>
      <DialogContent closeLabel={t('common.close')} className="sm:max-w-[440px]">
        <DialogHeader>
          <DialogTitle className="text-[19px]">{title}</DialogTitle>
          <DialogDescription className="text-[15px] leading-6">{body}</DialogDescription>
        </DialogHeader>
        <form onSubmit={onSubmit} className="flex flex-col gap-4" noValidate>
          <FormError message={error && !(needsManager && !manager) ? error : null} />
          {needsManager && (
            <FormField id="pool-manager" label={t('pools.chooseManager')} hint={t('pools.chooseManagerHint')}>
              <Select
                id="pool-manager"
                autoFocus
                required
                aria-required
                aria-invalid={error && !manager ? true : undefined}
                aria-describedby={error && !manager ? 'pool-manager-error pool-manager-hint' : 'pool-manager-hint'}
                value={manager}
                onChange={(e) => {
                  setManager(e.target.value)
                  setError(null)
                }}
              >
                <option value="">{t('pools.chooseManagerPlaceholder')}</option>
                {candidates.map((m) => (
                  <option key={m.user_id} value={m.user_id}>
                    {m.user?.name ?? '—'}
                  </option>
                ))}
              </Select>
              {error && !manager && (
                <p id="pool-manager-error" role="alert" className="text-xs text-negative">
                  {error}
                </p>
              )}
            </FormField>
          )}
          <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={save.isPending}>
              {t('manage.cancel')}
            </Button>
            <Button type="submit" disabled={save.isPending}>
              {save.isPending ? t('common.loading') : mode === 'make' ? t('pools.makeRecipient') : t('manage.save')}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
