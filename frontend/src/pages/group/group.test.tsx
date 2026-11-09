import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'

import type { Group, Idea } from '@/api/types'
import { renderApp } from '@/test/render'
import { HttpResponse } from 'msw'

import { api, eventStream, fail, group, groupDetail, http, idea, me, ok, server } from '@/test/server'

const PATH = `/groups/${group.id}`

function baseHandlers(opts: { role?: 'admin' | 'member'; ideas?: Idea[]; group?: Partial<Group> } = {}) {
  server.use(
    http.get(api('/me'), () => ok(me)),
    http.get(api('/groups'), () => ok([group])),
    http.get(api(`/groups/${group.id}`), () =>
      ok(groupDetail({ my_role: opts.role ?? 'admin', group: { ...group, ...opts.group } })),
    ),
    http.get(api(`/groups/${group.id}/ideas`), () => ok(opts.ideas ?? [idea()])),
    http.get(api(`/groups/${group.id}/events`), () => eventStream()),
  )
}

describe('group page', () => {
  beforeEach(() => localStorage.setItem('covertgift.token', 'tok'))

  it('shows the surprise screen to the person the group is for (Anti-Spoiler 403)', async () => {
    server.use(
      http.get(api('/me'), () => ok(me)),
      http.get(api('/groups'), () => ok([])),
      http.get(api(`/groups/${group.id}`), () =>
        fail(403, 'Erişim engellendi: Bu hediye grubu senin için oluşturulmuş bir sürprizdir.'),
      ),
    )
    renderApp(PATH)
    expect(await screen.findByRole('heading', { name: 'Bu bir sürpriz!' })).toBeInTheDocument()
    expect(screen.queryByText('Hediye havuzu')).not.toBeInTheDocument()
  })

  it('renders the idea with encoded marketplace links and the per-person share', async () => {
    baseHandlers()
    renderApp(PATH)
    const card = (await screen.findByRole('heading', { level: 3, name: 'Stanley termos 1 L' })).closest('article')!
    expect(within(card).getByRole('link', { name: /Trendyol/ })).toHaveAttribute(
      'href',
      'https://www.trendyol.com/sr?q=Stanley%20termos%201%20lt',
    )
    expect(screen.getByText('₺1.500,00')).toBeInTheDocument()
  })

  it('labels AI prices without sources as estimates and shows only the first paragraph', async () => {
    baseHandlers({
      ideas: [
        idea({
          id: 'i-ai',
          title: 'AeroPress Go',
          source_type: 'AI',
          description: 'Kampta kahve için.\n\nWeb araması yapılmadı. Fiyat tahminidir.',
        }),
      ],
    })
    renderApp(PATH)
    const card = (await screen.findByRole('heading', { level: 3, name: 'AeroPress Go' })).closest('article')!
    expect(within(card).getByText('Tahmini fiyat')).toBeInTheDocument()
    expect(within(card).getByText('Kampta kahve için.')).toBeInTheDocument()
  })

  it('updates the score as soon as the user votes, then refetches', async () => {
    let voted: unknown = null
    let current = idea({ score: 2 })
    baseHandlers()
    server.use(
      http.get(api(`/groups/${group.id}/ideas`), () => ok([current])),
      http.post(api(`/ideas/${current.id}/vote`), async ({ request }) => {
        voted = await request.json()
        current = { ...current, score: 3, my_vote: 1 }
        return ok({ message: 'ok' })
      }),
    )
    renderApp(PATH)
    const user = userEvent.setup()

    const upvote = await screen.findByRole('button', { name: 'Beğen' })
    expect(screen.getByRole('group', { name: 'Skor 2' })).toBeInTheDocument()
    await user.click(upvote)

    expect(await screen.findByRole('group', { name: 'Skor 3' })).toBeInTheDocument()
    expect(upvote).toHaveAttribute('aria-pressed', 'true')
    expect(voted).toEqual({ vote_type: 1 })
  })

  it('rolls the vote back when the server rejects it', async () => {
    baseHandlers({ ideas: [idea({ score: 2 })] })
    server.use(http.post(api('/ideas/i-termos/vote'), () => fail(500, 'Oy kaydedilirken bir hata oluştu')))
    renderApp(PATH)
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Beğen' }))
    expect(await screen.findByText(/Oy kaydedilemedi/)).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('group', { name: 'Skor 2' })).toBeInTheDocument())
  })

  it('hides admin controls from a regular member', async () => {
    baseHandlers({ role: 'member' })
    renderApp(PATH)
    await screen.findByRole('heading', { level: 3, name: 'Stanley termos 1 L' })
    expect(screen.queryByRole('button', { name: 'Grup işlemleri' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Bütçeyi düzenle' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Kamp ilgi alanını sil' })).not.toBeInTheDocument()
    // Her üye ilgi alanı ekleyebilir.
    expect(screen.getByRole('button', { name: 'İlgi alanı ekle' })).toBeInTheDocument()
  })
})

describe('group management', () => {
  beforeEach(() => localStorage.setItem('covertgift.token', 'tok'))

  it('validates a new interest before calling the API, then adds it', async () => {
    let sent: unknown = null
    baseHandlers({ role: 'member' })
    server.use(
      http.post(api(`/groups/${group.id}/interests`), async ({ request }) => {
        sent = await request.json()
        return ok({ interests: ['Kamp', 'Filtre kahve', 'Trekking'] })
      }),
    )
    renderApp(PATH)
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'İlgi alanı ekle' }))
    const input = screen.getByRole('textbox', { name: 'Yeni ilgi alanı' })
    await user.type(input, 'Kamp, kahve{Enter}')
    expect(await screen.findByRole('alert')).toHaveTextContent('Tek bir ilgi alanı yaz')
    expect(sent).toBeNull()

    await user.clear(input)
    await user.type(input, 'Trekking{Enter}')
    await waitFor(() => expect(sent).toEqual({ interest: 'Trekking' }))
  })

  it('lets the admin edit the budget and rejects an invalid amount locally', async () => {
    let sent: unknown = null
    baseHandlers()
    server.use(
      http.patch(api(`/groups/${group.id}/budget`), async ({ request }) => {
        sent = await request.json()
        return ok({ message: 'Bütçe güncellendi' })
      }),
    )
    renderApp(PATH)
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Bütçeyi düzenle' }))
    const input = screen.getByRole('textbox', { name: 'Yeni toplam bütçe (₺)' })
    expect(input).toHaveValue('3000')

    await user.clear(input)
    await user.type(input, '12,345{Enter}')
    expect(await screen.findByRole('alert')).toHaveTextContent('en fazla iki ondalıklı')

    await user.clear(input)
    await user.type(input, '4500,50{Enter}')
    await waitFor(() => expect(sent).toEqual({ total_budget: '4500.50' }))
    expect(await screen.findByText('Bütçe güncellendi.')).toBeInTheDocument()
  })

  it('deletes the group after a confirmation that names it, then goes home', async () => {
    let deleted = false
    baseHandlers()
    server.use(
      http.delete(api(`/groups/${group.id}`), () => {
        deleted = true
        return ok({ message: 'silindi' })
      }),
    )
    const { router } = renderApp(PATH)
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Grup işlemleri' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Grubu sil' }))
    const dialog = await screen.findByRole('dialog')
    expect(dialog).toHaveTextContent('“Kaan’ın 30. yaşı”')
    expect(deleted).toBe(false)

    await user.click(within(dialog).getByRole('button', { name: 'Grubu sil' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/'))
    expect(deleted).toBe(true)
  })
})

describe('add idea dialog', () => {
  beforeEach(() => localStorage.setItem('covertgift.token', 'tok'))

  it('fills the form from a partial link preview and posts the edited idea', async () => {
    let posted: Record<string, unknown> | null = null
    baseHandlers()
    server.use(
      http.post(api(`/groups/${group.id}/ideas/preview-link`), () =>
        ok({
          title: 'Hepsiburada ana sayfa',
          description: '',
          image_url: '',
          price: '',
          final_url: 'https://www.hepsiburada.com/',
          search_query: 'hepsiburada',
          source: 'generic',
          partial: true,
        }),
      ),
      http.post(api(`/groups/${group.id}/ideas`), async ({ request }) => {
        posted = (await request.json()) as Record<string, unknown>
        return ok(idea({ id: 'i-new', title: String(posted.title) }), 201)
      }),
    )
    renderApp(PATH)
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Manuel ekle' }))
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByLabelText('Ürün linki'), 'https://www.hepsiburada.com/')
    await user.click(within(dialog).getByRole('button', { name: 'Bilgileri getir' }))

    expect(await within(dialog).findByRole('status')).toHaveTextContent('tamamı alınamadı')
    const name = within(dialog).getByLabelText('Ürün adı')
    expect(name).toHaveValue('Hepsiburada ana sayfa')

    await user.clear(name)
    await user.type(name, 'Hediye çeki')
    await user.type(within(dialog).getByLabelText('Tahmini fiyat (₺)'), '500')
    await user.click(within(dialog).getByRole('button', { name: 'Havuza ekle' }))

    await waitFor(() =>
      expect(posted).toMatchObject({
        title: 'Hediye çeki',
        estimated_price: '500',
        product_url: 'https://www.hepsiburada.com/',
      }),
    )
  })
})

describe('unvote, winner and gift date', () => {
  beforeEach(() => localStorage.setItem('covertgift.token', 'tok'))

  it('removes the vote when the selected vote is pressed again', async () => {
    let unvoted = false
    let current = idea({ score: 3, my_vote: 1 })
    baseHandlers()
    server.use(
      http.get(api(`/groups/${group.id}/ideas`), () => ok([current])),
      http.delete(api(`/ideas/${current.id}/vote`), () => {
        unvoted = true
        current = { ...current, score: 2, my_vote: 0 }
        return ok({ message: 'Oy kaldırıldı' })
      }),
    )
    renderApp(PATH)
    const user = userEvent.setup()

    const upvote = await screen.findByRole('button', { name: 'Beğen' })
    expect(upvote).toHaveAttribute('aria-pressed', 'true')
    await user.click(upvote)

    expect(await screen.findByRole('group', { name: 'Skor 2' })).toBeInTheDocument()
    expect(upvote).toHaveAttribute('aria-pressed', 'false')
    expect(unvoted).toBe(true)
  })

  it('lets the admin pick several winners, showing the total against the budget', async () => {
    let sent: unknown = null
    baseHandlers({
      ideas: [
        idea({ estimated_price: '1450.00' }),
        idea({ id: 'i-kahve', title: 'Kahve seti', estimated_price: '1100.00' }),
        idea({ id: 'i-baton', title: 'Trekking batonu', estimated_price: '850.00' }),
      ],
    })
    server.use(
      http.post(api(`/groups/${group.id}/winner`), async ({ request }) => {
        sent = await request.json()
        return ok({ message: 'Kazanan seçildi, grup tamamlandı' })
      }),
    )
    renderApp(PATH)
    const user = userEvent.setup()

    const card = (await screen.findByRole('heading', { level: 3, name: 'Stanley termos 1 L' })).closest('article')!
    await user.click(within(card).getByRole('button', { name: 'Fikir işlemleri' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Kazanan olarak seç' }))
    const dialog = await screen.findByRole('dialog')

    // Kartın fikri önceden işaretli gelir.
    expect(within(dialog).getByRole('checkbox', { name: /Stanley termos 1 L/ })).toBeChecked()
    expect(within(dialog).getByText('Seçilenlerin toplamı: ₺1.450,00')).toBeInTheDocument()

    await user.click(within(dialog).getByRole('checkbox', { name: /Kahve seti/ }))
    expect(within(dialog).getByText('Seçilenlerin toplamı: ₺2.550,00')).toBeInTheDocument()
    expect(within(dialog).queryByText('Seçilenler toplam bütçeyi aşıyor.')).not.toBeInTheDocument()

    // Bütçe 3000; üçüncüyle 3400 olur — uyarılır ama engellenmez.
    await user.click(within(dialog).getByRole('checkbox', { name: /Trekking batonu/ }))
    expect(within(dialog).getByText('Seçilenler toplam bütçeyi aşıyor.')).toBeInTheDocument()
    await user.click(within(dialog).getByRole('checkbox', { name: /Trekking batonu/ }))
    expect(sent).toBeNull()

    await user.click(within(dialog).getByRole('button', { name: '2 kazananı seç' }))
    await waitFor(() => expect(sent).toEqual({ idea_ids: ['i-termos', 'i-kahve'] }))
  })

  it('cannot confirm the winners dialog with nothing selected', async () => {
    baseHandlers()
    renderApp(PATH)
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Fikir işlemleri' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Kazanan olarak seç' }))
    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('checkbox', { name: /Stanley termos 1 L/ }))
    expect(within(dialog).getByRole('button', { name: /kazananı seç/i })).toBeDisabled()
  })

  it('shows a completed group read-only with the winners on top', async () => {
    baseHandlers({
      group: { status: 'COMPLETED', completed_at: '2026-11-09T10:00:00Z' },
      ideas: [
        idea({ id: 'i-other', title: 'Kahve seti', score: 5 }),
        idea({ status: 'SELECTED', score: 2 }),
        idea({ id: 'i-baton', title: 'Trekking batonu', status: 'SELECTED', score: 1 }),
      ],
    })
    renderApp(PATH)

    expect(await screen.findByText('Grup tamamlandı. Kazananlar: Stanley termos 1 L, Trekking batonu')).toBeInTheDocument()
    const cards = screen.getAllByRole('article')
    expect(within(cards[0]).getByText('Kazanan')).toBeInTheDocument()
    expect(within(cards[1]).getByText('Kazanan')).toBeInTheDocument()
    expect(within(cards[2]).queryByText('Kazanan')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Beğen' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Manuel ekle' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Fikir işlemleri' })).not.toBeInTheDocument()
  })

  it('reopens a completed group from the admin menu', async () => {
    let reopened = false
    baseHandlers({ group: { status: 'COMPLETED' } })
    server.use(
      http.delete(api(`/groups/${group.id}/winner`), () => {
        reopened = true
        return ok({ message: 'Grup yeniden açıldı' })
      }),
    )
    renderApp(PATH)
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Grup işlemleri' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Grubu yeniden aç' }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Yeniden aç' }))
    await waitFor(() => expect(reopened).toBe(true))
  })

  it('saves and removes the gift date', async () => {
    const sent: unknown[] = []
    baseHandlers({ group: { event_date: '2031-01-15' } })
    server.use(
      http.patch(api(`/groups/${group.id}/event-date`), async ({ request }) => {
        sent.push(await request.json())
        return ok({ message: 'Hediye tarihi güncellendi' })
      }),
    )
    renderApp(PATH)
    const user = userEvent.setup()

    expect(await screen.findByText('15 Ocak 2031')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Grup işlemleri' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Hediye tarihini ayarla' }))
    const dialog = await screen.findByRole('dialog')
    const input = within(dialog).getByLabelText('Tarih')
    expect(input).toHaveValue('2031-01-15')

    await user.clear(input)
    await user.type(input, '2031-02-20')
    await user.click(within(dialog).getByRole('button', { name: 'Kaydet' }))
    await waitFor(() => expect(sent).toEqual([{ event_date: '2031-02-20' }]))

    await user.click(screen.getByRole('button', { name: 'Grup işlemleri' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Hediye tarihini ayarla' }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Tarihi kaldır' }))
    await waitFor(() => expect(sent).toEqual([{ event_date: '2031-02-20' }, { event_date: null }]))
  })
})

describe('live updates (SSE)', () => {
  beforeEach(() => localStorage.setItem('covertgift.token', 'tok'))

  it('opens the stream with the bearer token and refetches the pool on a change event', async () => {
    let auth: string | null = null
    let push: ((frame: string) => void) | null = null
    let current = idea({ score: 1 })
    baseHandlers()
    server.use(
      http.get(api(`/groups/${group.id}/ideas`), () => ok([current])),
      http.get(api(`/groups/${group.id}/events`), ({ request }) => {
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
    renderApp(PATH)
    expect(await screen.findByRole('group', { name: 'Skor 1' })).toBeInTheDocument()
    await waitFor(() => expect(push).not.toBeNull())
    expect(auth).toBe('Bearer tok')

    // Başka bir üye oy verdi; sunucu açık akışa yalnızca "changed" yazar (veri taşımaz).
    // Periyodik yenileme (60 sn) bu sürede devreye girmez; skoru olay günceller.
    current = { ...current, score: 4 }
    push!('event: changed\ndata: {}\n\n')
    expect(await screen.findByRole('group', { name: 'Skor 4' }, { timeout: 3000 })).toBeInTheDocument()
  })

  it('never opens the stream for the person the group is for', async () => {
    let opened = false
    server.use(
      http.get(api('/me'), () => ok(me)),
      http.get(api('/groups'), () => ok([])),
      http.get(api(`/groups/${group.id}`), () =>
        fail(403, 'Erişim engellendi: Bu hediye grubu senin için oluşturulmuş bir sürprizdir.'),
      ),
      http.get(api(`/groups/${group.id}/events`), () => {
        opened = true
        return eventStream()
      }),
    )
    renderApp(PATH)
    await screen.findByRole('heading', { name: 'Bu bir sürpriz!' })
    await new Promise((r) => setTimeout(r, 100))
    expect(opened).toBe(false)
  })
})

