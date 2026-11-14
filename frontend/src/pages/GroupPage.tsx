import { useEffect, useMemo, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useParams, useSearchParams } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { CalendarDays, ChevronLeft, Trophy } from 'lucide-react'

import { ApiError } from '@/api/client'
import { groupsApi, liveInterval, queryKeys } from '@/api/endpoints'
import { useGroupEvents } from '@/api/useGroupEvents'
import { useAuth } from '@/auth/AuthProvider'
import { GroupCoverImage } from '@/components/GroupCover'
import { Bar, BlockError } from '@/components/Panel'
import { SurpriseScreen } from '@/components/SurpriseScreen'
import { errorMessage, isAntiSpoiler } from '@/lib/errors'
import { countdownLabel, daysUntil, formatEventDate } from '@/lib/eventDate'
import { formatMoney } from '@/lib/format'
import { GiftPool } from './group/GiftPool'
import { GroupDescription, TargetStatus } from './group/GroupHeaderInfo'
import { BudgetTile, GroupMenu, InterestsTile } from './group/GroupManage'
import { MembersPanel } from './group/Members'
import { LockedPoolCard, NoPoolsCard, PoolSection, PoolTabs } from './group/MultiPools'
import { groupScope, PANEL_ID, tabId } from './group/scope'

// Grup ekranı: bütçe ve ilgi alanları (yerinde düzenleme), hediye havuzu (oylama, AI, manuel ekleme) ve üyeler.
export function GroupPage() {
  const { t } = useTranslation()
  const { id = '' } = useParams()
  const { user } = useAuth()
  // Yenileme aralığı SSE bağlantısının durumuna göre seçilir; durum aşağıda, sorgudan sonra bilinir.
  const liveRef = useRef(false)
  const detail = useQuery({
    queryKey: queryKeys.group(id),
    queryFn: ({ signal }) => groupsApi.detail(id, signal),
    retry: (count, err) => !(err instanceof ApiError && err.status >= 400 && err.status < 500) && count < 2,
    // Başka bir üyenin kazananı seçmesi, bütçe veya üye değişikliği sayfa yenilenmeden görünür.
    // Hata durumunda (ör. Anti-Spoiler 403) yenileme durur.
    refetchInterval: (query) => (query.state.status === 'error' ? false : liveInterval(liveRef.current)),
  })
  // Akış yalnızca grup başarıyla yüklendiyse açılır (hedef kişi 403 alır, akış hiç açılmaz).
  const live = useGroupEvents(id, detail.isSuccess)
  useEffect(() => {
    liveRef.current = live
  }, [live])
  const [searchParams, setSearchParams] = useSearchParams()
  const scope = useMemo(() => groupScope(id), [id])

  if (detail.isError && isAntiSpoiler(detail.error)) return <SurpriseScreen />

  const d = detail.data
  const multi = d?.group.mode === 'MULTI'
  const pools = d?.pools ?? []
  // Seçili havuz ?pool= adresinden gelir (bildirim bağlantıları buraya iner). Geçersizse (kaldırılmış
  // ya da yazım hatası) ilk açık havuz, o da yoksa ilk havuz seçilir; adres yeniden yazılmaz.
  const requested = searchParams.get('pool')
  const selected = pools.find((p) => p.id === requested) ?? pools.find((p) => !p.locked) ?? pools[0]
  const selectPool = (poolId: string) =>
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        next.set('pool', poolId)
        return next
      },
      { replace: true },
    )
  const completed = d?.group.status === 'COMPLETED'
  const eventDate = d?.group.event_date
  const days = eventDate ? daysUntil(eventDate) : null
  return (
    <div className="mx-auto max-w-[1100px] px-4 pt-4 pb-16 md:px-8 md:pt-6 lg:box-content lg:pt-8">
      <Link
        to="/"
        className="-ml-1 inline-flex h-9 items-center gap-1 rounded-full pr-3 pl-1 text-sm text-muted transition-colors hover:text-ink"
      >
        <ChevronLeft className="size-4" />
        {t('group.breadcrumb')}
      </Link>

      {detail.isError ? (
        <div className="mt-6">
          <BlockError
            message={
              detail.error instanceof ApiError && [403, 404].includes(detail.error.status)
                ? t('group.notFound')
                : `${t('group.loadError')} ${errorMessage(detail.error)}`
            }
            onRetry={() => void detail.refetch()}
            retryLabel={t('common.retry')}
          />
        </div>
      ) : (
        <>
          {/* Başlık: (varsa) kapak kutusu + başlık bilgileri + menü. Fotoğraf yoksa düzen aynen eski hali. */}
          <header className="mt-4 flex flex-col gap-4 sm:flex-row sm:items-start sm:gap-5">
            <GroupCoverImage
              groupId={id}
              version={d?.group.cover_version}
              alt={d ? t('cover.alt', { title: d.group.title }) : ''}
              className="aspect-[4/3] w-40 rounded-[16px] sm:w-44 lg:w-52"
            />
            {/* Metin sütunu: başlık + bilgi satırı (yanında menü), altında açıklama ve hesap durumu. */}
            <div className="flex min-w-0 flex-1 flex-col">
              <div className="flex items-start gap-3">
                <div className="min-w-0 flex-1">
                  <h1 className="font-display text-[26px] leading-[1.12] font-semibold break-words md:text-[40px]">
                    {d ? d.group.title : <Bar className="h-10 w-72" />}
                  </h1>
                  <p className="mt-2 text-[15px] text-muted">
                    {d ? (
                      <>
                        {multi
                          ? pools.length > 0
                            ? t('pools.forCount', { count: pools.length })
                            : t('pools.forNone')
                          : t('group.forPerson', { name: d.group.target_person_name })}{' '}
                        ·{' '}
                        {t('group.members', { count: d.member_count })}
                        {eventDate && (
                          <>
                            {' '}
                            ·{' '}
                            <span className="inline-flex items-center gap-1 whitespace-nowrap">
                              <CalendarDays className="size-3.5" aria-hidden />
                              {formatEventDate(eventDate)}
                              {!completed && <span className="text-ink">({countdownLabel(eventDate)})</span>}
                            </span>
                          </>
                        )}
                        {completed && (
                          <span className="ml-1.5 inline-flex h-6 items-center gap-1 rounded-full bg-primary px-2 align-[1px] text-xs font-medium text-on-primary">
                            <Trophy className="size-3.5" aria-hidden />
                            {t('group.completed')}
                          </span>
                        )}
                      </>
                    ) : (
                      <Bar className="mt-1 h-4 w-48" />
                    )}
                  </p>
                </div>
                {d && (
                  <GroupMenu
                    groupId={id}
                    title={d.group.title}
                    targetName={d.group.target_person_name}
                    targetLinked={!!d.group.target_user_id}
                    description={d.group.description}
                    eventDate={eventDate}
                    completed={completed}
                    isAdmin={d.my_role === 'admin'}
                    coverVersion={d.group.cover_version}
                    coverAdminOnly={!!d.group.cover_admin_only}
                    mode={d.group.mode}
                  />
                )}
              </div>
              {d && (
                <div className="mt-3 flex flex-col gap-3">
                  <GroupDescription groupId={id} text={d.group.description} isAdmin={d.my_role === 'admin'} multi={multi} />
                  {!multi && (
                    <TargetStatus
                      groupId={id}
                      targetName={d.group.target_person_name}
                      linked={!!d.group.target_user_id}
                      account={d.target_account}
                      isAdmin={d.my_role === 'admin'}
                    />
                  )}
                </div>
              )}
            </div>
          </header>

          {multi && d ? (
            <>
              {pools.length > 0 && selected && (
                <PoolTabs pools={pools} selectedId={selected.id} onSelect={selectPool} meId={user?.id} />
              )}
              <div role={selected ? 'tabpanel' : undefined} tabIndex={selected ? 0 : undefined} id={PANEL_ID} aria-labelledby={selected ? tabId(selected.id) : undefined}>
                {selected && !selected.locked ? (
                  <PoolSection
                    key={selected.id}
                    groupId={id}
                    pool={selected}
                    meId={user?.id}
                    live={live}
                    aside={<MembersPanel groupId={id} d={d} meId={user?.id} onPoolCreated={selectPool} />}
                  />
                ) : (
                  <div className="mt-6 grid grid-cols-1 items-start gap-4 md:grid-cols-12">
                    <div className="md:col-span-8">
                      {selected ? <LockedPoolCard /> : <NoPoolsCard isAdmin={d.my_role === 'admin'} />}
                    </div>
                    <MembersPanel groupId={id} d={d} meId={user?.id} onPoolCreated={selectPool} />
                  </div>
                )}
              </div>
            </>
          ) : (
            <>
              <dl className="mt-8 grid grid-cols-2 gap-4 md:grid-cols-12">
                <BudgetTile scope={scope} total={d?.group.total_budget} canManage={d?.my_role === 'admin'} />
                {/* Kişi başı tutar ekranın cevabı: hardal alan yalnızca burada. */}
                <div className="rounded-[22px] bg-field p-4 text-on-field md:col-span-4 md:p-5">
                  <dt className="text-sm font-medium text-on-field-muted">{t('group.perPerson')}</dt>
                  <dd className="font-display num mt-2 text-[22px] leading-8 font-semibold md:text-[32px] md:leading-10">
                    {d ? formatMoney(d.per_person_budget) : <Bar className="h-8 w-24" />}
                  </dd>
                  {d && <dd className="mt-1 text-xs text-on-field-muted">{t('group.perPersonCaption', { count: d.member_count })}</dd>}
                </div>
                <InterestsTile scope={scope} interests={d?.interests} canManage={d?.my_role === 'admin'} />
              </dl>

              <div className="mt-4 grid grid-cols-1 items-start gap-4 md:grid-cols-12">
                <div className="md:col-span-8">
                  {d ? (
                    <GiftPool
                      scope={scope}
                      meId={user?.id}
                      canManage={d.my_role === 'admin'}
                      completed={completed}
                      totalBudget={d.group.total_budget}
                      live={live}
                      datePassed={days !== null && days < 0}
                    />
                  ) : (
                    <Bar className="h-64 rounded-[22px]" />
                  )}
                </div>
                <MembersPanel groupId={id} d={d} meId={user?.id} />
              </div>
            </>
          )}
        </>
      )}
    </div>
  )
}
