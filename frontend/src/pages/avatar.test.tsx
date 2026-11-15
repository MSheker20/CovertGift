import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { HttpResponse } from 'msw'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as coverCrop from '@/lib/coverCrop'
import { renderApp } from '@/test/render'
import { api, eventStream, fail, group, groupDetail, http, me, ok, server } from '@/test/server'

// jsdom görsel çözmez ve canvas çizmez; kırpma adımının iki tarayıcı işlemi taklit edilir.
vi.mock('@/lib/coverCrop', async (importOriginal) => ({
  ...(await importOriginal<typeof coverCrop>()),
  prepareImage: vi.fn(),
  renderCrop: vi.fn(),
}))

const cropped = new Blob(['kare-jpeg'], { type: 'image/jpeg' })
const jpeg = () => new HttpResponse(new Blob(['jpeg'], { type: 'image/jpeg' }), { headers: { 'Content-Type': 'image/jpeg' } })

// Sayfadaki avatarların durumu (fotoğraf ya da baş harf); avatarlar dekoratif olduğu için DOM'dan okunur.
const avatarStates = () => [...document.querySelectorAll('[data-avatar]')].map((el) => el.getAttribute('data-avatar'))

beforeEach(() => {
  vi.mocked(coverCrop.prepareImage).mockResolvedValue({
    element: document.createElement('img'),
    url: 'blob:test/picked',
    width: 1200,
    height: 900,
    release: () => {},
  })
  vi.mocked(coverCrop.renderCrop).mockResolvedValue(cropped)
})

async function pickAndSave(user: ReturnType<typeof userEvent.setup>) {
  const dialog = await screen.findByRole('dialog', { name: 'Profil fotoğrafı' })
  await user.upload(within(dialog).getByLabelText('Fotoğraf seç', { selector: 'input' }), new File(['x'], 'ben.jpg', { type: 'image/jpeg' }))
  await within(dialog).findByRole('group', { name: 'Kırpma alanı' })
  await user.click(within(dialog).getByRole('button', { name: 'Fotoğrafı kaydet' }))
  return dialog
}

describe('profile photo', () => {
  it('adds a square photo, shows it, then removes it back to initials', async () => {
    const requests: string[] = []
    let uploadType = ''
    server.use(
      http.get(api('/me'), () => ok(me)),
      http.get(api('/groups'), () => ok([])),
      http.put(api('/me/avatar'), ({ request }) => {
        uploadType = request.headers.get('Content-Type') ?? ''
        return ok({ avatar_version: 'v1', width: 512, height: 512 })
      }),
      http.delete(api('/me/avatar'), () => ok({ message: 'ok' })),
      http.get(api(`/users/${me.id}/avatar`), ({ request }) => {
        requests.push(new URL(request.url).search)
        return jpeg()
      }),
    )
    renderApp('/profile', { token: 'tok' })
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Fotoğraf ekle' }))
    await pickAndSave(user)
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(uploadType).toMatch(/^multipart\/form-data; boundary=/)
    expect(coverCrop.renderCrop).toHaveBeenCalledWith(
      expect.any(HTMLImageElement),
      { sx: 150, sy: 0, sw: 900, sh: 900 },
      expect.objectContaining({ aspect: 1, maxWidth: 512 }),
    )
    expect(await screen.findByText('Profil fotoğrafın kaydedildi.')).toBeInTheDocument()
    await waitFor(() => expect(avatarStates().every((s) => s === 'photo')).toBe(true))
    expect(requests).toContain('?v=v1')

    await user.click(screen.getByRole('button', { name: 'Fotoğrafı kaldır' }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Fotoğrafı kaldır' }))
    expect(await screen.findByText('Profil fotoğrafın kaldırıldı.')).toBeInTheDocument()
    await waitFor(() => expect(avatarStates().every((s) => s === 'initials')).toBe(true))
    expect(screen.getByRole('button', { name: 'Fotoğraf ekle' })).toBeInTheDocument()
  })

  it('keeps the dialog open with the server message when the upload is rejected', async () => {
    server.use(
      http.get(api('/me'), () => ok(me)),
      http.get(api('/groups'), () => ok([])),
      http.put(api('/me/avatar'), () => fail(415, 'yalnızca JPEG, PNG veya WebP görseller desteklenir')),
    )
    renderApp('/profile', { token: 'tok' })
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Fotoğraf ekle' }))
    const dialog = await pickAndSave(user)
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('yalnızca JPEG, PNG veya WebP')
    expect(avatarStates().every((s) => s === 'initials')).toBe(true)
  })
})

describe('member avatars', () => {
  it('shows a member photo and falls back to initials when the photo cannot be fetched', async () => {
    const detail = groupDetail({ my_role: 'admin' })
    const withPhoto = (detail.members ?? []).map((m, i) => ({ ...m, user: { ...m.user!, avatar_version: i === 0 ? 'v-me' : 'v-other' } }))
    server.use(
      http.get(api('/me'), () => ok({ ...me, avatar_version: 'v-me' })),
      http.get(api('/groups'), () => ok([group])),
      http.get(api(`/groups/${group.id}`), () => ok({ ...detail, members: withPhoto })),
      http.get(api(`/groups/${group.id}/ideas`), () => ok([])),
      http.get(api(`/groups/${group.id}/events`), () => eventStream()),
      http.get(api('/users/:id/avatar'), ({ params }) => (params.id === me.id ? jpeg() : fail(404, 'profil fotoğrafı bulunamadı'))),
    )
    renderApp(`/groups/${group.id}`, { token: 'tok' })
    const panel = (await screen.findByRole('heading', { name: 'Üyeler' })).closest('section')!
    await waitFor(() => {
      const states = [...panel.querySelectorAll('[data-avatar]')].map((el) => el.getAttribute('data-avatar'))
      expect(states[0]).toBe('photo')
      expect(states.slice(1).every((s) => s === 'initials')).toBe(true)
    })
  })
})
