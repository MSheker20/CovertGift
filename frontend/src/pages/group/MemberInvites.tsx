import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, UserPlus, X } from 'lucide-react'
import { toast } from 'sonner'

import { friendsApi, groupInvitesApi, queryKeys } from '@/api/endpoints'
import type { GroupDetail } from '@/api/types'
import { Avatar } from '@/components/Avatar'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { errorMessage } from '@/lib/errors'
import { cn } from '@/lib/utils'

// Üyeler panelinde bekleyen davetler ("bekliyor") ve yönetici için "Arkadaş davet et".
// Davetliler kabul edene kadar üye sayılmaz; grup SSE'si listeyi yeniler.
export function MemberInvites({ groupId, d }: { groupId: string; d: GroupDetail }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isAdmin = d.my_role === 'admin'
  const [inviting, setInviting] = useState(false)
  const pending = useQuery({
    queryKey: queryKeys.memberInvites(groupId),
    queryFn: ({ signal }) => groupInvitesApi.forGroup(groupId, signal),
  })
  const cancel = useMutation({
    mutationFn: (userId: string) => groupInvitesApi.cancel(groupId, userId),
    onError: (err) => toast.error(errorMessage(err)),
    onSettled: () => queryClient.invalidateQueries({ queryKey: queryKeys.memberInvites(groupId) }),
  })
  const rows = pending.data ?? []

  return (
    <div className="mt-3">
      {rows.length > 0 && (
        <ul className="flex flex-col gap-1" aria-label={t('memberInvites.pendingList')}>
          {rows.map((r) => (
            <li key={r.user_id} className="flex min-h-11 items-center gap-3 opacity-80">
              <Avatar name={r.name} seed={r.user_id} userId={r.user_id} version={r.avatar_version} size="sm" />
              <span className="min-w-0 flex-1 truncate text-sm">{r.name}</span>
              <span className="rounded-full bg-pill-neutral-bg px-2 py-0.5 text-xs font-medium whitespace-nowrap text-pill-neutral-ink">
                {t('memberInvites.pending')}
              </span>
              {isAdmin && (
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={t('memberInvites.cancel', { name: r.name })}
                  onClick={() => cancel.mutate(r.user_id)}
                  disabled={cancel.isPending}
                >
                  <X />
                </Button>
              )}
            </li>
          ))}
        </ul>
      )}
      {isAdmin && (
        <Button variant="outline" className="mt-3 w-full" onClick={() => setInviting(true)}>
          <UserPlus />
          {t('memberInvites.inviteFriends')}
        </Button>
      )}
      {inviting && (
        <InviteFriendsDialog
          groupId={groupId}
          exclude={new Set([...(d.members ?? []).map((m) => m.user_id), ...rows.map((r) => r.user_id)])}
          targetId={d.target_account?.id}
          onOpenChange={(o) => !o && setInviting(false)}
        />
      )}
    </div>
  )
}

function InviteFriendsDialog({
  groupId,
  exclude,
  targetId,
  onOpenChange,
}: {
  groupId: string
  exclude: Set<string>
  targetId?: string
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const friends = useQuery({ queryKey: queryKeys.friends, queryFn: ({ signal }) => friendsApi.list(signal) })
  // Hedef kişi ve zaten üye ya da davetli olanlar listelenmez.
  const candidates = (friends.data?.friends ?? []).filter((f) => !exclude.has(f.id) && f.id !== targetId)
  const [selected, setSelected] = useState<string[]>([])
  const toggle = (id: string) => setSelected((s) => (s.includes(id) ? s.filter((x) => x !== id) : [...s, id]))

  const invite = useMutation({
    mutationFn: () => groupInvitesApi.invite(groupId, selected),
    onSuccess: (res) => {
      toast.success(t('memberInvites.sent', { count: res.invited.length }))
      void queryClient.invalidateQueries({ queryKey: queryKeys.memberInvites(groupId) })
      onOpenChange(false)
    },
  })

  return (
    <Dialog open onOpenChange={(o) => !invite.isPending && onOpenChange(o)}>
      <DialogContent closeLabel={t('common.close')} className="sm:max-w-[460px]">
        <DialogHeader>
          <DialogTitle className="text-[19px]">{t('memberInvites.dialogTitle')}</DialogTitle>
          <DialogDescription className="text-[15px] leading-6">{t('memberInvites.dialogLead')}</DialogDescription>
        </DialogHeader>
        {friends.isPending ? null : candidates.length === 0 ? (
          <p className="text-sm text-muted">
            {t('memberInvites.noCandidates')}{' '}
            <Link to="/friends" className="font-medium text-ink underline underline-offset-4">
              {t('newGroup.friends.addFriends')}
            </Link>
          </p>
        ) : (
          <fieldset>
            <legend className="sr-only">{t('memberInvites.dialogTitle')}</legend>
            <div className="flex max-h-[240px] flex-wrap gap-2 overflow-y-auto">
              {candidates.map((f) => {
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
          </fieldset>
        )}
        {invite.isError && (
          <p role="alert" className="text-sm text-negative">
            {errorMessage(invite.error)}
          </p>
        )}
        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={invite.isPending}>
            {t('manage.cancel')}
          </Button>
          <Button onClick={() => invite.mutate()} disabled={invite.isPending || selected.length === 0}>
            {invite.isPending ? t('common.loading') : t('memberInvites.send', { count: selected.length })}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
