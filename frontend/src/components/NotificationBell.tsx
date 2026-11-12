import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  AlarmClock,
  Bell,
  CalendarDays,
  CheckCheck,
  Lightbulb,
  RotateCcw,
  Sparkles,
  Trophy,
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
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { formatRelativeTime } from '@/lib/format'
import { badgeLabel, notificationText } from '@/lib/notifications'
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
  winner_overdue: AlarmClock,
}

// Üst bardaki zil: okunmamış sayacı, açılır bildirim listesi, "tümünü okundu yap".
// Bildirimler SSE ile anında gelir (useNotificationEvents); akış yalnızca "yeniden çek" der,
// veri her zaman GET /notifications ucundan gelir (Anti-Spoiler süzgeci sunucuda).
export function NotificationBell() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
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

  const open = (n: AppNotification) => {
    if (!n.read_at) markRead.mutate(n.id)
    void navigate(`/groups/${n.group_id}`)
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
      <DropdownMenuContent align="end" collisionPadding={12} className="w-[min(380px,calc(100vw-24px))] p-0">
        <div className="flex min-h-12 items-center justify-between gap-2 py-1.5 pr-1.5 pl-4">
          <DropdownMenuLabel className="p-0 text-sm font-semibold text-ink">{t('notifications.title')}</DropdownMenuLabel>
          {unread > 0 && (
            <DropdownMenuItem
              // Menü açık kalır; liste yerinde güncellenir.
              onSelect={(event) => {
                event.preventDefault()
                markAllRead.mutate()
              }}
              disabled={markAllRead.isPending}
              className="h-9 shrink-0 gap-1.5 px-2.5 text-xs font-medium text-accent [&_svg]:text-accent"
            >
              <CheckCheck aria-hidden="true" />
              {t('notifications.markAllRead')}
            </DropdownMenuItem>
          )}
        </div>
        <DropdownMenuSeparator className="my-0" />
        {items.length === 0 ? (
          <div className="flex flex-col items-center gap-1 px-6 py-10 text-center">
            <span className="mb-2 inline-flex size-11 items-center justify-center rounded-full bg-surface text-muted">
              <Bell className="size-5" aria-hidden="true" />
            </span>
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
        ) : (
          <div className="flex max-h-[min(60vh,440px)] flex-col gap-0.5 overflow-y-auto overscroll-contain p-1">
            {items.map((n) => {
              const Icon = ICONS[n.type] ?? Bell
              const isUnread = !n.read_at
              return (
                <DropdownMenuItem
                  key={n.id}
                  onSelect={() => open(n)}
                  className={cn('h-auto shrink-0 items-start gap-3 py-2.5 pr-2.5 pl-2.5', isUnread && 'bg-soft/60 data-[highlighted]:bg-soft')}
                >
                  <span
                    className={cn(
                      'mt-0.5 inline-flex size-8 shrink-0 items-center justify-center rounded-full',
                      isUnread ? 'bg-soft text-on-soft [&_svg]:!text-on-soft' : 'bg-surface',
                    )}
                  >
                    <Icon aria-hidden="true" />
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className={cn('block text-sm leading-5 [overflow-wrap:anywhere]', isUnread ? 'font-medium text-ink' : 'text-muted')}>
                      {notificationText(n)}
                    </span>
                    <span className="mt-0.5 block truncate text-xs leading-4 text-muted">
                      {n.group_title} · {formatRelativeTime(n.created_at)}
                    </span>
                  </span>
                  {isUnread && (
                    <span className="mt-2 inline-flex shrink-0 items-center">
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
