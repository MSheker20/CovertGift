import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import type { AppNotification, GroupListItem, PoolDetail, PoolView } from '@/api/types'
import { notificationPath, notificationText } from '@/lib/notifications'
import { renderApp } from '@/test/render'
import { api, eventStream, fail, group, groupDetail, http, idea, me, ok, server } from '@/test/server'
import { can, elif, lockedPool, multiDetail, multiGroup, openPool, poolDetail } from '@/test/multi'

const G = `/groups/${multiGroup.id}`
const POOL = `${G}/pools/p-elif`

// Her istek yolu kaydedilir: kendi havuzuma hiç istek atılmadığını kanıtlamak için.
let seen: string[] = []
beforeEach(() => {
  localStorage.setItem('covertgift.token', 'tok')
  seen = []
  server.events.on('request:start', ({ request }) => {
    seen.push(new URL(request.url).pathname)
  })
})
afterEach(() => server.events.removeAllListeners())

type Setup = {
  pools?: PoolView[]
  role?: 'admin' | 'member'
  detail?: () => PoolDetail
  ideas?: ReturnType<typeof idea>[]
}

// Grup + açık havuz uçları. Kendi havuzum (p-me) için hiçbir işleyici yoktur: istek atılırsa MSW hata verir.
function multiHandlers({ pools = [openPool(), lockedPool()], role = 'admin', detail, ideas }: Setup = {}) {
  server.use(
    http.get(api('/me'), () => ok(me)),
    http.get(api('/groups'), () => ok([])),
    http.get(api(G), () => ok(multiDetail(pools, { my_role: role }))),
    http.get(api(`${G}/events`), () => eventStream()),
    http.get(api(`${POOL}`), () => ok(detail ? detail() : poolDetail())),
    http.get(api(`${POOL}/ideas`), () => ok(ideas ?? [idea({ id: 'i-1', title: 'Kamp sandalyesi', pool_id: 'p-elif' })])),
  )
}

describe('new group dialog: mode', () => {
  it('posts mode MULTI without target, budget, interests or date, and hides those fields', async () => {
    let body: Record<string, unknown> | null = null
    server.use(
      http.get(api('/me'), () => ok(me)),
      http.get(api('/groups'), () => ok([])),
      http.post(api('/groups'), async ({ request }) => {
        body = (await request.json()) as Record<string, unknown>
        return ok({ ...multiGroup, id: 'g-new' }, 201)
      }),
      http.get(api('/groups/g-new'), () => ok(multiDetail([], { group: { ...multiGroup, id: 'g-new' } }))),
      http.get(api('/groups/g-new/events'), () => eventStream()),
    )
    renderApp('/')
    const user = userEvent.setup()

    await user.click((await screen.findAllByRole('button', { name: 'Yeni grup' }))[0])
    const dialog = await screen.findByRole('dialog')
    const radios = within(dialog).getByRole('group', { name: 'Grup türü' })
    expect(radios).toHaveAccessibleDescription(/Bir kişi için ortak bütçe/)
    // Varsayılan SINGLE: hedef alanları görünür.
    expect(within(radios).getByRole('radio', { name: 'Tek kişi için sürpriz' })).toBeChecked()
    expect(within(dialog).getByLabelText('Hediye kimin için?')).toBeInTheDocument()

    await user.click(within(radios).getByRole('radio', { name: 'Birden çok kişi için hediye' }))
    expect(within(dialog).queryByLabelText('Hediye kimin için?')).not.toBeInTheDocument()
    expect(within(dialog).queryByLabelText('Toplam bütçe (₺)')).not.toBeInTheDocument()
    expect(within(dialog).queryByLabelText(/Hedef kişinin e-postası/)).not.toBeInTheDocument()
    expect(within(dialog).queryByLabelText(/İlgi alanları/)).not.toBeInTheDocument()
    expect(within(dialog).queryByLabelText(/Hediye tarihi/)).not.toBeInTheDocument()
    expect(within(dialog).getByText(/Hediye alınacak kişileri üyeler katıldıktan sonra seçersin/)).toBeInTheDocument()

    await user.type(within(dialog).getByLabelText('Grup adı'), 'Ofis yılbaşı')
    await user.click(within(dialog).getByRole('button', { name: 'Grubu kur' }))
    await waitFor(() => expect(body).toEqual({ title: 'Ofis yılbaşı', mode: 'MULTI' }))
  })

  it('keeps the SINGLE request exactly as before', async () => {
    let body: Record<string, unknown> | null = null
    server.use(
      http.get(api('/me'), () => ok(me)),
      http.get(api('/groups'), () => ok([])),
      http.post(api('/groups'), async ({ request }) => {
        body = (await request.json()) as Record<string, unknown>
        return ok(group, 201)
      }),
      http.get(api(`/groups/${group.id}`), () => ok(groupDetail())),
      http.get(api(`/groups/${group.id}/ideas`), () => ok([])),
      http.get(api(`/groups/${group.id}/events`), () => eventStream()),
    )
    renderApp('/')
    const user = userEvent.setup()
    await user.click((await screen.findAllByRole('button', { name: 'Yeni grup' }))[0])
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByLabelText('Grup adı'), 'Kaan’ın 30. yaşı')
    await user.type(within(dialog).getByLabelText('Hediye kimin için?'), 'Kaan')
    await user.click(within(dialog).getByRole('button', { name: 'Grubu kur' }))
    await waitFor(() => expect(body).toEqual({ title: 'Kaan’ın 30. yaşı', target_person_name: 'Kaan', total_budget: '0' }))
  })
})

describe('MULTI group page', () => {
  it('renders a tab per recipient, says how many people, and keeps the selection in ?pool=', async () => {
    multiHandlers({ pools: [openPool(), openPool({ id: 'p-can', recipient: can }), lockedPool()] })
    server.use(
      http.get(api(`${G}/pools/p-can`), () => ok(poolDetail({ id: 'p-can', recipient: can }))),
      http.get(api(`${G}/pools/p-can/ideas`), () => ok([])),
    )
    const { router } = renderApp(G)
    const user = userEvent.setup()

    expect(await screen.findByText(/3 kişi için/)).toBeInTheDocument()
    const tabs = within(screen.getByRole('tablist', { name: 'Hediye alınacak kişiler' })).getAllByRole('tab')
    expect(tabs.map((t) => t.textContent)).toEqual(['Elif Kara', 'Can Er', 'Deniz Aksoy (sen)'])
    expect(tabs[0]).toHaveAttribute('aria-selected', 'true')
    expect(tabs[0]).toHaveAttribute('tabindex', '0')
    expect(tabs[1]).toHaveAttribute('tabindex', '-1')
    expect(await screen.findByRole('heading', { level: 2, name: 'Elif Kara için' })).toBeInTheDocument()
    expect(await screen.findByRole('heading', { level: 3, name: 'Kamp sandalyesi' })).toBeInTheDocument()

    // Tıklama seçer ve adresi yazar. Ok tuşları yalnızca odağı taşır; Enter/Space seçer.
    await user.click(tabs[1])
    expect(router.state.location.search).toBe('?pool=p-can')
    expect(await screen.findByRole('heading', { level: 2, name: 'Can Er için' })).toBeInTheDocument()
    expect(screen.getByRole('tabpanel')).toHaveAttribute('tabindex', '0')
    await user.keyboard('{ArrowLeft}')
    expect(tabs[0]).toHaveFocus()
    expect(tabs[0]).toHaveAttribute('tabindex', '0')
    expect(router.state.location.search).toBe('?pool=p-can')
    await user.keyboard('{Enter}')
    await waitFor(() => expect(router.state.location.search).toBe('?pool=p-elif'))
    await user.keyboard('{End}')
    expect(tabs[2]).toHaveFocus()
    expect(router.state.location.search).toBe('?pool=p-elif')
    await user.keyboard(' ')
    await waitFor(() => expect(router.state.location.search).toBe('?pool=p-me'))
    expect(screen.getByRole('tabpanel')).toHaveAttribute('aria-labelledby', 'pool-tab-p-me')
    expect(seen.filter((p) => p.includes('p-me'))).toEqual([])
  })

  it('shows only a locked card for my own pool and never requests anything for it', async () => {
    multiHandlers()
    renderApp(`${G}?pool=p-me`)

    expect(await screen.findByRole('heading', { name: 'Senin için bir sürpriz hazırlanıyor' })).toBeInTheDocument()
    // Sayfa tam yüklendi (üyeler paneli geldi); yine de havuza özgü hiçbir şey yok.
    await screen.findByRole('heading', { name: 'Üyeler' })
    for (const text of ['Hediye havuzu', 'Kişi başı', 'Toplam bütçe', 'Sorumlu', 'Katılımcılar']) {
      expect(screen.queryByText(new RegExp(text))).not.toBeInTheDocument()
    }
    await new Promise((r) => setTimeout(r, 150))
    expect(seen.filter((p) => p.includes('p-me'))).toEqual([])
    // Başka havuzun uçları da çağrılmaz: seçili olan kendi havuzum.
    expect(seen.filter((p) => p.includes('/pools/'))).toEqual([])
  })

  it('falls back to the first open pool when ?pool= is unknown and never opens my own pool by default', async () => {
    multiHandlers({ pools: [lockedPool(), openPool()] })
    renderApp(`${G}?pool=p-yok`)
    expect(await screen.findByRole('heading', { level: 2, name: 'Elif Kara için' })).toBeInTheDocument()
    expect(seen.filter((p) => p.includes('p-me'))).toEqual([])
  })

  it('does not show the "target not linked" warning in the invite section', async () => {
    multiHandlers()
    renderApp(G)
    await screen.findByRole('heading', { level: 2, name: 'Elif Kara için' })
    expect(await screen.findByText('Davet linki')).toBeInTheDocument()
    expect(screen.queryByText(/Hesap bağlı değil/)).not.toBeInTheDocument()
    expect(screen.queryByText(/hesabı bağlı değil/)).not.toBeInTheDocument()
  })

  it('shows the per-person share with the contributor count, and the no-contributors state when null', async () => {
    multiHandlers()
    renderApp(G)
    expect(await screen.findByText('₺100,00')).toBeInTheDocument()
    expect(screen.getByTestId('per-person-caption')).toHaveTextContent('3 katılımcıya bölünür')
  })

  it('shows a clear state instead of an amount when nobody contributes', async () => {
    multiHandlers({
      pools: [openPool({ per_person_budget: null, no_contributors: true, contributor_count: 0 })],
      detail: () => poolDetail({ per_person_budget: null, no_contributors: true, contributor_count: 0 }),
    })
    renderApp(G)
    expect(await screen.findByTestId('per-person-caption')).toHaveTextContent('Kimse katılmıyor')
    expect(screen.queryByText('₺0,00')).not.toBeInTheDocument()
  })

  it('shows a "no manager" banner to members when the admin’s pool has nobody in charge', async () => {
    multiHandlers({
      role: 'member',
      pools: [openPool({ manager: null, can_manage: false })],
      detail: () => poolDetail({ manager: null, can_manage: false }),
    })
    renderApp(G)
    expect(await screen.findByText(/Bu havuzun sorumlusu yok/)).toBeInTheDocument()
  })
})

describe('contribution switch', () => {
  it('toggles my own contribution and updates the share from the response', async () => {
    let sent: unknown = null
    let current = poolDetail()
    multiHandlers()
    server.use(
      http.get(api(POOL), () => ok(current)),
      http.put(api(`${POOL}/contribution`), async ({ request }) => {
        sent = await request.json()
        current = poolDetail({ my_contribution: false, contributor_count: 2, per_person_budget: '150' })
        return ok({ contributing: false, contributor_count: 2, per_person_budget: '150', no_contributors: false })
      }),
    )
    renderApp(G)
    const user = userEvent.setup()

    const toggle = await screen.findByRole('switch', { name: 'Bu hediyeye ortağım' }, { timeout: 3000 })
    expect(toggle).toBeChecked()
    await user.click(toggle)
    await waitFor(() => expect(sent).toEqual({ contributing: false }), { timeout: 3000 })
    await waitFor(() => expect(screen.getByRole('switch', { name: 'Bu hediyeye ortağım' })).not.toBeChecked(), { timeout: 3000 })
    expect(await screen.findByText('₺150,00', {}, { timeout: 3000 })).toBeInTheDocument()
    expect(screen.getByTestId('per-person-caption')).toHaveTextContent('2 katılımcıya bölünür')
  })

  it('rolls back on 409 and tells the user why', async () => {
    multiHandlers()
    server.use(
      http.put(api(`${POOL}/contribution`), () =>
        fail(409, 'havuz tamamlandı; hediye fikirleri, oylar ve katkı değişikliği kapalı'),
      ),
    )
    renderApp(G)
    const user = userEvent.setup()

    await user.click(await screen.findByRole('switch', { name: 'Bu hediyeye ortağım' }))
    expect(await screen.findByText(/katkı değişikliği kapalı/)).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('switch', { name: 'Bu hediyeye ortağım' })).toBeChecked())
    expect(screen.getByTestId('per-person-caption')).toHaveTextContent('3 katılımcıya bölünür')
    expect(screen.getByText('₺100,00')).toBeInTheDocument()
  })

  it('is disabled with an explanation while the pool is completed', async () => {
    multiHandlers({
      pools: [openPool({ status: 'COMPLETED' })],
      detail: () => poolDetail({ status: 'COMPLETED' }),
      ideas: [idea({ id: 'i-w', title: 'Kamp sandalyesi', status: 'SELECTED' })],
    })
    renderApp(G)
    const toggle = await screen.findByRole('switch', { name: 'Bu hediyeye ortağım' })
    expect(toggle).toBeDisabled()
    expect(toggle).toHaveAccessibleDescription(/Havuz tamamlandı; katkı değişikliği kapalı/)
    expect(await screen.findByText('Havuz tamamlandı. Kazanan: Kamp sandalyesi')).toBeInTheDocument()
  })
})

describe('manager controls follow can_manage, not the group role', () => {
  it('a pool manager who is only a member sees the manage controls', async () => {
    multiHandlers({ role: 'member', pools: [openPool({ can_manage: true })], detail: () => poolDetail({ can_manage: true }) })
    renderApp(G)
    expect(await screen.findByRole('button', { name: 'Bütçeyi düzenle' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Havuz işlemleri' })).toBeInTheDocument()
    expect(await screen.findByRole('button', { name: 'Kamp ilgi alanını sil' })).toBeInTheDocument()
    // Grup düzeyi yönetim hâlâ adminin: grup menüsü ve alıcı işlemleri yok.
    expect(screen.queryByRole('button', { name: 'Grup işlemleri' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /için işlemler/ })).not.toBeInTheDocument()
  })

  it('a plain member does not', async () => {
    multiHandlers({
      role: 'member',
      pools: [openPool({ can_manage: false })],
      detail: () => poolDetail({ can_manage: false }),
      ideas: [idea({ id: 'i-1', title: 'Kamp sandalyesi', pool_id: 'p-elif', created_by: elif.id })],
    })
    renderApp(G)
    await screen.findByRole('heading', { level: 3, name: 'Kamp sandalyesi' })
    await screen.findByText('Kamp')
    expect(screen.queryByRole('button', { name: 'Bütçeyi düzenle' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Havuz işlemleri' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Kamp ilgi alanını sil' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Fikir işlemleri' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Manuel ekle' })).toBeInTheDocument()
  })

  it('edits the pool budget through the pool route', async () => {
    let sent: unknown = null
    multiHandlers()
    server.use(
      http.patch(api(`${POOL}/budget`), async ({ request }) => {
        sent = await request.json()
        return ok({ message: 'Bütçe güncellendi' })
      }),
    )
    renderApp(G)
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Bütçeyi düzenle' }))
    const input = screen.getByRole('textbox', { name: 'Yeni toplam bütçe (₺)' })
    await user.clear(input)
    await user.type(input, '450{Enter}')
    await waitFor(() => expect(sent).toEqual({ total_budget: '450' }))
  })

  it('votes and suggests ideas inside the pool, never through the group routes', async () => {
    let voted = false
    multiHandlers()
    server.use(
      http.post(api('/ideas/i-1/vote'), () => {
        voted = true
        return ok({ message: 'ok' })
      }),
    )
    renderApp(G)
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Beğen' }))
    await waitFor(() => expect(voted).toBe(true))
    expect(seen.filter((p) => p === `/api/v1${G}/ideas`)).toEqual([])
  })
})

describe('recipient management (admin)', () => {
  it('blocks choosing yourself until another member is chosen as manager', async () => {
    let sent: unknown = null
    multiHandlers({ pools: [openPool()] })
    server.use(
      http.post(api(`${G}/recipients`), async ({ request }) => {
        sent = await request.json()
        return ok({ pool: lockedPool() }, 201)
      }),
    )
    renderApp(G)
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Deniz Aksoy için işlemler' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Hediye alınacak kişi yap' }))
    const dialog = await screen.findByRole('dialog')
    expect(dialog).toHaveTextContent('havuzu yönetecek başka bir üye')
    const select = within(dialog).getByRole('combobox', { name: 'Sorumlu üye' })
    // Kendisi listede yok.
    expect(within(select).queryByRole('option', { name: 'Deniz Aksoy' })).not.toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: 'Hediye alınacak kişi yap' }))
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('Havuzu yönetecek bir üye seçmelisin')
    expect(sent).toBeNull()

    await user.selectOptions(select, 'Elif Kara')
    await user.click(within(dialog).getByRole('button', { name: 'Hediye alınacak kişi yap' }))
    await waitFor(() => expect(sent).toEqual({ user_id: me.id, manager_user_id: elif.id }))
  })

  it('makes another member a recipient without asking for a manager', async () => {
    let sent: unknown = null
    multiHandlers({ pools: [lockedPool()] })
    server.use(
      http.post(api(`${G}/recipients`), async ({ request }) => {
        sent = await request.json()
        return ok({ pool: openPool() }, 201)
      }),
    )
    renderApp(G)
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Can Er için işlemler' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Hediye alınacak kişi yap' }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).queryByRole('combobox')).not.toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Hediye alınacak kişi yap' }))
    await waitFor(() => expect(sent).toEqual({ user_id: can.id }))
  })

  it('changes the manager of my own pool', async () => {
    let sent: unknown = null
    multiHandlers()
    server.use(
      http.patch(api(`${G}/recipients/${me.id}/manager`), async ({ request }) => {
        sent = await request.json()
        return ok({ pool_id: 'p-me' })
      }),
    )
    renderApp(G)
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Deniz Aksoy için işlemler' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Sorumluyu değiştir' }))
    const dialog = await screen.findByRole('dialog')
    await user.selectOptions(within(dialog).getByRole('combobox'), 'Can Er')
    await user.click(within(dialog).getByRole('button', { name: 'Kaydet' }))
    await waitFor(() => expect(sent).toEqual({ manager_user_id: can.id }))
  })

  it('preselects the current manager of my own pool when changing it', async () => {
    multiHandlers({ pools: [openPool(), lockedPool({ manager: { id: can.id, name: can.name } })] })
    renderApp(G)
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Deniz Aksoy için işlemler' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Sorumluyu değiştir' }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByRole('combobox')).toHaveValue(can.id)
  })

  it('on 409 asks for confirmation, then resends with confirm=true', async () => {
    const calls: (string | null)[] = []
    multiHandlers()
    server.use(
      http.delete(api(`${G}/recipients/${elif.id}`), ({ request }) => {
        const confirm = new URL(request.url).searchParams.get('confirm')
        calls.push(confirm)
        return confirm === 'true'
          ? ok({ message: 'Hediye alınacak kişi ve havuzu kaldırıldı' })
          : fail(409, 'bu havuzda hediye fikirleri olabilir; kaldırmak için onaylayın (confirm=true)')
      }),
    )
    renderApp(G)
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Elif Kara için işlemler' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Hediye alınacak kişilikten çıkar' }))
    const dialog = await screen.findByRole('dialog')
    expect(dialog).toHaveTextContent('fikirler ve oylar kalıcı olarak silinir')
    expect(calls).toEqual([null])

    await user.click(within(dialog).getByRole('button', { name: 'Havuzu sil' }))
    await waitFor(() => expect(calls).toEqual([null, 'true']))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('removes a pool without ideas straight away (no confirm needed)', async () => {
    const calls: (string | null)[] = []
    multiHandlers()
    server.use(
      http.delete(api(`${G}/recipients/${elif.id}`), ({ request }) => {
        calls.push(new URL(request.url).searchParams.get('confirm'))
        return ok({ message: 'ok' })
      }),
    )
    renderApp(G)
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Elif Kara için işlemler' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Hediye alınacak kişilikten çıkar' }))
    await waitFor(() => expect(calls).toEqual([null]))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('always sends confirm=true for my own pool after a generic confirmation, without trying first', async () => {
    const calls: (string | null)[] = []
    multiHandlers()
    server.use(
      http.delete(api(`${G}/recipients/${me.id}`), ({ request }) => {
        calls.push(new URL(request.url).searchParams.get('confirm'))
        return ok({ message: 'ok' })
      }),
    )
    renderApp(G)
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Deniz Aksoy için işlemler' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Hediye alınacak kişilikten çıkar' }))
    const dialog = await screen.findByRole('dialog')
    // Genel metin: fikir olup olmadığı hakkında hiçbir şey söylemez.
    expect(dialog).toHaveTextContent('Havuz ve içindeki her şey kalıcı olarak silinir')
    expect(dialog).not.toHaveTextContent(/fikirler ve oylar/)
    expect(calls).toEqual([])
    await user.click(within(dialog).getByRole('button', { name: 'Havuzu sil' }))
    await waitFor(() => expect(calls).toEqual(['true']))
  })

  it('hides recipient actions from non-admin members', async () => {
    multiHandlers({ role: 'member', pools: [openPool({ can_manage: false })], detail: () => poolDetail({ can_manage: false }) })
    renderApp(G)
    await screen.findByRole('heading', { level: 2, name: 'Elif Kara için' })
    expect(screen.queryByRole('button', { name: /için işlemler/ })).not.toBeInTheDocument()
    // Alıcı rozeti herkese görünür.
    expect(screen.getByText('Hediye alacak')).toBeInTheDocument()
  })
})

describe('cover and description hints', () => {
  it('tells the admin that recipients see the description', async () => {
    multiHandlers()
    renderApp(G)
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Grup işlemleri' }))
    // Tarih, yeniden açma ve hedef bağlama grup menüsünde yok (havuz düzeyinde).
    expect(screen.queryByRole('menuitem', { name: 'Hediye tarihini ayarla' })).not.toBeInTheDocument()
    expect(screen.queryByRole('menuitem', { name: 'Hedef kişiyi bağla' })).not.toBeInTheDocument()
    await user.click(await screen.findByRole('menuitem', { name: 'Açıklamayı düzenle' }))
    expect(await screen.findByRole('dialog')).toHaveTextContent('Hediye alınacak kişiler de bunu görür')
  })

  it('tells the admin that recipients see the cover', async () => {
    multiHandlers()
    renderApp(G)
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Grup işlemleri' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Fotoğraf ekle' }))
    expect(await screen.findByRole('dialog')).toHaveTextContent('Hediye alınacak kişiler de bunu görür')
  })
})

describe('live updates in MULTI groups', () => {
  it('refreshes pool queries on a change event and reconnects when the server closes the stream', async () => {
    let streams = 0
    let current = poolDetail()
    multiHandlers()
    server.use(
      http.get(api(POOL), () => ok(current)),
      http.get(api(`${G}/events`), () => {
        streams++
        // İlk akış bir olay yazıp kapanır (görünen havuz kümesi değişti); ikincisi açık kalır.
        return streams === 1 ? eventStream(['event: changed\ndata: {}\n\n'], { close: true }) : eventStream()
      }),
    )
    current = poolDetail({ per_person_budget: '150', contributor_count: 2 })
    renderApp(G)
    expect(await screen.findByText('₺150,00', {}, { timeout: 3000 })).toBeInTheDocument()
    await waitFor(() => expect(streams).toBeGreaterThanOrEqual(2), { timeout: 4000 })
  })
})

describe('home page: MULTI cards', () => {
  const row = (overrides: Partial<GroupListItem> = {}): GroupListItem => ({
    ...multiGroup,
    member_count: 4,
    idea_count: 3,
    pending_votes: 2,
    recipients: [elif, { id: me.id, name: me.name }],
    next_event_date: '2031-01-15',
    ...overrides,
  })

  it('lists recipients with "senin" last, uses next_event_date and shows the counters as given', async () => {
    server.use(http.get(api('/me'), () => ok(me)), http.get(api('/groups'), () => ok([row()])))
    renderApp('/')
    const card = (await screen.findAllByRole('link', { name: /Ofis yılbaşı/ })).at(-1)!
    expect(card).toHaveTextContent('Elif Kara ve senin için')
    expect(card).toHaveTextContent('4 üye')
    expect(card).toHaveTextContent('3 fikir')
    expect(card).toHaveTextContent('2 oy bekliyor')
    // event_date yok, next_event_date var: geri sayım etiketi çizilir ve "tarih ekle" çağrısı yoktur.
    expect(card).toHaveTextContent(/gün kaldı/)
    expect(screen.queryByText(/için tarih ekle/)).not.toBeInTheDocument()
  })

  it('joins several names and handles an empty recipient list', async () => {
    server.use(
      http.get(api('/me'), () => ok(me)),
      http.get(api('/groups'), () =>
        ok([
          row({ recipients: [elif, can, { id: me.id, name: me.name }] }),
          row({ id: 'g-bos', title: 'Yeni grup', recipients: [], next_event_date: undefined, idea_count: 0, pending_votes: 0 }),
        ]),
      ),
    )
    renderApp('/')
    expect((await screen.findAllByText(/Elif Kara, Can Er ve senin için/)).length).toBeGreaterThan(0)
    expect(screen.getByText(/Hediye alınacak kişiler henüz seçilmedi/)).toBeInTheDocument()
  })
})

describe('pool notifications', () => {
  const n = (overrides: Partial<AppNotification>): AppNotification => ({
    id: 'n-1',
    group_id: multiGroup.id,
    group_title: multiGroup.title,
    pool_id: 'p-elif',
    type: 'idea_added',
    params: { actor_name: 'Can Er', idea_title: 'Termos', recipient_name: 'Elif Kara' },
    created_at: new Date().toISOString(),
    ...overrides,
  })

  it('words pool events with the recipient and keeps the old wording without one', () => {
    expect(notificationText(n({}))).toBe('Can Er Elif Kara için yeni bir fikir ekledi: Termos')
    expect(notificationText(n({ type: 'budget_changed', params: { actor_name: 'Can Er', total_budget: '1500.00', recipient_name: 'Elif Kara' } }))).toBe(
      'Can Er Elif Kara için bütçeyi ₺1.500,00 yaptı',
    )
    expect(notificationText(n({ type: 'pool_added', params: { actor_name: 'Deniz Aksoy', recipient_name: 'Elif Kara' } }))).toBe(
      'Deniz Aksoy, Elif Kara için bir hediye havuzu açtı',
    )
    expect(notificationText(n({ type: 'pool_manager_assigned', params: { actor_name: 'Deniz Aksoy', recipient_name: 'Deniz Aksoy' } }))).toContain(
      'sorumlusu yaptı',
    )
    expect(notificationText(n({ pool_id: undefined, params: { actor_name: 'Can Er', idea_title: 'Termos' } }))).toBe(
      'Can Er yeni bir fikir ekledi: Termos',
    )
    expect(notificationPath(n({}))).toBe('/groups/g-ofis?pool=p-elif')
    expect(notificationPath(n({ pool_id: undefined }))).toBe('/groups/g-ofis')
  })

  it('opens the group on the pool tab when a pool notification is clicked', async () => {
    multiHandlers({ pools: [openPool(), openPool({ id: 'p-can', recipient: can }), lockedPool()] })
    server.use(
      http.get(api(`${G}/pools/p-can`), () => ok(poolDetail({ id: 'p-can', recipient: can }))),
      http.get(api(`${G}/pools/p-can/ideas`), () => ok([])),
      http.get(api('/notifications'), () =>
        ok({ items: [n({ pool_id: 'p-can', params: { actor_name: 'Elif Kara', idea_title: 'Termos', recipient_name: 'Can Er' } })], unread_count: 1 }),
      ),
      http.post(api('/notifications/n-1/read'), () => ok({ message: 'ok' })),
    )
    const { router } = renderApp('/')
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Bildirimler, 1 okunmamış' }))
    await user.click(await screen.findByRole('menuitem', { name: /Elif Kara Can Er için yeni bir fikir ekledi/ }))
    await waitFor(() => expect(router.state.location.pathname + router.state.location.search).toBe(`${G}?pool=p-can`))
    expect(await screen.findByRole('heading', { level: 2, name: 'Can Er için' })).toBeInTheDocument()
  })
})

describe('SINGLE group page is unchanged', () => {
  it('has no tabs, keeps the target status, per-person tile and invite warning', async () => {
    server.use(
      http.get(api('/me'), () => ok(me)),
      http.get(api('/groups'), () => ok([group])),
      http.get(api(`/groups/${group.id}`), () => ok(groupDetail())),
      http.get(api(`/groups/${group.id}/ideas`), () => ok([idea()])),
      http.get(api(`/groups/${group.id}/events`), () => eventStream()),
    )
    renderApp(`/groups/${group.id}`)
    expect(await screen.findByRole('heading', { level: 1, name: 'Kaan’ın 30. yaşı' })).toBeInTheDocument()
    expect(screen.getByText(/Kaan için sürpriz/)).toBeInTheDocument()
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument()
    expect(screen.getByText('Hesap bağlı değil')).toBeInTheDocument()
    expect(screen.getByText('Kişi başı')).toBeInTheDocument()
    expect(screen.getByText('2 üyeye bölünür')).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 2, name: 'Hediye havuzu' })).toBeInTheDocument()
    expect(await screen.findByRole('heading', { level: 3, name: 'Stanley termos 1 L' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Grup işlemleri' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Bütçeyi düzenle' })).toBeInTheDocument()
    expect(screen.getByText(/Hediye alınacak kişinin hesabı bağlı değil/)).toBeInTheDocument()
    expect(screen.queryByText('Hediye alacak')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /için işlemler/ })).not.toBeInTheDocument()
  })
})

describe('own pool: no request in any navigation path', () => {
  it('keyboard focus on my own tab loads nothing, and selecting it with Enter loads nothing', async () => {
    multiHandlers({ pools: [openPool(), lockedPool()] })
    renderApp(G)
    const user = userEvent.setup()
    await screen.findByRole('heading', { level: 2, name: 'Elif Kara için' })
    await user.click(screen.getByRole('tab', { name: 'Elif Kara' }))
    await user.keyboard('{ArrowRight}')
    expect(screen.getByRole('tab', { name: /Deniz Aksoy/ })).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(await screen.findByRole('heading', { name: 'Senin için bir sürpriz hazırlanıyor' })).toBeInTheDocument()
    await new Promise((r) => setTimeout(r, 100))
    expect(seen.filter((p) => p.includes('p-me'))).toEqual([])
  })

  it('a notification link carrying my own pool id opens the locked card without requests', async () => {
    multiHandlers({ pools: [openPool(), lockedPool()] })
    server.use(
      http.get(api('/notifications'), () =>
        ok({
          items: [
            {
              id: 'n-9',
              group_id: multiGroup.id,
              group_title: multiGroup.title,
              pool_id: 'p-me',
              type: 'budget_changed',
              params: { actor_name: 'Can Er', total_budget: '1.00', recipient_name: 'Deniz Aksoy' },
              created_at: new Date().toISOString(),
            },
          ],
          unread_count: 1,
        }),
      ),
      http.post(api('/notifications/n-9/read'), () => ok({ message: 'ok' })),
    )
    const { router } = renderApp('/')
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Bildirimler, 1 okunmamış' }))
    await user.click(await screen.findByRole('menuitem', { name: /bütçeyi/ }))
    await waitFor(() => expect(router.state.location.search).toBe('?pool=p-me'))
    expect(await screen.findByRole('heading', { name: 'Senin için bir sürpriz hazırlanıyor' })).toBeInTheDocument()
    await new Promise((r) => setTimeout(r, 100))
    expect(seen.filter((p) => p.includes('p-me'))).toEqual([])
  })

  it('making myself a recipient never requests my new pool', async () => {
    let created = false
    multiHandlers({ pools: [openPool()] })
    server.use(
      http.get(api(G), () => ok(multiDetail(created ? [openPool(), lockedPool()] : [openPool()]))),
      http.post(api(`${G}/recipients`), () => {
        created = true
        return ok({ pool: lockedPool() }, 201)
      }),
    )
    const { router } = renderApp(G)
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Deniz Aksoy için işlemler' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Hediye alınacak kişi yap' }))
    const dialog = await screen.findByRole('dialog')
    await user.selectOptions(within(dialog).getByRole('combobox'), 'Elif Kara')
    await user.click(within(dialog).getByRole('button', { name: 'Hediye alınacak kişi yap' }))
    expect(await screen.findByRole('tab', { name: /Deniz Aksoy/ })).toBeInTheDocument()
    await new Promise((r) => setTimeout(r, 100))
    expect(seen.filter((p) => p.includes('p-me'))).toEqual([])
    // Kendi havuzumun sekmesi seçilmez.
    expect(router.state.location.search).toBe('')
  })

  it('selects the new pool’s tab after making another member a recipient', async () => {
    let created = false
    multiHandlers({ pools: [lockedPool()] })
    server.use(
      http.get(api(G), () =>
        ok(multiDetail(created ? [lockedPool(), openPool({ id: 'p-can', recipient: can })] : [lockedPool()])),
      ),
      http.get(api(`${G}/pools/p-can`), () => ok(poolDetail({ id: 'p-can', recipient: can }))),
      http.get(api(`${G}/pools/p-can/ideas`), () => ok([])),
      http.post(api(`${G}/recipients`), () => {
        created = true
        return ok({ pool: openPool({ id: 'p-can', recipient: can }) }, 201)
      }),
    )
    const { router } = renderApp(G)
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Can Er için işlemler' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Hediye alınacak kişi yap' }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Hediye alınacak kişi yap' }))
    await waitFor(() => expect(router.state.location.search).toBe('?pool=p-can'))
    expect(await screen.findByRole('heading', { level: 2, name: 'Can Er için' })).toBeInTheDocument()
  })
})

describe('pool removed while open', () => {
  it('shows a not-found message on 404, then falls back to another tab when the group refreshes', async () => {
    let push: ((frame: string) => void) | null = null
    let pools: PoolView[] = [openPool(), openPool({ id: 'p-can', recipient: can })]
    multiHandlers()
    server.use(
      http.get(api(G), () => ok(multiDetail(pools))),
      http.get(api(`${G}/pools/p-can`), () => fail(404, 'Havuz bulunamadı')),
      http.get(api(`${G}/events`), () => {
        const encoder = new TextEncoder()
        return new Response(
          new ReadableStream<Uint8Array>({
            start(c) {
              c.enqueue(encoder.encode(': connected\n\n'))
              push = (f) => c.enqueue(encoder.encode(f))
            },
          }),
          { headers: { 'Content-Type': 'text/event-stream' } },
        )
      }),
    )
    renderApp(`${G}?pool=p-can`)
    expect(await screen.findByText(/Bu havuz bulunamadı/)).toBeInTheDocument()
    await waitFor(() => expect(push).not.toBeNull())
    pools = [openPool()]
    push!('event: changed\ndata: {}\n\n')
    expect(await screen.findByRole('heading', { level: 2, name: 'Elif Kara için' }, { timeout: 3000 })).toBeInTheDocument()
    expect(screen.queryByRole('tab', { name: 'Can Er' })).not.toBeInTheDocument()
  })
})

describe('pool manager routes', () => {
  it('selects winners and reopens through the pool routes', async () => {
    const winner: unknown[] = []
    let reopened = false
    let status: 'ACTIVE' | 'COMPLETED' = 'ACTIVE'
    multiHandlers()
    server.use(
      http.get(api(G), () => ok(multiDetail([openPool({ status })]))),
      http.get(api(POOL), () => ok(poolDetail({ status }))),
      http.get(api(`${POOL}/ideas`), () => ok([idea({ id: 'i-1', title: 'Kamp sandalyesi', pool_id: 'p-elif', status: status === 'COMPLETED' ? 'SELECTED' : 'ACTIVE' })])),
      http.post(api(`${POOL}/winner`), async ({ request }) => {
        winner.push(await request.json())
        status = 'COMPLETED'
        return ok({ message: 'Kazanan seçildi, havuz tamamlandı' })
      }),
      http.delete(api(`${POOL}/winner`), () => {
        reopened = true
        status = 'ACTIVE'
        return ok({ message: 'Havuz yeniden açıldı' })
      }),
    )
    renderApp(G)
    const user = userEvent.setup()

    const card = (await screen.findByRole('heading', { level: 3, name: 'Kamp sandalyesi' })).closest('article')!
    await user.click(within(card).getByRole('button', { name: 'Fikir işlemleri' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Kazanan olarak seç' }))
    const dialog = await screen.findByRole('dialog')
    expect(dialog).toHaveTextContent('bu havuz tamamlanır')
    await user.click(within(dialog).getByRole('button', { name: 'Kazananı seç' }))
    await waitFor(() => expect(winner).toEqual([{ idea_ids: ['i-1'] }]))
    expect(await screen.findByText(/Havuz tamamlandı\. Kazanan/)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Havuz işlemleri' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Havuzu yeniden aç' }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Yeniden aç' }))
    await waitFor(() => expect(reopened).toBe(true))
  })

  it('saves the pool event date through the pool route', async () => {
    const sent: unknown[] = []
    multiHandlers()
    server.use(
      http.patch(api(`${POOL}/event-date`), async ({ request }) => {
        sent.push(await request.json())
        return ok({ message: 'ok' })
      }),
    )
    renderApp(G)
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Havuz işlemleri' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Hediye tarihini ayarla' }))
    const dialog = await screen.findByRole('dialog')
    const input = within(dialog).getByLabelText('Tarih')
    expect(input).toHaveValue('2031-01-15')
    await user.clear(input)
    await user.type(input, '2031-02-20')
    await user.click(within(dialog).getByRole('button', { name: 'Kaydet' }))
    await waitFor(() => expect(sent).toEqual([{ event_date: '2031-02-20' }]))
  })

  it('lets the manager delete an interest through the pool route', async () => {
    let sent: unknown = null
    multiHandlers()
    server.use(
      http.delete(api(`${POOL}/interests`), async ({ request }) => {
        sent = await request.json()
        return ok({ interests: [] })
      }),
    )
    renderApp(G)
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Kamp ilgi alanını sil' }))
    await waitFor(() => expect(sent).toEqual({ interest: 'Kamp' }))
  })
})

describe('contribution switch: focus and races', () => {
  it('ignores a rapid second toggle, keeps focus, and announces the result', async () => {
    let puts = 0
    multiHandlers()
    server.use(
      http.put(api(`${POOL}/contribution`), async () => {
        puts++
        await new Promise((r) => setTimeout(r, 150))
        return ok({ contributing: false, contributor_count: 2, per_person_budget: '150', no_contributors: false })
      }),
    )
    renderApp(G)
    const user = userEvent.setup()
    const toggle = await screen.findByRole('switch', { name: 'Bu hediyeye ortağım' })
    await user.click(toggle)
    expect(toggle).toHaveAttribute('aria-disabled', 'true')
    expect(toggle).not.toBeDisabled()
    await user.click(toggle)
    await waitFor(() => expect(toggle).not.toHaveAttribute('aria-disabled'))
    expect(puts).toBe(1)
    expect(toggle).toHaveFocus()
    await waitFor(() =>
      expect(screen.getAllByRole('status').some((e) => /Katkın kaydedildi\. 2 katılımcı, kişi başı ₺150,00/.test(e.textContent ?? ''))).toBe(true),
    )
  })
})

describe('SSE refresh is deferred while my own mutation runs', () => {
  it('refetches the group after the vote settles instead of dropping the event', async () => {
    let detailCalls = 0
    let push: ((frame: string) => void) | null = null
    let release: (() => void) | null = null
    multiHandlers()
    server.use(
      http.get(api(G), () => {
        detailCalls++
        return ok(multiDetail([openPool(), lockedPool()]))
      }),
      http.get(api(`${G}/events`), () => {
        const encoder = new TextEncoder()
        return new Response(
          new ReadableStream<Uint8Array>({
            start(c) {
              c.enqueue(encoder.encode(': connected\n\n'))
              push = (f) => c.enqueue(encoder.encode(f))
            },
          }),
          { headers: { 'Content-Type': 'text/event-stream' } },
        )
      }),
      http.post(api('/ideas/i-1/vote'), async () => {
        await new Promise<void>((r) => (release = r))
        return ok({ message: 'ok' })
      }),
    )
    renderApp(G)
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Beğen' }))
    await waitFor(() => expect(release).not.toBeNull())
    await waitFor(() => expect(push).not.toBeNull())
    const before = detailCalls
    push!('event: changed\ndata: {}\n\n')
    await new Promise((r) => setTimeout(r, 500))
    expect(detailCalls).toBe(before)
    release!()
    await waitFor(() => expect(detailCalls).toBeGreaterThan(before), { timeout: 3000 })
  })
})

describe('MULTI copy', () => {
  it('does not claim the recipient can’t be added or can’t join', async () => {
    multiHandlers()
    renderApp(G)
    await screen.findByRole('heading', { level: 2, name: 'Elif Kara için' })
    expect(await screen.findByText(/Hediye alınacak kişileri üyeler katıldıktan sonra sen seçersin/)).toBeInTheDocument()
    expect(screen.getByText(/Hediye alınacak kişileri sonradan sen seçersin/)).toBeInTheDocument()
    expect(screen.queryByText(/Hediye alınacak kişi eklenemez/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Hediye alınacak kişi katılamaz/)).not.toBeInTheDocument()
  })
})

describe('home dates with next_event_date', () => {
  const day = (offset: number) => {
    const d = new Date()
    d.setDate(d.getDate() + offset)
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
  }

  it('a group whose next_event_date is upcoming is not overdue and shows a countdown', async () => {
    const upcoming: GroupListItem = {
      ...multiGroup,
      member_count: 3,
      idea_count: 1,
      pending_votes: 1,
      recipients: [elif, can],
      next_event_date: day(10),
    }
    server.use(http.get(api('/me'), () => ok(me)), http.get(api('/groups'), () => ok([upcoming])))
    renderApp('/')
    const row = (await screen.findAllByRole('link', { name: /Ofis yılbaşı/ })).at(-1)!
    expect(row).toHaveTextContent('10 gün kaldı')
    expect(row).not.toHaveTextContent('geçti')
    // Görev satırı "tarih geçti" demez.
    expect(screen.queryByText(/Tarih \d+ gün geçti/)).not.toBeInTheDocument()
    expect(screen.getByText(/1 fikir oyunu bekliyor/)).toBeInTheDocument()
  })

  it('treats an overdue next_event_date (no upcoming pool) as overdue', async () => {
    const overdue: GroupListItem = {
      ...multiGroup,
      member_count: 3,
      idea_count: 1,
      pending_votes: 0,
      recipients: [elif],
      next_event_date: day(-3),
    }
    server.use(http.get(api('/me'), () => ok(me)), http.get(api('/groups'), () => ok([overdue])))
    renderApp('/')
    expect(await screen.findByText(/Tarih 3 gün geçti/)).toBeInTheDocument()
  })
})
