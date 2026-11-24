import { expect, test, type APIRequestContext, type Page } from '@playwright/test'

import { acceptGroupInvite } from './invites'

// MULTI grup (birden çok kişi için hediye): yönetici grubu kurar, iki üye ekler, birini ve kendini
// hediye alınacak kişi yapar (kendi havuzunu diğer üye yönetir). Sorumlu yöneticinin havuzuna fikir ve
// bütçe girer; yönetici hiçbirini görmez, ilk alıcı katkı anahtarını değiştirince kişi başı tutar değişir.
// Gerçek API'ye karşı koşar, kendi geçici kullanıcılarını oluşturur ve sonunda siler. Gemini çağrılmaz.

const PASSWORD = 'e2e-Test-7731'
const IDEA = 'Stanley termos 1 L'

type Account = { name: string; email: string; token?: string }

function account(name: string, tag: string, projectName: string): Account {
  const stamp = `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`
  return { name, email: `e2e-${tag}-${projectName}-${stamp}@demo.test` }
}

async function registerViaApi(request: APIRequestContext, api: string, a: Account) {
  const reg = await request.post(`${api}/auth/register`, { data: { name: a.name, email: a.email, password: PASSWORD } })
  expect(reg.status(), await reg.text()).toBe(201)
  const login = await request.post(`${api}/auth/login`, { data: { email: a.email, password: PASSWORD } })
  a.token = (await login.json()).data.token
}

async function login(page: Page, a: Account) {
  await page.goto('/login')
  await page.evaluate(() => localStorage.removeItem('covertgift.token'))
  await page.goto('/login')
  await page.getByLabel('E-posta').fill(a.email)
  await page.getByLabel('Şifre').fill(PASSWORD)
  await page.getByRole('button', { name: 'Giriş yap' }).click()
  await expect(page.getByRole('heading', { level: 1, name: new RegExp(`${a.name.split(' ')[0]}\\.$`) })).toBeVisible()
}

test('multi-recipient group: manager runs the admin’s pool, the admin sees nothing, a contributor toggles', async ({ page, request, baseURL }, info) => {
  const api = `${baseURL ?? 'http://localhost:5174'}/api/v1`
  const admin = account('Deniz Aksoy', 'madmin', info.project.name)
  const first = account('Elif Kara', 'mfirst', info.project.name)
  const manager = account('Can Er', 'mmanager', info.project.name)
  const accounts = [admin, first, manager]
  let groupId = ''

  try {
    for (const a of accounts) await registerViaApi(request, api, a)

    // 1. Yönetici MULTI grubu kurar; hedef/bütçe alanları yoktur.
    await login(page, admin)
    await page.getByRole('button', { name: 'Yeni grup' }).first().click()
    const create = page.getByRole('dialog')
    await create.getByRole('radio', { name: 'Birden çok kişi için hediye' }).check()
    await expect(create.getByLabel('Hediye kimin için?')).toHaveCount(0)
    await create.getByLabel('Grup adı').fill('Ofis yılbaşı')
    await create.getByRole('button', { name: 'Grubu kur' }).click()
    await expect(page.getByRole('heading', { level: 1, name: 'Ofis yılbaşı' })).toBeVisible()
    const groupUrl = page.url()
    groupId = groupUrl.split('/groups/')[1]

    // 2. İki kişiyi davet eder; ikisi de kabul eder.
    for (const a of [first, manager]) {
      await page.getByLabel('Üye ekle').fill(a.email)
      await page.getByRole('button', { name: 'Ekle', exact: true }).click()
      await expect(page.getByText(`${a.name} davet edildi; kabul edince gruba katılır.`)).toBeVisible()
      await acceptGroupInvite(request, api, a.token!, groupId)
    }
    await expect(page.getByRole('button', { name: `${manager.name} için işlemler` })).toBeVisible()

    // 3. İlk üyeyi hediye alınacak kişi yapar.
    await page.getByRole('button', { name: `${first.name} için işlemler` }).click()
    await page.getByRole('menuitem', { name: 'Hediye alınacak kişi yap' }).click()
    const makeFirst = page.getByRole('dialog')
    await makeFirst.getByRole('button', { name: 'Hediye alınacak kişi yap' }).click()
    await expect(page.getByRole('tab', { name: first.name })).toBeVisible()

    // 4. Kendini alıcı yapar: sorumlu seçmeden gönderilmez, ikinci üyeyi seçince olur.
    await page.getByRole('button', { name: `${admin.name} için işlemler` }).click()
    await page.getByRole('menuitem', { name: 'Hediye alınacak kişi yap' }).click()
    const makeSelf = page.getByRole('dialog')
    await makeSelf.getByRole('button', { name: 'Hediye alınacak kişi yap' }).click()
    await expect(makeSelf.getByRole('alert')).toContainText('Havuzu yönetecek bir üye seçmelisin')
    await makeSelf.getByLabel('Sorumlu üye').selectOption({ label: manager.name })
    await makeSelf.getByRole('button', { name: 'Hediye alınacak kişi yap' }).click()
    const ownTab = page.getByRole('tab', { name: `${admin.name} (sen)` })
    await expect(ownTab).toBeVisible()
    const adminToken = await page.evaluate(() => localStorage.getItem('covertgift.token'))
    const detail = await request.get(`${api}/groups/${groupId}`, { headers: { Authorization: `Bearer ${adminToken}` } })
    const ownPoolId = ((await detail.json()).data.pools as { id: string; locked: boolean }[]).find((p) => p.locked)!.id

    // 5. Sorumlu, yöneticinin havuzuna fikir ve bütçe girer.
    await login(page, manager)
    await page.goto(groupUrl)
    await page.getByRole('tab', { name: admin.name }).click()
    await expect(page.getByRole('heading', { level: 2, name: `${admin.name} için` })).toBeVisible()
    await page.getByRole('button', { name: 'Bütçeyi düzenle' }).click()
    await page.getByLabel('Yeni toplam bütçe (₺)').fill('300')
    await page.getByLabel('Yeni toplam bütçe (₺)').press('Enter')
    await expect(page.getByTestId('per-person-caption')).toContainText('2 katılımcıya bölünür')
    await page.getByRole('button', { name: 'Manuel ekle' }).click()
    const add = page.getByRole('dialog')
    await add.getByLabel('Ürün adı').fill(IDEA)
    await add.getByLabel('Tahmini fiyat (₺)').fill('250')
    await add.getByRole('button', { name: 'Havuza ekle' }).click()
    await expect(page.getByRole('heading', { level: 3, name: IDEA })).toBeVisible()

    // 6. Yönetici hiçbirini görmez; kendi havuzuna istek bile atılmaz; zil boştur.
    const poolRequests: string[] = []
    page.on('request', (r) => {
      if (r.url().includes('/pools/')) poolRequests.push(r.url())
    })
    await login(page, admin)
    await page.goto(groupUrl)
    await ownTab.click()
    await expect(page.getByRole('heading', { name: 'Senin için bir sürpriz hazırlanıyor' })).toBeVisible()
    await expect(page.getByText(IDEA)).toHaveCount(0)
    await expect(page.getByText(/₺300,00/)).toHaveCount(0)
    // Zilde yalnızca davet ettiği kişilerin "katıldı" bildirimleri var; kendi havuzuna dair hiçbir şey yok.
    const bell = page.getByRole('button', { name: /^Bildirimler/ })
    await bell.click()
    const menu = page.getByRole('menu')
    for (const a of [first, manager]) {
      await expect(menu.getByRole('menuitem', { name: new RegExp(`${a.name} gruba katıldı`) })).toBeVisible()
    }
    await expect(menu.getByText(IDEA)).toHaveCount(0)
    await expect(menu.getByRole('menuitem', { name: /fikir|bütçe|kazanan/ })).toHaveCount(0)
    await page.keyboard.press('Escape')
    expect(poolRequests.filter((u) => u.includes(ownPoolId))).toEqual([])

    // 7. İlk alıcı yöneticinin havuzunda katkı anahtarını kapatır; kişi başı tutar değişir.
    await login(page, first)
    await page.goto(groupUrl)
    await page.getByRole('tab', { name: admin.name }).click()
    await expect(page.getByRole('heading', { level: 3, name: IDEA })).toBeVisible()
    await expect(page.getByTestId('per-person-caption')).toContainText('2 katılımcıya bölünür')
    await expect(page.getByText('₺150,00').first()).toBeVisible()
    await page.getByRole('switch', { name: 'Bu hediyeye ortağım' }).click()
    await expect(page.getByTestId('per-person-caption')).toContainText('1 katılımcıya bölünür')
    await expect(page.getByText('₺300,00').first()).toBeVisible()
  } finally {
    if (groupId && admin.token) {
      await request.delete(`${api}/groups/${groupId}`, { headers: { Authorization: `Bearer ${admin.token}` } })
    }
    for (const a of accounts) {
      if (a.token) await request.delete(`${api}/me`, { headers: { Authorization: `Bearer ${a.token}` } })
    }
  }
})
