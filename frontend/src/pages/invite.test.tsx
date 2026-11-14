import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { HttpResponse } from 'msw'
import { beforeEach, describe, expect, it } from 'vitest'

import type { InvitePreview } from '@/api/types'
import { returnPath } from '@/lib/returnPath'
import { renderApp } from '@/test/render'
import { api, eventStream, fail, group, groupDetail, http, me, ok, server } from '@/test/server'

const TOKEN = 'B'.repeat(43)
const PATH = `/invite/${TOKEN}`

const preview = (overrides: Partial<InvitePreview> = {}): InvitePreview => ({
  group_title: 'Kaan’ın 30. yaşı',
  inviter_name: 'Elif Kara',
  member_count: 4,
  already_member: false,
  ...overrides,
})

function authed() {
  server.use(http.get(api('/me'), () => ok(me)), http.get(api('/groups'), () => ok([])))
}

describe('invite page', () => {
  beforeEach(() => localStorage.setItem('covertgift.token', 'tok'))

  it('shows the group, the inviter and the member count, then joins and opens the group', async () => {
    authed()
    let joined = false
    let groupsRefetched = 0
    server.use(
      http.get(api(`/invites/${TOKEN}`), () => ok(preview())),
      http.get(api('/groups'), () => {
        groupsRefetched++
        return ok([])
      }),
      http.post(api(`/invites/${TOKEN}/accept`), () => {
        joined = true
        return ok({ group_id: group.id })
      }),
      http.get(api(`/groups/${group.id}`), () => ok(groupDetail({ my_role: 'member' }))),
      http.get(api(`/groups/${group.id}/ideas`), () => ok([])),
      http.get(api(`/groups/${group.id}/events`), () => eventStream()),
    )
    const user = userEvent.setup()
    const { router } = renderApp(PATH)

    expect(await screen.findByRole('heading', { level: 1, name: 'Kaan’ın 30. yaşı' })).toBeInTheDocument()
    expect(screen.getByText('Elif Kara seni davet etti')).toBeInTheDocument()
    expect(screen.getByText('4 üye')).toBeInTheDocument()
    const before = groupsRefetched

    await user.click(screen.getByRole('button', { name: 'Katıl' }))
    await waitFor(() => expect(router.state.location.pathname).toBe(`/groups/${group.id}`))
    expect(joined).toBe(true)
    expect(groupsRefetched).toBeGreaterThan(before)
  })

  it('shows a retryable error when the server cannot be reached', async () => {
    authed()
    server.use(http.get(api(`/invites/${TOKEN}`), () => HttpResponse.error()))
    renderApp(PATH)
    expect(await screen.findByRole('alert')).toHaveTextContent('Davet yüklenemedi.')
    expect(screen.getByRole('button', { name: 'Tekrar dene' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Katıl' })).not.toBeInTheDocument()
  })

  it('tells an existing member so and links to the group instead of offering to join', async () => {
    authed()
    server.use(http.get(api(`/invites/${TOKEN}`), () => ok(preview({ already_member: true, group_id: group.id }))))
    renderApp(PATH)

    expect(await screen.findByText('Zaten üyesin')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Katıl' })).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Gruba git' })).toHaveAttribute('href', `/groups/${group.id}`)
  })

  it('shows one message for an invalid, expired or revoked link (404)', async () => {
    authed()
    server.use(http.get(api(`/invites/${TOKEN}`), () => fail(404, 'davet linki geçersiz veya süresi dolmuş')))
    renderApp(PATH)

    expect(await screen.findByRole('heading', { name: 'Bu davet linki geçersiz veya süresi dolmuş' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Katıl' })).not.toBeInTheDocument()
  })

  it('shows the invalid state when the link dies between preview and join', async () => {
    authed()
    server.use(
      http.get(api(`/invites/${TOKEN}`), () => ok(preview())),
      http.post(api(`/invites/${TOKEN}/accept`), () => fail(404, 'davet linki geçersiz veya süresi dolmuş')),
    )
    const user = userEvent.setup()
    renderApp(PATH)
    await user.click(await screen.findByRole('button', { name: 'Katıl' }))
    expect(await screen.findByRole('heading', { name: 'Bu davet linki geçersiz veya süresi dolmuş' })).toBeInTheDocument()
  })

  it('shows the surprise screen to the person the group is for and nothing about the group (403)', async () => {
    authed()
    server.use(
      http.get(api(`/invites/${TOKEN}`), () =>
        fail(403, 'Erişim engellendi: Bu hediye grubu senin için oluşturulmuş bir sürprizdir.'),
      ),
    )
    renderApp(PATH)

    expect(await screen.findByRole('heading', { name: 'Bu bir sürpriz!' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Katıl' })).not.toBeInTheDocument()
    expect(screen.queryByText(/davet etti/)).not.toBeInTheDocument()
  })
})

describe('invite page when signed out', () => {
  function guestHandlers() {
    server.use(
      http.get(api('/me'), () => ok(me)),
      http.get(api('/groups'), () => ok([])),
      http.post(api('/auth/register'), () => ok(me, 201)),
      http.post(api('/auth/login'), () => {
        return ok({ token: 'new-token', user: me })
      }),
      http.get(api(`/invites/${TOKEN}`), () => ok(preview())),
    )
  }

  it('redirects to login, remembers the invite path and returns to it after login', async () => {
    guestHandlers()
    const user = userEvent.setup()
    const { router } = renderApp(PATH)

    expect(await screen.findByRole('heading', { name: 'Giriş yap' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/login')
    expect(router.state.location.state).toEqual({ from: PATH })

    await user.type(screen.getByLabelText('E-posta'), 'deniz@ornek.com')
    await user.type(screen.getByLabelText('Şifre'), 'sifre123')
    await user.click(screen.getByRole('button', { name: 'Giriş yap' }))

    await waitFor(() => expect(router.state.location.pathname).toBe(PATH))
    expect(await screen.findByRole('heading', { level: 1, name: 'Kaan’ın 30. yaşı' })).toBeInTheDocument()
  })

  it('keeps the invite path through the register page and returns to it after registering', async () => {
    guestHandlers()
    const user = userEvent.setup()
    const { router } = renderApp(PATH)
    await screen.findByRole('heading', { name: 'Giriş yap' })

    await user.click(screen.getByRole('link', { name: 'Hesap aç' }))
    expect(await screen.findByRole('heading', { name: 'Hesap aç' })).toBeInTheDocument()
    await user.type(screen.getByLabelText('Adın'), 'Deniz Aksoy')
    await user.type(screen.getByLabelText('E-posta'), 'deniz@ornek.com')
    await user.type(screen.getByLabelText('Şifre'), 'sifre123')
    await user.click(screen.getByRole('button', { name: 'Hesabı aç' }))

    await waitFor(() => expect(router.state.location.pathname).toBe(PATH))
    expect(await screen.findByRole('heading', { level: 1, name: 'Kaan’ın 30. yaşı' })).toBeInTheDocument()
  })

  it('ignores an unsafe return path', () => {
    expect(returnPath({ from: '//evil.example' })).toBe('/')
    expect(returnPath({ from: 'https://evil.example' })).toBe('/')
    expect(returnPath(null)).toBe('/')
    for (const bad of ['/\\evil.com', '/a\\b', '/%2Fevil.com', '/%2fevil.com', '/%5Cevil.com', 'invite/x', '']) {
      expect(returnPath({ from: bad })).toBe('/')
    }
    expect(returnPath({ from: PATH })).toBe(PATH)
  })
})
