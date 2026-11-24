import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation } from '@tanstack/react-query'
import { X } from 'lucide-react'
import { toast } from 'sonner'

import { ApiError } from '@/api/client'
import { friendsApi } from '@/api/endpoints'
import type { Friend } from '@/api/types'
import { Avatar } from '@/components/Avatar'
import { FormField } from '@/components/FormField'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { errorMessage } from '@/lib/errors'
import { useRefreshFriends } from '@/lib/friends'

// Arkadaşlar sayfası ve üst bardaki Arkadaşlar popup'ının ortak parçaları.

export function AddByEmail({ compact = false }: { compact?: boolean } = {}) {
  const { t } = useTranslation()
  const refresh = useRefreshFriends()
  const id = compact ? 'friend-email-quick' : 'friend-email'
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
      <FormField id={id} label={t('friends.add.emailLabel')} hint={compact ? undefined : t('friends.add.emailHint')}>
        <div className="flex gap-2">
          <Input
            id={id}
            type="email"
            autoComplete="off"
            maxLength={150}
            required
            placeholder={t('friends.add.emailPlaceholder')}
            aria-describedby={[message && `${id}-error`, !compact && `${id}-hint`].filter(Boolean).join(' ') || undefined}
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
          <p id={`${id}-error`} role="alert" className="text-xs text-negative">
            {message}
          </p>
        )}
      </FormField>
    </form>
  )
}

export function Person({ f, note }: { f: Friend; note?: string }) {
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

export function IncomingRow({ f }: { f: Friend }) {
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

