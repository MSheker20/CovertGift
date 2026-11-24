import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation, useNavigate } from 'react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { MailPlus } from 'lucide-react'
import { toast } from 'sonner'

import { groupInvitesApi, queryKeys } from '@/api/endpoints'
import type { MyGroupInvite } from '@/api/types'
import { Button } from '@/components/ui/button'
import { errorMessage } from '@/lib/errors'

// Ana sayfanın üstünde gelen grup davetleri. Davet yoksa hiçbir şey gösterilmez. Zil bildirimi
// buraya (#invites) getirir. Hedefi olduğum grupların davetleri sunucudan hiç gelmez.
export function GroupInvites() {
  const { t } = useTranslation()
  const location = useLocation()
  const invites = useQuery({ queryKey: queryKeys.myGroupInvites, queryFn: ({ signal }) => groupInvitesApi.mine(signal) })
  const rows = invites.data ?? []

  useEffect(() => {
    if (location.hash === '#invites' && rows.length > 0) {
      document.getElementById('invites')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
  }, [location.hash, rows.length])

  if (rows.length === 0) return null
  return (
    <section id="invites" aria-labelledby="invites-title" className="scroll-mt-24 rounded-[24px] bg-soft p-5 text-on-soft md:col-span-12 md:p-6">
      <h2 id="invites-title" className="flex items-center gap-2 text-[17px] font-semibold">
        <MailPlus className="size-5" aria-hidden />
        {t('groupInvites.title', { count: rows.length })}
      </h2>
      <ul className="mt-4 flex flex-col gap-3">
        {rows.map((r) => (
          <InviteRow key={r.id} r={r} />
        ))}
      </ul>
    </section>
  )
}

function InviteRow({ r }: { r: MyGroupInvite }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: queryKeys.myGroupInvites })
    void queryClient.invalidateQueries({ queryKey: queryKeys.notifications })
  }
  const accept = useMutation({
    mutationFn: () => groupInvitesApi.accept(r.id),
    onSuccess: async ({ group_id }) => {
      toast.success(t('groupInvites.joined', { title: r.group_title }))
      await queryClient.invalidateQueries({ queryKey: queryKeys.groups })
      refresh()
      navigate(`/groups/${group_id}`)
    },
    // Hedef kişi davetleri hiç görmez (sunucu süzer); kabul yine de reddedilirse liste yenilenir.
    onError: (err) => {
      toast.error(errorMessage(err))
      refresh()
    },
  })
  const decline = useMutation({
    mutationFn: () => groupInvitesApi.decline(r.id),
    onSuccess: () => toast.success(t('groupInvites.declined')),
    onError: (err) => toast.error(errorMessage(err)),
    onSettled: refresh,
  })
  const busy = accept.isPending || decline.isPending
  return (
    <li className="flex flex-col gap-3 rounded-[16px] bg-raised p-4 text-ink sm:flex-row sm:items-center">
      <div className="min-w-0 flex-1">
        <p className="truncate font-medium">{r.group_title}</p>
        <p className="text-sm text-muted">
          {t('groupInvites.from', { name: r.inviter_name })} · {t('home.members', { count: r.member_count })}
        </p>
      </div>
      <div className="flex gap-2">
        <Button onClick={() => accept.mutate()} disabled={busy}>
          {accept.isPending ? t('common.loading') : t('groupInvites.accept')}
        </Button>
        <Button variant="outline" onClick={() => decline.mutate()} disabled={busy}>
          {t('groupInvites.decline')}
        </Button>
      </div>
    </li>
  )
}
