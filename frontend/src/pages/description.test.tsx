import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { Group, GroupDetail } from '@/api/types'
import i18n from '@/i18n'
import { renderApp } from '@/test/render'
import { api, fail, group, groupDetail, http, me, ok, server } from '@/test/server'

const PATH = `/groups/${group.id}`
const LONG = 'Kampı seven Kaan için ortak hediye. '.repeat(12).trim()
const SHORT = 'Doğum günü 14 Aralık.'

// jsdom yerleşim hesaplamaz. Açıklama paragrafı için 60 karakterlik satırlar ve 24 px satır
// yüksekliği varsayılır; line-clamp-3 sınıfı varken görünen yükseklik 3 satırla sınırlanır.
function mockLayout() {
  const lines = (el: Element) => Math.max(1, Math.ceil((el.textContent?.length ?? 0) / 60))
  const isDescription = (el: Element) => el instanceof HTMLElement && el.dataset.testid === 'group-description'
  vi.spyOn(Element.prototype, 'scrollHeight', 'get').mockImplementation(function (this: Element) {
    return isDescription(this) ? lines(this) * 24 : 0
  })
  vi.spyOn(Element.prototype, 'clientHeight', 'get').mockImplementation(function (this: Element) {
    if (!isDescription(this)) return 0
    const full = lines(this) * 24
    return this.classList.contains('line-clamp-3') ? Math.min(full, 72) : full
  })
}

function serve(detail: () => GroupDetail) {
  server.use(
    http.get(api('/me'), () => ok(me)),
    http.get(api('/groups'), () => ok([detail().group])),
    http.get(api(`/groups/${group.id}`), () => ok(detail())),
    http.get(api(`/groups/${group.id}/ideas`), () => ok([])),
  )
}

const withGroup = (g: Partial<Group>, rest: Partial<GroupDetail> = {}) => groupDetail({ group: { ...group, ...g }, ...rest })

beforeEach(() => {
  localStorage.setItem('covertgift.token', 'tok')
  mockLayout()
})
afterEach(() => vi.restoreAllMocks())

describe('group description', () => {
  it('clamps a long description to three lines and toggles it accessibly', async () => {
    serve(() => withGroup({ description: LONG }, { my_role: 'member' }))
    renderApp(PATH)
    const user = userEvent.setup()

    const text = await screen.findByTestId('group-description')
    expect(text).toHaveTextContent(LONG)
    expect(text).toHaveClass('line-clamp-3')
    const more = await screen.findByRole('button', { name: 'Devamını gör' })
    expect(more).toHaveAttribute('aria-expanded', 'false')
    expect(more).toHaveAttribute('aria-controls', text.id)

    await user.click(more)
    const less = screen.getByRole('button', { name: 'Daha az' })
    expect(less).toHaveAttribute('aria-expanded', 'true')
    expect(text).not.toHaveClass('line-clamp-3')

    await user.click(less)
    expect(screen.getByRole('button', { name: 'Devamını gör' })).toHaveAttribute('aria-expanded', 'false')
    expect(text).toHaveClass('line-clamp-3')
  })

  it('shows no toggle when the description fits', async () => {
    serve(() => withGroup({ description: SHORT }))
    renderApp(PATH)
    expect(await screen.findByTestId('group-description')).toHaveTextContent(SHORT)
    expect(screen.queryByRole('button', { name: 'Devamını gör' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Açıklama ekle' })).not.toBeInTheDocument()
  })

  it('offers the admin an “add description” hint that opens the editor', async () => {
    serve(() => groupDetail())
    renderApp(PATH)
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Açıklama ekle' }))
    const dialog = await screen.findByRole('dialog', { name: 'Grup açıklaması' })
    expect(within(dialog).getByLabelText('Açıklama')).toHaveValue('')
    expect(within(dialog).getByText('0 / 1000 karakter')).toBeInTheDocument()
  })

  it('shows a member nothing when there is no description', async () => {
    serve(() => groupDetail({ my_role: 'member' }))
    renderApp(PATH)
    expect(await screen.findByText('Kaan için sürpriz', { exact: false })).toBeInTheDocument()
    expect(screen.queryByTestId('group-description')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Açıklama ekle' })).not.toBeInTheDocument()
  })

  it('edits the description from the group menu and shows the saved text', async () => {
    let current: Group = { ...group, description: SHORT }
    let sent: unknown = null
    serve(() => groupDetail({ group: current }))
    server.use(
      http.patch(api(`/groups/${group.id}/description`), async ({ request }) => {
        sent = await request.json()
        current = { ...current, description: ((sent as { description: string }).description ?? '').trim() || undefined }
        return ok(current)
      }),
    )
    renderApp(PATH)
    const user = userEvent.setup()

    await screen.findByTestId('group-description')
    await user.click(screen.getByRole('button', { name: 'Grup işlemleri' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Açıklamayı düzenle' }))
    const dialog = await screen.findByRole('dialog', { name: 'Grup açıklaması' })
    const field = within(dialog).getByLabelText('Açıklama')
    expect(field).toHaveValue(SHORT)
    expect(field).toHaveAccessibleDescription(`${[...SHORT].length} / 1000 karakter`)

    await user.clear(field)
    await user.type(field, '  Hediye cuma günü alınacak.  ')
    expect(within(dialog).getByText('26 / 1000 karakter')).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Kaydet' }))

    expect(await screen.findByText('Açıklama kaydedildi.')).toBeInTheDocument()
    expect(sent).toEqual({ description: '  Hediye cuma günü alınacak.  ' })
    await waitFor(() => expect(screen.getByTestId('group-description')).toHaveTextContent('Hediye cuma günü alınacak.'))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()

    // Boş kaydetmek açıklamayı kaldırır; yöneticiye yine ipucu görünür.
    await user.click(screen.getByRole('button', { name: 'Grup işlemleri' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Açıklamayı düzenle' }))
    const again = await screen.findByRole('dialog', { name: 'Grup açıklaması' })
    await user.clear(within(again).getByLabelText('Açıklama'))
    await user.click(within(again).getByRole('button', { name: 'Kaydet' }))
    expect(await screen.findByText('Açıklama kaldırıldı.')).toBeInTheDocument()
    expect(await screen.findByRole('button', { name: 'Açıklama ekle' })).toBeInTheDocument()
    expect(screen.queryByTestId('group-description')).not.toBeInTheDocument()
  })

  it('blocks a description over 1000 characters without calling the API', async () => {
    let called = false
    serve(() => groupDetail())
    server.use(
      http.patch(api(`/groups/${group.id}/description`), () => {
        called = true
        return ok(group)
      }),
    )
    renderApp(PATH)
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Açıklama ekle' }))
    const dialog = await screen.findByRole('dialog', { name: 'Grup açıklaması' })
    const field = within(dialog).getByLabelText('Açıklama')
    await user.click(field)
    await user.paste('ş'.repeat(1001))
    expect(within(dialog).getByText('1001 / 1000 karakter')).toBeInTheDocument()
    expect(field).toHaveAttribute('aria-invalid', 'true')
    await user.click(within(dialog).getByRole('button', { name: 'Kaydet' }))
    expect(within(dialog).getByRole('alert')).toHaveTextContent('Açıklama en fazla 1000 karakter olabilir.')
    expect(called).toBe(false)
  })

  it('keeps the dialog open and shows a server error', async () => {
    serve(() => groupDetail())
    server.use(
      http.patch(api(`/groups/${group.id}/description`), () =>
        fail(403, 'grup açıklamasını sadece grup yöneticisi (admin) değiştirebilir'),
      ),
    )
    renderApp(PATH)
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Açıklama ekle' }))
    const dialog = await screen.findByRole('dialog', { name: 'Grup açıklaması' })
    await user.type(within(dialog).getByLabelText('Açıklama'), 'Deneme')
    await user.click(within(dialog).getByRole('button', { name: 'Kaydet' }))
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('sadece grup yöneticisi')
    expect(within(dialog).getByLabelText('Açıklama')).toHaveAttribute('aria-invalid', 'true')
  })

  it('does not offer description editing to a member', async () => {
    serve(() => withGroup({ description: SHORT }, { my_role: 'member' }))
    renderApp(PATH)
    const user = userEvent.setup()
    // Kısıt kapalıyken üye yalnızca kapak fotoğrafı menüsünü görür.
    await user.click(await screen.findByRole('button', { name: 'Kapak fotoğrafı' }))
    expect(await screen.findByRole('menu')).toBeInTheDocument()
    expect(screen.queryByRole('menuitem', { name: 'Açıklamayı düzenle' })).not.toBeInTheDocument()
  })

  it('labels the new group field as “Açıklama” and caps it at 1000 characters', async () => {
    server.use(http.get(api('/me'), () => ok(me)), http.get(api('/groups'), () => ok([])))
    renderApp('/')
    const user = userEvent.setup()
    await user.click((await screen.findAllByRole('button', { name: 'Yeni grup' }))[0])
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByLabelText(/^Açıklama/)).toHaveAttribute('maxlength', '1000')
  })
})

describe('target account status', () => {
  it('shows the linked account’s name with a cobalt shield and an explanation', async () => {
    serve(() =>
      withGroup({ target_user_id: 'u-kaan' }, { my_role: 'member', target_account: { id: 'u-kaan', name: 'Kaan Demir' } }),
    )
    renderApp(PATH)

    const label = await screen.findByText('Kaan Demir hesabı bağlı')
    const chip = label.closest('[data-state]') as HTMLElement
    expect(chip).toHaveAttribute('data-state', 'linked')
    expect(chip).toHaveAttribute('title', 'Kaan Demir bu grubu göremez.')
    // Renk tek başına anlam taşımaz: ikon gizli, durum ve açıklama okunur metin.
    expect(chip.querySelector('svg')).toHaveAttribute('aria-hidden', 'true')
    expect(chip).toHaveTextContent('Kaan Demir hesabı bağlı. Kaan Demir bu grubu göremez.')
    expect(screen.queryByText('Hesap bağlı değil')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Hesabı bağla' })).not.toBeInTheDocument()
  })

  it('falls back to the person’s name when the account name is not returned', async () => {
    serve(() => withGroup({ target_user_id: 'u-kaan' }))
    renderApp(PATH)
    expect(await screen.findByText('Kaan hesabı bağlı')).toBeInTheDocument()
  })

  it('lets the admin link the account straight from the status', async () => {
    let current: GroupDetail = groupDetail()
    let sent: unknown = null
    serve(() => current)
    server.use(
      http.patch(api(`/groups/${group.id}/target`), async ({ request }) => {
        sent = await request.json()
        current = withGroup({ target_user_id: 'u-kaan' }, { target_account: { id: 'u-kaan', name: 'Kaan Demir' } })
        return ok(current.group)
      }),
    )
    renderApp(PATH)
    const user = userEvent.setup()

    const label = await screen.findByText('Hesap bağlı değil')
    const chip = label.closest('[data-state]') as HTMLElement
    expect(chip).toHaveAttribute('data-state', 'not-linked')
    expect(chip.querySelector('svg')).toHaveAttribute('aria-hidden', 'true')

    await user.click(screen.getByRole('button', { name: 'Hesabı bağla' }))
    const dialog = await screen.findByRole('dialog', { name: 'Hedef kişiyi bağla' })
    await user.type(within(dialog).getByLabelText('E-posta'), 'kaan@ornek.com')
    await user.click(within(dialog).getByRole('button', { name: 'Bağla' }))

    expect(await screen.findByText('Kaan Demir hesabı bağlı')).toBeInTheDocument()
    expect(sent).toEqual({ email: 'kaan@ornek.com' })
    expect(screen.queryByRole('button', { name: 'Hesabı bağla' })).not.toBeInTheDocument()
    // Yönetici menüsündeki eski giriş noktası da artık görünmez.
    await user.click(screen.getByRole('button', { name: 'Grup işlemleri' }))
    expect(await screen.findByRole('menuitem', { name: 'Grubu sil' })).toBeInTheDocument()
    expect(screen.queryByRole('menuitem', { name: 'Hedef kişiyi bağla' })).not.toBeInTheDocument()
  })

  it('only informs a member when the account is not linked', async () => {
    serve(() => groupDetail({ my_role: 'member' }))
    renderApp(PATH)
    expect(await screen.findByText('Hesap bağlı değil')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Hesabı bağla' })).not.toBeInTheDocument()
  })

  it('speaks English', async () => {
    await i18n.changeLanguage('en')
    serve(() =>
      withGroup({ target_user_id: 'u-kaan', description: LONG }, { target_account: { id: 'u-kaan', name: 'Kaan' } }),
    )
    renderApp(PATH)
    expect(await screen.findByText('Kaan’s account is linked')).toBeInTheDocument()
    expect(await screen.findByRole('button', { name: 'Show more' })).toBeInTheDocument()
  })
})
