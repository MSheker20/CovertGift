import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ArrowUpRight, Ban, ChevronDown, ChevronUp, MoreHorizontal, Trash2, Trophy } from 'lucide-react'

import type { Idea } from '@/api/types'
import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { formatMoney } from '@/lib/format'
import { marketplaceLinks, safeHttpUrl } from '@/lib/marketplace'
import { cn } from '@/lib/utils'

type Props = {
  idea: Idea
  canReject: boolean
  canDelete: boolean
  canSelectWinner: boolean
  // Grup tamamlandıysa oylar ve fikir işlemleri kapalıdır; skor yalnızca gösterilir.
  locked: boolean
  // 0: oyu geri al.
  onVote: (vote: -1 | 0 | 1) => void
  onReject: () => void
  onDelete: () => void
  onSelectWinner: () => void
  // Bir kez oynayan vurgu: "fresh" yeni gelen fikir, "winner" yeni seçilen kazanan.
  highlight?: 'fresh' | 'winner'
}

// Satır kartı: 72px görsel yuvası, ortada başlık/fiyat/rozetler/linkler, sağda dikey oy sütunu.
export function IdeaCard({
  idea,
  canReject,
  canDelete,
  canSelectWinner,
  locked,
  onVote,
  onReject,
  onDelete,
  onSelectWinner,
  highlight,
}: Props) {
  const { t } = useTranslation()
  const rejected = idea.status === 'REJECTED'
  const winner = idea.status === 'SELECTED'
  const price = Number(idea.estimated_price)
  const productUrl = idea.source_type === 'MANUAL' ? safeHttpUrl(idea.product_url) : null
  const sources = (idea.ai_sources ?? []).filter((s) => safeHttpUrl(s.url))
  // Search kapalıyken AI fiyatı doğrulanmamış bir tahmindir; backend bunu açıklamanın son
  // paragrafına not olarak ekler. Kartta not ayrı bir etiketle gösterilir, açıklamanın ilk paragrafı kalır.
  const estimated = idea.source_type === 'AI' && sources.length === 0
  const summary = idea.description?.split(/\n\s*\n/)[0]?.trim()

  return (
    <article
      className={cn(
        'rounded-[18px] border border-border bg-raised p-3 transition-colors hover:border-control md:p-3.5',
        rejected && 'bg-transparent',
        winner && 'border-2 border-primary hover:border-primary',
        highlight === 'fresh' && 'animate-[fresh-glow_1.8s_ease-out]',
        highlight === 'winner' && 'origin-top animate-[winner-reveal_700ms_cubic-bezier(0.2,0.8,0.2,1)]',
      )}
    >
      <div className="flex gap-3 md:gap-4">
        <Thumb idea={idea} dim={rejected} />

        <div className="min-w-0 flex-1">
          <h3 className={cn('line-clamp-2 text-[15px] leading-5 font-medium', rejected && 'text-muted line-through')}>
            {idea.title}
          </h3>
          <div className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1.5">
            <span className="num text-sm font-semibold whitespace-nowrap">{price > 0 ? formatMoney(price) : t('pool.noPrice')}</span>
            {idea.source_type === 'AI' ? (
              <Chip className="bg-soft text-on-soft">{t('pool.ai')}</Chip>
            ) : (
              <Chip>{t('pool.manual')}</Chip>
            )}
            {estimated && price > 0 && (
              <Chip className="bg-transparent px-0 font-normal text-muted" title={t('pool.estimatedHint')}>
                {t('pool.estimated')}
              </Chip>
            )}
            {idea.is_over_budget && !rejected && (
              <Chip className="bg-pill-warn-bg text-pill-warn-ink">{t('pool.overBudget')}</Chip>
            )}
            {rejected && <Chip className="bg-pill-negative-bg text-pill-negative-ink">{t('pool.rejectedBadge')}</Chip>}
            {winner && (
              <Chip className="gap-1 bg-primary text-on-primary">
                <Trophy className="size-3" aria-hidden />
                {t('pool.winnerBadge')}
              </Chip>
            )}
          </div>
          {summary && (
            <p className="mt-1.5 line-clamp-2 text-[13px] leading-5 text-muted" title={idea.description}>
              {summary}
            </p>
          )}
        </div>

        {!rejected && <VoteColumn score={idea.score} myVote={idea.my_vote} locked={locked} onVote={onVote} />}
      </div>

      {/* Linkler kartın tam genişliğinde; masaüstünde metin sütununun hizasından başlar. */}
      <div className="mt-2.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-[13px] md:pl-[88px]">
        {productUrl && <ExternalLink href={productUrl}>{t('pool.goToProduct')}</ExternalLink>}
        {idea.search_query &&
          marketplaceLinks(idea.search_query).map((m) => (
            <ExternalLink key={m.name} href={m.href}>
              {m.name}
            </ExternalLink>
          ))}
        {idea.creator?.name && <span className="text-muted">{t('pool.by', { name: idea.creator.name })}</span>}
        {!locked && (canReject || canDelete || canSelectWinner) && (
          <IdeaMenu
            canReject={canReject && !rejected}
            canDelete={canDelete}
            canSelectWinner={canSelectWinner && !rejected}
            onReject={onReject}
            onDelete={onDelete}
            onSelectWinner={onSelectWinner}
          />
        )}
      </div>

      {(sources.length > 0 || idea.ai_search_suggestions) && (
        <div className="md:pl-[88px]">
          <Sources sources={sources} suggestionsHtml={idea.ai_search_suggestions} />
        </div>
      )}
    </article>
  )
}

function Thumb({ idea, dim }: { idea: Idea; dim: boolean }) {
  const [failed, setFailed] = useState(false)
  const src = safeHttpUrl(idea.image_url)
  return (
    <div
      className={cn(
        'flex size-14 shrink-0 items-center justify-center overflow-hidden rounded-[14px] md:size-[72px]',
        // Ürün fotoğrafları beyaz zeminli gelir; iki temada da beyaz kutuda gösterilir.
        src && !failed ? 'bg-white p-1' : 'bg-soft',
        dim && 'opacity-50 grayscale',
      )}
    >
      {src && !failed ? (
        <img
          src={src}
          alt=""
          loading="lazy"
          referrerPolicy="no-referrer"
          onError={() => setFailed(true)}
          className="size-full object-contain"
        />
      ) : (
        <span aria-hidden="true" className="font-display text-[24px] leading-none font-bold text-on-soft md:text-[28px]">
          {idea.title.trim().charAt(0).toLocaleUpperCase('tr-TR')}
        </span>
      )}
    </div>
  )
}

function VoteColumn({
  score,
  myVote,
  locked,
  onVote,
}: {
  score: number
  myVote: -1 | 0 | 1
  locked: boolean
  onVote: (v: -1 | 0 | 1) => void
}) {
  const { t } = useTranslation()
  // Yalnızca az önce basılan buton zıplar; sayfa yüklenirken seçili oylar sabit durur.
  const [pop, setPop] = useState<-1 | 0 | 1>(0)
  // Skorun değişim yönü (render sırasında önceki değerle karşılaştırılır; ilk gösterimde 0).
  const [lastScore, setLastScore] = useState(score)
  const [scoreMove, setScoreMove] = useState<-1 | 0 | 1>(0)
  if (score !== lastScore) {
    setScoreMove(score > lastScore ? 1 : -1)
    setLastScore(score)
  }
  // Seçili oya tekrar basmak oyu geri alır.
  const vote = (v: 1 | -1) => {
    if (myVote === v) {
      onVote(0)
      return
    }
    setPop(v)
    onVote(v)
  }
  if (locked) {
    return (
      <div className="flex w-11 shrink-0 items-center justify-center" role="group" aria-label={t('pool.score', { score })}>
        <span className="font-display num text-[15px] font-semibold">{score > 0 ? `+${score}` : score}</span>
      </div>
    )
  }
  return (
    <div className="-my-1 flex w-11 shrink-0 flex-col items-center justify-center gap-0.5" role="group" aria-label={t('pool.score', { score })}>
      <button
        type="button"
        aria-label={t('pool.upvote')}
        aria-pressed={myVote === 1}
        onClick={() => vote(1)}
        onAnimationEnd={() => setPop(0)}
        className={cn(
          'inline-flex size-11 cursor-pointer items-center justify-center rounded-full text-muted transition-[background-color,color,transform] hover:bg-surface hover:text-ink active:scale-90',
          myVote === 1 && 'bg-primary text-on-primary hover:bg-primary-hover hover:text-on-primary',
          pop === 1 && 'animate-[vote-pop_280ms_ease-out]',
        )}
      >
        <ChevronUp className="size-6" strokeWidth={2} />
      </button>
      {/* Skor değişince yeni değer değişim yönünden kayarak gelir. */}
      <span className="font-display num h-5 overflow-hidden text-[15px] leading-5 font-semibold">
        <span
          key={score}
          className={cn(
            'block',
            scoreMove === 1 && 'animate-[text-in-up_220ms_ease-out]',
            scoreMove === -1 && 'animate-[text-in-down_220ms_ease-out]',
          )}
        >
          {score > 0 ? `+${score}` : score}
        </span>
      </span>
      <button
        type="button"
        aria-label={t('pool.downvote')}
        aria-pressed={myVote === -1}
        onClick={() => vote(-1)}
        onAnimationEnd={() => setPop(0)}
        className={cn(
          'inline-flex size-11 cursor-pointer items-center justify-center rounded-full text-muted transition-[background-color,color,transform] hover:bg-surface hover:text-ink active:scale-90',
          myVote === -1 && 'bg-pill-negative-bg text-negative hover:bg-pill-negative-bg hover:text-negative',
          pop === -1 && 'animate-[vote-pop_280ms_ease-out]',
        )}
      >
        <ChevronDown className="size-6" strokeWidth={2} />
      </button>
    </div>
  )
}

function IdeaMenu({
  canReject,
  canDelete,
  canSelectWinner,
  onReject,
  onDelete,
  onSelectWinner,
}: {
  canReject: boolean
  canDelete: boolean
  canSelectWinner: boolean
  onReject: () => void
  onDelete: () => void
  onSelectWinner: () => void
}) {
  const { t } = useTranslation()
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" className="-my-2 ml-auto size-9 text-muted" aria-label={t('pool.actions')}>
          <MoreHorizontal />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {canSelectWinner && (
          <DropdownMenuItem onSelect={onSelectWinner}>
            <Trophy />
            {t('pool.selectWinner')}
          </DropdownMenuItem>
        )}
        {canReject && (
          <DropdownMenuItem onSelect={onReject}>
            <Ban />
            {t('pool.reject')}
          </DropdownMenuItem>
        )}
        {canDelete && (
          <DropdownMenuItem variant="destructive" onSelect={onDelete}>
            <Trash2 />
            {t('pool.remove')}
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

// AI kaynakları yalnızca grounding metadata'sından gelir. Google'ın arama önerisi HTML'i
// betiklere izin vermeyen bir sandbox iframe'de, sayfadan yalıtılmış gösterilir.
function Sources({ sources, suggestionsHtml }: { sources: { title: string; url: string }[]; suggestionsHtml?: string }) {
  const { t } = useTranslation()
  return (
    <details className="group/src mt-2 text-[13px]">
      <summary className="inline-flex cursor-pointer list-none items-center gap-1 text-muted hover:text-ink">
        {sources.length > 0 ? t('pool.sources', { count: sources.length }) : t('pool.searchSuggestions')}
        <ChevronDown className="size-3.5 transition-transform group-open/src:rotate-180" />
      </summary>
      {sources.length > 0 && (
        <ul className="mt-2 flex flex-col gap-1">
          {sources.map((s) => (
            <li key={s.url} className="truncate">
              <ExternalLink href={safeHttpUrl(s.url)!}>{s.title || new URL(s.url).hostname}</ExternalLink>
            </li>
          ))}
        </ul>
      )}
      {suggestionsHtml && (
        <iframe
          title={t('pool.searchSuggestions')}
          sandbox="allow-popups allow-popups-to-escape-sandbox"
          srcDoc={suggestionsHtml}
          className="mt-2 h-20 w-full rounded-[10px] border border-border"
        />
      )}
    </details>
  )
}

function ExternalLink({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className="inline-flex items-center gap-0.5 font-medium text-ink underline-offset-4 hover:underline"
    >
      {children}
      <ArrowUpRight className="size-3.5 text-muted" />
    </a>
  )
}

function Chip({ className, title, children }: { className?: string; title?: string; children: React.ReactNode }) {
  return (
    <span
      title={title}
      className={cn(
        'inline-flex h-5 flex-none items-center rounded-full bg-pill-neutral-bg px-2 text-[11px] font-medium whitespace-nowrap text-pill-neutral-ink',
        className,
      )}
    >
      {children}
    </span>
  )
}
