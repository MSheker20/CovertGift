import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, ChevronRight, Link2, Users } from 'lucide-react'
import { toast } from 'sonner'

import { friendsApi, queryKeys } from '@/api/endpoints'
import { Avatar } from '@/components/Avatar'
import { AddByEmail, IncomingRow } from '@/components/FriendParts'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { friendLinkUrl } from '@/lib/friends'
import { badgeLabel } from '@/lib/notifications'

// Üst bardaki Arkadaşlar popup'ı: hızlı işler burada (gelen istekleri kabul/ret, e-postayla ekle,
// linki kopyala); liste yönetimi (giden istekler, çıkarma, linki sıfırlama) /friends sayfasında.
// Arkadaşlık istekleri zile düşmez; rozet gelen istekleri sayar ve bildirim akışıyla yenilenir.
export function FriendsPopover() {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const friends = useQuery({
    queryKey: queryKeys.friends,
    queryFn: ({ signal }) => friendsApi.list(signal),
    refetchInterval: 60_000,
  })
  const incoming = friends.data?.incoming ?? []
  const list = friends.data?.friends ?? []

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          aria-label={incoming.length > 0 ? t('nav.friendsWithRequests', { count: incoming.length }) : t('nav.friends')}
          className="relative inline-flex size-10 shrink-0 cursor-pointer items-center justify-center rounded-full transition-colors hover:bg-surface data-[state=open]:bg-surface md:size-11"
        >
          <Users className="size-5" aria-hidden="true" />
          {incoming.length > 0 && (
            <span
              aria-hidden="true"
              className="num absolute top-0.5 right-0.5 inline-flex h-[18px] min-w-[18px] items-center justify-center rounded-full bg-primary px-1 text-[11px] leading-none font-semibold text-on-primary ring-2 ring-raised md:top-1 md:right-1"
            >
              {badgeLabel(incoming.length)}
            </span>
          )}
        </button>
      </PopoverTrigger>
      <PopoverContent
        collisionPadding={12}
        aria-label={t('friends.title')}
        className="flex max-h-[min(560px,calc(100dvh-96px))] w-[min(380px,calc(100vw-24px))] flex-col overflow-hidden"
      >
        <div className="flex min-h-9 items-center justify-between gap-2 px-4 pt-3.5">
          <h2 className="font-display text-[15px] font-semibold">{t('friends.title')}</h2>
          <CopyLinkButton />
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto px-4 pb-3">
          {incoming.length > 0 && (
            <section aria-labelledby="fp-incoming" className="mt-3">
              <h3 id="fp-incoming" className="text-xs font-medium text-muted">
                {t('friends.incoming')}
              </h3>
              <ul className="mt-1 flex flex-col gap-1">
                {incoming.map((f) => (
                  <IncomingRow key={f.friendship_id} f={f} />
                ))}
              </ul>
            </section>
          )}

          <div className="mt-3">
            <AddByEmail compact />
          </div>

          {list.length > 0 && (
            <div className="mt-4 flex items-center gap-2">
              <div className="flex -space-x-2">
                {list.slice(0, 5).map((f) => (
                  <Avatar key={f.id} name={f.name} seed={f.id} userId={f.id} version={f.avatar_version} size="sm" className="ring-2 ring-raised" />
                ))}
              </div>
              <span className="text-sm text-muted">{t('friends.popover.count', { count: list.length })}</span>
            </div>
          )}
        </div>

        <div className="border-t border-border p-1">
          <Button asChild variant="ghost" className="w-full justify-between">
            <Link to="/friends" onClick={() => setOpen(false)}>
              {t('friends.popover.manage')}
              <ChevronRight className="text-muted" />
            </Link>
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}

// Linki ilk tıklamada çeker (popup her açıldığında istek atılmaz) ve kopyalar.
function CopyLinkButton() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [state, setState] = useState<'idle' | 'busy' | 'ok'>('idle')
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  useEffect(() => () => clearTimeout(timer.current), [])

  const copy = async () => {
    setState('busy')
    try {
      const link = await queryClient.fetchQuery({ queryKey: queryKeys.friendLink, queryFn: ({ signal }) => friendsApi.link(signal) })
      await navigator.clipboard.writeText(friendLinkUrl(link.code))
      setState('ok')
      clearTimeout(timer.current)
      timer.current = setTimeout(() => setState('idle'), 2000)
    } catch {
      setState('idle')
      toast.error(t('friends.link.copyFailed'))
    }
  }

  return (
    <>
      <Button size="sm" variant="outline" onClick={() => void copy()} disabled={state === 'busy'}>
        {state === 'ok' ? <Check className="text-positive" /> : <Link2 />}
        {state === 'ok' ? t('friends.link.copied') : t('friends.popover.copyLink')}
      </Button>
      <span role="status" className="sr-only">
        {state === 'ok' ? t('friends.link.copied') : ''}
      </span>
    </>
  )
}
