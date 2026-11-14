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
import { returnPath } from '@/lib/returnPath'
import { AuthLayout } from './AuthLayout'

export function RegisterPage() {
  const { t } = useTranslation()
  const { register } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const from = returnPath(location.state)
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')

  const mutation = useMutation({
    mutationFn: () => register(name.trim(), email.trim(), password),
    onSuccess: () => navigate(from, { replace: true }),
  })

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    mutation.mutate()
  }

  const error = mutation.error
    ? mutation.error instanceof ApiError && mutation.error.status === 409
      ? t('auth.emailTaken')
      : errorMessage(mutation.error)
    : null

  return (
    <AuthLayout title={t('auth.registerTitle')} lead={t('auth.registerLead')}>
      <form onSubmit={onSubmit} className="flex flex-col gap-4">
        <FormError message={error} />
        <FormField id="name" label={t('auth.name')}>
          <Input
            id="name"
            autoComplete="name"
            required
            minLength={2}
            maxLength={100}
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </FormField>
        <FormField id="email" label={t('auth.email')}>
          <Input
            id="email"
            type="email"
            autoComplete="email"
            required
            maxLength={150}
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </FormField>
        <FormField id="password" label={t('auth.password')} hint={t('auth.passwordHint')}>
          <Input
            id="password"
            type="password"
            autoComplete="new-password"
            required
            minLength={6}
            maxLength={100}
            aria-describedby="password-hint"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </FormField>
        <Button type="submit" size="lg" className="mt-2 w-full" disabled={mutation.isPending}>
          {mutation.isPending ? t('common.loading') : t('auth.submitRegister')}
        </Button>
      </form>
      <p className="mt-6 text-sm text-muted">
        {t('auth.toLogin')}{' '}
        <Link to="/login" state={location.state} className="font-medium text-ink underline-offset-4 hover:underline">
          {t('auth.toLoginLink')}
        </Link>
      </p>
    </AuthLayout>
  )
}
