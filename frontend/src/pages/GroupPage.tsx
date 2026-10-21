import { useTranslation } from 'react-i18next'
import { Link, useParams } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { ChevronLeft } from 'lucide-react'

import { ApiError } from '@/api/client'
import { groupsApi, queryKeys } from '@/api/endpoints'
import { useAuth } from '@/auth/AuthProvider'
import { Avatar } from '@/components/Avatar'
import { LogoMark } from '@/components/Logo'
import { Bar, BlockError, Panel } from '@/components/Panel'
import { Button } from '@/components/ui/button'
import { errorMessage, isAntiSpoiler } from '@/lib/errors'
import { formatMoney } from '@/lib/format'
import { AddMemberForm } from './group/AddMemberForm'
import { GiftPool } from './group/GiftPool'

// Grup ekranı: bütçe ve ilgi alanları, hediye havuzu (oylama, AI, manuel ekleme) ve üyeler.
export function GroupPage() {
  const { t } = useTranslation()
  const { id = '' } = useParams()
  const { user } = useAuth()
  const detail = useQuery({
    queryKey: queryKeys.group(id),
    queryFn: ({ signal }) => groupsApi.detail(id, signal),
    retry: (count, err) => !(err instanceof ApiError && err.status >= 400 && err.status < 500) && count < 2,
  })

  if (detail.isError && isAntiSpoiler(detail.error)) return <SurpriseScreen />

  const d = detail.data
  return (
    <div className="mx-auto max-w-[1100px] px-4 pt-6 pb-16 md:px-8 lg:box-content lg:pt-12">
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
          <header className="mt-4">
            <h1 className="font-display text-[26px] leading-[1.12] font-semibold md:text-[40px]">
              {d ? d.group.title : <Bar className="h-10 w-72" />}
            </h1>
            <p className="mt-2 text-[15px] text-muted">
              {d ? (
                <>
                  {t('group.forPerson', { name: d.group.target_person_name })} ·{' '}
                  {t('group.members', { count: d.member_count })}
                </>
              ) : (
                <Bar className="mt-1 h-4 w-48" />
              )}
            </p>
          </header>

          <dl className="mt-8 grid grid-cols-2 gap-4 md:grid-cols-12">
            <div className="rounded-[22px] bg-surface p-4 md:col-span-4 md:p-5">
              <dt className="text-sm font-medium text-muted">{t('group.totalBudget')}</dt>
              <dd className="font-display num mt-2 text-[22px] leading-8 font-semibold md:text-[32px] md:leading-10">
                {d ? formatMoney(d.group.total_budget) : <Bar className="h-8 w-28" />}
              </dd>
            </div>
            {/* Kişi başı tutar ekranın cevabı: hardal alan yalnızca burada. */}
            <div className="rounded-[22px] bg-field p-4 text-on-field md:col-span-4 md:p-5">
              <dt className="text-sm font-medium text-on-field-muted">{t('group.perPerson')}</dt>
              <dd className="font-display num mt-2 text-[22px] leading-8 font-semibold md:text-[32px] md:leading-10">
                {d ? formatMoney(d.per_person_budget) : <Bar className="h-8 w-24" />}
              </dd>
              {d && <dd className="mt-1 text-xs text-on-field-muted">{t('group.perPersonCaption', { count: d.member_count })}</dd>}
            </div>
            <div className="col-span-2 rounded-[22px] bg-surface p-4 md:col-span-4 md:p-5">
              <dt className="text-sm font-medium text-muted">{t('group.interestsPanel')}</dt>
              <dd className="mt-3 flex flex-wrap gap-1.5">
                {d && d.interests.length === 0 && <span className="text-sm text-muted">{t('group.interestsEmpty')}</span>}
                {d?.interests.map((i) => (
                  <span key={i} className="rounded-full bg-soft px-3 py-1 text-sm font-medium text-on-soft">
                    {i}
                  </span>
                ))}
              </dd>
            </div>
          </dl>

          <div className="mt-4 grid grid-cols-1 items-start gap-4 md:grid-cols-12">
            <div className="md:col-span-8">
              {d ? <GiftPool groupId={id} meId={user?.id} isAdmin={d.my_role === 'admin'} /> : <Bar className="h-64 rounded-[22px]" />}
            </div>
            <Panel title={t('group.membersPanel')} count={d?.member_count} className="md:col-span-4">
              <ul className="flex flex-col gap-1">
                {(d?.members ?? []).map((m) => (
                  <li key={m.id} className="flex min-h-11 items-center gap-3">
                    <Avatar name={m.user?.name ?? '?'} seed={m.user_id} size="sm" />
                    <span className="min-w-0 flex-1 truncate text-sm">
                      {m.user?.name ?? '—'}
                      {m.user_id === user?.id && <span className="text-muted"> ({t('group.you')})</span>}
                    </span>
                    {m.role === 'admin' && (
                      <span className="rounded-full bg-soft px-2 py-0.5 text-xs font-medium whitespace-nowrap text-on-soft">
                        {t('home.admin')}
                      </span>
                    )}
                  </li>
                ))}
              </ul>
              {d && <AddMemberForm groupId={id} />}
            </Panel>
          </div>
        </>
      )}
    </div>
  )
}

function SurpriseScreen() {
  const { t } = useTranslation()
  return (
    <div className="flex min-h-[calc(100dvh-56px)] items-center justify-center p-4 lg:min-h-dvh lg:p-8">
      <div className="flex w-full max-w-[640px] flex-col items-center rounded-[28px] bg-field px-6 py-14 text-center text-on-field md:py-20">
        <LogoMark className="size-20 origin-bottom animate-[wiggle_2.4s_ease-in-out_infinite] text-primary" />
        <h1 className="font-display mt-8 text-[32px] leading-[1.05] font-bold md:text-[48px]">{t('group.surpriseTitle')}</h1>
        <p className="mt-4 max-w-[420px] text-[16px] leading-6 text-on-field-muted">{t('group.surpriseLead')}</p>
        <Button asChild size="lg" className="mt-8">
          <Link to="/">{t('group.surpriseBack')}</Link>
        </Button>
      </div>
    </div>
  )
}
