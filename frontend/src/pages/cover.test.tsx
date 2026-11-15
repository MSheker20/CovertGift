import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { HttpResponse } from 'msw'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { Group, GroupListItem } from '@/api/types'
import * as coverCrop from '@/lib/coverCrop'
import { renderApp } from '@/test/render'
import { api, eventStream, fail, group, groupDetail, http, me, ok, server } from '@/test/server'

// jsdom görsel çözmez ve canvas çizmez; kırpma adımının iki tarayıcı işlemi taklit edilir.
vi.mock('@/lib/coverCrop', async (importOriginal) => ({
  ...(await importOriginal<typeof coverCrop>()),
  prepareImage: vi.fn(),
  renderCrop: vi.fn(),
}))

const PATH = `/groups/${group.id}`
const cropped = new Blob(['kırpılmış-jpeg'], { type: 'image/jpeg' })

const jpeg = () => new HttpResponse(new Blob(['jpeg'], { type: 'image/jpeg' }), { headers: { 'Content-Type': 'image/jpeg' } })

// Grup sayfası: `state` testin içinde değiştirilebilir (yükleme/silme sonrası yeniden çekim).
function groupPage(role: 'admin' | 'member', state: { group: Partial<Group> }) {
  server.use(
    http.get(api('/me'), () => ok(me)),
    http.get(api('/groups'), () => ok([{ ...group, ...state.group }])),
    http.get(api(`/groups/${group.id}`), () => ok(groupDetail({ my_role: role, group: { ...group, ...state.group } }))),
    http.get(api(`/groups/${group.id}/ideas`), () => ok([])),
    http.get(api(`/groups/${group.id}/events`), () => eventStream()),
    http.get(api(`/groups/${group.id}/cover`), () => jpeg()),
  )
}

// Yönetici "Grup işlemleri" menüsünü, üye yalnızca fotoğraf işlemlerini içeren "Kapak fotoğrafı" düğmesini görür.
const openMenu = async (user: ReturnType<typeof userEvent.setup>, name = 'Grup işlemleri') => {
  await user.click(await screen.findByRole('button', { name }))
  return screen.findByRole('menu')
}

async function pickPhoto(user: ReturnType<typeof userEvent.setup>) {
  const dialog = await screen.findByRole('dialog', { name: 'Kapak fotoğrafı' })
  await user.upload(within(dialog).getByLabelText('Fotoğraf seç', { selector: 'input' }), new File(['x'], 'tatil.jpg', { type: 'image/jpeg' }))
  await within(dialog).findByRole('group', { name: 'Kırpma alanı' })
  return dialog
}

beforeEach(() => {
  localStorage.setItem('covertgift.token', 'tok')
  vi.mocked(coverCrop.prepareImage).mockResolvedValue({
    element: document.createElement('img'),
    url: 'blob:test/picked',
    width: 1920,
    height: 1080,
    release: () => {},
  })
  vi.mocked(coverCrop.renderCrop).mockResolvedValue(cropped)
})

describe('group page cover: who sees what', () => {
  it('admin without a photo: can add one and sees the restriction switch; the header has no image', async () => {
    groupPage('admin', { group: {} })
    renderApp(PATH)
    const menu = await openMenu(userEvent.setup())

    expect(within(menu).getByRole('menuitem', { name: 'Fotoğraf ekle' })).toBeInTheDocument()
    expect(within(menu).queryByRole('menuitem', { name: 'Fotoğrafı kaldır' })).not.toBeInTheDocument()
    expect(within(menu).getByRole('menuitemcheckbox', { name: 'Fotoğrafı yalnızca yönetici değiştirsin' })).not.toBeChecked()
    expect(within(menu).getByRole('menuitem', { name: 'Grubu sil' })).toBeInTheDocument()
    expect(screen.queryByRole('img', { name: /kapak fotoğrafı/ })).not.toBeInTheDocument()
  })

  it('member, restriction off: only the photo actions; no admin actions and no switch', async () => {
    groupPage('member', { group: { cover_version: 'v1', cover_admin_only: false } })
    renderApp(PATH)
    const menu = await openMenu(userEvent.setup(), 'Kapak fotoğrafı')
    expect(screen.queryByRole('button', { name: 'Grup işlemleri' })).not.toBeInTheDocument()

    expect(within(menu).getByRole('menuitem', { name: 'Fotoğrafı değiştir' })).toBeInTheDocument()
    expect(within(menu).getByRole('menuitem', { name: 'Fotoğrafı kaldır' })).toBeInTheDocument()
    expect(within(menu).queryByRole('menuitemcheckbox')).not.toBeInTheDocument()
    expect(within(menu).queryByRole('menuitem', { name: 'Grubu sil' })).not.toBeInTheDocument()
    expect(within(menu).queryByRole('menuitem', { name: 'Hediye tarihini ayarla' })).not.toBeInTheDocument()
    expect(within(menu).getAllByRole('menuitem')).toHaveLength(2)
  })

  it('member, restriction on: no menu at all, but the photo is still shown', async () => {
    groupPage('member', { group: { cover_version: 'v1', cover_admin_only: true } })
    renderApp(PATH)

    expect(await screen.findByRole('img', { name: `“${group.title}” kapak fotoğrafı` })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Grup işlemleri' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Kapak fotoğrafı' })).not.toBeInTheDocument()
  })

  it('admin, restriction on: keeps every photo action and the switch is checked', async () => {
    groupPage('admin', { group: { cover_version: 'v1', cover_admin_only: true } })
    renderApp(PATH)
    const menu = await openMenu(userEvent.setup())

    expect(within(menu).getByRole('menuitem', { name: 'Fotoğrafı değiştir' })).toBeInTheDocument()
    expect(within(menu).getByRole('menuitem', { name: 'Fotoğrafı kaldır' })).toBeInTheDocument()
    expect(within(menu).getByRole('menuitemcheckbox', { name: 'Fotoğrafı yalnızca yönetici değiştirsin' })).toBeChecked()
  })
})

describe('group page cover: actions', () => {
  it('the switch sends the new policy and reflects it without closing the menu', async () => {
    const state = { group: { cover_admin_only: false } as Partial<Group> }
    const sent: unknown[] = []
    groupPage('admin', state)
    server.use(
      http.patch(api(`/groups/${group.id}/cover-policy`), async ({ request }) => {
        const body = (await request.json()) as { cover_admin_only: boolean }
        sent.push(body)
        state.group = { cover_admin_only: body.cover_admin_only }
        return ok(body)
      }),
    )
    renderApp(PATH)
    const user = userEvent.setup()
    const menu = await openMenu(user)
    const toggle = within(menu).getByRole('menuitemcheckbox', { name: 'Fotoğrafı yalnızca yönetici değiştirsin' })

    await user.click(toggle)
    expect(await screen.findByText('Kapak fotoğrafını artık yalnızca yönetici değiştirebilir.')).toBeInTheDocument()
    expect(sent).toEqual([{ cover_admin_only: true }])
    await waitFor(() => expect(screen.getByRole('menuitemcheckbox')).toBeChecked())

    await user.click(screen.getByRole('menuitemcheckbox'))
    expect(await screen.findByText('Kapak fotoğrafını artık tüm üyeler değiştirebilir.')).toBeInTheDocument()
    expect(sent).toEqual([{ cover_admin_only: true }, { cover_admin_only: false }])
  })

  it('uploads the cropped photo as multipart "file" and shows it in the header', async () => {
    const state = { group: {} as Partial<Group> }
    // Gövdenin içeriği (alan adı "file", JPEG blob) src/api/client.test.ts'de denenir; jsdom'un
    // FormData'sı Node fetch'i üzerinden okunamadığı için burada yalnızca istek ve türü denetlenir.
    const uploads: string[] = []
    groupPage('member', state)
    server.use(
      http.put(api(`/groups/${group.id}/cover`), ({ request }) => {
        uploads.push(request.headers.get('Content-Type') ?? '')
        state.group = { cover_version: 'v-yeni' }
        return ok({ cover_version: 'v-yeni', width: 1440, height: 1080 })
      }),
    )
    renderApp(PATH)
    const user = userEvent.setup()
    await user.click(within(await openMenu(user, 'Kapak fotoğrafı')).getByRole('menuitem', { name: 'Fotoğraf ekle' }))
    const dialog = await pickPhoto(user)
    await user.click(within(dialog).getByRole('button', { name: 'Fotoğrafı kaydet' }))

    expect(await screen.findByText('Kapak fotoğrafı kaydedildi.')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(uploads).toHaveLength(1)
    expect(uploads[0]).toMatch(/^multipart\/form-data; boundary=/)
    expect(coverCrop.renderCrop).toHaveBeenCalledWith(expect.any(HTMLImageElement), { sx: 240, sy: 0, sw: 1440, sh: 1080 }, expect.objectContaining({ aspect: 4 / 3 }))
    expect(await screen.findByRole('img', { name: `“${group.title}” kapak fotoğrafı` })).toBeInTheDocument()
  })

  it('keeps the dialog open and shows the server message when the upload is refused', async () => {
    groupPage('member', { group: {} })
    server.use(
      http.put(api(`/groups/${group.id}/cover`), () =>
        fail(403, 'bu grupta kapak fotoğrafını sadece grup yöneticisi (admin) değiştirebilir'),
      ),
    )
    renderApp(PATH)
    const user = userEvent.setup()
    await user.click(within(await openMenu(user, 'Kapak fotoğrafı')).getByRole('menuitem', { name: 'Fotoğraf ekle' }))
    const dialog = await pickPhoto(user)
    await user.click(within(dialog).getByRole('button', { name: 'Fotoğrafı kaydet' }))

    expect(await within(dialog).findByRole('alert')).toHaveTextContent('sadece grup yöneticisi')
    expect(within(dialog).getByRole('button', { name: 'Fotoğrafı kaydet' })).toBeEnabled()
    expect(screen.queryByRole('img', { name: /kapak fotoğrafı/ })).not.toBeInTheDocument()
  })

  it('removes the photo after confirmation', async () => {
    const state = { group: { cover_version: 'v1' } as Partial<Group> }
    let deleted = 0
    groupPage('admin', state)
    server.use(
      http.delete(api(`/groups/${group.id}/cover`), () => {
        deleted += 1
        state.group = {}
        return ok({ message: 'ok' })
      }),
    )
    renderApp(PATH)
    const user = userEvent.setup()
    expect(await screen.findByRole('img', { name: `“${group.title}” kapak fotoğrafı` })).toBeInTheDocument()
    await user.click(within(await openMenu(user)).getByRole('menuitem', { name: 'Fotoğrafı kaldır' }))

    const confirm = await screen.findByRole('dialog', { name: 'Kapak fotoğrafı kaldırılsın mı?' })
    expect(deleted).toBe(0)
    await user.click(within(confirm).getByRole('button', { name: 'Fotoğrafı kaldır' }))

    expect(await screen.findByText('Kapak fotoğrafı kaldırıldı.')).toBeInTheDocument()
    expect(deleted).toBe(1)
    await waitFor(() => expect(screen.queryByRole('img', { name: /kapak fotoğrafı/ })).not.toBeInTheDocument())
  })
})

describe('home screen cover', () => {
  const upcoming = (overrides: Partial<GroupListItem> = {}): GroupListItem => ({
    ...group,
    event_date: '2099-01-01',
    member_count: 2,
    idea_count: 0,
    pending_votes: 0,
    ...overrides,
  })

  function home(groups: GroupListItem[]) {
    server.use(
      http.get(api('/me'), () => ok(me)),
      http.get(api('/groups'), () => ok(groups)),
    )
  }

  it('shows the photo in the next-surprise card, the group row and the group switcher', async () => {
    const requested: string[] = []
    home([upcoming({ cover_version: 'v1' })])
    server.use(
      http.get(api(`/groups/${group.id}/cover`), ({ request }) => {
        requested.push(new URL(request.url).search)
        return jpeg()
      }),
    )
    renderApp('/')

    const card = (await screen.findByText('Sıradaki sürpriz')).closest('a')!
    await waitFor(() => expect(card.querySelector('img')).not.toBeNull())
    expect(card.querySelector('img')!.getAttribute('src')).toMatch(/^blob:/)
    // Kartın bağlantı adı fotoğraf yüzünden değişmez (görsel dekoratif).
    expect(card.querySelector('img')).toHaveAttribute('alt', '')
    expect(within(card).getByText(group.title)).toBeInTheDocument()

    const row = screen.getByText(/2 üye/).closest('a')!
    await waitFor(() => expect(row.querySelector('img')).not.toBeNull())
    expect(row.querySelector('.avatar')).toBeNull()

    await userEvent.click(screen.getByRole('button', { name: 'Grup değiştir' }))
    const item = within(await screen.findByRole('menu')).getByRole('menuitem', { name: new RegExp(group.title) })
    await waitFor(() => expect(item.querySelector('img')).not.toBeNull())
    // Üç görünüm tek indirmeyi paylaşır.
    expect(requested).toEqual(['?v=v1'])
  })

  it('keeps the plain card and the avatars when the group has no photo (no cover request)', async () => {
    // /cover için handler yok: istek atılırsa test hata ile durur.
    home([upcoming()])
    renderApp('/')

    const card = (await screen.findByText('Sıradaki sürpriz')).closest('a')!
    expect(card.querySelector('img')).toBeNull()
    expect(card.querySelector('[data-cover]')).toBeNull()
    const row = screen.getByText(/2 üye/).closest('a')!
    expect(row.querySelector('img')).toBeNull()
    expect(row.querySelector('.avatar')).toHaveTextContent('K')
  })
})

describe('new group with a photo', () => {
  async function fillAndPick() {
    const user = userEvent.setup()
    await user.click((await screen.findAllByRole('button', { name: 'Yeni grup' }))[0])
    const form = await screen.findByRole('dialog', { name: 'Yeni sürpriz grubu' })
    await user.type(within(form).getByLabelText('Grup adı'), 'Kaan’ın 30. yaşı')
    await user.type(within(form).getByLabelText('Hediye kimin için?'), 'Kaan')

    await user.click(within(form).getByRole('button', { name: 'Fotoğraf seç' }))
    const dialog = await pickPhoto(user)
    await user.click(within(dialog).getByRole('button', { name: 'Bu fotoğrafı kullan' }))
    // Kırpma diyaloğu kapanır, form önizlemeyi gösterir; henüz hiçbir şey yüklenmedi.
    expect(await within(form).findByRole('img', { name: 'Seçilen kapak fotoğrafı' })).toBeInTheDocument()
    expect(within(form).getByRole('button', { name: 'Fotoğrafı değiştir' })).toBeInTheDocument()
    return { user, form }
  }

  function handlers(coverResponse: () => Response) {
    const calls: string[] = []
    server.use(
      http.get(api('/me'), () => ok(me)),
      http.get(api('/groups'), () => ok([])),
      http.post(api('/groups'), () => {
        calls.push('create')
        return ok(group, 201)
      }),
      http.put(api(`/groups/${group.id}/cover`), () => {
        calls.push('cover')
        return coverResponse()
      }),
      http.get(api(`/groups/${group.id}`), () => ok(groupDetail())),
      http.get(api(`/groups/${group.id}/ideas`), () => ok([])),
      http.get(api(`/groups/${group.id}/events`), () => eventStream()),
    )
    return calls
  }

  it('creates the group first, then uploads the chosen photo', async () => {
    const calls = handlers(() => ok({ cover_version: 'v1', width: 1440, height: 1080 }))
    const { router } = renderApp('/')
    const { user, form } = await fillAndPick()
    await user.click(within(form).getByRole('button', { name: 'Grubu kur' }))

    await waitFor(() => expect(router.state.location.pathname).toBe(PATH))
    expect(calls).toEqual(['create', 'cover'])
    expect(screen.queryByText(/fotoğraf yüklenemedi/)).not.toBeInTheDocument()
  })

  it('keeps the group when the photo upload fails and says how to add it later', async () => {
    const calls = handlers(() => fail(500, 'Kapak fotoğrafı kaydedilirken bir hata oluştu'))
    const { router } = renderApp('/')
    const { user, form } = await fillAndPick()
    await user.click(within(form).getByRole('button', { name: 'Grubu kur' }))

    await waitFor(() => expect(router.state.location.pathname).toBe(PATH))
    expect(calls).toEqual(['create', 'cover'])
    expect(await screen.findByText('Grup kuruldu ama fotoğraf yüklenemedi. Grup menüsünden tekrar ekleyebilirsin.')).toBeInTheDocument()
    // Grup kuruldu bildirimi de gösterilir (önceki testten kalan bildirim de ekranda olabilir).
    expect(screen.getAllByText('“Kaan’ın 30. yaşı” kuruldu.').length).toBeGreaterThan(0)
  })

  it('creates the group without any upload when the chosen photo is removed again', async () => {
    const calls = handlers(() => ok({ cover_version: 'v1', width: 1440, height: 1080 }))
    const { router } = renderApp('/')
    const { user, form } = await fillAndPick()
    await user.click(within(form).getByRole('button', { name: 'Fotoğrafı kaldır' }))
    expect(within(form).queryByRole('img', { name: 'Seçilen kapak fotoğrafı' })).not.toBeInTheDocument()
    await user.click(within(form).getByRole('button', { name: 'Grubu kur' }))

    await waitFor(() => expect(router.state.location.pathname).toBe(PATH))
    expect(calls).toEqual(['create'])
  })
})
