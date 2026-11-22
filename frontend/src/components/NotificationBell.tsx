import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  AlarmClock,
  AlarmClockOff,
  Bell,
  CalendarDays,
  CheckCheck,
  Gift,
  Lightbulb,
  RotateCcw,
  Sparkles,
  Trophy,
  UserCog,
  UserPlus,
  Wallet,
  type LucideIcon,
} from 'lucide-react'

import { NOTIFICATIONS_FALLBACK_MS, NOTIFICATIONS_REFRESH_MS, notificationsApi, queryKeys } from '@/api/endpoints'
import type { AppNotification, NotificationList } from '@/api/types'
import { useNotificationEvents } from '@/api/useNotificationEvents'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { EmptyArt } from '@/components/EmptyArt'
import { formatRelativeTime } from '@/lib/format'
import {
  badgeLabel,
  notificationPath,
  notificationText,
  notificationTone,
  type NotificationTone,
} from '@/lib/notifications'
import { cn } from '@/lib/utils'

const ICONS: Record<string, LucideIcon> = {
  idea_added: Lightbulb,
  ai_ideas_added: Sparkles,
  added_to_group: UserPlus,
  member_joined: UserPlus,
  winner_selected: Trophy,
  group_reopened: RotateCcw,
  event_date_changed: CalendarDays,
  budget_changed: Wallet,
  event_reminder: AlarmClock,
  winner_overdue: AlarmClockOff,
  pool_added: Gift,
  pool_manager_assigned: UserCog,
}

// İkon kutusunun rengi türden gelir (src/lib/notifications.ts). Renkler yalnızca token.
const TONES: Record<NotificationTone, string> = {
  idea: 'bg-soft text-on-soft',
  money: 'bg-field text-on-field',
  winner: 'bg-pill-warn-bg text-pill-warn-ink',
  reminder: 'bg-pill-high-bg text-pill-high-ink',
  overdue: 'bg-pill-negative-bg text-pill-negative-ink',
  member: 'bg-pill-positive-bg text-pill-positive-ink',
  neutral: 'bg-pill-neutral-bg text-pill-neutral-ink',
}

type Filter = 'all' | 'unread'

// Metin işlemi yapanın adıyla başlıyorsa ad kalın yazılır; metnin tamamı yine tek parça okunur.
function Message({ n }: { n: AppNotification }) {
  const text = notificationText(n)
  const actor = n.params?.actor_name
  if (!actor || !text.startsWith(actor)) return text
  return (
    <>
      <strong className="font-semibold text-ink">{actor}</strong>
      {text.slice(actor.length)}
    </>
  )
}

// Üst bardaki zil: okunmamış sayacı, açılır bildirim listesi, Tümü/Okunmamış süzgeci,
// "tümünü okundu yap". Bildirimler SSE ile anında gelir (useNotificationEvents); akış yalnızca
// "yeniden çek" der, veri her zaman GET /notifications ucundan gelir (Anti-Spoiler süzgeci sunucuda).
export function NotificationBell() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [filter, setFilter] = useState<Filter>('all')
  const live = useNotificationEvents(true)
  const list = useQuery({
    queryKey: queryKeys.notifications,
    queryFn: ({ signal }) => notificationsApi.list(signal),
    refetchInterval: live ? NOTIFICATIONS_FALLBACK_MS : NOTIFICATIONS_REFRESH_MS,
  })

  // İyimser güncelleme: rozet ve vurgu tıklanır tıklanmaz değişir; sonuç ne olursa olsun
  // liste sunucudan yeniden çekilir.
  const patch = (change: (current: NotificationList) => NotificationList) => {
    queryClient.setQueryData<NotificationList>(queryKeys.notifications, (current) => (current ? change(current) : current))
  }
  const settle = () => void queryClient.invalidateQueries({ queryKey: queryKeys.notifications })
  const markRead = useMutation({
    mutationFn: (id: string) => notificationsApi.markRead(id),
    onMutate: (id) => {
      const readAt = new Date().toISOString()
      patch((current) => {
        const wasUnread = current.items.some((n) => n.id === id && !n.read_at)
        return {
          items: current.items.map((n) => (n.id === id && !n.read_at ? { ...n, read_at: readAt } : n)),
          unread_count: Math.max(0, current.unread_count - (wasUnread ? 1 : 0)),
        }
      })
    },
    onSettled: settle,
  })
  const markAllRead = useMutation({
    mutationFn: () => notificationsApi.markAllRead(),
    onMutate: () => {
      const readAt = new Date().toISOString()
      patch((current) => ({
        items: current.items.map((n) => (n.read_at ? n : { ...n, read_at: readAt })),
        unread_count: 0,
      }))
    },
    onSettled: settle,
  })

  const items = list.data?.items ?? []
  const unread = list.data?.unread_count ?? 0
  const visible = filter === 'unread' ? items.filter((n) => !n.read_at) : items

  const open = (n: AppNotification) => {
    if (!n.read_at) markRead.mutate(n.id)
    void navigate(notificationPath(n))
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={unread > 0 ? t('notifications.openUnread', { count: unread }) : t('notifications.open')}
          className="relative inline-flex size-10 shrink-0 cursor-pointer items-center justify-center rounded-full transition-colors hover:bg-surface data-[state=open]:bg-surface md:size-11"
        >
          <Bell className="size-5" aria-hidden="true" />
          {unread > 0 && (
            <span
              aria-hidden="true"
              className="num absolute top-0.5 right-0.5 inline-flex h-[18px] min-w-[18px] items-center justify-center rounded-full bg-primary px-1 text-[11px] leading-none font-semibold text-on-primary ring-2 ring-raised md:top-1 md:right-1"
            >
              {badgeLabel(unread)}
            </span>
          )}
        </button>
      </DropdownMenuTrigger>
      {/* Sayaç değişince ekran okuyucuya duyurulur (zil düğmesinin adı da sayıyı içerir). */}
      <span className="sr-only" role="status" aria-live="polite">
        {unread > 0 ? t('notifications.openUnread', { count: unread }) : ''}
      </span>
      <DropdownMenuContent align="end" collisionPadding={12} className="w-[min(400px,calc(100vw-24px))] overflow-hidden p-0">
        <div className="px-4 pt-3.5 pb-3">
          <div className="flex min-h-9 items-center justify-between gap-2">
            <DropdownMenuLabel className="font-display p-0 text-[15px] font-semibold text-ink">
              {t('notifications.title')}
            </DropdownMenuLabel>
            {unread > 0 && (
              <DropdownMenuItem
                // Menü açık kalır; liste yerinde güncellenir.
                onSelect={(event) => {
                  event.preventDefault()
                  markAllRead.mutate()
                }}
                disabled={markAllRead.isPending}
                className="-mr-2 h-9 shrink-0 gap-1.5 px-2.5 text-xs font-medium text-accent [&_svg]:text-accent"
              >
                <CheckCheck aria-hidden="true" />
                {t('notifications.markAllRead')}
              </DropdownMenuItem>
            )}
          </div>
          {items.length > 0 && (
            <DropdownMenuRadioGroup
              aria-label={t('notifications.filterLabel')}
              value={filter}
              onValueChange={(value) => setFilter(value as Filter)}
              className="mt-2.5 grid grid-cols-2 gap-1 rounded-[12px] bg-surface p-1"
            >
              {(['all', 'unread'] as const).map((value) => (
                <DropdownMenuRadioItem
                  key={value}
                  value={value}
                  // Süzgeç seçilince menü kapanmaz.
                  onSelect={(event) => event.preventDefault()}
                  className="h-8 justify-center gap-1.5 rounded-[9px] text-[13px] font-medium text-muted transition-colors data-[highlighted]:text-ink data-[state=checked]:bg-raised data-[state=checked]:text-ink dark:data-[state=checked]:bg-active data-[state=checked]:shadow-[0_1px_2px_rgba(0,0,0,0.08)]"
                >
                  <span>{value === 'all' ? t('notifications.filterAll') : t('notifications.filterUnread')}</span>
                  {value === 'unread' && unread > 0 && (
                    <span
                      aria-hidden="true"
                      className="num inline-flex h-[18px] min-w-[18px] items-center justify-center rounded-full bg-primary px-1 text-[11px] leading-none font-semibold text-on-primary"
                    >
                      {badgeLabel(unread)}
                    </span>
                  )}
                </DropdownMenuRadioItem>
              ))}
            </DropdownMenuRadioGroup>
          )}
        </div>
        <DropdownMenuSeparator className="my-0" />
        {items.length === 0 ? (
          <div className="flex flex-col items-center gap-1 px-6 py-12 text-center">
            <EmptyArt kind="bell" className="mb-2" />
            {list.isError ? (
              <p className="text-sm text-muted">{t('notifications.loadError')}</p>
            ) : list.isPending ? (
              <p className="text-sm text-muted">{t('common.loading')}…</p>
            ) : (
              <>
                <p className="text-sm font-medium">{t('notifications.empty')}</p>
                <p className="text-xs text-muted">{t('notifications.emptyHint')}</p>
              </>
            )}
          </div>
        ) : visible.length === 0 ? (
          <div className="flex flex-col items-center gap-1 px-6 py-12 text-center">
            <span className="mb-3 inline-flex size-12 items-center justify-center rounded-[14px] bg-pill-positive-bg text-pill-positive-ink">
              <CheckCheck className="size-5" aria-hidden="true" />
            </span>
            <p className="text-sm font-medium">{t('notifications.allCaughtUp')}</p>
            <p className="text-xs text-muted">{t('notifications.allCaughtUpHint')}</p>
          </div>
        ) : (
          <div className="flex max-h-[min(60vh,460px)] flex-col gap-0.5 overflow-y-auto overscroll-contain p-1.5">
            {visible.map((n) => {
              const Icon = ICONS[n.type] ?? Bell
              const isUnread = !n.read_at
              return (
                <DropdownMenuItem
                  key={n.id}
                  onSelect={() => open(n)}
                  className="relative h-auto shrink-0 items-start gap-3 py-2.5 pr-3 pl-3.5"
                >
                  {isUnread && (
                    <span aria-hidden="true" className="absolute top-2.5 bottom-2.5 left-1 w-[3px] rounded-full bg-primary" />
                  )}
                  <span
                    className={cn(
                      'inline-flex size-9 shrink-0 items-center justify-center rounded-[11px] [&_svg]:!size-[18px] [&_svg]:!text-current',
                      TONES[notificationTone(n.type)],
                      !isUnread && 'opacity-60',
                    )}
                  >
                    <Icon aria-hidden="true" />
                  </span>
                  <span className="min-w-0 flex-1">
                    <span
                      className={cn('block text-sm leading-5 [overflow-wrap:anywhere]', isUnread ? 'text-ink' : 'text-muted')}
                    >
                      <Message n={n} />
                    </span>
                    <span className="mt-1 flex min-w-0 items-center gap-1.5 text-xs leading-4 text-muted">
                      <span className="truncate">{n.group_title}</span>
                      <span aria-hidden="true">·</span>
                      <time dateTime={n.created_at} className="shrink-0">
                        {formatRelativeTime(n.created_at)}
                      </time>
                    </span>
                  </span>
                  {isUnread && (
                    <span className="mt-1.5 inline-flex shrink-0 items-center">
                      <span className="size-2 rounded-full bg-primary" aria-hidden="true" />
                      <span className="sr-only">{t('notifications.unread')}</span>
                    </span>
                  )}
                </DropdownMenuItem>
              )
            })}
          </div>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
