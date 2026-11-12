import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { HttpResponse } from 'msw'
import { beforeEach, describe, expect, it } from 'vitest'

import type { AppNotification, NotificationList } from '@/api/types'
import i18n from '@/i18n'
import { badgeLabel, notificationText } from '@/lib/notifications'
import { renderApp } from '@/test/render'
import { api, eventStream, group, groupDetail, http, idea, me, ok, server } from '@/test/server'

const note = (overrides: Partial<AppNotification> = {}): AppNotification => ({
  id: 'n-1',
  group_id: group.id,
  group_title: group.title,
  type: 'idea_added',
  params: { actor_name: 'Elif Kara', idea_title: 'Kamp sandalyesi' },
  created_at: new Date().toISOString(),
  ...overrides,
})

const unreadItems = [
  note(),
  note({ id: 'n-2', type: 'budget_changed', params: { actor_name: 'Elif Kara', total_budget: '4000.00' } }),
  note({ id: 'n-3', type: 'event_reminder', params: { days_left: 3, event_date: '2026-12-01' }, read_at: '2026-11-11T10:00:00Z' }),
]

// Sunucudaki bildirim durumunu taklit eder: okundu istekleri listeyi değiştirir.
function notificationServer(initial: AppNotification[]) {
  const state = { items: initial, calls: [] as string[] }
  const list = (): NotificationList => ({
    items: state.items,
    unread_count: state.items.filter((n) => !n.read_at).length,
  })
  server.use(
    http.get(api('/me'), () => ok(me)),
    http.get(api('/groups'), () => ok([])),
    http.get(api('/notifications'), () => ok(list())),
    http.post(api('/notifications/read-all'), () => {
      state.calls.push('read-all')
      state.items = state.items.map((n) => ({ ...n, read_at: n.read_at ?? new Date().toISOString() }))
      return ok({ message: 'ok' })
    }),
    http.post(api('/notifications/:id/read'), ({ params }) => {
      state.calls.push(`read:${String(params.id)}`)
      state.items = state.items.map((n) => (n.id === params.id ? { ...n, read_at: new Date().toISOString() } : n))
      return ok({ message: 'ok' })
    }),
  )
  return state
}

describe('notification bell', () => {
  beforeEach(() => localStorage.setItem('covertgift.token', 'tok'))

  it('shows the unread count on the bell and lists notifications with unread ones marked', async () => {
    notificationServer(unreadItems)
    renderApp('/')
    const user = userEvent.setup()

    // Sayaç hem düğmenin adında hem (yalnızca görsel) rozette.
    const bell = await screen.findByRole('button', { name: 'Bildirimler, 2 okunmamış' })
    expect(within(bell).getByText('2')).toHaveAttribute('aria-hidden', 'true')
    expect(screen.getByRole('status')).toHaveTextContent('Bildirimler, 2 okunmamış')

    await user.click(bell)
    const menu = await screen.findByRole('menu')
    const rows = within(menu).getAllByRole('menuitem')
    // İlk öğe "Tümünü okundu yap", ardından üç bildirim (yeniden eskiye).
    expect(rows).toHaveLength(4)
    expect(rows[1]).toHaveTextContent('Elif Kara yeni bir fikir ekledi: Kamp sandalyesi')
    expect(rows[1]).toHaveTextContent(group.title)
    expect(rows[2]).toHaveTextContent('Elif Kara bütçeyi ₺4.000,00 yaptı')
    expect(rows[3]).toHaveTextContent('Hediye gününe 3 gün kaldı (1 Aralık 2026)')
    expect(within(rows[1]).getByText('Okunmamış')).toBeInTheDocument()
    expect(within(rows[2]).getByText('Okunmamış')).toBeInTheDocument()
    expect(within(rows[3]).queryByText('Okunmamış')).not.toBeInTheDocument()
  })

  it('marks a notification read and opens its group when clicked', async () => {
    const state = notificationServer(unreadItems)
    server.use(
      http.get(api(`/groups/${group.id}`), () => ok(groupDetail())),
      http.get(api(`/groups/${group.id}/ideas`), () => ok([idea()])),
      http.get(api(`/groups/${group.id}/events`), () => eventStream()),
    )
    const { router } = renderApp('/')
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Bildirimler, 2 okunmamış' }))
    await user.click(await screen.findByRole('menuitem', { name: /Elif Kara yeni bir fikir ekledi/ }))

    await waitFor(() => expect(router.state.location.pathname).toBe(`/groups/${group.id}`))
    await waitFor(() => expect(state.calls).toEqual(['read:n-1']))
    expect(await screen.findByRole('button', { name: 'Bildirimler, 1 okunmamış' })).toBeInTheDocument()
  })

  it('does not call the API again for an already read notification', async () => {
    const state = notificationServer(unreadItems)
    server.use(
      http.get(api(`/groups/${group.id}`), () => ok(groupDetail())),
      http.get(api(`/groups/${group.id}/ideas`), () => ok([idea()])),
      http.get(api(`/groups/${group.id}/events`), () => eventStream()),
    )
    const { router } = renderApp('/')
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Bildirimler, 2 okunmamış' }))
    await user.click(await screen.findByRole('menuitem', { name: /Hediye gününe 3 gün kaldı/ }))

    await waitFor(() => expect(router.state.location.pathname).toBe(`/groups/${group.id}`))
    expect(state.calls).toEqual([])
  })

  it('marks everything read, clears the badge and keeps the list open', async () => {
    const state = notificationServer(unreadItems)
    renderApp('/')
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Bildirimler, 2 okunmamış' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Tümünü okundu yap' }))

    await waitFor(() => expect(state.calls).toEqual(['read-all']))
    const menu = screen.getByRole('menu')
    await waitFor(() => expect(within(menu).queryByText('Okunmamış')).not.toBeInTheDocument())
    expect(within(menu).queryByRole('menuitem', { name: 'Tümünü okundu yap' })).not.toBeInTheDocument()
    expect(within(menu).getAllByRole('menuitem')).toHaveLength(3)
    // Menü açıkken arka plan aria-hidden olduğu için zil düğmesi etiketiyle aranır.
    expect(screen.getByLabelText('Bildirimler', { selector: 'button' })).toBeInTheDocument()
    expect(screen.queryByLabelText(/okunmamış/, { selector: 'button' })).not.toBeInTheDocument()
  })

  it('shows an empty state and no badge when there is nothing', async () => {
    notificationServer([])
    renderApp('/')
    const user = userEvent.setup()

    const bell = await screen.findByRole('button', { name: 'Bildirimler' })
    await waitFor(() => expect(screen.getByRole('status')).toBeEmptyDOMElement())
    await user.click(bell)
    expect(await screen.findByText('Henüz bildirimin yok.')).toBeInTheDocument()
    expect(screen.queryByRole('menuitem', { name: 'Tümünü okundu yap' })).not.toBeInTheDocument()
  })

  it('refetches when the stream says "changed" and sends the token in a header', async () => {
    const state = notificationServer([])
    let auth: string | null = null
    let push: ((frame: string) => void) | null = null
    server.use(
      http.get(api('/notifications/stream'), ({ request }) => {
        auth = request.headers.get('Authorization')
        const encoder = new TextEncoder()
        const body = new ReadableStream<Uint8Array>({
          start(controller) {
            controller.enqueue(encoder.encode(': connected\n\n'))
            push = (frame) => controller.enqueue(encoder.encode(frame))
          },
        })
        return new HttpResponse(body, { headers: { 'Content-Type': 'text/event-stream' } })
      }),
    )
    renderApp('/')
    await screen.findByRole('button', { name: 'Bildirimler' })
    await waitFor(() => expect(push).not.toBeNull())
    expect(auth).toBe('Bearer tok')

    // Sunucu yalnızca "changed" yazar (veri taşımaz); yeni bildirim listeden gelir.
    state.items = [note()]
    push!('event: changed\ndata: {}\n\n')
    expect(await screen.findByRole('button', { name: 'Bildirimler, 1 okunmamış' }, { timeout: 3000 })).toBeInTheDocument()
  })

  it('reconnects after the stream closes and catches up on what it missed', async () => {
    const state = notificationServer([])
    let opened = 0
    server.use(
      http.get(api('/notifications/stream'), () => {
        opened++
        // İlk akış hemen kapanır (sunucu ömrü doldu); ikincisi açık kalır.
        return eventStream([], { close: opened === 1 })
      }),
    )
    renderApp('/')
    await screen.findByRole('button', { name: 'Bildirimler' })
    await waitFor(() => expect(opened).toBe(1))

    state.items = [note()] // bağlantı kopukken gelen bildirim
    await waitFor(() => expect(opened).toBe(2), { timeout: 3000 })
    expect(await screen.findByRole('button', { name: 'Bildirimler, 1 okunmamış' }, { timeout: 3000 })).toBeInTheDocument()
  })
})

describe('notification text', () => {
  const text = (type: string, params: AppNotification['params']) => notificationText(note({ type, params }))

  it('builds Turkish messages for every type', () => {
    expect(text('ai_ideas_added', { actor_name: 'Elif', count: 4 })).toBe('Elif yapay zekâdan 4 fikir aldı')
    expect(text('added_to_group', { actor_name: 'Elif' })).toBe('Elif seni bu gruba ekledi')
    expect(text('member_joined', { actor_name: 'Elif', member_name: 'Can' })).toBe('Elif, Can adlı kişiyi gruba ekledi')
    expect(text('winner_selected', { actor_name: 'Elif', count: 1 })).toBe('Elif kazananı seçti')
    expect(text('winner_selected', { actor_name: 'Elif', count: 3 })).toBe('Elif 3 kazanan seçti')
    expect(text('group_reopened', { actor_name: 'Elif' })).toBe('Elif grubu yeniden açtı')
    expect(text('event_date_changed', { actor_name: 'Elif', event_date: '2026-12-24' })).toBe(
      'Elif hediye tarihini 24 Aralık 2026 yaptı',
    )
    expect(text('event_date_changed', { actor_name: 'Elif', event_date: null })).toBe('Elif hediye tarihini kaldırdı')
    expect(text('event_reminder', { days_left: 1, event_date: '2026-12-24' })).toBe('Hediye günü yarın (24 Aralık 2026)')
    expect(text('event_reminder', { days_left: 0, event_date: '2026-12-24' })).toBe('Hediye günü bugün (24 Aralık 2026)')
    expect(text('winner_overdue', { event_date: '2026-12-24' })).toBe(
      'Hediye günü geçti (24 Aralık 2026), kazanan hâlâ seçilmedi',
    )
  })

  it('falls back for a missing actor and an unknown type, and follows the language', async () => {
    expect(text('group_reopened', {})).toBe('Bir üye grubu yeniden açtı')
    expect(text('something_new', {})).toBe('Grupta bir değişiklik oldu')
    await i18n.changeLanguage('en')
    expect(text('winner_selected', { actor_name: 'Elif', count: 2 })).toBe('Elif picked 2 winners')
    expect(text('event_reminder', { days_left: 2, event_date: '2026-12-24' })).toBe(
      '2 days until the gift day (24 December 2026)',
    )
  })

  it('caps the badge at 9+', () => {
    expect(badgeLabel(3)).toBe('3')
    expect(badgeLabel(9)).toBe('9')
    expect(badgeLabel(27)).toBe('9+')
  })
})
