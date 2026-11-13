import { useState } from 'react'
import { act, fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { HttpResponse } from 'msw'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { CoverCropDialog } from '@/components/CoverCropDialog'
import { GroupCoverImage, GroupThumb } from '@/components/GroupCover'
import * as coverCrop from '@/lib/coverCrop'
import { renderWithProviders } from '@/test/render'
import { api, fail, http, server } from '@/test/server'

// jsdom görsel çözmez ve canvas çizmez; yükleme ve çizim adımları taklit edilir. Kırpma
// hesapları (cropRect, clampCrop…) gerçektir.
vi.mock('@/lib/coverCrop', async (importOriginal) => ({
  ...(await importOriginal<typeof coverCrop>()),
  prepareImage: vi.fn(),
  renderCrop: vi.fn(),
}))

const prepareImage = vi.mocked(coverCrop.prepareImage)
const renderCrop = vi.mocked(coverCrop.renderCrop)

const release = vi.fn()
const loaded = (width: number, height: number): coverCrop.LoadedImage => ({
  element: document.createElement('img'),
  url: 'blob:test/picked',
  width,
  height,
  release,
})

const photo = (name = 'tatil.jpg', type = 'image/jpeg', size = 2048) => {
  const file = new File(['x'], name, { type })
  Object.defineProperty(file, 'size', { value: size })
  return file
}

// Kırpma alanının görseldeki karşılığı: [sx, sy, sw, sh].
const cropOf = () =>
  screen
    .getByRole('group', { name: 'Kırpma alanı' })
    .getAttribute('data-crop')!
    .split(',')
    .map(Number)

function Harness({ onCropped = vi.fn(), pending = false, error = null as string | null }) {
  const [open, setOpen] = useState(true)
  return (
    <>
      <button onClick={() => setOpen(true)}>aç</button>
      <CoverCropDialog
        open={open}
        onOpenChange={setOpen}
        onCropped={onCropped}
        confirmLabel="Fotoğrafı kaydet"
        pending={pending}
        error={error}
      />
    </>
  )
}

// Dosya seçici gizli bir <input type="file">; applyAccept kapalı ki tür denetimini bileşen yapsın.
async function pick(file: File) {
  const user = userEvent.setup({ applyAccept: false })
  await user.upload(screen.getByLabelText('Fotoğraf seç', { selector: 'input' }), file)
  return user
}

describe('cover crop dialog', () => {
  beforeEach(() => {
    prepareImage.mockReset()
    renderCrop.mockReset()
    release.mockReset()
  })

  it('rejects oversized and unsupported files before reading them', async () => {
    renderWithProviders(<Harness />)
    await pick(photo('buyuk.jpg', 'image/jpeg', 40 * 1024 * 1024 + 1))
    expect(await screen.findByRole('alert')).toHaveTextContent('Fotoğraf en fazla 40 MB olabilir.')

    await pick(photo('hareketli.gif', 'image/gif'))
    expect(screen.getByRole('alert')).toHaveTextContent('Yalnızca JPEG, PNG veya WebP')

    expect(prepareImage).not.toHaveBeenCalled()
    expect(screen.queryByRole('group', { name: 'Kırpma alanı' })).not.toBeInTheDocument()
  })

  it('accepts a large original photo (over the old 10 MB limit) and shrinks it in the browser', async () => {
    prepareImage.mockResolvedValue(loaded(3200, 2400))
    renderWithProviders(<Harness />)
    const original = photo('IMG_48MP.jpg', 'image/jpeg', 25 * 1024 * 1024)
    await pick(original)
    expect(await screen.findByRole('group', { name: 'Kırpma alanı' })).toBeInTheDocument()
    expect(prepareImage).toHaveBeenCalledWith(original)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('reports unreadable, HEIC, too-small and too-large images', async () => {
    renderWithProviders(<Harness />)
    prepareImage.mockRejectedValueOnce(new Error('decode'))
    await pick(photo())
    expect(await screen.findByRole('alert')).toHaveTextContent('Fotoğraf okunamadı')

    // Tarayıcı HEIC'i açamadı (ör. Chrome): anlaşılır mesaj.
    prepareImage.mockRejectedValueOnce(new coverCrop.CoverImageError('heic'))
    await pick(photo('IMG_0001.HEIC', 'image/heic'))
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Bu tarayıcı HEIC fotoğrafları açamıyor'))

    prepareImage.mockRejectedValueOnce(new coverCrop.CoverImageError('dimensions'))
    await pick(photo())
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('çözünürlüğü çok yüksek'))

    prepareImage.mockResolvedValueOnce(loaded(40, 30))
    await pick(photo())
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Fotoğraf çok küçük'))
    expect(release).toHaveBeenCalled()
    expect(screen.queryByRole('group', { name: 'Kırpma alanı' })).not.toBeInTheDocument()
  })

  it('explains when the output cannot be made small enough for the server', async () => {
    const onCropped = vi.fn()
    prepareImage.mockResolvedValue(loaded(1920, 1080))
    renderCrop.mockRejectedValue(new coverCrop.CoverImageError('outputTooLarge'))
    renderWithProviders(<Harness onCropped={onCropped} />)
    const user = await pick(photo())
    await screen.findByRole('group', { name: 'Kırpma alanı' })
    await user.click(screen.getByRole('button', { name: 'Fotoğrafı kaydet' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Fotoğraf yeterince küçültülemedi')
    expect(onCropped).not.toHaveBeenCalled()
  })

  it('starts with a centred 4:3 crop and exports exactly the framed area', async () => {
    const onCropped = vi.fn()
    const blob = new Blob(['jpeg'], { type: 'image/jpeg' })
    prepareImage.mockResolvedValue(loaded(1920, 1080))
    renderCrop.mockResolvedValue(blob)
    renderWithProviders(<Harness onCropped={onCropped} />)
    const user = await pick(photo())

    const frame = await screen.findByRole('group', { name: 'Kırpma alanı' })
    expect(frame).toHaveAccessibleDescription(/ok tuşları/)
    expect(cropOf()).toEqual([240, 0, 1440, 1080])
    expect(screen.getByRole('img', { name: 'Seçilen fotoğrafın kırpma önizlemesi' })).toHaveAttribute('src', 'blob:test/picked')

    await user.click(screen.getByRole('button', { name: 'Fotoğrafı kaydet' }))
    await waitFor(() => expect(onCropped).toHaveBeenCalledWith(blob))
    expect(renderCrop).toHaveBeenCalledWith(expect.any(HTMLImageElement), { sx: 240, sy: 0, sw: 1440, sh: 1080 })
  })

  it('moves with the arrow keys, zooms with + and − and stays inside the image', async () => {
    prepareImage.mockResolvedValue(loaded(1920, 1080))
    renderWithProviders(<Harness />)
    const user = await pick(photo())
    const frame = await screen.findByRole('group', { name: 'Kırpma alanı' })
    frame.focus()

    // Sağ ok: çerçeve görselde sağa kayar (görünen genişliğin %5'i = 72 piksel).
    await user.keyboard('{ArrowRight}')
    expect(cropOf()).toEqual([312, 0, 1440, 1080])
    // Yakınlaştırma 1 iken dikeyde yer yok: aşağı ok bir şey değiştirmez.
    await user.keyboard('{ArrowDown}')
    expect(cropOf()[1]).toBe(0)
    // Kenara dayanınca durur.
    await user.keyboard('{Shift>}{ArrowLeft}{ArrowLeft}{ArrowLeft}{/Shift}')
    expect(cropOf()[0]).toBe(0)

    await user.keyboard('+')
    const [, , zoomedWidth, zoomedHeight] = cropOf()
    expect(zoomedWidth).toBe(1309) // 1440 / 1.1
    expect(zoomedWidth / zoomedHeight).toBeCloseTo(4 / 3, 2)
    await user.keyboard('{ArrowDown}')
    expect(cropOf()[1]).toBeGreaterThan(0)

    await user.keyboard('-')
    await user.keyboard('-')
    expect(cropOf().slice(2)).toEqual([1440, 1080]) // 1'in altına inmez
  })

  it('zooms with the labelled slider', async () => {
    prepareImage.mockResolvedValue(loaded(1920, 1080))
    renderWithProviders(<Harness />)
    await pick(photo())
    const slider = await screen.findByRole('slider', { name: 'Yakınlaştırma' })
    expect(slider).toHaveAttribute('min', '1')
    expect(slider).toHaveAttribute('max', '4')
    expect(slider).toHaveAttribute('aria-valuetext', '%100')

    fireEvent.change(slider, { target: { value: '2' } })
    expect(cropOf()).toEqual([600, 270, 720, 540])
    expect(slider).toHaveAttribute('aria-valuetext', '%200')
  })

  it('drags with the pointer and pinches with two fingers', async () => {
    prepareImage.mockResolvedValue(loaded(1920, 1080))
    renderWithProviders(<Harness />)
    await pick(photo())
    const frame = await screen.findByRole('group', { name: 'Kırpma alanı' })
    // Çerçeve ekranda 480 piksel genişlikte: 1 ekran pikseli = 3 görsel pikseli.
    frame.getBoundingClientRect = () => ({ width: 480, height: 360 }) as DOMRect

    fireEvent.pointerDown(frame, { pointerId: 1, clientX: 100, clientY: 100 })
    fireEvent.pointerMove(frame, { pointerId: 1, clientX: 80, clientY: 100 }) // görsel 20 px sola
    expect(cropOf()).toEqual([300, 0, 1440, 1080])
    fireEvent.pointerUp(frame, { pointerId: 1 })
    fireEvent.pointerMove(frame, { pointerId: 1, clientX: 0, clientY: 0 }) // bırakıldıktan sonra etkisiz
    expect(cropOf()).toEqual([300, 0, 1440, 1080])

    // İki parmak arası mesafe iki katına çıkar: iki kat yakınlaşır.
    fireEvent.pointerDown(frame, { pointerId: 1, clientX: 100, clientY: 100 })
    fireEvent.pointerDown(frame, { pointerId: 2, clientX: 200, clientY: 100 })
    fireEvent.pointerMove(frame, { pointerId: 2, clientX: 300, clientY: 100 })
    expect(cropOf().slice(2)).toEqual([720, 540])
  })

  it('shows the upload state and the server error, and blocks closing while uploading', async () => {
    prepareImage.mockResolvedValue(loaded(1920, 1080))
    const view = renderWithProviders(<Harness pending />)
    await pick(photo())
    const save = await screen.findByRole('button', { name: 'Yükleniyor…' })
    expect(save).toBeDisabled()
    expect(save).toHaveAttribute('aria-busy', 'true')
    expect(screen.getByRole('status')).toHaveTextContent('Yükleniyor…')
    expect(screen.getByRole('slider', { name: 'Yakınlaştırma' })).toBeDisabled()
    await userEvent.keyboard('{Escape}')
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    view.unmount()

    renderWithProviders(<Harness error="görsel okunamadı; dosya bozuk olabilir" />)
    expect(await screen.findByRole('alert')).toHaveTextContent('görsel okunamadı; dosya bozuk olabilir')
  })

  it('forgets the previous photo when reopened and releases its object URL', async () => {
    prepareImage.mockResolvedValue(loaded(1920, 1080))
    renderWithProviders(<Harness />)
    const user = await pick(photo())
    await screen.findByRole('group', { name: 'Kırpma alanı' })

    await user.click(screen.getByRole('button', { name: 'Vazgeç' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(release).toHaveBeenCalledTimes(1)

    await user.click(screen.getByRole('button', { name: 'aç' }))
    expect(await screen.findByRole('button', { name: /Fotoğraf seç/ })).toBeInTheDocument()
    expect(screen.queryByRole('group', { name: 'Kırpma alanı' })).not.toBeInTheDocument()
  })
})

describe('group cover image', () => {
  const jpeg = () => new HttpResponse(new Blob(['jpeg-bytes'], { type: 'image/jpeg' }), { headers: { 'Content-Type': 'image/jpeg' } })

  beforeEach(() => localStorage.setItem('covertgift.token', 'tok'))
  afterEach(() => vi.restoreAllMocks())

  it('fetches the image with the bearer token and shows it from an object URL', async () => {
    let auth: string | null = null
    let query = ''
    server.use(
      http.get(api('/groups/g1/cover'), ({ request }) => {
        auth = request.headers.get('Authorization')
        query = new URL(request.url).search
        return jpeg()
      }),
    )
    renderWithProviders(<GroupCoverImage groupId="g1" version="v1" alt="“Kaan” kapak fotoğrafı" className="w-40" />)
    const img = await screen.findByRole('img', { name: '“Kaan” kapak fotoğrafı' })
    expect(img.getAttribute('src')).toMatch(/^blob:/)
    expect(auth).toBe('Bearer tok')
    // Token URL'de yok; yalnızca sürüm var.
    expect(query).toBe('?v=v1')
  })

  it('makes no request and shows the fallback when the group has no photo', () => {
    // Tanımsız istek testi durdurur: /cover çağrılırsa bu test kırılır.
    renderWithProviders(<GroupCoverImage groupId="g1" alt="" fallback={<span>fotoğraf yok</span>} />)
    expect(screen.getByText('fotoğraf yok')).toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
  })

  it('falls back when the image cannot be loaded (deleted meanwhile or forbidden)', async () => {
    server.use(http.get(api('/groups/g1/cover'), () => fail(404, 'bu grubun kapak fotoğrafı yok')))
    renderWithProviders(<GroupCoverImage groupId="g1" version="v1" alt="kapak" fallback={<span>fotoğraf yok</span>} />)
    expect(await screen.findByText('fotoğraf yok')).toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
  })

  it('downloads a version once for several views and refetches when the version changes', async () => {
    const versions: string[] = []
    server.use(
      http.get(api('/groups/g1/cover'), ({ request }) => {
        versions.push(new URL(request.url).searchParams.get('v') ?? '')
        return jpeg()
      }),
    )
    function Views() {
      const [version, setVersion] = useState('v1')
      const group = { id: 'g1', cover_version: version, target_person_name: 'Kaan' }
      return (
        <>
          <GroupCoverImage groupId="g1" version={version} alt="büyük" />
          <GroupThumb group={group} />
          <GroupThumb group={group} size="sm" />
          <button onClick={() => setVersion('v2')}>değiştir</button>
        </>
      )
    }
    const { container } = renderWithProviders(<Views />)
    await waitFor(() => expect(container.querySelectorAll('img')).toHaveLength(3))
    expect(versions).toEqual(['v1'])
    const first = container.querySelector('img')!.getAttribute('src')

    await userEvent.click(screen.getByRole('button', { name: 'değiştir' }))
    await waitFor(() => expect(versions).toEqual(['v1', 'v2']))
    await waitFor(() => expect(container.querySelectorAll('img')).toHaveLength(3))
    expect(container.querySelector('img')!.getAttribute('src')).not.toBe(first)
  })

  it('shows the person avatar instead of a thumbnail when there is no photo', () => {
    const { container } = renderWithProviders(<GroupThumb group={{ id: 'g1', target_person_name: 'Kaan' }} />)
    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('.avatar')).toHaveTextContent('K')
  })

  it('revokes the object URL after the last view unmounts', async () => {
    const revoke = vi.spyOn(URL, 'revokeObjectURL')
    server.use(http.get(api('/groups/g1/cover'), () => jpeg()))
    const view = renderWithProviders(<GroupCoverImage groupId="g1" version="v1" alt="kapak" />)
    const src = (await screen.findByRole('img', { name: 'kapak' })).getAttribute('src')!
    expect(revoke).not.toHaveBeenCalledWith(src)

    view.unmount()
    await act(() => new Promise((resolve) => setTimeout(resolve, 10)))
    expect(revoke).toHaveBeenCalledWith(src)
  })
})
