import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useLocation, useNavigate } from 'react-router'
import { useMutation } from '@tanstack/react-query'

import { ApiError } from '@/api/client'
import { useAuth } from '@/auth/AuthProvider'
import { FormError, FormField } from '@/components/FormField'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { errorMessage } from '@/lib/errors'
import { AuthLayout } from './AuthLayout'

export function LoginPage() {
  const { t } = useTranslation()
  const { login, sessionExpired } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const from = (location.state as { from?: string } | null)?.from ?? '/'
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')

  const mutation = useMutation({
    mutationFn: () => login(email.trim(), password),
    onSuccess: () => navigate(from, { replace: true }),
  })

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    mutation.mutate()
  }

  const error = mutation.error
    ? mutation.error instanceof ApiError && mutation.error.status === 401
      ? t('auth.invalid')
      : errorMessage(mutation.error)
    : sessionExpired
      ? t('auth.sessionExpired')
      : null

  return (
    <AuthLayout title={t('auth.loginTitle')} lead={t('auth.loginLead')}>
      <form onSubmit={onSubmit} className="flex flex-col gap-4" noValidate={false}>
        <FormError message={error} />
        <FormField id="email" label={t('auth.email')}>
          <Input
            id="email"
            type="email"
            autoComplete="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </FormField>
        <FormField id="password" label={t('auth.password')}>
          <Input
            id="password"
            type="password"
            autoComplete="current-password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </FormField>
        <Button type="submit" size="lg" className="mt-2 w-full" disabled={mutation.isPending}>
          {mutation.isPending ? t('common.loading') : t('auth.submitLogin')}
        </Button>
      </form>
      <p className="mt-6 text-sm text-muted">
        {t('auth.toRegister')}{' '}
        <Link to="/register" className="font-medium text-ink underline-offset-4 hover:underline">
          {t('auth.toRegisterLink')}
        </Link>
      </p>
    </AuthLayout>
  )
}
