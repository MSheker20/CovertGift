import { useState, type ComponentProps, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router'
import { useMutation } from '@tanstack/react-query'
import { ChevronLeft, Eye, EyeOff } from 'lucide-react'
import { toast } from 'sonner'

import { ApiError } from '@/api/client'
import { authApi } from '@/api/endpoints'
import type { User } from '@/api/types'
import { useAuth } from '@/auth/AuthProvider'
import { Avatar } from '@/components/Avatar'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { FormField } from '@/components/FormField'
import { Bar, Panel } from '@/components/Panel'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { errorMessage } from '@/lib/errors'

// Kayıt ve PUT /me/password ile aynı sınırlar (backend binding kuralları).
const NAME_MIN = 2
const NAME_MAX = 100
const EMAIL_MAX = 150
const PASSWORD_MIN = 6
const PASSWORD_MAX = 100
// Basit biçim kontrolü; asıl doğrulama sunucudadır.
const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

// Sunucunun 400 mesajları (services.ErrWrongPassword / ErrSamePassword). İlgili alanın altında gösterilir.
const WRONG_PASSWORD = 'mevcut şifre hatalı'
const SAME_PASSWORD = 'yeni şifre mevcut şifreyle aynı olamaz'

// Profil ekranı: tek sütun, ayrı kartlar. Hesap bilgileri, şifre değiştirme ve hesabı silme.
export function ProfilePage() {
  const { t } = useTranslation()
  const { user } = useAuth()

  return (
    <div className="mx-auto max-w-[640px] px-4 pt-4 pb-16 md:px-0 md:pt-6 lg:pt-8">
      <Link
        to="/"
        className="-ml-1 inline-flex h-9 items-center gap-1 rounded-full pr-3 pl-1 text-sm text-muted transition-colors hover:text-ink"
      >
        <ChevronLeft className="size-4" />
        {t('profile.back')}
      </Link>

      <header className="mt-4 flex items-center gap-4">
        {user ? (
          <Avatar name={user.name} seed={user.id} size="lg" className="size-14 text-base md:size-16 md:text-lg" />
        ) : (
          <Bar className="size-14 md:size-16" />
        )}
        <div className="min-w-0">
          <h1 className="truncate font-display text-[26px] leading-[1.12] font-semibold md:text-[32px]">
            {user ? user.name : <Bar className="h-8 w-48" />}
          </h1>
          <p className="mt-1 truncate text-[15px] text-muted">{user ? user.email : <Bar className="h-4 w-40" />}</p>
        </div>
      </header>

      <div className="mt-8 flex flex-col gap-4">
        {/* Kullanıcı değişince (kayıt sonrası) form kaydedilen değerlerle yeniden başlar. */}
        {user && <AccountCard key={`${user.name}\u0000${user.email}`} user={user} />}
        <PasswordCard />
        <DeleteAccountCard />
      </div>
    </div>
  )
}

function AccountCard({ user }: { user: User }) {
  const { t } = useTranslation()
  const { updateUser } = useAuth()
  const [name, setName] = useState(user.name)
  const [email, setEmail] = useState(user.email)
  const [emailError, setEmailError] = useState<string | null>(null)

  const trimmedName = name.trim()
  const trimmedEmail = email.trim()
  const nameValid = trimmedName.length >= NAME_MIN && trimmedName.length <= NAME_MAX
  const emailValid = EMAIL_RE.test(trimmedEmail) && trimmedEmail.length <= EMAIL_MAX
  const dirty = trimmedName !== user.name || trimmedEmail !== user.email

  const mutation = useMutation({
    mutationFn: () => authApi.updateProfile(trimmedName, trimmedEmail),
    onSuccess: (res) => {
      updateUser(res)
      toast.success(t('profile.accountSaved'))
    },
    onError: (err) => {
      if (err instanceof ApiError && err.status === 409) setEmailError(t('profile.emailTaken'))
      else toast.error(errorMessage(err))
    },
  })

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    if (!dirty || !nameValid || !emailValid || mutation.isPending) return
    mutation.mutate()
  }

  // Alan hataları yalnızca kullanıcı alanı değiştirdiyse gösterilir.
  const nameMessage = trimmedName !== user.name && !nameValid ? t('profile.nameInvalid') : null
  const emailMessage = emailError ?? (trimmedEmail !== user.email && !emailValid ? t('profile.emailInvalid') : null)

  return (
    <Panel title={t('profile.accountTitle')}>
      <form onSubmit={onSubmit} noValidate className="flex flex-col gap-4">
        <FormField id="profile-name" label={t('profile.name')}>
          <Input
            id="profile-name"
            autoComplete="name"
            required
            maxLength={NAME_MAX}
            value={name}
            aria-invalid={nameMessage ? true : undefined}
            aria-describedby={nameMessage ? 'profile-name-error' : undefined}
            onChange={(e) => setName(e.target.value)}
          />
          <FieldError id="profile-name-error" message={nameMessage} />
        </FormField>
        <FormField id="profile-email" label={t('profile.email')}>
          <Input
            id="profile-email"
            type="email"
            autoComplete="email"
            required
            maxLength={EMAIL_MAX}
            value={email}
            aria-invalid={emailMessage ? true : undefined}
            aria-describedby={emailMessage ? 'profile-email-error' : undefined}
            onChange={(e) => {
              setEmail(e.target.value)
              setEmailError(null)
            }}
          />
          <FieldError id="profile-email-error" message={emailMessage} />
        </FormField>
        <div className="flex justify-end">
          <Button type="submit" disabled={!dirty || !nameValid || !emailValid || mutation.isPending}>
            {mutation.isPending ? t('profile.saving') : t('profile.save')}
          </Button>
        </div>
      </form>
    </Panel>
  )
}

function PasswordCard() {
  const { t } = useTranslation()
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [nextTouched, setNextTouched] = useState(false)
  const [currentError, setCurrentError] = useState<string | null>(null)
  const [serverNextError, setServerNextError] = useState<string | null>(null)

  const lengthError =
    next.length > PASSWORD_MAX
      ? t('profile.passwordTooLong', { count: PASSWORD_MAX })
      : nextTouched && next.length > 0 && next.length < PASSWORD_MIN
        ? t('profile.passwordTooShort', { count: PASSWORD_MIN })
        : null
  const sameError = next.length > 0 && next === current ? t('profile.passwordSame') : null
  const nextMessage = serverNextError ?? lengthError ?? sameError
  const mismatch = confirm.length > 0 && confirm !== next ? t('profile.passwordMismatch') : null

  const valid =
    current.length > 0 &&
    current.length <= PASSWORD_MAX &&
    next.length >= PASSWORD_MIN &&
    next.length <= PASSWORD_MAX &&
    next !== current &&
    confirm === next

  const mutation = useMutation({
    mutationFn: () => authApi.changePassword(current, next),
    onSuccess: () => {
      setCurrent('')
      setNext('')
      setConfirm('')
      setNextTouched(false)
      toast.success(t('profile.passwordChanged'))
    },
    onError: (err) => {
      if (err instanceof ApiError && err.status === 400 && err.message === WRONG_PASSWORD) {
        setCurrentError(t('profile.wrongPassword'))
      } else if (err instanceof ApiError && err.status === 400 && err.message === SAME_PASSWORD) {
        setServerNextError(t('profile.passwordSame'))
      } else {
        toast.error(errorMessage(err))
      }
    },
  })

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    if (!valid || mutation.isPending) return
    mutation.mutate()
  }

  return (
    <Panel title={t('profile.passwordTitle')}>
      <form onSubmit={onSubmit} noValidate className="flex flex-col gap-4">
        <FormField id="profile-current-password" label={t('profile.currentPassword')}>
          <PasswordInput
            id="profile-current-password"
            autoComplete="current-password"
            value={current}
            error={currentError}
            onChange={(e) => {
              setCurrent(e.target.value)
              setCurrentError(null)
              setServerNextError(null)
            }}
          />
        </FormField>
        <FormField id="profile-new-password" label={t('profile.newPassword')}>
          <PasswordInput
            id="profile-new-password"
            autoComplete="new-password"
            value={next}
            error={nextMessage}
            hint={t('profile.passwordHint', { count: PASSWORD_MIN })}
            onBlur={() => setNextTouched(true)}
            onChange={(e) => {
              setNext(e.target.value)
              setServerNextError(null)
            }}
          />
        </FormField>
        <FormField id="profile-confirm-password" label={t('profile.confirmPassword')}>
          <PasswordInput
            id="profile-confirm-password"
            autoComplete="new-password"
            value={confirm}
            error={mismatch}
            onChange={(e) => setConfirm(e.target.value)}
          />
        </FormField>
        <div className="flex justify-end">
          <Button type="submit" disabled={!valid || mutation.isPending}>
            {mutation.isPending ? t('profile.saving') : t('profile.changePassword')}
          </Button>
        </div>
      </form>
    </Panel>
  )
}

type PasswordInputProps = Omit<ComponentProps<'input'>, 'type'> & {
  id: string
  error: string | null
  hint?: string
}

// Göster/gizle düğmeli şifre alanı. İpucu ve hata mesajı alanın altında (önce ipucu), aria-describedby ile bağlı.
function PasswordInput({ id, error, hint, ...props }: PasswordInputProps) {
  const { t } = useTranslation()
  const [visible, setVisible] = useState(false)
  const errorId = `${id}-error`
  const hintId = hint ? `${id}-hint` : null
  const describedBy = [error ? errorId : null, hintId].filter(Boolean).join(' ') || undefined
  return (
    <>
      <div className="relative">
        <Input
          id={id}
          type={visible ? 'text' : 'password'}
          maxLength={PASSWORD_MAX + 1}
          className="pr-12"
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy}
          {...props}
        />
        <button
          type="button"
          aria-label={visible ? t('profile.hidePassword') : t('profile.showPassword')}
          aria-controls={id}
          aria-pressed={visible}
          onClick={() => setVisible((v) => !v)}
          className="absolute top-1/2 right-1 inline-flex size-9 -translate-y-1/2 cursor-pointer items-center justify-center rounded-full text-muted transition-colors hover:bg-active hover:text-ink focus-visible:ring-2 focus-visible:ring-accent/40 focus-visible:outline-none md:size-8"
        >
          {visible ? <EyeOff className="size-4" aria-hidden /> : <Eye className="size-4" aria-hidden />}
        </button>
      </div>
      {hint && (
        <p id={`${id}-hint`} className="text-xs text-muted">
          {hint}
        </p>
      )}
      <FieldError id={errorId} message={error} />
    </>
  )
}

function FieldError({ id, message }: { id: string; message: string | null }) {
  if (!message) return null
  return (
    <p id={id} role="alert" className="text-sm text-negative">
      {message}
    </p>
  )
}

function DeleteAccountCard() {
  const { t } = useTranslation()
  const { logout } = useAuth()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)

  const mutation = useMutation({
    mutationFn: () => authApi.deleteMe(),
    onSuccess: () => {
      setOpen(false)
      logout()
      toast.success(t('profile.deleted'))
      navigate('/login', { replace: true })
    },
    onError: (err) => toast.error(errorMessage(err)),
  })

  return (
    <Panel title={t('profile.deleteTitle')} className="bg-pill-negative-bg text-pill-negative-ink">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-sm leading-6">{t('profile.deleteBody')}</p>
        <Button
          className="self-start bg-negative text-on-negative hover:bg-negative-hover sm:self-auto"
          onClick={() => setOpen(true)}
        >
          {t('profile.deleteButton')}
        </Button>
      </div>
      <ConfirmDialog
        open={open}
        onOpenChange={(o) => !mutation.isPending && setOpen(o)}
        title={t('profile.deleteConfirmTitle')}
        body={t('profile.deleteConfirmBody')}
        confirmLabel={t('profile.deleteConfirm')}
        cancelLabel={t('profile.cancel')}
        pending={mutation.isPending}
        onConfirm={() => mutation.mutate()}
      />
    </Panel>
  )
}
