import { expect, test, type APIRequestContext } from '@playwright/test'

// Bildirim zili: başka bir üyenin işlemi sayfa yenilenmeden zile düşer (SSE), tıklanınca
// okundu olur ve gruba gidilir; "Tümünü okundu yap" rozeti temizler.
// Gerçek API'ye karşı koşar ve kendi geçici kullanıcılarını oluşturup sonunda siler.

const API = 'http://localhost:5174/api/v1'
const PASSWORD = 'e2e-Test-7731'

async function register(request: APIRequestContext, name: string, email: string) {
  const reg = await request.post(`${API}/auth/register`, { data: { name, email, password: PASSWORD } })
  expect(reg.status(), await reg.text()).toBe(201)
  const login = await request.post(`${API}/auth/login`, { data: { email, password: PASSWORD } })
  expect(login.status()).toBe(200)
  return { Authorization: `Bearer ${(await login.json()).data.token as string}` }
}

test('notifications: live bell, mark read on click and mark all read', async ({ page, request }, info) => {
  const stamp = `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`
  const viewerEmail = `e2e-bell-viewer-${info.project.name}-${stamp}@demo.test`
  const adminEmail = `e2e-bell-admin-${info.project.name}-${stamp}@demo.test`
  const viewer = await register(request, 'Zil İzleyen', viewerEmail)
  const admin = await register(request, 'Zil Yönetici', adminEmail)

  try {
    await page.goto('/login')
    await page.getByLabel('E-posta').fill(viewerEmail)
    await page.getByLabel('Şifre').fill(PASSWORD)
    await page.getByRole('button', { name: 'Giriş yap' }).click()

    // Başlangıçta bildirim yok: rozet yok, boş durum görünür.
    const bell = page.getByRole('button', { name: 'Bildirimler', exact: true })
    await expect(bell).toBeVisible()
    await bell.click()
    await expect(page.getByText('Henüz bildirimin yok.')).toBeVisible()
    await page.keyboard.press('Escape')

    // Yönetici grup kurar, izleyeni ekler ve bir fikir ekler; sayfa yenilenmeden zile düşer.
    const created = await request.post(`${API}/groups`, {
      headers: admin,
      data: { title: `Zil grubu ${stamp}`, target_person_name: 'Hedef', total_budget: '1000' },
    })
    expect(created.status(), await created.text()).toBe(201)
    const groupId = (await created.json()).data.id as string
    const added = await request.post(`${API}/groups/${groupId}/members`, { headers: admin, data: { email: viewerEmail } })
    expect(added.status(), await added.text()).toBe(201)
    await expect(page.getByRole('button', { name: 'Bildirimler, 1 okunmamış' })).toBeVisible()

    const idea = await request.post(`${API}/groups/${groupId}/ideas`, {
      headers: admin,
      data: { title: 'Zil termosu', estimated_price: '500', search_query: 'termos' },
    })
    expect(idea.status(), await idea.text()).toBe(201)
    const unread = page.getByRole('button', { name: 'Bildirimler, 2 okunmamış' })
    await expect(unread).toBeVisible()

    // Liste: en yeni üstte. Telefonda da yatay taşma olmamalı.
    await unread.click()
    const menu = page.getByRole('menu')
    await expect(menu.getByRole('menuitem', { name: /Zil Yönetici yeni bir fikir ekledi: Zil termosu/ })).toBeVisible()
    await expect(menu.getByRole('menuitem', { name: /Zil Yönetici seni bu gruba ekledi/ })).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)

    // Tıklama: okundu olur ve gruba gider.
    await menu.getByRole('menuitem', { name: /Zil termosu/ }).click()
    await expect(page).toHaveURL(new RegExp(`/groups/${groupId}$`))
    const one = page.getByRole('button', { name: 'Bildirimler, 1 okunmamış' })
    await expect(one).toBeVisible()

    // Tümünü okundu yap: rozet kalkar, liste açık kalır.
    await one.click()
    await page.getByRole('menuitem', { name: 'Tümünü okundu yap' }).click()
    // "Okunmamış" süzgeç seçeneğinde de yazar; işaret yalnızca bildirim satırlarında aranır.
    await expect(page.getByRole('menu').getByRole('menuitem').getByText('Okunmamış')).toHaveCount(0)
    await page.keyboard.press('Escape')
    await expect(page.getByRole('button', { name: 'Bildirimler', exact: true })).toBeVisible()
  } finally {
    // Yönetici silinince grubu ve bildirimleri de silinir.
    for (const headers of [admin, viewer]) {
      await request.delete(`${API}/me`, { headers })
    }
  }
})
