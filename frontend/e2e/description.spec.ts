import { expect, test, type APIRequestContext, type Page } from '@playwright/test'

// Grup açıklaması ve hedef hesap durumu: açıklama başlığın altında en çok 3 satır görünür,
// "Devamını gör / Daha az" ile açılır; yönetici menüden düzenler. Kalkan durumu bağlı değilken
// uyarı tonunda ve yöneticiye "Hesabı bağla" eylemiyle, bağlanınca hesabın adıyla görünür.
// Üye yalnızca bilgi görür. Masaüstü ve telefon projelerinde koşar; gerçek API'ye karşı çalışır
// ve kendi geçici kullanıcılarını oluşturup sonunda siler.

const API = 'http://localhost:5174/api/v1'
const PASSWORD = 'e2e-Test-7731'
const LONG = Array.from({ length: 14 }, (_, i) => `Satır ${i + 1}: kampı seven Hatice için ortak hediye notu.`).join(' ')

type Account = { email: string; headers: { Authorization: string } }

async function account(request: APIRequestContext, name: string, tag: string): Promise<Account> {
  const email = `e2e-desc-${tag}-${Date.now()}-${Math.random().toString(36).slice(2, 7)}@demo.test`
  const reg = await request.post(`${API}/auth/register`, { data: { name, email, password: PASSWORD } })
  expect(reg.status(), await reg.text()).toBe(201)
  const login = await request.post(`${API}/auth/login`, { data: { email, password: PASSWORD } })
  return { email, headers: { Authorization: `Bearer ${(await login.json()).data.token as string}` } }
}

async function signIn(page: Page, email: string) {
  await page.goto('/login')
  await page.getByLabel('E-posta').fill(email)
  await page.getByLabel('Şifre').fill(PASSWORD)
  await page.getByRole('button', { name: 'Giriş yap' }).click()
  await expect(page).toHaveURL(/\/$/)
}

const noHorizontalOverflow = (page: Page) =>
  page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)

test('group description and target account status', async ({ page, request }, info) => {
  const tag = info.project.name
  const admin = await account(request, 'Açıklama Yönetici', `${tag}-admin`)
  const member = await account(request, 'Açıklama Üye', `${tag}-member`)
  const target = await account(request, 'Hatice Yılmaz', `${tag}-target`)
  try {
    const created = await request.post(`${API}/groups`, {
      headers: admin.headers,
      data: { title: `Açıklama grubu ${Date.now()}`, target_person_name: 'Hatice', total_budget: '1000', description: LONG },
    })
    expect(created.status(), await created.text()).toBe(201)
    const groupId = (await created.json()).data.id as string
    const add = await request.post(`${API}/groups/${groupId}/members`, { headers: admin.headers, data: { email: member.email } })
    expect(add.status()).toBe(201)

    await signIn(page, admin.email)
    await page.goto(`/groups/${groupId}`)

    // Uzun açıklama 3 satırla kesilir; düğme açar ve kapatır.
    const text = page.getByTestId('group-description')
    await expect(text).toHaveText(LONG)
    const clamped = (await text.boundingBox())!.height
    const lineHeight = await text.evaluate((el) => parseFloat(getComputedStyle(el).lineHeight))
    expect(Math.round(clamped / lineHeight)).toBe(3)
    const more = page.getByRole('button', { name: 'Devamını gör' })
    await expect(more).toHaveAttribute('aria-expanded', 'false')
    await more.click()
    const less = page.getByRole('button', { name: 'Daha az' })
    await expect(less).toHaveAttribute('aria-expanded', 'true')
    expect((await text.boundingBox())!.height).toBeGreaterThan(clamped)
    expect(await noHorizontalOverflow(page)).toBe(true)
    await less.click()
    await expect(page.getByRole('button', { name: 'Devamını gör' })).toBeVisible()

    // Hesap bağlı değil: uyarı tonu + yöneticiye eylem.
    await expect(page.getByText('Hesap bağlı değil')).toBeVisible()
    await page.getByRole('button', { name: 'Hesabı bağla' }).click()
    const link = page.getByRole('dialog', { name: 'Hedef kişiyi bağla' })
    await link.getByLabel('E-posta').fill(target.email)
    await link.getByRole('button', { name: 'Bağla' }).click()
    await expect(page.getByText('Hatice Yılmaz hesabı bağlı')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Hesabı bağla' })).toHaveCount(0)

    // Kısa açıklama: düğme yok.
    await page.getByRole('button', { name: 'Grup işlemleri' }).click()
    await page.getByRole('menuitem', { name: 'Açıklamayı düzenle' }).click()
    const edit = page.getByRole('dialog', { name: 'Grup açıklaması' })
    await expect(edit.getByLabel('Açıklama')).toHaveValue(LONG)
    await edit.getByLabel('Açıklama').fill('  Cuma akşamı sürpriz.  ')
    await expect(edit.getByText('20 / 1000 karakter')).toBeVisible()
    expect(await noHorizontalOverflow(page)).toBe(true)
    await edit.getByRole('button', { name: 'Kaydet' }).click()
    await expect(page.getByText('Açıklama kaydedildi.')).toBeVisible()
    await expect(text).toHaveText('Cuma akşamı sürpriz.')
    await expect(page.getByRole('button', { name: 'Devamını gör' })).toHaveCount(0)
    expect(await noHorizontalOverflow(page)).toBe(true)

    // Üye: açıklamayı ve bağlı hesabın adını görür, düzenleyemez.
    await page.evaluate(() => localStorage.clear())
    await signIn(page, member.email)
    await page.goto(`/groups/${groupId}`)
    await expect(text).toHaveText('Cuma akşamı sürpriz.')
    await expect(page.getByText('Hatice Yılmaz hesabı bağlı')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Grup işlemleri' })).toHaveCount(0)
    expect(await noHorizontalOverflow(page)).toBe(true)

    // Hedef kişi: sürpriz ekranı; açıklama ve kendi adı görünmez.
    await page.evaluate(() => localStorage.clear())
    await signIn(page, target.email)
    await page.goto(`/groups/${groupId}`)
    await expect(page.getByRole('heading', { name: 'Bu bir sürpriz!' })).toBeVisible()
    await expect(page.getByText('Cuma akşamı sürpriz.')).toHaveCount(0)
    await expect(page.getByText(/hesabı bağlı$/)).toHaveCount(0)
  } finally {
    for (const a of [member, target, admin]) await request.delete(`${API}/me`, { headers: a.headers })
  }
})
