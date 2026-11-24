import { useEffect, useId, useRef, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, Copy, RefreshCw, UserMinus, UserPlus, X } from 'lucide-react'
import { toast } from 'sonner'

import { ApiError } from '@/api/client'
import { friendsApi, queryKeys } from '@/api/endpoints'
import type { Friend } from '@/api/types'
import { Avatar } from '@/components/Avatar'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { EmptyArt } from '@/components/EmptyArt'
import { FormField } from '@/components/FormField'
import { Bar, BlockError, Panel } from '@/components/Panel'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { errorMessage } from '@/lib/errors'
import { friendLinkUrl } from '@/lib/friends'
import { NewGroupDialog } from './NewGroupDialog'

// Arkadaşlar sayfası: e-postayla istek, kişisel link, gelen/giden istekler ve arkadaş listesi.
// Arkadaşlık olayları zile düşmez; üst bardaki Arkadaşlar rozeti gelen istekleri sayar.
export function FriendsPage() {
  const { t } = useTranslation()
  const list = useQuery({ queryKey: queryKeys.friends, queryFn: ({ signal }) => friendsApi.list(signal) })
  const data = list.data

  return (
    <div className="mx-auto max-w-[1148px] px-4 pt-6 pb-16 md:px-6 md:pt-10">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="font-display text-[30px] leading-[1.1] font-bold md:text-[38px]">{t('friends.title')}</h1>
          <p className="mt-2 max-w-[560px] text-[15px] text-muted">{t('friends.lead')}</p>
        </div>
        {(data?.friends.length ?? 0) > 0 && (
          <NewGroupDialog
            trigger={
              <Button size="lg">
                <UserPlus />
                {t('friends.quickGroup')}
              </Button>
            }
          />
        )}
      </div>

      <div className="mt-8 grid grid-cols-1 gap-4 md:grid-cols-12">
        <div className="flex flex-col gap-4 md:col-span-5">
          <Panel title={t('friends.add.panel')}>
            <AddByEmail />
            <MyLink />
          </Panel>
          {(data?.incoming.length ?? 0) > 0 && (
            <Panel title={t('friends.incoming')} count={data?.incoming.length}>
              <ul className="flex flex-col gap-1">
                {data?.incoming.map((f) => <IncomingRow key={f.friendship_id} f={f} />)}
              </ul>
            </Panel>
          )}
          {(data?.outgoing.length ?? 0) > 0 && (
            <Panel title={t('friends.outgoing')} count={data?.outgoing.length}>
              <ul className="flex flex-col gap-1">
                {data?.outgoing.map((f) => <OutgoingRow key={f.friendship_id} f={f} />)}
              </ul>
            </Panel>
          )}
        </div>

        <Panel title={t('friends.list')} count={data?.friends.length} className="md:col-span-7">
          {list.isPending ? (
            <div role="status" className="flex flex-col gap-3">
              <span className="sr-only">{t('common.loading')}</span>
              <Bar className="h-10 w-full" />
              <Bar className="h-10 w-5/6" />
              <Bar className="h-10 w-2/3" />
            </div>
          ) : list.isError ? (
            <BlockError message={t('friends.loadError')} onRetry={() => void list.refetch()} retryLabel={t('common.retry')} />
          ) : data && data.friends.length === 0 ? (
            <div className="flex flex-col items-center py-8 text-center">
              <EmptyArt kind="friends" />
              <p className="mt-4 font-medium">{t('friends.emptyTitle')}</p>
              <p className="mt-1 max-w-[320px] text-sm text-muted">{t('friends.emptyBody')}</p>
            </div>
          ) : (
            <ul className="flex flex-col gap-1">
              {data?.friends.map((f) => <FriendRow key={f.friendship_id} f={f} />)}
            </ul>
          )}
        </Panel>
      </div>
    </div>
  )
}

function useRefreshFriends() {
  const queryClient = useQueryClient()
  return () => queryClient.invalidateQueries({ queryKey: queryKeys.friends, exact: true })
}

function AddByEmail() {
  const { t } = useTranslation()
  const refresh = useRefreshFriends()
  const [email, setEmail] = useState('')
  const request = useMutation({
    mutationFn: () => friendsApi.request(email.trim()),
    onSuccess: (res) => {
      toast.success(res.status === 'ACCEPTED' ? t('friends.add.nowFriends') : t('friends.add.sent'))
      setEmail('')
      void refresh()
    },
  })
  const err = request.error
  const message =
    err instanceof ApiError && err.status === 404
      ? t('friends.add.notFound')
      : err instanceof ApiError && err.status === 409
        ? t('friends.add.exists')
        : err
          ? errorMessage(err)
          : null

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    if (!email.trim()) return
    request.mutate()
  }

  return (
    <form onSubmit={onSubmit} className="flex flex-col gap-2" noValidate>
      <FormField id="friend-email" label={t('friends.add.emailLabel')} hint={t('friends.add.emailHint')}>
        <div className="flex gap-2">
          <Input
            id="friend-email"
            type="email"
            autoComplete="off"
            maxLength={150}
            required
            placeholder={t('friends.add.emailPlaceholder')}
            aria-describedby={message ? 'friend-email-error friend-email-hint' : 'friend-email-hint'}
            aria-invalid={message ? true : undefined}
            value={email}
            onChange={(e) => {
              setEmail(e.target.value)
              if (request.isError) request.reset()
            }}
          />
          <Button type="submit" disabled={request.isPending || !email.trim()}>
            {request.isPending ? t('common.loading') : t('friends.add.send')}
          </Button>
        </div>
        {message && (
          <p id="friend-email-error" role="alert" className="text-xs text-negative">
            {message}
          </p>
        )}
      </FormField>
    </form>
  )
}

function MyLink() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const urlId = useId()
  const link = useQuery({ queryKey: queryKeys.friendLink, queryFn: ({ signal }) => friendsApi.link(signal) })
  const [copied, setCopied] = useState(false)
  const [confirmReset, setConfirmReset] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  useEffect(() => () => clearTimeout(timer.current), [])

  const reset = useMutation({
    mutationFn: () => friendsApi.resetLink(),
    onSuccess: (res) => {
      queryClient.setQueryData(queryKeys.friendLink, res)
      setConfirmReset(false)
      toast.success(t('friends.link.resetDone'))
    },
    onError: (err) => {
      setConfirmReset(false)
      toast.error(errorMessage(err))
    },
  })

  const url = link.data ? friendLinkUrl(link.data.code) : ''
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(url)
      setCopied(true)
      clearTimeout(timer.current)
      timer.current = setTimeout(() => setCopied(false), 2000)
    } catch {
      toast.error(t('friends.link.copyFailed'))
    }
  }

  return (
    <div className="mt-6 border-t border-border pt-5">
      <p id={urlId} className="text-sm font-medium">
        {t('friends.link.title')}
      </p>
      <p className="mt-1 text-xs text-muted">{t('friends.link.lead')}</p>
      {link.isPending ? (
        <Bar className="mt-3 h-[38px] w-full" />
      ) : link.isError ? (
        <p role="alert" className="mt-3 text-sm text-negative">
          {t('friends.link.loadError')}
        </p>
      ) : (
        <div className="mt-3 flex gap-2">
          <Input readOnly aria-labelledby={urlId} value={url} onFocus={(e) => e.currentTarget.select()} className="text-xs" />
          <Button type="button" variant="outline" size="icon" onClick={() => void copy()} aria-label={t('friends.link.copy')}>
            {copied ? <Check className="text-positive" /> : <Copy />}
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={() => setConfirmReset(true)}
            aria-label={t('friends.link.reset')}
            disabled={reset.isPending}
          >
            <RefreshCw />
          </Button>
        </div>
      )}
      <span role="status" className="sr-only">
        {copied ? t('friends.link.copied') : ''}
      </span>
      <ConfirmDialog
        open={confirmReset}
        onOpenChange={(o) => !reset.isPending && setConfirmReset(o)}
        title={t('friends.link.resetTitle')}
        body={t('friends.link.resetBody')}
        confirmLabel={t('friends.link.reset')}
        cancelLabel={t('manage.cancel')}
        pending={reset.isPending}
        onConfirm={() => reset.mutate()}
      />
    </div>
  )
}

function Person({ f, note }: { f: Friend; note?: string }) {
  return (
    <>
      <Avatar name={f.name} seed={f.id} userId={f.id} version={f.avatar_version} size="sm" />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-sm">{f.name}</span>
        {note && <span className="block truncate text-xs text-muted">{note}</span>}
      </span>
    </>
  )
}

function IncomingRow({ f }: { f: Friend }) {
  const { t } = useTranslation()
  const refresh = useRefreshFriends()
  const accept = useMutation({
    mutationFn: () => friendsApi.accept(f.friendship_id),
    onSuccess: () => toast.success(t('friends.accepted', { name: f.name })),
    onError: (err) => toast.error(errorMessage(err)),
    onSettled: refresh,
  })
  const decline = useMutation({
    mutationFn: () => friendsApi.removeRequest(f.friendship_id),
    onError: (err) => toast.error(errorMessage(err)),
    onSettled: refresh,
  })
  const busy = accept.isPending || decline.isPending
  return (
    <li className="flex min-h-11 items-center gap-3">
      <Person f={f} note={t('friends.wantsToBeFriends')} />
      <Button size="sm" onClick={() => accept.mutate()} disabled={busy}>
        {t('friends.accept')}
      </Button>
      <Button
        size="icon"
        variant="ghost"
        onClick={() => decline.mutate()}
        disabled={busy}
        aria-label={t('friends.decline', { name: f.name })}
      >
        <X />
      </Button>
    </li>
  )
}

function OutgoingRow({ f }: { f: Friend }) {
  const { t } = useTranslation()
  const refresh = useRefreshFriends()
  const cancel = useMutation({
    mutationFn: () => friendsApi.removeRequest(f.friendship_id),
    onError: (err) => toast.error(errorMessage(err)),
    onSettled: refresh,
  })
  return (
    <li className="flex min-h-11 items-center gap-3">
      <Person f={f} note={t('friends.waiting')} />
      <Button size="sm" variant="outline" onClick={() => cancel.mutate()} disabled={cancel.isPending}>
        {t('friends.cancelRequest')}
      </Button>
    </li>
  )
}

function FriendRow({ f }: { f: Friend }) {
  const { t } = useTranslation()
  const refresh = useRefreshFriends()
  const [confirm, setConfirm] = useState(false)
  const remove = useMutation({
    mutationFn: () => friendsApi.unfriend(f.id),
    onSuccess: () => setConfirm(false),
    onError: (err) => {
      setConfirm(false)
      toast.error(errorMessage(err))
    },
    onSettled: refresh,
  })
  return (
    <li className="flex min-h-11 items-center gap-3">
      <Person f={f} />
      <Button size="icon" variant="ghost" onClick={() => setConfirm(true)} aria-label={t('friends.remove', { name: f.name })}>
        <UserMinus />
      </Button>
      <ConfirmDialog
        open={confirm}
        onOpenChange={(o) => !remove.isPending && setConfirm(o)}
        title={t('friends.removeTitle', { name: f.name })}
        body={t('friends.removeBody')}
        confirmLabel={t('friends.removeConfirm')}
        cancelLabel={t('manage.cancel')}
        pending={remove.isPending}
        onConfirm={() => remove.mutate()}
      />
    </li>
  )
}
