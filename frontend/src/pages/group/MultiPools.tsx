import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CalendarDays, Check, Lock, Minus, MoreHorizontal, Plus, RotateCcw, ShieldAlert, Trophy } from 'lucide-react'
import { toast } from 'sonner'

import { ApiError } from '@/api/client'
import { liveInterval, mutationKeys, poolsApi, queryKeys } from '@/api/endpoints'
import type { OpenPool, PoolDetail, PoolView } from '@/api/types'
import { Avatar } from '@/components/Avatar'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { Bar, BlockError, Panel } from '@/components/Panel'
import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Switch } from '@/components/ui/switch'
import { errorMessage } from '@/lib/errors'
import { countdownLabel, daysUntil, formatEventDate } from '@/lib/eventDate'
import { formatMoney } from '@/lib/format'
import { cn } from '@/lib/utils'
import { GiftPool } from './GiftPool'
import { BudgetTile, EventDateDialog, InterestsTile } from './GroupManage'
import { PANEL_ID, poolScope, tabId } from './scope'

// MULTI grubun kişi seçicisi: havuz sayfasının üst kenarını boydan boya kaplayan bitişik bir şerit.
// Kişiler eşit genişlikte, içerik ortalı; aralarında yalnızca tek bir dikey çizgi var. Seçili kişi
// sayfayla aynı yüzeydedir ve altında çizgi yoktur (sayfaya açılır); diğerleri gri zeminde ve alt
// çizgiyle sayfadan ayrılır. Şeridin üst köşeleri sayfanınkiyle aynı yuvarlaklıktadır (overflow kırpar).
// Çok kişide şerit satır atlamaz, kendi içinde yatay kayar (sayfa 375px'te taşmaz) ve seçili sekme
// görünür alana getirilir. Tarih ve durum panel başlığındadır; sekmede yalnızca kişi, oy ve kupa.
// WAI-ARIA sekme kalıbı, elle etkinleştirme: ←/→, Home, End yalnızca odağı taşır (tabindex 0 odaktaki
// sekmededir); Enter/Space ya da tıklama seçer. Kendi (kilitli) sekmeme ok tuşuyla gezmek hiçbir şey
// yüklemez. Seçim URL'deki ?pool= parametresindedir; çağıran taraf yönetir.
export function PoolTabs({
  pools,
  selectedId,
  onSelect,
  meId,
}: {
  pools: PoolView[]
  selectedId: string
  onSelect: (poolId: string) => void
  meId?: string
}) {
  const { t } = useTranslation()
  const refs = useRef(new Map<string, HTMLButtonElement>())
  // Ok tuşlarıyla odaklanan sekme; null: odak sekme listesinde değil, seçili sekme tabindex 0 alır.
  const [roving, setRoving] = useState<string | null>(null)
  const tabbable = pools.some((p) => p.id === roving) ? roving : selectedId

  // Dar ekranda seçili sekme (ör. bildirimden ?pool= ile gelince) şeridin dışında kalmasın.
  useEffect(() => {
    refs.current.get(selectedId)?.scrollIntoView?.({ block: 'nearest', inline: 'nearest' })
  }, [selectedId])

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const index = pools.findIndex((p) => p.id === tabbable)
    let next = index
    if (e.key === 'ArrowRight') next = (index + 1) % pools.length
    else if (e.key === 'ArrowLeft') next = (index - 1 + pools.length) % pools.length
    else if (e.key === 'Home') next = 0
    else if (e.key === 'End') next = pools.length - 1
    else return
    e.preventDefault()
    setRoving(pools[next].id)
    refs.current.get(pools[next].id)?.focus()
  }

  return (
    <div
      role="tablist"
      aria-label={t('pools.tabsLabel')}
      onKeyDown={onKeyDown}
      onBlur={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget)) setRoving(null)
      }}
      className="flex overflow-x-auto rounded-t-[21px] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
    >
      {pools.map((p, i) => {
        const selected = p.id === selectedId
        const open = p.locked ? null : p
        return (
          <button
            key={p.id}
            ref={(el) => {
              if (el) refs.current.set(p.id, el)
              else refs.current.delete(p.id)
            }}
            type="button"
            role="tab"
            id={tabId(p.id)}
            aria-selected={selected}
            aria-controls={PANEL_ID}
            tabIndex={p.id === tabbable ? 0 : -1}
            onClick={() => {
              setRoving(null)
              onSelect(p.id)
            }}
            className={cn(
              'flex h-14 min-w-[140px] flex-1 basis-0 cursor-pointer items-center justify-center gap-2 px-4 text-sm transition-colors focus-visible:outline-offset-[-3px]',
              i > 0 && 'border-l border-border',
              selected ? 'bg-raised font-semibold text-ink' : 'border-b border-border bg-surface font-medium text-muted hover:text-ink',
            )}
          >
            {p.locked ? (
              <Lock className="size-3.5 shrink-0" aria-hidden />
            ) : (
              <Avatar name={p.recipient.name} seed={p.recipient.id} size="xs" />
            )}
            <span className="min-w-0 truncate">
              {p.recipient.name}
              {p.locked && meId === p.recipient.id && (
                <>
                  {' '}
                  <span className="font-normal">({t('group.you')})</span>
                </>
              )}
            </span>
            {open?.status === 'COMPLETED' && (
              <>
                <Trophy className="size-3.5 shrink-0" aria-hidden />
                <span className="sr-only">{t('group.completed')}</span>
              </>
            )}
            {open && open.status !== 'COMPLETED' && open.pending_votes > 0 && (
              <>
                <span aria-hidden className="num text-xs font-semibold text-primary">
                  {open.pending_votes}
                </span>
                <span className="sr-only">{t('home.pendingVotes', { count: open.pending_votes })}</span>
              </>
            )}
          </button>
        )
      })}
    </div>
  )
}

// Kendi havuzum: kilit ve tek cümle. Bütçe, sayı, tarih, durum ya da sorumlu gösterilmez ve bu
// havuz için hiçbir istek yapılmaz.
export function LockedPoolCard() {
  const { t } = useTranslation()
  return (
    <section className="flex flex-col items-center gap-3 rounded-[22px] bg-surface px-6 py-14 text-center">
      <Lock className="size-8 text-muted" aria-hidden />
      <h2 className="font-display text-[17px] font-semibold">{t('pools.lockedTitle')}</h2>
    </section>
  )
}

// Havuz henüz yokken (ya da hiç sekme seçilemezken) boş durum.
export function NoPoolsCard({ isAdmin }: { isAdmin: boolean }) {
  const { t } = useTranslation()
  return (
    <section className="rounded-[22px] bg-surface p-5 md:p-6">
      <h2 className="font-display text-[15px] font-semibold">{t('pools.emptyTitle')}</h2>
      <p className="mt-2 text-sm text-muted">{isAdmin ? t('pools.emptyAdmin') : t('pools.emptyMember')}</p>
    </section>
  )
}

type SectionProps = {
  groupId: string
  pool: OpenPool
  meId?: string
  live: boolean
}

// Seçili (kilitli olmayan) havuz: bütçe, kişi başı tutar, ilgi alanları, tarih, fikir havuzu ve
// katılımcılar. Yönetici denetimleri grup rolüne değil sunucunun can_manage alanına bağlıdır.
export function PoolSection({ groupId, pool, meId, live }: SectionProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const detail = useQuery({
    queryKey: queryKeys.pool(groupId, pool.id),
    queryFn: ({ signal }) => poolsApi.detail(groupId, pool, signal),
    retry: (count, err) => !(err instanceof ApiError && err.status >= 400 && err.status < 500) && count < 2,
    // Katkı işlemim sürerken periyodik yenileme durur; iyimser güncelleme eski yanıtla ezilmez.
    refetchInterval: (query) =>
      query.state.status === 'error' || queryClient.isMutating({ mutationKey: mutationKeys.contribution(groupId) }) > 0
        ? false
        : liveInterval(live),
  })
  const scope = useMemo(() => poolScope(groupId, pool), [groupId, pool])
  const [dating, setDating] = useState(false)
  const [reopening, setReopening] = useState(false)

  // Detay gelene kadar gruptan gelen kart gösterilir; sıçrama olmaz.
  const view: OpenPool = detail.data ?? pool
  const completed = view.status === 'COMPLETED'
  const canManage = view.can_manage
  const days = view.event_date ? daysUntil(view.event_date) : null

  const reopen = useMutation({
    mutationFn: () => scope.reopen(),
    onSuccess: () => {
      toast.success(t('pools.reopened'))
      setReopening(false)
      void queryClient.invalidateQueries({ queryKey: queryKeys.groups })
    },
    onError: (err) => toast.error(errorMessage(err)),
  })

  if (detail.isError && !detail.data) {
    const gone = detail.error instanceof ApiError && detail.error.status === 404
    return (
      <BlockError
        message={gone ? t('pools.notFound') : `${t('pools.loadError')} ${errorMessage(detail.error)}`}
        onRetry={() => void detail.refetch()}
        retryLabel={t('common.retry')}
      />
    )
  }

  const hasShare = view.per_person_budget !== null && !view.no_contributors

  return (
    <>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <h2 className="font-display min-w-0 text-[19px] leading-tight font-semibold break-words md:text-[22px]">
          {t('pools.forRecipient', { name: view.recipient.name })}
        </h2>
        {view.event_date && (
          <span className="inline-flex items-center gap-1 text-sm whitespace-nowrap text-muted">
            <CalendarDays className="size-3.5" aria-hidden />
            {formatEventDate(view.event_date)}
            {!completed && <span className="text-ink">({countdownLabel(view.event_date)})</span>}
          </span>
        )}
        {completed && (
          <span className="inline-flex h-6 items-center gap-1 rounded-full bg-primary px-2 text-xs font-medium text-on-primary">
            <Trophy className="size-3.5" aria-hidden />
            {t('group.completed')}
          </span>
        )}
        {view.manager && (
          <span className="text-sm text-muted">
            {t('pools.manager', { name: view.manager.name })}
            {view.manager.id === meId && ` (${t('group.you')})`}
          </span>
        )}
        {canManage && !view.event_date && !completed && (
          <button
            type="button"
            onClick={() => setDating(true)}
            className="inline-flex h-9 cursor-pointer items-center gap-1 rounded-full border border-dashed border-control px-3 text-sm text-muted transition-colors hover:border-accent hover:text-ink"
          >
            <Plus className="size-3.5" aria-hidden />
            {t('pools.addDate')}
          </button>
        )}
        {canManage && (
          <div className="ml-auto">
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="outline" size="icon" aria-label={t('pools.poolActions')}>
                  <MoreHorizontal />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-[min(288px,calc(100vw-24px))]">
                <DropdownMenuItem onSelect={() => setDating(true)}>
                  <CalendarDays />
                  {t('manage.eventDate')}
                </DropdownMenuItem>
                {completed && (
                  <DropdownMenuItem onSelect={() => setReopening(true)}>
                    <RotateCcw />
                    {t('pools.reopen')}
                  </DropdownMenuItem>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        )}
      </div>

      {view.manager === null && !canManage && (
        <p
          role="status"
          className="mt-3 flex items-start gap-3 rounded-[14px] bg-pill-high-bg px-4 py-3.5 text-sm font-medium text-pill-high-ink"
        >
          <ShieldAlert className="mt-0.5 size-4 shrink-0" aria-hidden />
          {t('pools.noManager')}
        </p>
      )}

      <dl className="mt-4 grid grid-cols-2 gap-4 md:grid-cols-12">
        <BudgetTile scope={scope} total={view.total_budget} canManage={canManage} />
        {/* Kişi başı tutar ekranın cevabı: hardal alan yalnızca burada. */}
        <div className="rounded-[22px] bg-field p-4 text-on-field md:col-span-4 md:p-5">
          <dt className="text-sm font-medium text-on-field-muted">{t('group.perPerson')}</dt>
          <dd className="font-display num mt-2 text-[22px] leading-8 font-semibold md:text-[32px] md:leading-10">
            {hasShare ? formatMoney(view.per_person_budget ?? '0') : <span aria-hidden>—</span>}
          </dd>
          <dd className="mt-1 text-xs text-on-field-muted" data-testid="per-person-caption">
            {hasShare ? t('pools.perPersonCaption', { count: view.contributor_count }) : t('pools.noContributors')}
          </dd>
        </div>
        <InterestsTile scope={scope} interests={detail.data?.interests} canManage={canManage} />
      </dl>

      <div className="mt-4 grid grid-cols-1 items-start gap-4 md:grid-cols-12">
        <div className="md:col-span-7">
          <GiftPool
            scope={scope}
            meId={meId}
            canManage={canManage}
            completed={completed}
            totalBudget={view.total_budget}
            live={live}
            datePassed={days !== null && days < 0}
          />
        </div>
        <div className="flex flex-col gap-4 md:col-span-5">
          <ContributorsPanel groupId={groupId} pool={pool} detail={detail.data} meId={meId} />
        </div>
      </div>

      <EventDateDialog scope={scope} current={view.event_date} open={dating} onOpenChange={setDating} />
      <ConfirmDialog
        open={reopening}
        onOpenChange={(o) => !reopen.isPending && setReopening(o)}
        title={t('pools.reopenTitle')}
        body={t('pools.reopenBody')}
        confirmLabel={t('manage.reopenConfirm')}
        cancelLabel={t('manage.cancel')}
        pending={reopen.isPending}
        onConfirm={() => reopen.mutate()}
      />
    </>
  )
}

// Havuza kimin katıldığı (kişi başı tutarın bölüneni). Alıcı listede yoktur. Herkes yalnızca
// kendi bayrağını değiştirir; havuz tamamlandıysa anahtar kapalıdır ve nedeni yazılır.
function ContributorsPanel({
  groupId,
  pool,
  detail,
  meId,
}: {
  groupId: string
  pool: OpenPool
  detail?: PoolDetail
  meId?: string
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const key = queryKeys.pool(groupId, pool.id)
  const completed = (detail ?? pool).status === 'COMPLETED'
  const labelId = `contrib-label-${pool.id}`
  const noteId = `contrib-note-${pool.id}`
  // Ekran okuyucu için sonuç duyurusu (bildirimlere ek olarak).
  const [message, setMessage] = useState('')

  // İyimser güncelleme: anahtar ve sayaç hemen değişir; hata (ör. havuz bu arada tamamlandı: 409)
  // olursa önceki hâle döner.
  const toggle = useMutation({
    mutationKey: [...mutationKeys.contribution(groupId), pool.id],
    mutationFn: (value: boolean) => poolsApi.setContribution(groupId, pool, value),
    onMutate: async (value) => {
      await queryClient.cancelQueries({ queryKey: key })
      const previous = queryClient.getQueryData<PoolDetail>(key)
      queryClient.setQueryData<PoolDetail>(key, (d) => {
        if (!d) return d
        const count = Math.max(0, d.contributor_count + (value ? 1 : -1))
        return {
          ...d,
          my_contribution: value,
          contributor_count: count,
          no_contributors: count === 0,
          per_person_budget: count === 0 ? null : (Number(d.total_budget) / count).toFixed(2),
          contributors: d.contributors.map((c) => (c.user_id === meId ? { ...c, contributing: value } : c)),
        }
      })
      return { previous, value }
    },
    onSuccess: (res) => {
      setMessage(
        res.no_contributors || res.per_person_budget === null
          ? t('pools.announceNoShare')
          : t('pools.announceSaved', { count: res.contributor_count, share: formatMoney(res.per_person_budget) }),
      )
      queryClient.setQueryData<PoolDetail>(key, (d) =>
        d
          ? {
              ...d,
              my_contribution: res.contributing,
              contributor_count: res.contributor_count,
              per_person_budget: res.per_person_budget,
              no_contributors: res.no_contributors,
            }
          : d,
      )
    },
    onError: (err, _value, ctx) => {
      // Yalnızca önbellekte hâlâ bizim iyimser değerimiz varsa geri al; daha yeni sunucu verisi ezilmez.
      const now = queryClient.getQueryData<PoolDetail>(key)
      if (ctx?.previous && now && now.my_contribution === ctx.value) queryClient.setQueryData(key, ctx.previous)
      setMessage(t('pools.announceFailed'))
      toast.error(
        err instanceof ApiError && err.status === 409 ? t('pools.lockedWhileCompleted') : `${t('pools.toggleError')} ${errorMessage(err)}`,
      )
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: queryKeys.group(groupId) }),
  })

  return (
    <Panel title={t('pools.contributors')} count={detail?.contributors.length}>
      {!detail ? (
        <ul className="flex flex-col gap-2" aria-hidden="true">
          {[0, 1, 2].map((i) => (
            <li key={i} className="flex h-11 items-center gap-3">
              <Bar className="size-7" />
              <Bar className="h-3.5 w-1/2" />
            </li>
          ))}
        </ul>
      ) : (
        <ul className="flex flex-col gap-1">
          {detail.contributors.map((c) => {
            const mine = c.user_id === meId
            return (
              <li key={c.user_id} className="flex min-h-11 items-center gap-3">
                <Avatar name={c.name} seed={c.user_id} size="sm" />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm">
                    {c.name}
                    {mine && <span className="text-muted"> ({t('group.you')})</span>}
                  </span>
                  {mine && (
                    <span id={labelId} className="block text-xs text-muted">
                      {t('pools.toggleContribution')}
                    </span>
                  )}
                </span>
                {mine ? (
                  <span className="flex items-center gap-2">
                    <span className="text-xs whitespace-nowrap text-muted" aria-hidden>
                      {c.contributing ? t('pools.contributing') : t('pools.notContributing')}
                    </span>
                    <Switch
                      aria-labelledby={labelId}
                      aria-describedby={completed ? noteId : undefined}
                      checked={c.contributing}
                      disabled={completed}
                      // Sürerken odak düşmesin diye disabled yerine aria-disabled; girdi yok sayılır.
                      aria-disabled={toggle.isPending || undefined}
                      onCheckedChange={(v) => {
                        if (!toggle.isPending) toggle.mutate(v)
                      }}
                    />
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-1 text-xs whitespace-nowrap text-muted">
                    {c.contributing ? <Check className="size-3.5" aria-hidden /> : <Minus className="size-3.5" aria-hidden />}
                    {c.contributing ? t('pools.contributing') : t('pools.notContributing')}
                  </span>
                )}
              </li>
            )
          })}
        </ul>
      )}
      <p role="status" aria-live="polite" className="sr-only">
        {message}
      </p>
      {completed && (
        <p id={noteId} className="mt-3 text-xs text-muted">
          {t('pools.lockedWhileCompleted')}
        </p>
      )}
    </Panel>
  )
}
