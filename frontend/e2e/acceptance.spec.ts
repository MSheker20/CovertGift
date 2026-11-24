import { expect, test, type APIRequestContext, type Page } from '@playwright/test'

import { acceptGroupInvite } from './invites'

// Faz 7 kabul akışı (Gemini hariç): kayıt → grup kur → üye ekle → manuel fikir ekle → oyla.
// Ayrıca Anti-Spoiler: hedef kişi arayüzden e-postayla seçilir veya sonradan bağlanır;
// grup adresini açınca yalnızca sürpriz ekranını görür.

const API = 'http://localhost:5174/api/v1'
const PASSWORD = 'e2e-Test-7731'

type Account = { name: string; email: string; token?: string; id?: string }

function account(name: string, tag: string, projectName: string): Account {
  const stamp = `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`
  return { name, email: `e2e-${tag}-${projectName}-${stamp}@demo.test` }
}

async function registerViaApi(request: APIRequestContext, a: Account) {
  const reg = await request.post(`${API}/auth/register`, { data: { name: a.name, email: a.email, password: PASSWORD } })
  expect(reg.status(), await reg.text()).toBe(201)
  const login = await request.post(`${API}/auth/login`, { data: { email: a.email, password: PASSWORD } })
  const body = await login.json()
  a.token = body.data.token
  a.id = body.data.user.id
}

async function tokenOf(page: Page) {
  return page.evaluate(() => localStorage.getItem('covertgift.token'))
}

async function login(page: Page, a: Account) {
  await page.goto('/login')
  await page.getByLabel('E-posta').fill(a.email)
  await page.getByLabel('Şifre').fill(PASSWORD)
  await page.getByRole('button', { name: 'Giriş yap' }).click()
  await expect(page.getByRole('heading', { level: 1, name: new RegExp(`${a.name.split(' ')[0]}\\.$`) })).toBeVisible()
}

// Hedef kişi grubu listesinde görmez; adresini açınca yalnızca sürpriz ekranını görür.
async function expectSurpriseFor(page: Page, target: Account, title: string, groupUrl: string) {
  await page.evaluate(() => localStorage.removeItem('covertgift.token'))
  await login(page, target)
  await expect(page.getByText(title)).toHaveCount(0)
  await page.goto(groupUrl)
  await expect(page.getByRole('heading', { name: 'Bu bir sürpriz!' })).toBeVisible()
  await expect(page.getByText('Hediye havuzu')).toHaveCount(0)
}

async function deleteAccounts(request: APIRequestContext, accounts: Account[]) {
  for (const a of accounts) {
    if (!a.token) continue
    await request.delete(`${API}/me`, { headers: { Authorization: `Bearer ${a.token}` } })
  }
}

test.describe('acceptance flow', () => {
  const accounts: Account[] = []
  test.afterEach(async ({ request }) => {
    await deleteAccounts(request, accounts.splice(0))
  })

  test('register, create a group, add a member, add an idea and vote', async ({ page, request }, info) => {
    const admin = account('Deniz Aksoy', 'admin', info.project.name)
    const friend = account('Elif Kara', 'friend', info.project.name)
    accounts.push(admin, friend)
    await registerViaApi(request, friend)

    // Kayıt (kayıt sonrası otomatik giriş)
    await page.goto('/register')
    await page.getByLabel('Adın').fill(admin.name)
    await page.getByLabel('E-posta').fill(admin.email)
    await page.getByLabel('Şifre').fill(PASSWORD)
    await page.getByRole('button', { name: 'Hesabı aç' }).click()
    await expect(page.getByRole('heading', { level: 1, name: /Deniz\.$/ })).toBeVisible()
    admin.token = (await tokenOf(page)) ?? undefined

    // Grup kur
    await page.getByRole('button', { name: 'Yeni grup' }).first().click()
    const create = page.getByRole('dialog')
    await create.getByLabel('Grup adı').fill('Kaan’ın 30. yaşı')
    await create.getByLabel('Hediye kimin için?').fill('Kaan')
    await create.getByLabel('Toplam bütçe (₺)').fill('3000')
    await create.getByLabel('İlgi alanları').fill('Kamp, kahve')
    await create.getByRole('button', { name: 'Grubu kur' }).click()
    await expect(page.getByRole('heading', { level: 1, name: 'Kaan’ın 30. yaşı' })).toBeVisible()
    const perPerson = page.locator('div:has(> dt:text-is("Kişi başı"))')
    await expect(perPerson).toContainText('₺3.000,00')

    // Üye ekle → davet gider ("Bekliyor"); kabul edince kişi başı tutar ikiye bölünür
    await page.getByLabel('Üye ekle').fill(friend.email)
    await page.getByRole('button', { name: 'Ekle', exact: true }).click()
    await expect(page.getByText('Elif Kara davet edildi; kabul edince gruba katılır.')).toBeVisible()
    await expect(page.getByText('Bekliyor')).toBeVisible()
    await expect(perPerson).toContainText('₺3.000,00')
    await acceptGroupInvite(request, API, friend.token!, page.url().split('/groups/')[1])
    await expect(perPerson).toContainText('₺1.500,00')

    // Manuel fikir ekle (link önizlemesi dış ağa çıktığı için form elle doldurulur)
    await page.getByRole('button', { name: 'Manuel ekle' }).click()
    const add = page.getByRole('dialog')
    await add.getByLabel('Ürün adı').fill('Stanley termos 1 L')
    await add.getByLabel('Tahmini fiyat (₺)').fill('1450')
    await add.getByRole('button', { name: 'Havuza ekle' }).click()
    const card = page.getByRole('article').filter({ hasText: 'Stanley termos 1 L' })
    await expect(card).toBeVisible()
    await expect(card.getByRole('link', { name: /Trendyol/ })).toHaveAttribute('href', /trendyol\.com\/sr\?q=Stanley/)

    // Oyla: skor hemen artar ve sunucudan gelen sonuçla aynı kalır
    await card.getByRole('button', { name: 'Beğen', exact: true }).click()
    await expect(card.getByRole('group', { name: 'Skor 1' })).toBeVisible()
    await page.reload()
    await expect(page.getByRole('article').filter({ hasText: 'Stanley termos 1 L' }).getByRole('group', { name: 'Skor 1' })).toBeVisible()
  })

  test('a group created with the recipient’s email hides itself from them', async ({ page, request }, info) => {
    const admin = account('Deniz Aksoy', 'owner', info.project.name)
    const target = account('Kaan Demir', 'target', info.project.name)
    accounts.push(admin, target)
    await registerViaApi(request, admin)
    await registerViaApi(request, target)

    // Admin grubu arayüzden kurar; hedef kişiyi e-postasıyla seçer.
    await login(page, admin)
    await page.getByRole('button', { name: 'Yeni grup' }).first().click()
    const create = page.getByRole('dialog')
    await create.getByLabel('Grup adı').fill('Kaan’ın sürprizi')
    await create.getByLabel('Hediye kimin için?').fill('Kaan')
    await create.getByLabel(/Hedef kişinin e-postası/).fill(target.email)
    await create.getByLabel('Toplam bütçe (₺)').fill('2000')
    await create.getByRole('button', { name: 'Grubu kur' }).click()
    await expect(page.getByRole('heading', { level: 1, name: 'Kaan’ın sürprizi' })).toBeVisible()
    await expect(page.getByText('hesabı bağlı')).toBeVisible()
    const groupUrl = page.url()

    await expectSurpriseFor(page, target, 'Kaan’ın sürprizi', groupUrl)
  })

  test('an unregistered email stops the dialog; the admin links the recipient later', async ({ page, request }, info) => {
    const admin = account('Deniz Aksoy', 'linker', info.project.name)
    const target = account('Kaan Demir', 'later', info.project.name)
    accounts.push(admin, target)
    await registerViaApi(request, admin)

    await login(page, admin)
    await page.getByRole('button', { name: 'Yeni grup' }).first().click()
    const create = page.getByRole('dialog')
    await create.getByLabel('Grup adı').fill('Kaan’ın hediyesi')
    await create.getByLabel('Hediye kimin için?').fill('Kaan')
    const email = create.getByLabel(/Hedef kişinin e-postası/)
    await email.fill(target.email)
    await create.getByRole('button', { name: 'Grubu kur' }).click()
    await expect(create.getByText(/Bu e-posta kayıtlı değil/)).toBeVisible()

    // İsim yedeğiyle kurulur; hedef kişi henüz bağlı değildir.
    await email.clear()
    await create.getByRole('button', { name: 'Grubu kur' }).click()
    await expect(page.getByRole('heading', { level: 1, name: 'Kaan’ın hediyesi' })).toBeVisible()
    await expect(page.getByText(/hesabı bağlı$/)).toHaveCount(0)
    const groupUrl = page.url()

    // Kişi kayıt olduktan sonra admin menüden bağlar.
    await registerViaApi(request, target)
    await page.getByRole('button', { name: 'Grup işlemleri' }).click()
    await page.getByRole('menuitem', { name: 'Hedef kişiyi bağla' }).click()
    const link = page.getByRole('dialog')
    await link.getByLabel('E-posta').fill(target.email)
    await link.getByRole('button', { name: 'Bağla' }).click()
    await expect(page.getByText('Kaan artık bu grubu göremez.', { exact: false })).toBeVisible()
    await expect(page.getByText('hesabı bağlı')).toBeVisible()

    await expectSurpriseFor(page, target, 'Kaan’ın hediyesi', groupUrl)
  })
})
