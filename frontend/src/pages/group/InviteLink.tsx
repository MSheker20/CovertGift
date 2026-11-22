import { useEffect, useId, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, Copy, Link2, TriangleAlert } from 'lucide-react'
import { toast } from 'sonner'

import { invitesApi, queryKeys } from '@/api/endpoints'
import type { InviteCreated, InviteDays } from '@/api/types'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { Bar } from '@/components/Panel'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { errorMessage } from '@/lib/errors'
import { formatDateTime } from '@/lib/format'

const DAY_OPTIONS: InviteDays[] = [1, 7, 30]

// Yalnızca admin için (çağıran denetler; API de ayrıca 403 döner). Sunucu ham token'ı yalnızca
// oluşturma yanıtında verir; bu yüzden token yalnızca bu bileşenin belleğinde tutulur.
export function InviteLink({ groupId, targetLinked, multi = false }: { groupId: string; targetLinked: boolean; multi?: boolean }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const legendId = useId()
  const urlId = useId()
  const [days, setDays] = useState<InviteDays>(7)
  // Token hangi gruba ait olduğuyla birlikte tutulur; başka bir grupta asla gösterilmez.
  const [createdRaw, setCreated] = useState<(InviteCreated & { groupId: string }) | null>(null)
  const [copyState, setCopyState] = useState<'idle' | 'ok' | 'failed'>('idle')
  const copyTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  useEffect(() => () => clearTimeout(copyTimer.current), [])
  const [confirm, setConfirm] = useState<'regenerate' | 'revoke' | null>(null)

  const status = useQuery({
    queryKey: queryKeys.invite(groupId),
    queryFn: ({ signal }) => invitesApi.status(groupId, signal),
  })

  // Durum sorgusu (ör. SSE ile yenilenen) artık aktif link yok diyorsa ya da bitiş zamanı
  // oluşturduğumuzdan farklıysa (başka bir yönetici yeniledi/iptal etti) eldeki token bayattır.
  const sameInstant = (a: string, b: string) => Math.abs(new Date(a).getTime() - new Date(b).getTime()) < 1000
  const created =
    createdRaw && createdRaw.groupId === groupId && !(
      status.data && (!status.data.active || (status.data.expires_at && !sameInstant(status.data.expires_at, createdRaw.expires_at)))
    )
      ? createdRaw
      : null

  const refresh = () => queryClient.invalidateQueries({ queryKey: queryKeys.invite(groupId) })

  const create = useMutation({
    mutationFn: () => invitesApi.create(groupId, days),
    onSuccess: (res) => {
      setCreated({ ...res, groupId })
      setConfirm(null)
      // Yeni durumu hemen yaz; yenileme gelene kadar token "bayat" sayılmasın.
      queryClient.setQueryData(queryKeys.invite(groupId), { active: true, expires_at: res.expires_at })
      void refresh()
    },
    onError: () => setConfirm(null),
  })

  const revoke = useMutation({
    mutationFn: () => invitesApi.revoke(groupId),
    onSuccess: () => {
      setCreated(null)
      setConfirm(null)
      toast.success(t('invite.revoked'))
      void refresh()
    },
    onError: () => setConfirm(null),
  })

  const url = created ? `${window.location.origin}/invite/${created.token}` : ''
  const expiresAt = created?.expires_at ?? status.data?.expires_at
  const active = created !== null || status.data?.active === true
  const error = create.error ?? revoke.error

  const copy = async () => {
    clearTimeout(copyTimer.current)
    try {
      await navigator.clipboard.writeText(url)
      setCopyState('ok')
      toast.success(t('invite.copied'))
    } catch {
      setCopyState('failed')
      toast.error(t('invite.copyFailed'))
    }
    copyTimer.current = setTimeout(() => setCopyState('idle'), 2500)
  }

  const startCreate = () => {
    revoke.reset()
    create.mutate()
  }
  const startRevoke = () => {
    create.reset()
    revoke.mutate()
  }

  return (
    <section aria-labelledby={`${urlId}-h`} className="mt-4 border-t border-rule pt-4">
      <h3 id={`${urlId}-h`} className="flex items-center gap-2 text-sm font-medium">
        <Link2 className="size-4" aria-hidden="true" />
        {t('invite.section')}
      </h3>
      <p className="mt-1 text-xs text-muted">{t(multi ? 'invite.leadMulti' : 'invite.lead')}</p>
      {!targetLinked && (
        <p className="mt-3 flex items-start gap-2 rounded-[12px] bg-pill-warn-bg px-3 py-2 text-xs text-pill-warn-ink">
          <TriangleAlert className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
          <span>
            <strong className="font-semibold">{t('invite.noTargetWarningLabel')}</strong> {t('invite.noTargetWarning')}
          </span>
        </p>
      )}

      {status.isPending && !created ? (
        <Bar className="mt-3 h-10" />
      ) : status.isError && !created ? (
        <p role="alert" className="mt-3 text-xs text-negative">
          {t('invite.loadFailed')}{' '}
          <button type="button" onClick={() => void status.refetch()} className="cursor-pointer font-medium underline underline-offset-4">
            {t('common.retry')}
          </button>
        </p>
      ) : (
        <div className="mt-3 flex flex-col gap-3">
          {created && (
            <div>
              <label htmlFor={urlId} className="text-xs font-medium">
                {t('invite.linkLabel')}
              </label>
              <div className="mt-1.5 flex gap-2">
                <Input id={urlId} readOnly value={url} onFocus={(e) => e.currentTarget.select()} aria-describedby={`${urlId}-note`} />
                <Button type="button" variant="outline" className="h-11 shrink-0 md:h-10" onClick={() => void copy()}>
                  {copyState === 'ok' ? (
                    <Check aria-hidden="true" className="animate-[check-in_320ms_cubic-bezier(0.2,0.8,0.2,1)] text-positive" />
                  ) : (
                    <Copy aria-hidden="true" />
                  )}
                  {copyState === 'ok' ? t('invite.copiedShort') : t('invite.copy')}
                </Button>
              </div>
              <p role="status" aria-live="polite" className={copyState === 'failed' ? 'mt-2 text-xs text-negative' : 'sr-only'}>
                {copyState === 'ok' ? t('invite.copied') : copyState === 'failed' ? t('invite.copyFailed') : ''}
              </p>
              <p id={`${urlId}-note`} className="mt-2 text-xs text-muted">
                {t('invite.shownOnce')}
              </p>
            </div>
          )}

          <p className="text-sm" data-testid="invite-state">
            {active
              ? expiresAt
                ? t('invite.activeUntil', { date: formatDateTime(expiresAt) })
                : t('invite.activeNoDate')
              : t('invite.noActive')}
          </p>
          {active && !created && <p className="-mt-2 text-xs text-muted">{t('invite.activeHidden')}</p>}

          <fieldset>
            <legend id={legendId} className="text-xs font-medium">
              {t('invite.validity')}
            </legend>
            <div className="mt-1.5 flex flex-wrap gap-2">
              {DAY_OPTIONS.map((d) => (
                <label
                  key={d}
                  className="inline-flex h-11 cursor-pointer items-center rounded-full border border-control bg-raised px-4 text-sm has-[:checked]:border-primary has-[:checked]:bg-primary has-[:checked]:font-semibold has-[:checked]:text-on-primary has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-primary md:h-10"
                >
                  <input
                    type="radio"
                    name={`${urlId}-days`}
                    value={d}
                    checked={days === d}
                    onChange={() => setDays(d)}
                    className="sr-only"
                  />
                  {t('invite.days', { count: d })}
                </label>
              ))}
            </div>
          </fieldset>

          <div className="flex flex-wrap gap-2">
            {active ? (
              <>
                <Button type="button" variant="outline" onClick={() => setConfirm('regenerate')} disabled={create.isPending}>
                  {t('invite.regenerate')}
                </Button>
                <Button type="button" variant="outline" onClick={() => setConfirm('revoke')} disabled={revoke.isPending}>
                  {t('invite.revoke')}
                </Button>
              </>
            ) : (
              <Button type="button" onClick={startCreate} disabled={create.isPending}>
                {create.isPending ? t('invite.creating') : t('invite.create')}
              </Button>
            )}
          </div>
          {error && (
            <p role="alert" className="text-xs text-negative">
              {errorMessage(error)}
            </p>
          )}
        </div>
      )}

      <ConfirmDialog
        open={confirm === 'regenerate'}
        onOpenChange={(open) => !open && setConfirm(null)}
        title={t('invite.regenerateTitle')}
        body={t('invite.regenerateBody', { duration: t('invite.days', { count: days }) })}
        confirmLabel={t('invite.regenerate')}
        cancelLabel={t('invite.cancel')}
        pending={create.isPending}
        onConfirm={startCreate}
      />
      <ConfirmDialog
        open={confirm === 'revoke'}
        onOpenChange={(open) => !open && setConfirm(null)}
        title={t('invite.revokeTitle')}
        body={t('invite.revokeBody')}
        confirmLabel={t('invite.revoke')}
        cancelLabel={t('invite.cancel')}
        pending={revoke.isPending}
        onConfirm={startRevoke}
      />
    </section>
  )
}
