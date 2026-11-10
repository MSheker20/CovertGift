import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { renderApp } from '@/test/render'
import { api, fail, http, me, ok, server } from '@/test/server'

function openProfile() {
  server.use(
    http.get(api('/me'), () => ok(me)),
    http.get(api('/groups'), () => ok([])),
  )
  return renderApp('/profile', { token: 'tok-eski' })
}

async function accountForm() {
  const name = await screen.findByLabelText('Adın')
  const email = screen.getByLabelText('E-posta')
  const save = screen.getByRole('button', { name: 'Kaydet' })
  return { name, email, save }
}

describe('profile page', () => {
  it('opens from the account menu and shows the user with a way back home', async () => {
    server.use(
      http.get(api('/me'), () => ok(me)),
      http.get(api('/groups'), () => ok([])),
    )
    const { router } = renderApp('/', { token: 'tok-eski' })
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Hesap menüsü' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Profil' }))

    expect(await screen.findByRole('heading', { level: 1, name: me.name })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/profile')
    expect(screen.getByRole('link', { name: 'Ana ekran' })).toHaveAttribute('href', '/')
  })

  it('enables save only for a valid change', async () => {
    openProfile()
    const user = userEvent.setup()
    const { name, email, save } = await accountForm()

    expect(name).toHaveValue(me.name)
    expect(save).toBeDisabled()

    await user.type(name, ' Y')
    expect(save).toBeEnabled()

    await user.clear(email)
    await user.type(email, 'gecersiz')
    expect(save).toBeDisabled()
    expect(screen.getByText('Geçerli bir e-posta adresi yaz.')).toHaveAttribute('id', 'profile-email-error')
    expect(email).toHaveAttribute('aria-invalid', 'true')
    expect(email).toHaveAttribute('aria-describedby', 'profile-email-error')

    // Değişiklik geri alınınca kaydet yine pasif.
    await user.clear(email)
    await user.type(email, me.email)
    await user.clear(name)
    await user.type(name, me.name)
    expect(save).toBeDisabled()
  })

  it('saves name and email, stores the new token and updates the header', async () => {
    let body: unknown
    server.use(
      http.patch(api('/me'), async ({ request }) => {
        body = await request.json()
        return ok({ user: { ...me, name: 'Deniz Yıldız', email: 'deniz.yeni@ornek.com' }, token: 'tok-yeni' })
      }),
    )
    openProfile()
    const user = userEvent.setup()
    const { name, email, save } = await accountForm()

    await user.clear(name)
    await user.type(name, '  Deniz Yıldız ')
    await user.clear(email)
    await user.type(email, 'deniz.yeni@ornek.com')
    await user.click(save)

    expect(await screen.findByText('Hesap bilgilerin güncellendi.')).toBeInTheDocument()
    expect(body).toEqual({ name: 'Deniz Yıldız', email: 'deniz.yeni@ornek.com' })
    expect(await screen.findByRole('heading', { level: 1, name: 'Deniz Yıldız' })).toBeInTheDocument()
    expect(localStorage.getItem('covertgift.token')).toBe('tok-yeni')
    // Kaydedilen değerler yeni başlangıç: değişiklik yok, kaydet pasif.
    await waitFor(() => expect(screen.getByRole('button', { name: 'Kaydet' })).toBeDisabled())
    expect(screen.getByLabelText('E-posta')).toHaveValue('deniz.yeni@ornek.com')
  })

  it('shows a taken email under the email field', async () => {
    server.use(http.patch(api('/me'), () => fail(409, 'bu e-posta adresi zaten kullanımda')))
    openProfile()
    const user = userEvent.setup()
    const { email, save } = await accountForm()

    await user.clear(email)
    await user.type(email, 'elif@ornek.com')
    await user.click(save)

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('Bu e-posta başka bir hesapta kullanılıyor.')
    expect(email).toHaveAttribute('aria-describedby', alert.id)
    expect(localStorage.getItem('covertgift.token')).toBe('tok-eski')

    // Yazmaya başlayınca hata kalkar.
    await user.type(email, 'x')
    expect(screen.queryByText('Bu e-posta başka bir hesapta kullanılıyor.')).not.toBeInTheDocument()
  })
})

describe('profile password change', () => {
  async function passwordForm() {
    const current = await screen.findByLabelText('Mevcut şifre')
    const next = screen.getByLabelText('Yeni şifre')
    const confirm = screen.getByLabelText('Yeni şifre (tekrar)')
    const submit = screen.getByRole('button', { name: 'Şifreyi değiştir' })
    return { current, next, confirm, submit }
  }

  it('validates length and confirmation on the client', async () => {
    openProfile()
    const user = userEvent.setup()
    const { current, next, confirm, submit } = await passwordForm()

    expect(submit).toBeDisabled()
    await user.type(current, 'eski-sifre')
    await user.type(next, '12345')
    await user.tab()
    expect(screen.getByText('Yeni şifre en az 6 karakter olmalı.')).toBeInTheDocument()
    expect(next).toHaveAttribute('aria-describedby', 'profile-new-password-error profile-new-password-hint')
    expect(submit).toBeDisabled()

    await user.type(next, '6')
    await user.type(confirm, '123457')
    expect(screen.getByText('Şifreler eşleşmiyor.')).toBeInTheDocument()
    expect(submit).toBeDisabled()

    await user.clear(confirm)
    await user.type(confirm, '123456')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(submit).toBeEnabled()

    await user.clear(next)
    await user.type(next, 'eski-sifre')
    expect(screen.getByText('Yeni şifre mevcut şifreyle aynı olamaz.')).toBeInTheDocument()
    expect(submit).toBeDisabled()
  })

  it('toggles password visibility with a labelled button', async () => {
    openProfile()
    const user = userEvent.setup()
    const { current } = await passwordForm()

    expect(current).toHaveAttribute('type', 'password')
    const [toggle] = screen.getAllByRole('button', { name: 'Şifreyi göster' })
    await user.click(toggle)
    expect(current).toHaveAttribute('type', 'text')
    expect(toggle).toHaveAccessibleName('Şifreyi gizle')
    expect(toggle).toHaveAttribute('aria-pressed', 'true')
  })

  it('shows a wrong current password under that field and keeps the session', async () => {
    server.use(http.put(api('/me/password'), () => fail(400, 'mevcut şifre hatalı')))
    openProfile()
    const user = userEvent.setup()
    const { current, next, confirm, submit } = await passwordForm()

    await user.type(current, 'yanlis')
    await user.type(next, 'yeni-sifre')
    await user.type(confirm, 'yeni-sifre')
    await user.click(submit)

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('Mevcut şifre hatalı.')
    expect(alert.id).toBe('profile-current-password-error')
    expect(current).toHaveAttribute('aria-invalid', 'true')
    expect(localStorage.getItem('covertgift.token')).toBe('tok-eski')
  })

  it('changes the password and clears the fields', async () => {
    let body: unknown
    server.use(
      http.put(api('/me/password'), async ({ request }) => {
        body = await request.json()
        return ok({ message: 'ok' })
      }),
    )
    openProfile()
    const user = userEvent.setup()
    const { current, next, confirm, submit } = await passwordForm()

    await user.type(current, 'eski-sifre')
    await user.type(next, 'yeni-sifre')
    await user.type(confirm, 'yeni-sifre')
    await user.click(submit)

    expect(await screen.findByText('Şifren değiştirildi.')).toBeInTheDocument()
    expect(body).toEqual({ current_password: 'eski-sifre', new_password: 'yeni-sifre' })
    expect(current).toHaveValue('')
    expect(next).toHaveValue('')
    expect(confirm).toHaveValue('')
  })
})

describe('profile account deletion', () => {
  it('asks for confirmation, deletes the account and goes to login', async () => {
    let deleted = 0
    server.use(
      http.delete(api('/me'), () => {
        deleted++
        return ok({ message: 'silindi' })
      }),
    )
    const { router } = openProfile()
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Hesabı sil' }))
    const dialog = await screen.findByRole('dialog', { name: 'Hesabın silinsin mi?' })

    // Vazgeç hiçbir şey silmez.
    await user.click(within(dialog).getByRole('button', { name: 'Vazgeç' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(deleted).toBe(0)

    await user.click(screen.getByRole('button', { name: 'Hesabı sil' }))
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', { name: 'Evet, hesabımı sil' }),
    )

    expect(await screen.findByRole('heading', { name: 'Giriş yap' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/login')
    expect(deleted).toBe(1)
    expect(localStorage.getItem('covertgift.token')).toBeNull()
    expect(await screen.findByText('Hesabın silindi.')).toBeInTheDocument()
  })

  it('keeps the session when deletion fails', async () => {
    server.use(http.delete(api('/me'), () => fail(500, 'Hesap silinirken bir hata oluştu')))
    const { router } = openProfile()
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Hesabı sil' }))
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', { name: 'Evet, hesabımı sil' }),
    )

    expect(await screen.findByText('Hesap silinirken bir hata oluştu')).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/profile')
    expect(localStorage.getItem('covertgift.token')).toBe('tok-eski')
  })
})
