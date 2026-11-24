import { deflateSync } from 'node:zlib'

import { expect, test, type Locator, type Page } from '@playwright/test'

// Grup kapak fotoğrafı: grup sayfasından seçilir, 4:3 kırpılır ve yüklenir; grup başlığında,
// ana ekrandaki "Sıradaki sürpriz" kartında, grup satırında ve grup seçicide görünür; yönetici
// izni kısıtlar ve fotoğrafı kaldırır. Masaüstü ve telefon projelerinde koşar.
// Gerçek API'ye karşı koşar ve kendi geçici kullanıcısını oluşturup sonunda siler.

const API = 'http://localhost:5174/api/v1'
const PASSWORD = 'e2e-Test-7731'

// Bağımlılıksız PNG üretici: w x h, yatay renk geçişli (sıkıştırılmış gövde birkaç KB).
function png(width: number, height: number): Buffer {
  const crcTable = Array.from({ length: 256 }, (_, n) => {
    let c = n
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1
    return c >>> 0
  })
  const crc = (buf: Buffer) => {
    let c = 0xffffffff
    for (const byte of buf) c = crcTable[(c ^ byte) & 0xff] ^ (c >>> 8)
    return (c ^ 0xffffffff) >>> 0
  }
  const chunk = (type: string, data: Buffer) => {
    const body = Buffer.concat([Buffer.from(type, 'ascii'), data])
    const out = Buffer.alloc(8 + data.length + 4)
    out.writeUInt32BE(data.length, 0)
    body.copy(out, 4)
    out.writeUInt32BE(crc(body), 8 + data.length)
    return out
  }
  const header = Buffer.alloc(13)
  header.writeUInt32BE(width, 0)
  header.writeUInt32BE(height, 4)
  header.set([8, 2, 0, 0, 0], 8) // 8 bit, RGB
  const raw = Buffer.alloc((width * 3 + 1) * height)
  for (let y = 0; y < height; y++) {
    const row = y * (width * 3 + 1)
    for (let x = 0; x < width; x++) {
      raw.set([Math.round((x / width) * 255), 120, Math.round((y / height) * 255)], row + 1 + x * 3)
    }
  }
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk('IHDR', header),
    chunk('IDAT', deflateSync(raw)),
    chunk('IEND', Buffer.alloc(0)),
  ])
}

// Görsel gerçekten çözüldü mü (object URL geçerli, içerik görsel)?
async function expectLoaded(img: Locator) {
  await expect(img).toBeVisible()
  await expect.poll(() => img.evaluate((el: HTMLImageElement) => el.complete && el.naturalWidth > 0)).toBe(true)
}

const noHorizontalOverflow = (page: Page) =>
  page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)

test('group cover: crop and upload, shown in header and home card, restrict and remove', async ({ page, request }, info) => {
  const phone = info.project.name === 'phone'
  const stamp = `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`
  const email = `e2e-cover-${info.project.name}-${stamp}@demo.test`
  const title = `Kapak grubu ${stamp}`
  const reg = await request.post(`${API}/auth/register`, { data: { name: 'Kapak Deneme', email, password: PASSWORD } })
  expect(reg.status(), await reg.text()).toBe(201)
  const login = await request.post(`${API}/auth/login`, { data: { email, password: PASSWORD } })
  const headers = { Authorization: `Bearer ${(await login.json()).data.token as string}` }

  try {
    // Tarihli grup: ana ekranda "Sıradaki sürpriz" kartı olur.
    const soon = new Date(Date.now() + 10 * 86_400_000).toISOString().slice(0, 10)
    const created = await request.post(`${API}/groups`, {
      headers,
      data: { title, target_person_name: 'Hedef', total_budget: '1000', event_date: soon },
    })
    expect(created.status(), await created.text()).toBe(201)
    const groupId = (await created.json()).data.id as string
    const groupState = async () => (await (await request.get(`${API}/groups/${groupId}`, { headers })).json()).data.group

    await page.goto('/login')
    await page.getByLabel('E-posta').fill(email)
    await page.getByLabel('Şifre').fill(PASSWORD)
    await page.getByRole('button', { name: 'Giriş yap' }).click()

    // Fotoğraf yokken: kart düz, görsel yok.
    const card = page.getByRole('link', { name: /Sıradaki sürpriz/ })
    await expect(card).toBeVisible()
    await expect(card.locator('img')).toHaveCount(0)
    const plainHeight = (await card.boundingBox())!.height

    await page.goto(`/groups/${groupId}`)
    const coverAlt = `“${title}” kapak fotoğrafı`
    await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible()
    await expect(page.getByRole('img', { name: coverAlt })).toHaveCount(0)

    // Seç → kırp (klavye ile) → yükle. 1280x720 (16:9) görsel 4:3'e kırpılır.
    await page.getByRole('button', { name: 'Grup işlemleri' }).click()
    await page.getByRole('menuitem', { name: 'Fotoğraf ekle' }).click()
    const dialog = page.getByRole('dialog', { name: 'Kapak fotoğrafı' })
    await dialog.locator('input[type=file]').setInputFiles({ name: 'manzara.png', mimeType: 'image/png', buffer: png(1280, 720) })
    const frame = dialog.getByRole('group', { name: 'Kırpma alanı' })
    await expect(frame).toHaveAttribute('data-crop', '160,0,960,720')
    const box = (await frame.boundingBox())!
    expect(box.width / box.height).toBeCloseTo(4 / 3, 1)

    await frame.focus()
    await page.keyboard.press('ArrowRight')
    await expect(frame).toHaveAttribute('data-crop', '208,0,960,720')
    await page.keyboard.press('+')
    await expect(dialog.getByRole('slider', { name: 'Yakınlaştırma' })).toHaveValue('1.1')
    // Sürükleme (fare/dokunma): görsel sağa çekilir, çerçeve görselde sola kayar.
    const before = await frame.getAttribute('data-crop')
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2)
    await page.mouse.down()
    await page.mouse.move(box.x + box.width / 2 + 40, box.y + box.height / 2, { steps: 4 })
    await page.mouse.up()
    await expect(frame).not.toHaveAttribute('data-crop', before!)
    expect(await noHorizontalOverflow(page)).toBe(true)

    await dialog.getByRole('button', { name: 'Fotoğrafı kaydet' }).click()
    await expect(page.getByText('Kapak fotoğrafı kaydedildi.')).toBeVisible()
    await expect(dialog).toBeHidden()

    // Grup başlığında görünür.
    const headerCover = page.getByRole('img', { name: coverAlt })
    await expectLoaded(headerCover)
    const headerBox = (await headerCover.boundingBox())!
    expect(headerBox.width / headerBox.height).toBeCloseTo(4 / 3, 1)
    expect(await noHorizontalOverflow(page)).toBe(true)

    // Sunucu 4:3 JPEG saklar; ETag grubun cover_version değeridir.
    const version = (await groupState()).cover_version as string
    expect(version).toBeTruthy()
    const stored = await request.get(`${API}/groups/${groupId}/cover`, { headers })
    expect(stored.status()).toBe(200)
    expect(stored.headers()['content-type']).toBe('image/jpeg')
    expect(stored.headers()['etag']).toBe(`"${version}"`)
    expect((await stored.body()).subarray(0, 3)).toEqual(Buffer.from([0xff, 0xd8, 0xff]))
    const notModified = await request.get(`${API}/groups/${groupId}/cover`, { headers: { ...headers, 'If-None-Match': `"${version}"` } })
    expect(notModified.status()).toBe(304)

    // Grup seçicide küçük görsel.
    await expectLoaded(page.getByRole('button', { name: 'Grup değiştir' }).locator('img'))

    // Ana ekran: kartta fotoğraf (masaüstünde sağda 4:3, telefonda üstte 16:9 şerit) ve satırda küçük görsel.
    await page.goto('/')
    const photo = card.locator('img')
    await expectLoaded(photo)
    const cardBox = (await card.boundingBox())!
    const photoBox = (await photo.boundingBox())!
    if (phone) {
      expect(photoBox.width / photoBox.height).toBeCloseTo(16 / 9, 1)
      expect(photoBox.width).toBeGreaterThan(cardBox.width * 0.8)
      expect(photoBox.y).toBeLessThan((await card.getByText(title).boundingBox())!.y)
      // Telefonda şerit kartın üstüne eklenir: kart uzar.
      expect(cardBox.height).toBeGreaterThan(plainHeight)
    } else {
      // Masaüstünde düz kart da en az 232px'tir (satır yüksekliğini o belirler); fotoğraf
      // kartı büyütmez, en fazla aynı yükseklikte kalır ya da biraz uzar.
      expect(cardBox.height).toBeGreaterThanOrEqual(plainHeight)
      expect(photoBox.width / photoBox.height).toBeCloseTo(4 / 3, 1)
      expect(photoBox.width / cardBox.width).toBeGreaterThan(0.33)
      expect(photoBox.width / cardBox.width).toBeLessThan(0.45)
      expect(photoBox.x).toBeGreaterThan(cardBox.x + cardBox.width / 2)
      expect(cardBox.height).toBeGreaterThanOrEqual(220)
      expect(cardBox.height).toBeLessThanOrEqual(260)
    }
    await expect(card.getByText(title)).toBeVisible()
    await expect(card.getByText('gün kaldı')).toBeVisible()
    await expectLoaded(page.getByRole('link', { name: new RegExp(`${title}.*1 üye`) }).locator('img'))
    expect(await noHorizontalOverflow(page)).toBe(true)

    // Yönetici: yalnızca yönetici değiştirsin.
    await card.click()
    await expect(page).toHaveURL(new RegExp(`/groups/${groupId}$`))
    await page.getByRole('button', { name: 'Grup işlemleri' }).click()
    const restrict = page.getByRole('menuitemcheckbox', { name: 'Fotoğrafı yalnızca yönetici değiştirsin' })
    await expect(restrict).not.toBeChecked()
    await restrict.click()
    await expect(page.getByText('Kapak fotoğrafını artık yalnızca yönetici değiştirebilir.')).toBeVisible()
    await expect(restrict).toBeChecked()
    expect((await groupState()).cover_admin_only).toBe(true)

    // Kaldır: onaydan sonra başlıktaki görsel gider.
    await page.getByRole('menuitem', { name: 'Fotoğrafı kaldır' }).click()
    await page.getByRole('dialog', { name: 'Kapak fotoğrafı kaldırılsın mı?' }).getByRole('button', { name: 'Fotoğrafı kaldır' }).click()
    await expect(page.getByText('Kapak fotoğrafı kaldırıldı.')).toBeVisible()
    await expect(page.getByRole('img', { name: coverAlt })).toHaveCount(0)
    expect((await groupState()).cover_version).toBeUndefined()
    expect((await request.get(`${API}/groups/${groupId}/cover`, { headers })).status()).toBe(404)
  } finally {
    // Kullanıcı silinince grubu ve kapağı da silinir.
    await request.delete(`${API}/me`, { headers })
  }
})

// Büyük fotoğrafı tarayıcı küçültür: 6000x4000 (24 MP) görsel seçilir, kırpma çalışma kopyası
// üzerinde yapılır ve sunucuya yalnızca 1600x1200 JPEG (2 MiB altında) gider. Sunucu ise
// küçültülmemiş görseli çözmeden reddeder.
test('group cover: a large photo is shrunk in the browser before upload', async ({ page, request }, info) => {
  test.setTimeout(60_000)
  const stamp = `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`
  const email = `e2e-cover-big-${info.project.name}-${stamp}@demo.test`
  const reg = await request.post(`${API}/auth/register`, { data: { name: 'Kapak Büyük', email, password: PASSWORD } })
  expect(reg.status(), await reg.text()).toBe(201)
  const login = await request.post(`${API}/auth/login`, { data: { email, password: PASSWORD } })
  const headers = { Authorization: `Bearer ${(await login.json()).data.token as string}` }

  try {
    const created = await request.post(`${API}/groups`, {
      headers,
      data: { title: `Büyük kapak ${stamp}`, target_person_name: 'Hedef', total_budget: '1000' },
    })
    expect(created.status(), await created.text()).toBe(201)
    const groupId = (await created.json()).data.id as string
    const big = png(6000, 4000)

    // Sunucu küçültülmemiş görseli kabul etmez (başlıktan, çözmeden 400).
    const direct = await request.put(`${API}/groups/${groupId}/cover`, {
      headers,
      multipart: { file: { name: 'buyuk.png', mimeType: 'image/png', buffer: big } },
    })
    expect(direct.status()).toBe(400)
    expect((await direct.json()).error).toContain('2000 piksel')

    // Yüklenen dosyanın boyutunu ve ilk baytlarını kaydeden fetch sarmalayıcısı.
    await page.addInitScript(() => {
      const uploads: { size: number; type: string; head: number[] }[] = []
      ;(window as unknown as { __coverUploads: typeof uploads }).__coverUploads = uploads
      const original = window.fetch.bind(window)
      window.fetch = async (input, init) => {
        const file = init?.body instanceof FormData ? init.body.get('file') : null
        if (file instanceof Blob) {
          const head = [...new Uint8Array(await file.slice(0, 3).arrayBuffer())]
          uploads.push({ size: file.size, type: file.type, head })
        }
        return original(input, init)
      }
    })
    await page.goto('/login')
    await page.getByLabel('E-posta').fill(email)
    await page.getByLabel('Şifre').fill(PASSWORD)
    await page.getByRole('button', { name: 'Giriş yap' }).click()
    await expect(page).not.toHaveURL(/\/login$/)
    await page.goto(`/groups/${groupId}`)
    await page.getByRole('button', { name: 'Grup işlemleri' }).click()
    await page.getByRole('menuitem', { name: 'Fotoğraf ekle' }).click()
    const dialog = page.getByRole('dialog', { name: 'Kapak fotoğrafı' })
    await dialog.locator('input[type=file]').setInputFiles({ name: 'buyuk.png', mimeType: 'image/png', buffer: big })

    // Çalışma kopyası 3200x2133: ortadan 4:3 kırpma 2844x2133.
    const frame = dialog.getByRole('group', { name: 'Kırpma alanı' })
    await expect(frame).toHaveAttribute('data-crop', '178,0,2844,2133')
    await expect.poll(() => dialog.getByRole('img').evaluate((el: HTMLImageElement) => [el.naturalWidth, el.naturalHeight])).toEqual([3200, 2133])

    const response = page.waitForResponse((r) => r.request().method() === 'PUT' && r.url().endsWith(`/groups/${groupId}/cover`))
    await dialog.getByRole('button', { name: 'Fotoğrafı kaydet' }).click()
    const res = await response
    // Chromium blob içeren multipart gövdesini Playwright'a vermez; gönderilen dosya fetch
    // sarmalayıcısında kaydedildi.
    const sent = await page.evaluate(() => (window as unknown as { __coverUploads: { size: number; type: string; head: number[] }[] }).__coverUploads)
    expect(sent).toHaveLength(1)
    expect(sent[0].type).toBe('image/jpeg')
    expect(sent[0].head).toEqual([0xff, 0xd8, 0xff]) // PNG değil, JPEG gönderildi
    expect(sent[0].size).toBeLessThan(2 * 1024 * 1024)
    expect(res.status()).toBe(200)
    expect((await res.json()).data).toMatchObject({ width: 1600, height: 1200 })
    await expect(page.getByText('Kapak fotoğrafı kaydedildi.')).toBeVisible()
  } finally {
    await request.delete(`${API}/me`, { headers })
  }
})
