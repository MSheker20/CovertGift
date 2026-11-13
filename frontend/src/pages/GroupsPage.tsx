import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { useQuery, type UseQueryResult } from '@tanstack/react-query'
import { CalendarDays, CalendarPlus, CheckCircle2, ChevronRight, Plus } from 'lucide-react'

import { groupsApi, queryKeys } from '@/api/endpoints'
import type { GroupListItem } from '@/api/types'
import { useAuth } from '@/auth/AuthProvider'
import { Avatar } from '@/components/Avatar'
import { GroupCoverImage, GroupThumb } from '@/components/GroupCover'
import { Bar, BlockError, Panel } from '@/components/Panel'
import { Button } from '@/components/ui/button'
import { errorMessage } from '@/lib/errors'
import { cn } from '@/lib/utils'
import { countdownLabel, daysUntil, formatEventDate, sortByUpcoming } from '@/lib/eventDate'
import { formatDate } from '@/lib/format'
import { nextSurprise, prioritizedTasks, type PriorityLevel, type PriorityTask } from '@/lib/priority'
import { NewGroupDialog } from './NewGroupDialog'

// Ekranın sorusu: "Sıradaki sürpriz ne ve benden ne bekleniyor?"
// Izgara: 1100px sütun, 12 × 77px + 11 × 16px = 1100. Üstte 8 + 4 (sıradaki sürpriz, bekleyenler),
// altta 8 + 4 (açık gruplar, geçmiş sürprizler). Mobilde tek sütun, aynı sırayla.
export function GroupsPage() {
  const { t } = useTranslation()
  const { user } = useAuth()
  // Oylar ve fikirler grup listesini geçersiz kılmaz; ana ekrana her dönüşte sayaçlar tazelenir.
  const groups = useQuery({
    queryKey: queryKeys.groups,
    queryFn: ({ signal }) => groupsApi.list(signal),
    refetchOnMount: 'always',
  })

  const view = useMemo(() => split(groups.data ?? []), [groups.data])
  const firstName = user?.name.split(/\s+/)[0] ?? ''

  const newGroupButton = (
    <NewGroupDialog
      trigger={
        <Button>
          <Plus />
          {t('home.newGroup')}
        </Button>
      }
    />
  )

  return (
    <div className="mx-auto max-w-[1100px] px-4 pt-7 pb-16 md:px-8 md:pt-10 lg:box-content lg:pt-12">
      <header className="flex flex-col gap-5 md:flex-row md:items-end md:justify-between">
        <div className="min-w-0">
          <h1 className="font-display text-[26px] leading-[1.12] font-semibold md:text-[40px]">
            {t(greetingKey())}
            {firstName && `, ${firstName}`}.
          </h1>
          <p className="mt-2 text-[15px] text-muted">
            {groups.isPending ? (
              <Bar className="mt-1 h-4 w-64" />
            ) : groups.isError ? null : view.all.length === 0 ? (
              t('home.noGroups')
            ) : view.open.length > 0 ? (
              t('home.openCount', { count: view.open.length })
            ) : (
              t('home.openNone')
            )}
          </p>
        </div>
        <div className="hidden md:block">{newGroupButton}</div>
      </header>

      <div className="mt-8 grid grid-cols-1 items-start gap-4 md:mt-[50px] md:grid-cols-12">
        <NextSurpriseCard state={groups} open={view.open} meId={user?.id} />
        <TasksPanel state={groups} groups={view.open} meId={user?.id} />
        <Panel
          title={t('home.groupsPanel')}
          count={groups.isSuccess ? view.open.length : undefined}
          action={<div className="md:hidden">{newGroupButton}</div>}
          className="md:col-span-8"
        >
          <GroupList state={groups} open={view.open} meId={user?.id} />
        </Panel>
        <HistoryPanel state={groups} completed={view.completed} />
      </div>
    </div>
  )
}

function split(groups: GroupListItem[]) {
  const open = sortByUpcoming(groups.filter((g) => g.status !== 'COMPLETED'))
  const completed = groups
    .filter((g) => g.status === 'COMPLETED')
    .sort((a, b) => (b.completed_at ?? '').localeCompare(a.completed_at ?? ''))
  return { all: groups, open, completed }
}

// 1. Sıradaki sürpriz: en yakın tarihli açık grup. Kobalt kart; hardal yalnızca para için.
function NextSurpriseCard({
  state,
  open,
  meId,
}: {
  state: UseQueryResult<GroupListItem[]>
  open: GroupListItem[]
  meId?: string
}) {
  const { t } = useTranslation()
  const box = 'flex min-h-[176px] flex-col rounded-[22px] p-5 md:col-span-8 md:self-stretch md:p-6'

  if (state.isPending)
    return (
      <div className={cn(box, 'bg-surface')} aria-hidden="true">
        <Bar className="h-3.5 w-28" />
        <Bar className="mt-5 h-7 w-2/3" />
        <Bar className="mt-3 h-3.5 w-1/3" />
      </div>
    )
  if (state.isError)
    return (
      <div className={cn(box, 'justify-center bg-surface text-sm text-muted')}>{t('home.loadError')}</div>
    )

  const next = nextSurprise(open)
  if (!next) {
    // Tarih yalnızca yönetici tarafından eklenir; tarihsiz ilk grubumuza yönlendir.
    const undated = open.find((g) => !g.event_date && g.creator_id === meId)
    return (
      <div className={cn(box, 'bg-surface')}>
        <p className="text-sm font-medium text-muted">{t('home.next.label')}</p>
        <div className="mt-auto flex items-start gap-3 pt-4">
          <CalendarPlus className="mt-0.5 size-5 shrink-0 text-muted" aria-hidden />
          <div className="min-w-0">
            <p className="font-display text-[17px] font-semibold">{t('home.next.emptyTitle')}</p>
            <p className="mt-1 text-sm text-muted">{t('home.next.emptyBody')}</p>
            {undated && (
              <Link
                to={`/groups/${undated.id}`}
                className="mt-3 inline-flex items-center gap-1 text-sm font-medium text-accent underline-offset-4 hover:underline"
              >
                {t('home.next.addDate', { title: undated.title })}
                <ChevronRight className="size-4" aria-hidden />
              </Link>
            )}
          </div>
        </div>
      </div>
    )
  }

  const { group, days } = next
  const soon = days <= 1
  // Fotoğraf varsa: masaüstünde solda metin, sağda sabit 4:3 kutu; telefonda kartın üstünde
  // 16:9 şerit. Fotoğraf yoksa kart aynen eski düz kobalt halidir.
  const hasCover = !!group.cover_version
  return (
    <Link
      to={`/groups/${group.id}`}
      className={cn(
        box,
        'group bg-primary text-on-primary transition-[background-color,transform] hover:bg-primary-hover active:scale-[0.995]',
        // Masaüstünde fotoğraf kartın sağına 8px içeriden oturur: kart ~232px, fotoğraf ~%40 genişlikte.
        hasCover && 'gap-4 md:min-h-[232px] md:flex-row md:items-stretch md:gap-6 md:py-2 md:pr-2 md:pl-6',
      )}
    >
      {hasCover && (
        <GroupCoverImage
          groupId={group.id}
          version={group.cover_version}
          alt=""
          className="aspect-[16/9] w-full rounded-[14px] bg-on-primary/15 md:order-last md:aspect-[4/3] md:w-[40%] md:self-center md:rounded-[16px]"
        />
      )}
      {/* Fotoğrafsız kartta sarmalayıcı düzene katılmaz (display: contents): eski yerleşim birebir korunur. */}
      <div className={hasCover ? 'flex min-w-0 flex-1 flex-col md:py-4' : 'contents'}>
        <p className="flex items-center gap-1.5 text-sm font-medium opacity-80">
          <CalendarDays className="size-4" aria-hidden />
          {t('home.next.label')}
        </p>
        <div
          className={cn(
            'mt-auto flex items-end gap-4 pt-5',
            // Orta genişlikte metin sütunu dar: gün sayısı başlığın altına iner.
            hasCover && 'md:max-lg:flex-col md:max-lg:items-start md:max-lg:gap-3',
          )}
        >
          <div className="min-w-0 flex-1">
            <p className="font-display line-clamp-2 text-[22px] leading-tight font-semibold break-words md:text-[28px]">
              {group.title}
            </p>
            <p className={cn('mt-2 text-sm opacity-80', hasCover ? 'line-clamp-2' : 'truncate')}>
              {t('home.forPerson', { name: group.target_person_name })} · {formatEventDate(group.event_date!)}
            </p>
          </div>
          <div className={cn('shrink-0 text-right', hasCover && 'md:max-lg:flex md:max-lg:items-baseline md:max-lg:gap-2 md:max-lg:text-left')}>
            {soon ? (
              <p className="font-display text-[28px] leading-none font-semibold md:text-[36px]">
                {days === 0 ? t('home.next.today') : t('home.next.tomorrow')}
              </p>
            ) : (
              <>
                <p className="font-display num text-[44px] leading-none font-semibold md:text-[56px]">{days}</p>
                <p className="mt-1 text-sm opacity-80">{t('home.next.daysLeft', { count: days })}</p>
              </>
            )}
          </div>
          <ChevronRight
            className={cn(
              'mb-1 size-5 shrink-0 opacity-80 transition-transform group-hover:translate-x-0.5',
              hasCover && 'md:max-lg:hidden',
            )}
            aria-hidden
          />
        </div>
      </div>
    </Link>
  )
}

// Etiket her zaman yazı içerir; renk tek başına anlam taşımaz.
const LEVEL_PILL: Record<PriorityLevel, string> = {
  urgent: 'bg-pill-negative-bg text-pill-negative-ink',
  high: 'bg-pill-high-bg text-pill-high-ink',
  normal: 'bg-soft text-on-soft',
  low: 'bg-pill-neutral-bg text-pill-neutral-ink',
}

// 2. Senden bekleyenler: öncelik etiketli görevler (kurallar src/lib/priority.ts'de).
function TasksPanel({
  state,
  groups,
  meId,
}: {
  state: UseQueryResult<GroupListItem[]>
  groups: GroupListItem[]
  meId?: string
}) {
  const { t } = useTranslation()
  const tasks = useMemo(() => prioritizedTasks(groups, meId), [groups, meId])

  return (
    <Panel
      title={t('home.tasks.panel')}
      count={state.isSuccess && tasks.length > 0 ? tasks.length : undefined}
      className="md:col-span-4"
    >
      {state.isPending ? (
        <RowsSkeleton rows={3} compact />
      ) : state.isError ? (
        <p className="text-sm text-muted">{t('home.loadError')}</p>
      ) : tasks.length === 0 ? (
        <div className="flex items-start gap-3 rounded-[14px] bg-raised px-4 py-3.5">
          <CheckCircle2 className="mt-0.5 size-5 shrink-0 text-positive" aria-hidden />
          <div>
            <p className="text-sm font-medium">{t('home.tasks.empty')}</p>
            <p className="mt-0.5 text-xs text-muted">{t('home.tasks.emptyBody')}</p>
          </div>
        </div>
      ) : (
        <ul className="-mx-2 flex flex-col">
          {tasks.map((task) => (
            <li key={task.group.id}>
              <Link
                to={`/groups/${task.group.id}`}
                className="group flex min-h-12 items-center gap-3 rounded-[12px] px-2 py-2 transition-colors hover:bg-raised"
              >
                <span
                  className={cn(
                    'inline-flex min-w-[60px] shrink-0 justify-center rounded-full px-2 py-0.5 text-xs font-semibold whitespace-nowrap',
                    LEVEL_PILL[task.level],
                  )}
                >
                  {t(`home.tasks.level.${task.level}`)}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm font-medium">{task.group.title}</span>
                  <span className="mt-0.5 block truncate text-xs text-muted">{taskText(task, t)}</span>
                </span>
                <ChevronRight className="size-4 shrink-0 text-muted transition-transform group-hover:translate-x-0.5" />
              </Link>
            </li>
          ))}
        </ul>
      )}
    </Panel>
  )
}

function taskText(task: PriorityTask, t: ReturnType<typeof useTranslation>['t']): string {
  const when = task.group.event_date ? countdownLabel(task.group.event_date) : null
  const withWhen = (s: string) => (when ? `${s} · ${when}` : s)
  switch (task.kind) {
    case 'overdue':
      return t(task.isAdmin ? 'home.tasks.overdueAdmin' : 'home.tasks.overdueMember', { count: -(task.days ?? 0) })
    case 'votes':
      return withWhen(t('home.tasks.votes', { count: task.group.pending_votes }))
    case 'pickWinner':
      return withWhen(t('home.tasks.pickWinner'))
    case 'addDate':
      return t('home.tasks.addDate')
  }
}

// 3. Gruplarım: yalnızca açık gruplar; satırda bütçe yerine üye ve fikir sayısı.
function GroupList({
  state,
  open,
  meId,
}: {
  state: UseQueryResult<GroupListItem[]>
  open: GroupListItem[]
  meId?: string
}) {
  const { t } = useTranslation()
  if (state.isPending) return <RowsSkeleton rows={3} />
  if (state.isError)
    return (
      <BlockError
        message={`${t('home.loadError')} ${errorMessage(state.error)}`}
        onRetry={() => void state.refetch()}
        retryLabel={t('common.retry')}
      />
    )
  if (open.length === 0)
    return (
      <div className="rounded-[14px] border border-dashed border-control px-4 py-6 text-sm text-muted">
        {state.data.length === 0 ? t('home.emptyGroups') : t('home.emptyOpen')}
      </div>
    )
  return (
    <ul className="flex flex-col gap-2">
      {open.map((g) => {
        const chips = (
          <>
            {g.pending_votes > 0 && (
              <span className="inline-flex shrink-0 items-center rounded-full bg-primary px-2 py-0.5 text-xs font-medium whitespace-nowrap text-on-primary">
                {t('home.pendingVotes', { count: g.pending_votes })}
              </span>
            )}
            <CountdownChip group={g} />
          </>
        )
        return (
          <li key={g.id}>
            <Link
              to={`/groups/${g.id}`}
              className="group flex min-h-[63px] items-center gap-3 rounded-[14px] border border-border bg-raised px-3.5 py-3 transition-[border-color,transform] hover:border-accent active:scale-[0.99]"
            >
              <GroupThumb group={g} />
              <div className="min-w-0 flex-1">
                <p className="truncate text-[15px] font-medium">{g.title}</p>
                <p className="mt-0.5 truncate text-xs text-muted">
                  {t('home.forPerson', { name: g.target_person_name })} · {t('home.members', { count: g.member_count })}{' '}
                  · {t('home.ideas', { count: g.idea_count })}
                </p>
                {/* Dar ekranda etiketler başlığı sıkıştırmasın diye alt satırda. */}
                <div className="mt-1.5 flex flex-wrap gap-1.5 empty:hidden sm:hidden">{chips}</div>
              </div>
              <div className="hidden items-center gap-1.5 sm:flex">{chips}</div>
              {g.creator_id === meId && (
                <span className="hidden rounded-full bg-soft px-2 py-0.5 text-xs font-medium whitespace-nowrap text-on-soft sm:inline">
                  {t('home.admin')}
                </span>
              )}
              <ChevronRight className="size-4 shrink-0 text-muted transition-transform group-hover:translate-x-0.5" />
            </Link>
          </li>
        )
      })}
    </ul>
  )
}

// Tarihi olan açık grup geri sayım gösterir (7 gün ve altı vurgulu).
function CountdownChip({ group }: { group: GroupListItem }) {
  if (!group.event_date) return null
  const days = daysUntil(group.event_date)
  return (
    <span
      className={cn(
        'inline-flex shrink-0 items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium whitespace-nowrap',
        days !== null && days >= 0 && days <= 7 ? 'bg-pill-warn-bg text-pill-warn-ink' : 'bg-pill-neutral-bg text-pill-neutral-ink',
      )}
    >
      <CalendarDays className="size-3" aria-hidden />
      {countdownLabel(group.event_date)}
    </span>
  )
}

// 4. Geçmiş sürprizler: tamamlanan gruplar, yeniden eskiye. Kazanan hediye listede yok.
function HistoryPanel({ state, completed }: { state: UseQueryResult<GroupListItem[]>; completed: GroupListItem[] }) {
  const { t } = useTranslation()
  return (
    <Panel
      title={t('home.history.panel')}
      count={state.isSuccess && completed.length > 0 ? completed.length : undefined}
      className="md:col-span-4"
    >
      {state.isPending ? (
        <RowsSkeleton rows={2} compact />
      ) : state.isError ? (
        <p className="text-sm text-muted">{t('home.loadError')}</p>
      ) : completed.length === 0 ? (
        <p className="text-sm text-muted">{t('home.history.empty')}</p>
      ) : (
        <ul className="-mx-2 flex flex-col">
          {completed.map((g) => (
            <li key={g.id}>
              <Link
                to={`/groups/${g.id}`}
                className="flex min-h-12 items-center gap-3 rounded-[12px] px-2 py-1.5 transition-colors hover:bg-raised"
              >
                <Avatar name={g.target_person_name} size="sm" />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm font-medium">{g.target_person_name}</span>
                  <span className="block truncate text-xs text-muted">{g.title}</span>
                </span>
                {g.completed_at && (
                  <span className="num shrink-0 text-xs whitespace-nowrap text-muted">{formatDate(g.completed_at)}</span>
                )}
              </Link>
            </li>
          ))}
        </ul>
      )}
    </Panel>
  )
}

function RowsSkeleton({ rows, compact = false }: { rows: number; compact?: boolean }) {
  return (
    <ul className="flex flex-col gap-2" aria-hidden="true">
      {Array.from({ length: rows }, (_, i) => (
        <li
          key={i}
          className={
            compact
              ? 'flex h-11 items-center gap-3'
              : 'flex min-h-[63px] items-center gap-3 rounded-[14px] border border-border bg-raised px-3.5'
          }
        >
          <Bar className={compact ? 'size-7' : 'size-9'} />
          <div className="flex-1">
            <Bar className="h-3.5 w-1/2" />
            {!compact && <Bar className="mt-2 h-3 w-1/3" />}
          </div>
        </li>
      ))}
    </ul>
  )
}

function greetingKey() {
  const h = new Date().getHours()
  if (h >= 5 && h < 12) return 'home.morning' as const
  if (h >= 12 && h < 18) return 'home.day' as const
  if (h >= 18 && h < 23) return 'home.evening' as const
  return 'home.night' as const
}
