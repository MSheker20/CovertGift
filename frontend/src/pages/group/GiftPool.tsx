import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useIsMutating, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CalendarClock, ChevronDown, Link2, LoaderCircle, Sparkles, Trophy } from 'lucide-react'
import { toast } from 'sonner'

import { ApiError } from '@/api/client'
import { ideasApi, liveInterval, mutationKeys, queryKeys } from '@/api/endpoints'
import type { Idea } from '@/api/types'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { EmptyArt } from '@/components/EmptyArt'
import { Bar, BlockError } from '@/components/Panel'
import { Button } from '@/components/ui/button'
import { celebrate } from '@/lib/celebrate'
import { errorMessage } from '@/lib/errors'
import { INTRO_CLASS, staggerStyle, useIntroWindow, useNewIds } from '@/lib/motion'
import { cn } from '@/lib/utils'
import { AddIdeaDialog } from './AddIdeaDialog'
import { AiThinking } from './AiThinking'
import { IdeaCard } from './IdeaCard'
import type { Scope } from './scope'
import { WinnersDialog } from './WinnersDialog'

type Pending = { kind: 'reject' | 'remove'; idea: Idea } | null

type Props = {
  // Fikirlerin yaşadığı yer: grubun kendisi (SINGLE) ya da bir hediye alacak kişinin havuzu (MULTI).
  scope: Scope
  meId?: string
  // Veto, kazanan seçimi ve başkasının fikrini silme: SINGLE'da grup yöneticisi, MULTI'de havuzun
  // yöneticisi (sunucudaki can_manage). Yetki API'de ayrıca denetlenir.
  canManage: boolean
  // Kazanan seçildiyse grup (ya da havuz) tamamlanmıştır: oylar ve yeni öneriler kapalı.
  completed: boolean
  // Kazanan seçim penceresinde seçilenlerin toplamı bununla karşılaştırılır.
  totalBudget?: string
  // SSE bağlantısı açıksa değişiklikler anında gelir; periyodik yenileme seyrekleşir.
  live: boolean
  // Hediye tarihi geçmiş ve grup hâlâ açıksa admin'e kazananı seçmesi hatırlatılır.
  datePassed: boolean
}

// Ekranın sorusu: "Hangi hediyede birleşiyoruz?" En büyük blok bu; en çok oy alan üstte.
export function GiftPool({ scope, meId, canManage, completed, totalBudget, live, datePassed }: Props) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const key = scope.ideasKey
  const isPool = scope.kind === 'pool'
  // Bir oy ya da fikir işlemi sürerken periyodik yenileme durur; iyimser güncelleme ezilmez.
  const busy = useIsMutating({ mutationKey: scope.mutationKey }) > 0
  const ideas = useQuery({
    queryKey: key,
    queryFn: ({ signal }) => scope.listIdeas(signal),
    refetchInterval: busy || completed ? false : liveInterval(live),
  })
  const [pending, setPending] = useState<Pending>(null)
  // Kazanan penceresi açıksa, menüsünden açıldığı (önceden işaretli) fikir.
  const [winnerFor, setWinnerFor] = useState<Idea | null>(null)
  const [showRejected, setShowRejected] = useState(false)

  // İyimser oy: skor ve seçili oy hemen güncellenir; yanıt yeni skoru döndürmediği ve çoğunluk
  // vetosu durumu değiştirebildiği için ardından liste yeniden çekilir. 0 oyu geri alır.
  const vote = useMutation({
    mutationKey: scope.mutationKey,
    mutationFn: ({ idea, value }: { idea: Idea; value: -1 | 0 | 1 }) =>
      value === 0 ? ideasApi.unvote(idea.id) : ideasApi.vote(idea.id, value),
    onMutate: async ({ idea, value }) => {
      await queryClient.cancelQueries({ queryKey: key })
      const previous = queryClient.getQueryData<Idea[]>(key)
      queryClient.setQueryData<Idea[]>(key, (list) =>
        list?.map((i) => (i.id === idea.id ? { ...i, score: i.score - i.my_vote + value, my_vote: value } : i)),
      )
      return { previous }
    },
    onError: (err, _vars, ctx) => {
      if (ctx?.previous) queryClient.setQueryData(key, ctx.previous)
      toast.error(`${t('pool.voteError')} ${errorMessage(err)}`)
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: key }),
  })

  const gemini = useMutation({
    mutationKey: mutationKeys.ai(scope.groupId),
    mutationFn: () => scope.generateAI(),
    onSuccess: (added) => {
      void queryClient.invalidateQueries({ queryKey: key })
      if (added.length > 0) toast.success(t('pool.geminiDone', { count: added.length }))
      else toast(t('pool.geminiNone'))
    },
    onError: (err) => {
      if (err instanceof ApiError && err.status === 429) toast.error(t('pool.geminiQuota'))
      else if (err instanceof ApiError && err.status === 503) toast.error(t('pool.geminiMissing'))
      else if (err instanceof ApiError && err.status === 422) toast(t('pool.geminiNone'))
      else toast.error(errorMessage(err))
    },
  })

  const act = useMutation({
    mutationKey: scope.mutationKey,
    mutationFn: (p: NonNullable<Pending>) => (p.kind === 'reject' ? ideasApi.reject(p.idea.id) : ideasApi.remove(p.idea.id)),
    onSuccess: (_d, p) => {
      toast.success(t(p.kind === 'reject' ? 'pool.rejected' : 'pool.removed', { title: p.idea.title }))
      setPending(null)
    },
    onError: (err) => toast.error(errorMessage(err)),
    onSettled: () => queryClient.invalidateQueries({ queryKey: key }),
  })

  const pickWinners = useMutation({
    mutationKey: scope.mutationKey,
    mutationFn: (ids: string[]) => scope.selectWinners(ids),
    onSuccess: (_d, ids) => {
      const titles = all.filter((i) => ids.includes(i.id)).map((i) => i.title)
      toast.success(t(isPool ? 'pools.winnersSelected' : 'pool.winnersSelected', { count: ids.length, titles: titles.join(', ') }))
      setWinnerFor(null)
    },
    onError: (err) => toast.error(errorMessage(err)),
    // Kazanan seçimi grubun durumunu da değiştirir (detay, fikirler ve liste).
    onSettled: () => queryClient.invalidateQueries({ queryKey: queryKeys.groups }),
  })

  // Kazanan en üstte; diğerleri sunucunun skor sırasıyla.
  const all = ideas.data ?? []
  const winners = all.filter((i) => i.status === 'SELECTED')
  const candidates = all.filter((i) => i.status === 'ACTIVE')
  const active = [...winners, ...candidates]
  const rejected = all.filter((i) => i.status === 'REJECTED')

  // Hareket: ilk açılışta kartlar sırayla gelir; sonradan (SSE ya da kendi işlemimizle) gelen fikir
  // bir kez parlar; yeni seçilen kazanan dönerek açılır ve konfeti patlar. İlk yükleme kutlanmaz.
  const ready = ideas.isSuccess
  const intro = useIntroWindow(ready)
  const fresh = useNewIds(all.map((i) => i.id), ready)
  const newWinners = useNewIds(winners.map((w) => w.id), ready, 1200)
  useEffect(() => {
    if (newWinners.size > 0) void celebrate()
  }, [newWinners])

  // Sonradan gelen fikirler (ör. Gemini'nin birden çok önerisi) kendi aralarında sırayla düşer.
  const freshOrder = all.filter((i) => fresh.has(i.id)).map((i) => i.id)
  const entrance = (idea: Idea, index: number) => {
    if (intro) return staggerStyle(index)
    if (fresh.has(idea.id)) return staggerStyle(freshOrder.indexOf(idea.id))
    return undefined
  }

  const card = (idea: Idea, index: number) => (
    <li key={idea.id} className={cn((intro || fresh.has(idea.id)) && INTRO_CLASS)} style={entrance(idea, index)}>
      <IdeaCard
        idea={idea}
        highlight={newWinners.has(idea.id) ? 'winner' : fresh.has(idea.id) ? 'fresh' : undefined}
        locked={completed}
        canReject={canManage}
        canDelete={canManage || idea.created_by === meId}
        canSelectWinner={canManage}
        onVote={(value) => vote.mutate({ idea, value })}
        onReject={() => setPending({ kind: 'reject', idea })}
        onDelete={() => setPending({ kind: 'remove', idea })}
        onSelectWinner={() => setWinnerFor(idea)}
      />
    </li>
  )

  const dialog = pending
    ? ({
        reject: { title: 'pool.rejectTitle', body: 'pool.rejectBody', confirm: 'pool.rejectConfirm' },
        remove: { title: 'pool.removeTitle', body: 'pool.removeBody', confirm: 'pool.removeConfirm' },
      } as const)[pending.kind]
    : null

  return (
    <section className="-mx-1 rounded-[22px] bg-surface p-3 md:mx-0 md:p-5" aria-busy={gemini.isPending}>
      <header className="mb-4 flex flex-wrap items-center gap-x-3 gap-y-3">
        <div className="mr-auto min-w-0">
          <h2 className="font-display text-[15px] font-semibold">{t('pool.title')}</h2>
          {ideas.data && (
            <p className="mt-0.5 text-xs text-muted">
              {completed ? t(isPool ? 'pools.completedLead' : 'pool.completedLead') : t('pool.lead', { count: active.length })}
            </p>
          )}
        </div>
        {!completed && (
          <>
            <AddIdeaDialog
              scope={scope}
              trigger={
                <Button variant="outline">
                  <Link2 />
                  {t('pool.addManual')}
                </Button>
              }
            />
            <Button onClick={() => gemini.mutate()} disabled={gemini.isPending}>
              {gemini.isPending ? <LoaderCircle className="animate-spin" /> : <Sparkles />}
              {gemini.isPending ? t('pool.geminiWorking') : t('pool.askGemini')}
            </Button>
          </>
        )}
      </header>

      {completed && (
        <p role="status" className="mb-3 flex items-center gap-3 rounded-[14px] bg-soft px-4 py-3.5 text-sm font-medium text-on-soft">
          <Trophy className="size-4 shrink-0" aria-hidden />
          {winners.length > 0
            ? t(isPool ? 'pools.completedBanner' : 'pool.completedBanner', {
                count: winners.length,
                titles: winners.map((w) => w.title).join(', '),
              })
            : t(isPool ? 'pools.completedNoWinner' : 'pool.completedNoWinner')}
        </p>
      )}

      {!completed && datePassed && canManage && (
        <p className="mb-3 flex items-center gap-3 rounded-[14px] border border-dashed border-control px-4 py-3 text-sm text-muted">
          <CalendarClock className="size-4 shrink-0" aria-hidden />
          {t(isPool ? 'pools.datePassedHint' : 'pool.datePassedHint')}
        </p>
      )}

      {gemini.isPending && <AiThinking />}

      {ideas.isPending ? (
        <PoolSkeleton />
      ) : ideas.isError ? (
        <BlockError
          message={`${t('pool.loadError')} ${errorMessage(ideas.error)}`}
          onRetry={() => void ideas.refetch()}
          retryLabel={t('common.retry')}
        />
      ) : (
        <>
          {active.length === 0 && !gemini.isPending ? (
            <div className="flex items-center gap-4 rounded-[14px] border border-dashed border-control px-4 py-5 text-sm text-muted">
              <EmptyArt kind="ideas" />
              <span>{t('pool.empty')}</span>
            </div>
          ) : (
            <ul className="flex flex-col gap-2">{active.map(card)}</ul>
          )}

          {rejected.length > 0 && (
            <div className="mt-4">
              <button
                type="button"
                aria-expanded={showRejected}
                onClick={() => setShowRejected((v) => !v)}
                className="inline-flex h-9 cursor-pointer items-center gap-1.5 text-sm text-muted hover:text-ink"
              >
                {t('pool.rejectedSection', { count: rejected.length })}
                <ChevronDown className={`size-4 transition-transform ${showRejected ? 'rotate-180' : ''}`} />
              </button>
              {showRejected && (
                <>
                  <p className="mb-2 text-xs text-muted">{t('pool.rejectedNote')}</p>
                  <ul className="flex flex-col gap-2">{rejected.map(card)}</ul>
                </>
              )}
            </div>
          )}
        </>
      )}

      <ConfirmDialog
        open={pending !== null}
        onOpenChange={(o) => !o && !act.isPending && setPending(null)}
        title={dialog ? t(dialog.title) : ''}
        body={pending && dialog ? t(dialog.body, { title: pending.idea.title }) : ''}
        confirmLabel={dialog ? t(dialog.confirm) : ''}
        cancelLabel={t('pool.cancel')}
        pending={act.isPending}
        onConfirm={() => pending && act.mutate(pending)}
      />
      {winnerFor && (
        <WinnersDialog
          key={winnerFor.id}
          scopeKind={scope.kind}
          open
          onOpenChange={(o) => !o && !pickWinners.isPending && setWinnerFor(null)}
          ideas={candidates}
          initialId={winnerFor.id}
          budget={totalBudget}
          pending={pickWinners.isPending}
          onConfirm={(ids) => pickWinners.mutate(ids)}
        />
      )}
    </section>
  )
}

function PoolSkeleton() {
  return (
    <ul className="flex flex-col gap-2" aria-hidden="true">
      {[0, 1, 2].map((i) => (
        <li key={i} className="flex gap-3 rounded-[14px] border border-border bg-raised p-3 md:gap-4 md:p-3.5">
          <Bar className="size-[72px] rounded-[12px]" />
          <div className="flex-1 pt-1">
            <Bar className="h-4 w-2/3" />
            <Bar className="mt-2.5 h-3.5 w-1/3" />
            <Bar className="mt-4 h-3 w-1/2" />
          </div>
          <Bar className="w-11 rounded-full" />
        </li>
      ))}
    </ul>
  )
}
