import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { useQuery, type UseQueryResult } from '@tanstack/react-query'
import { ChevronRight, Plus } from 'lucide-react'

import { groupsApi, queryKeys } from '@/api/endpoints'
import type { Group } from '@/api/types'
import { useAuth } from '@/auth/AuthProvider'
import { Avatar } from '@/components/Avatar'
import { Bar, BlockError, Panel } from '@/components/Panel'
import { Button } from '@/components/ui/button'
import { errorMessage } from '@/lib/errors'
import { cn } from '@/lib/utils'
import { formatDate, formatMoney } from '@/lib/format'
import { NewGroupDialog } from './NewGroupDialog'

// Ekranın sorusu: "Hangi sürprizlerin içindeyim ve ne kadar bütçe topladık?"
// Izgara: 1100px sütun, 12 × 77px + 11 × 16px = 1100. Kutucuklar 4/4/4, sonra 8 + 4.
export function GroupsPage() {
  const { t } = useTranslation()
  const { user } = useAuth()
  const groups = useQuery({ queryKey: queryKeys.groups, queryFn: ({ signal }) => groupsApi.list(signal) })

  const stats = useMemo(() => summarize(groups.data ?? [], user?.id), [groups.data, user?.id])
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
            ) : stats.count > 0 ? (
              t('home.summary', { count: stats.count, budget: formatMoney(stats.budget) })
            ) : (
              t('home.summaryEmpty')
            )}
          </p>
        </div>
        <div className="hidden md:block">{newGroupButton}</div>
      </header>

      <dl className="mt-8 grid grid-cols-2 gap-4 md:mt-[50px] md:grid-cols-12">
        <Tile label={t('home.tileGroups')} caption={t('home.tileGroupsCaption')} loading={groups.isPending}>
          {stats.count}
        </Tile>
        <Tile label={t('home.tileAdmin')} caption={t('home.tileAdminCaption')} loading={groups.isPending}>
          {stats.admin}
        </Tile>
        <Tile
          label={t('home.tileBudget')}
          caption={t('home.tileBudgetCaption')}
          loading={groups.isPending}
          tone="field"
          className="col-span-2"
        >
          {formatMoney(stats.budget)}
        </Tile>
      </dl>

      <div className="mt-4 grid grid-cols-1 items-start gap-4 md:grid-cols-12">
        <Panel
          title={t('home.groupsPanel')}
          count={groups.data?.length}
          action={<div className="md:hidden">{newGroupButton}</div>}
          className="md:col-span-8"
        >
          <GroupList state={groups} meId={user?.id} />
        </Panel>
        <Panel title={t('home.targetsPanel')} className="md:col-span-4">
          {groups.isPending ? (
            <RowsSkeleton rows={2} compact />
          ) : stats.targets.length === 0 ? (
            <p className="text-sm text-muted">{t('home.targetsEmpty')}</p>
          ) : (
            <ul className="flex flex-col gap-1">
              {stats.targets.map((p) => (
                <li key={p.name} className="flex h-11 items-center gap-3">
                  <Avatar name={p.name} size="sm" />
                  <span className="min-w-0 flex-1 truncate text-sm">{p.name}</span>
                  <span className="num text-xs text-muted">{t('home.targetGroups', { count: p.count })}</span>
                </li>
              ))}
            </ul>
          )}
        </Panel>
      </div>
    </div>
  )
}

function GroupList({ state, meId }: { state: UseQueryResult<Group[]>; meId?: string }) {
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
  if (state.data.length === 0)
    return (
      <div className="rounded-[14px] border border-dashed border-control px-4 py-6 text-sm text-muted">
        {t('home.emptyGroups')}
      </div>
    )
  return (
    <ul className="flex flex-col gap-2">
      {state.data.map((g) => (
        <li key={g.id}>
          <Link
            to={`/groups/${g.id}`}
            className="group flex min-h-[63px] items-center gap-3 rounded-[14px] border border-border bg-raised px-3.5 py-3 transition-[border-color,transform] hover:border-accent active:scale-[0.99]"
          >
            <Avatar name={g.target_person_name} />
            <div className="min-w-0 flex-1">
              <p className="truncate text-[15px] font-medium">{g.title}</p>
              <p className="mt-0.5 truncate text-xs text-muted">
                {t('home.forPerson', { name: g.target_person_name })} · {formatDate(g.created_at)}
              </p>
            </div>
            {g.creator_id === meId && (
              <span className="hidden rounded-full bg-soft px-2 py-0.5 text-xs font-medium whitespace-nowrap text-on-soft sm:inline">
                {t('home.admin')}
              </span>
            )}
            <span className="num w-[112px] text-right text-sm font-semibold whitespace-nowrap">{formatMoney(g.total_budget)}</span>
            <ChevronRight className="size-4 shrink-0 text-muted transition-transform group-hover:translate-x-0.5" />
          </Link>
        </li>
      ))}
    </ul>
  )
}

function Tile({
  label,
  caption,
  loading,
  tone = 'plain',
  className,
  children,
}: {
  label: string
  caption: string
  loading: boolean
  tone?: 'plain' | 'field'
  className?: string
  children: React.ReactNode
}) {
  // Hardal yalnızca parayı taşıyan kutucukta; sayaçlar tonal yüzeyde kalır.
  const field = tone === 'field'
  return (
    <div
      className={cn(
        'flex min-h-[112px] flex-col rounded-[22px] p-4 md:col-span-4 md:min-h-[128px] md:p-5',
        field ? 'bg-field text-on-field' : 'bg-surface',
        className,
      )}
    >
      <dt className={cn('text-sm font-medium', field ? 'text-on-field-muted' : 'text-muted')}>{label}</dt>
      <dd className="font-display num mt-2 text-[26px] leading-8 font-semibold md:text-[32px] md:leading-10">
        {loading ? <Bar className="mt-1 h-8 w-24" /> : children}
      </dd>
      <dd className={cn('mt-auto pt-2 text-xs', field ? 'text-on-field-muted' : 'text-muted')}>{caption}</dd>
    </div>
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

function summarize(groups: Group[], meId?: string) {
  let budget = 0
  let admin = 0
  const targets = new Map<string, number>()
  for (const g of groups) {
    budget += Number(g.total_budget) || 0
    if (g.creator_id === meId) admin++
    const name = g.target_person_name.trim()
    targets.set(name, (targets.get(name) ?? 0) + 1)
  }
  return {
    count: groups.length,
    admin,
    // Kuruş hassasiyeti: toplam kuruş cinsinden yuvarlanır.
    budget: Math.round(budget * 100) / 100,
    targets: [...targets.entries()].map(([name, count]) => ({ name, count })).sort((a, b) => b.count - a.count),
  }
}
