import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { renderApp } from '@/test/render'
import { api, fail, group, groupDetail, http, me, ok, server } from '@/test/server'
import { eventStream } from '@/test/server'

const GROUP_B = { ...group, id: 'g-b', title: 'Başka grup' }

const PATH = `/groups/${group.id}`
const TOKEN = 'A'.repeat(43)

function baseHandlers(role: 'admin' | 'member') {
  server.use(
    http.get(api('/me'), () => ok(me)),
    http.get(api('/groups'), () => ok([group])),
    http.get(api(`/groups/${group.id}`), () => ok(groupDetail({ my_role: role }))),
    http.get(api(`/groups/${group.id}/ideas`), () => ok([])),
    http.get(api(`/groups/${group.id}/events`), () => eventStream()),
  )
}

describe('invite link panel (admin)', () => {
  beforeEach(() => localStorage.setItem('covertgift.token', 'tok'))

  it('is invisible to members and never asks for the invite status', async () => {
    baseHandlers('member')
    let asked = false
    server.use(
      http.get(api(`/groups/${group.id}/invite`), () => {
        asked = true
        return ok({ active: false })
      }),
    )
    renderApp(PATH)
    await screen.findByRole('heading', { level: 2, name: 'Üyeler' })
    expect(screen.queryByText('Davet linki')).not.toBeInTheDocument()
    expect(asked).toBe(false)
  })

  it('creates a link with the chosen validity and shows the full URL once, with copy', async () => {
    baseHandlers('admin')
    let created = false
    let body: unknown
    server.use(
      http.get(api(`/groups/${group.id}/invite`), () =>
        created ? ok({ active: true, expires_at: '2026-12-01T10:00:00Z' }) : ok({ active: false }),
      ),
      http.post(api(`/groups/${group.id}/invite`), async ({ request }) => {
        body = await request.json()
        created = true
        return ok({ token: TOKEN, expires_at: '2026-12-01T10:00:00Z' }, 201)
      }),
    )
    const user = userEvent.setup()
    const writeText = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue()
    renderApp(PATH)

    expect(await screen.findByText('Aktif davet linki yok.')).toBeInTheDocument()
    const group30 = screen.getByRole('radio', { name: '30 gün' })
    expect(screen.getByRole('radio', { name: '7 gün' })).toBeChecked()
    await user.click(group30)
    await user.click(screen.getByRole('button', { name: 'Link oluştur' }))

    const url = `${window.location.origin}/invite/${TOKEN}`
    expect(await screen.findByDisplayValue(url)).toHaveAttribute('readonly')
    expect(body).toEqual({ expires_in_days: 30 })
    expect(screen.getByText(/yalnızca şimdi gösteriliyor/)).toBeInTheDocument()
    expect(screen.getByText(/Aktif link var — .*2026.* tarihine kadar geçerli/)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Kopyala' }))
    await waitFor(() => expect(writeText).toHaveBeenCalledWith(url))
    expect((await screen.findAllByText('Davet linki kopyalandı.')).length).toBeGreaterThan(0)
    expect(screen.getByRole('button', { name: 'Kopyalandı' })).toBeInTheDocument()
  })

  it('with an active link but no token in memory shows expiry, never a URL, and regenerates after confirmation', async () => {
    baseHandlers('admin')
    let posts = 0
    let expiry = '2026-11-20T10:00:00Z'
    server.use(
      http.get(api(`/groups/${group.id}/invite`), () => ok({ active: true, expires_at: expiry })),
      http.post(api(`/groups/${group.id}/invite`), () => {
        posts++
        expiry = '2026-11-27T10:00:00Z'
        return ok({ token: TOKEN, expires_at: expiry }, 201)
      }),
    )
    const user = userEvent.setup()
    renderApp(PATH)

    expect(await screen.findByText(/Aktif link var — .*2026.* tarihine kadar geçerli/)).toBeInTheDocument()
    expect(screen.queryByRole('textbox', { name: 'Davet linki' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Link oluştur' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Yenile' }))
    const dialog = await screen.findByRole('dialog', { name: 'Davet linki yenilensin mi?' })
    expect(within(dialog).getByText(/Eski link hemen çalışmayı bırakır/)).toBeInTheDocument()
    expect(posts).toBe(0)
    await user.click(within(dialog).getByRole('button', { name: 'Yenile' }))

    expect(await screen.findByDisplayValue(`${window.location.origin}/invite/${TOKEN}`)).toBeInTheDocument()
    expect(posts).toBe(1)
  })

  it('revokes only after confirmation', async () => {
    baseHandlers('admin')
    let revoked = false
    let deletes = 0
    server.use(
      http.get(api(`/groups/${group.id}/invite`), () =>
        revoked ? ok({ active: false }) : ok({ active: true, expires_at: '2026-11-20T10:00:00Z' }),
      ),
      http.delete(api(`/groups/${group.id}/invite`), () => {
        deletes++
        revoked = true
        return ok({ message: 'ok' })
      }),
    )
    const user = userEvent.setup()
    renderApp(PATH)

    await user.click(await screen.findByRole('button', { name: 'İptal et' }))
    const dialog = await screen.findByRole('dialog', { name: 'Davet linki iptal edilsin mi?' })
    await user.click(within(dialog).getByRole('button', { name: 'Vazgeç' }))
    expect(deletes).toBe(0)

    await user.click(screen.getByRole('button', { name: 'İptal et' }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'İptal et' }))
    expect(await screen.findByText('Aktif davet linki yok.')).toBeInTheDocument()
    expect(deletes).toBe(1)
  })

  it('shows API errors as an alert', async () => {
    baseHandlers('admin')
    server.use(
      http.get(api(`/groups/${group.id}/invite`), () => ok({ active: false })),
      http.post(api(`/groups/${group.id}/invite`), () => fail(500, 'Davet linki oluşturulurken bir hata oluştu')),
    )
    const user = userEvent.setup()
    renderApp(PATH)
    await user.click(await screen.findByRole('button', { name: 'Link oluştur' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Davet linki oluşturulurken bir hata oluştu')
  })

  function createHandlers(statusRef: { current: object }) {
    server.use(
      http.get(api(`/groups/${group.id}/invite`), () => ok(statusRef.current)),
      http.post(api(`/groups/${group.id}/invite`), () => {
        statusRef.current = { active: true, expires_at: '2026-12-01T10:00:00Z' }
        return ok({ token: TOKEN, expires_at: '2026-12-01T10:00:00Z' }, 201)
      }),
    )
  }

  it('never shows one group’s raw token in another group', async () => {
    baseHandlers('admin')
    createHandlers({ current: { active: false } })
    server.use(
      // Açık SSE akışı grup değişince iptal edilir (undici testte uyarı atar); akış kapalı olsun.
      http.get(api(`/groups/${group.id}/events`), () => fail(404, 'akış testte kapalı')),
      http.get(api('/groups/g-b/events'), () => fail(404, 'akış testte kapalı')),
      http.get(api('/groups/g-b'), () => ok(groupDetail({ my_role: 'admin', group: GROUP_B }))),
      http.get(api('/groups/g-b/ideas'), () => ok([])),
      http.get(api('/groups/g-b/invite'), () => ok({ active: true, expires_at: '2026-12-01T10:00:00Z' })),
    )
    const user = userEvent.setup()
    const { router } = renderApp(PATH)
    await user.click(await screen.findByRole('button', { name: 'Link oluştur' }))
    expect(await screen.findByDisplayValue(/\/invite\//)).toBeInTheDocument()

    await router.navigate('/groups/g-b')
    expect(await screen.findByRole('heading', { name: 'Başka grup' })).toBeInTheDocument()
    expect(await screen.findByText(/Aktif link var — /)).toBeInTheDocument()
    expect(screen.queryByDisplayValue(/\/invite\//)).not.toBeInTheDocument()
    expect(screen.queryByText(/yalnızca şimdi gösteriliyor/)).not.toBeInTheDocument()
  })

  it('drops the token when the status says the link was revoked or replaced elsewhere', async () => {
    baseHandlers('admin')
    const status: { current: object } = { current: { active: false } }
    createHandlers(status)
    const user = userEvent.setup()
    renderApp(PATH)
    await user.click(await screen.findByRole('button', { name: 'Link oluştur' }))
    expect(await screen.findByDisplayValue(/\/invite\//)).toBeInTheDocument()

    status.current = { active: false } // başka bir yönetici iptal etti
    window.dispatchEvent(new Event('visibilitychange'))
    expect(await screen.findByText('Aktif davet linki yok.')).toBeInTheDocument()
    expect(screen.queryByDisplayValue(/\/invite\//)).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Link oluştur' })).toBeInTheDocument()
  })

  it('drops the token when another admin replaced the link (different expiry)', async () => {
    baseHandlers('admin')
    const status: { current: object } = { current: { active: false } }
    createHandlers(status)
    const user = userEvent.setup()
    renderApp(PATH)
    await user.click(await screen.findByRole('button', { name: 'Link oluştur' }))
    await screen.findByDisplayValue(/\/invite\//)

    status.current = { active: true, expires_at: '2027-01-01T10:00:00Z' }
    window.dispatchEvent(new Event('visibilitychange'))
    await waitFor(() => expect(screen.queryByDisplayValue(/\/invite\//)).not.toBeInTheDocument())
    expect(screen.getByText(/Aktif link var — .*2027/)).toBeInTheDocument()
  })

  it('reports a clipboard failure in a live region', async () => {
    baseHandlers('admin')
    createHandlers({ current: { active: false } })
    const user = userEvent.setup()
    vi.spyOn(navigator.clipboard, 'writeText').mockRejectedValue(new Error('denied'))
    renderApp(PATH)
    await user.click(await screen.findByRole('button', { name: 'Link oluştur' }))
    await user.click(await screen.findByRole('button', { name: 'Kopyala' }))
    const live = await screen.findAllByText('Kopyalanamadı. Linki seçip elle kopyala.')
    expect(live.some((el) => el.getAttribute('role') === 'status')).toBe(true)
    expect(screen.getByRole('button', { name: 'Kopyala' })).toBeInTheDocument()
  })

  it('clears an old error once the other action succeeds', async () => {
    baseHandlers('admin')
    let failNext = true
    server.use(
      http.get(api(`/groups/${group.id}/invite`), () => ok({ active: true, expires_at: '2026-11-20T10:00:00Z' })),
      http.post(api(`/groups/${group.id}/invite`), () =>
        failNext ? fail(500, 'Davet linki oluşturulurken bir hata oluştu') : ok({ token: TOKEN, expires_at: '2026-11-20T10:00:00Z' }, 201),
      ),
      http.delete(api(`/groups/${group.id}/invite`), () => ok({ message: 'ok' })),
    )
    const user = userEvent.setup()
    renderApp(PATH)
    await user.click(await screen.findByRole('button', { name: 'Yenile' }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Yenile' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('oluşturulurken')
    failNext = false
    await user.click(screen.getByRole('button', { name: 'İptal et' }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'İptal et' }))
    await waitFor(() => expect(screen.queryByText(/oluşturulurken/)).not.toBeInTheDocument())
  })

  it('says "active" without a contradictory "no link" text when no expiry is known', async () => {
    baseHandlers('admin')
    server.use(http.get(api(`/groups/${group.id}/invite`), () => ok({ active: true })))
    renderApp(PATH)
    expect(await screen.findByText('Aktif link var')).toBeInTheDocument()
    expect(screen.queryByText('Aktif davet linki yok.')).not.toBeInTheDocument()
  })

  it('warns, without blocking, when the target account is not linked', async () => {
    baseHandlers('admin') // groupDetail varsayılanında target_account yok
    createHandlers({ current: { active: false } })
    renderApp(PATH)
    expect(await screen.findByText(/sürpriz koruması sağlamaz/)).toBeInTheDocument()
    expect(screen.getByText('Uyarı:')).toBeInTheDocument()
    expect(await screen.findByRole('button', { name: 'Link oluştur' })).toBeEnabled()
  })

  it('shows no warning when the target account is linked', async () => {
    server.use(
      http.get(api('/me'), () => ok(me)),
      http.get(api('/groups'), () => ok([group])),
      http.get(api(`/groups/${group.id}`), () =>
        ok(groupDetail({ my_role: 'admin', target_account: { id: 'u-kaan', name: 'Kaan' } })),
      ),
      http.get(api(`/groups/${group.id}/ideas`), () => ok([])),
      http.get(api(`/groups/${group.id}/events`), () => eventStream()),
    )
    renderApp(PATH)
    expect(await screen.findByText('Aktif davet linki yok.')).toBeInTheDocument()
    expect(screen.queryByText(/sürpriz koruması sağlamaz/)).not.toBeInTheDocument()
  })
})
