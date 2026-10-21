import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronDown, Link2, LoaderCircle, Sparkles } from 'lucide-react'
import { toast } from 'sonner'

import { ApiError } from '@/api/client'
import { ideasApi, queryKeys } from '@/api/endpoints'
import type { Idea } from '@/api/types'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { Bar, BlockError } from '@/components/Panel'
import { Button } from '@/components/ui/button'
import { errorMessage } from '@/lib/errors'
import { AddIdeaDialog } from './AddIdeaDialog'
import { IdeaCard } from './IdeaCard'

type Pending = { kind: 'reject' | 'remove'; idea: Idea } | null

// Ekranın sorusu: "Hangi hediyede birleşiyoruz?" En büyük blok bu; en çok oy alan üstte.
export function GiftPool({ groupId, meId, isAdmin }: { groupId: string; meId?: string; isAdmin: boolean }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const key = queryKeys.ideas(groupId)
  const ideas = useQuery({ queryKey: key, queryFn: ({ signal }) => ideasApi.list(groupId, signal) })
  const [pending, setPending] = useState<Pending>(null)
  const [showRejected, setShowRejected] = useState(false)

  // İyimser oy: skor ve seçili oy hemen güncellenir; yanıt yeni skoru döndürmediği ve çoğunluk
  // vetosu durumu değiştirebildiği için ardından liste yeniden çekilir.
  const vote = useMutation({
    mutationFn: ({ idea, value }: { idea: Idea; value: 1 | -1 }) => ideasApi.vote(idea.id, value),
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
    mutationFn: () => ideasApi.generateAI(groupId),
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
    mutationFn: (p: NonNullable<Pending>) => (p.kind === 'reject' ? ideasApi.reject(p.idea.id) : ideasApi.remove(p.idea.id)),
    onSuccess: (_d, p) => {
      toast.success(t(p.kind === 'reject' ? 'pool.rejected' : 'pool.removed', { title: p.idea.title }))
      setPending(null)
    },
    onError: (err) => toast.error(errorMessage(err)),
    onSettled: () => queryClient.invalidateQueries({ queryKey: key }),
  })

  const active = ideas.data?.filter((i) => i.status !== 'REJECTED') ?? []
  const rejected = ideas.data?.filter((i) => i.status === 'REJECTED') ?? []

  const card = (idea: Idea) => (
    <li key={idea.id}>
      <IdeaCard
        idea={idea}
        canReject={isAdmin}
        canDelete={isAdmin || idea.created_by === meId}
        onVote={(value) => vote.mutate({ idea, value })}
        onReject={() => setPending({ kind: 'reject', idea })}
        onDelete={() => setPending({ kind: 'remove', idea })}
      />
    </li>
  )

  return (
    <section className="-mx-1 rounded-[22px] bg-surface p-3 md:mx-0 md:p-5" aria-busy={gemini.isPending}>
      <header className="mb-4 flex flex-wrap items-center gap-x-3 gap-y-3">
        <div className="mr-auto min-w-0">
          <h2 className="font-display text-[15px] font-semibold">{t('pool.title')}</h2>
          {ideas.data && <p className="mt-0.5 text-xs text-muted">{t('pool.lead', { count: active.length })}</p>}
        </div>
        <AddIdeaDialog
          groupId={groupId}
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
      </header>

      {gemini.isPending && (
        <p role="status" className="mb-2 flex items-center gap-3 rounded-[14px] bg-soft px-4 py-3.5 text-sm text-on-soft">
          <Sparkles className="size-4 shrink-0" />
          {t('pool.geminiWorkingLong')}
        </p>
      )}

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
            <div className="rounded-[14px] border border-dashed border-control px-4 py-6 text-sm text-muted">{t('pool.empty')}</div>
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
        title={pending?.kind === 'reject' ? t('pool.rejectTitle') : t('pool.removeTitle')}
        body={
          pending
            ? t(pending.kind === 'reject' ? 'pool.rejectBody' : 'pool.removeBody', { title: pending.idea.title })
            : ''
        }
        confirmLabel={pending?.kind === 'reject' ? t('pool.rejectConfirm') : t('pool.removeConfirm')}
        cancelLabel={t('pool.cancel')}
        pending={act.isPending}
        onConfirm={() => pending && act.mutate(pending)}
      />
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
