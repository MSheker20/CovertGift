import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { renderApp } from '@/test/render'
import { api, fail, http, me, ok, server } from '@/test/server'

describe('authentication flow', () => {
  it('sends a visitor without a session to the login screen', async () => {
    const { router } = renderApp('/')
    expect(await screen.findByRole('heading', { name: 'Giriş yap' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/login')
  })

  it('logs out a session whose account no longer exists (/me 404)', async () => {
    server.use(
      http.get(api('/me'), () => fail(404, 'kullanıcı bulunamadı')),
      http.get(api('/groups'), () => ok([])),
    )
    const { router } = renderApp('/', { token: 'ghost-token' })

    expect(await screen.findByRole('heading', { name: 'Giriş yap' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/login')
    expect(localStorage.getItem('covertgift.token')).toBeNull()
    expect(screen.getByRole('alert')).toHaveTextContent('Oturumun sona erdi')
  })

  it('shows a clear message for wrong credentials', async () => {
    server.use(http.post(api('/auth/login'), () => fail(401, 'geçersiz e-posta veya şifre')))
    renderApp('/login')
    const user = userEvent.setup()

    await user.type(await screen.findByLabelText('E-posta'), 'deniz@ornek.com')
    await user.type(screen.getByLabelText('Şifre'), 'yanlis-sifre')
    await user.click(screen.getByRole('button', { name: 'Giriş yap' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('E-posta veya şifre hatalı.')
    expect(localStorage.getItem('covertgift.token')).toBeNull()
  })

  it('logs in, stores the token and greets the user by first name', async () => {
    server.use(
      http.post(api('/auth/login'), () => ok({ token: 'tok-ok', user: me })),
      http.get(api('/me'), () => ok(me)),
      http.get(api('/groups'), () => ok([])),
    )
    const { router } = renderApp('/login')
    const user = userEvent.setup()

    await user.type(await screen.findByLabelText('E-posta'), me.email)
    await user.type(screen.getByLabelText('Şifre'), 'dogru-sifre')
    await user.click(screen.getByRole('button', { name: 'Giriş yap' }))

    expect(await screen.findByRole('heading', { level: 1, name: /Deniz\.$/ })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/')
    expect(localStorage.getItem('covertgift.token')).toBe('tok-ok')
  })

  it('explains an already registered email on sign-up', async () => {
    server.use(http.post(api('/auth/register'), () => fail(409, 'bu e-posta zaten kayıtlı')))
    renderApp('/register')
    const user = userEvent.setup()

    await user.type(await screen.findByLabelText('Adın'), 'Deniz Aksoy')
    await user.type(screen.getByLabelText('E-posta'), me.email)
    await user.type(screen.getByLabelText('Şifre'), 'uzun-sifre')
    await user.click(screen.getByRole('button', { name: 'Hesabı aç' }))

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Bu e-posta ile zaten bir hesap var.'))
  })
})
