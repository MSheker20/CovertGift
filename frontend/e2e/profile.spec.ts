import { expect, test } from '@playwright/test'

// Profil ekranı: hesap menüsünden açılır; isim/e-posta güncellenir, şifre değişir, hesap silinir.
// acceptance.spec.ts gibi gerçek API'ye karşı koşar ve kendi geçici kullanıcısını oluşturur.

const API = 'http://localhost:5174/api/v1'
const PASSWORD = 'e2e-Test-7731'
const NEW_PASSWORD = 'e2e-Yeni-8842'

test('profile: update details, change password and delete the account', async ({ page, request }, info) => {
  const stamp = `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`
  const email = `e2e-profile-${info.project.name}-${stamp}@demo.test`
  const newEmail = `e2e-profile-new-${info.project.name}-${stamp}@demo.test`
  const reg = await request.post(`${API}/auth/register`, { data: { name: 'Profil Deneme', email, password: PASSWORD } })
  expect(reg.status(), await reg.text()).toBe(201)

  // Test nerede durursa dursun hesap finally bloğunda silinir (hangi e-posta/şifre geçerliyse).
  try {
    await page.goto('/login')
    await page.getByLabel('E-posta').fill(email)
    await page.getByLabel('Şifre').fill(PASSWORD)
    await page.getByRole('button', { name: 'Giriş yap' }).click()
    await expect(page.getByRole('heading', { level: 1, name: /Profil\.$/ })).toBeVisible()

    await page.getByRole('button', { name: 'Hesap menüsü' }).click()
    await page.getByRole('menuitem', { name: 'Profil' }).click()
    await expect(page).toHaveURL(/\/profile$/)
    await expect(page.getByRole('heading', { level: 1, name: 'Profil Deneme' })).toBeVisible()

    // Hesap bilgileri: kaydet önce pasif, değişince aktif; e-posta değişince yeni token saklanır.
    const save = page.getByRole('button', { name: 'Kaydet' })
    await expect(save).toBeDisabled()
    const oldToken = await page.evaluate(() => localStorage.getItem('covertgift.token'))
    await page.getByLabel('Adın').fill('Profil Yeni')
    await page.getByLabel('E-posta').fill(newEmail)
    await save.click()
    await expect(page.getByText('Hesap bilgilerin güncellendi.')).toBeVisible()
    await expect(page.getByRole('heading', { level: 1, name: 'Profil Yeni' })).toBeVisible()
    await expect(save).toBeDisabled()
    expect(await page.evaluate(() => localStorage.getItem('covertgift.token'))).not.toBe(oldToken)

    // Yanlış mevcut şifre alanın altında görünür, oturum kapanmaz.
    await page.getByLabel('Mevcut şifre').fill('yanlis-sifre')
    await page.getByLabel('Yeni şifre', { exact: true }).fill(NEW_PASSWORD)
    await page.getByLabel('Yeni şifre (tekrar)').fill(NEW_PASSWORD)
    await page.getByRole('button', { name: 'Şifreyi değiştir' }).click()
    await expect(page.getByRole('alert').filter({ hasText: 'Mevcut şifre hatalı.' })).toBeVisible()
    await expect(page).toHaveURL(/\/profile$/)

    await page.getByLabel('Mevcut şifre').fill(PASSWORD)
    await page.getByRole('button', { name: 'Şifreyi değiştir' }).click()
    await expect(page.getByText('Şifren değiştirildi.')).toBeVisible()
    await expect(page.getByLabel('Mevcut şifre')).toHaveValue('')

    // Yeni e-posta ve yeni şifreyle giriş olur, eskisiyle olmaz.
    const fresh = await request.post(`${API}/auth/login`, { data: { email: newEmail, password: NEW_PASSWORD } })
    expect(fresh.status()).toBe(200)
    const stale = await request.post(`${API}/auth/login`, { data: { email: newEmail, password: PASSWORD } })
    expect(stale.status()).toBe(401)

    // Hesabı sil: onaydan sonra giriş ekranı, hesap artık yok.
    await page.getByRole('button', { name: 'Hesabı sil' }).click()
    await page.getByRole('dialog').getByRole('button', { name: 'Evet, hesabımı sil' }).click()
    await expect(page).toHaveURL(/\/login$/)
    await expect(page.getByRole('heading', { name: 'Giriş yap' })).toBeVisible()
    const gone = await request.post(`${API}/auth/login`, { data: { email: newEmail, password: NEW_PASSWORD } })
    expect(gone.status()).toBe(401)
  } finally {
    for (const [e, p] of [
      [newEmail, NEW_PASSWORD],
      [newEmail, PASSWORD],
      [email, PASSWORD],
    ]) {
      const res = await request.post(`${API}/auth/login`, { data: { email: e, password: p } })
      if (res.status() !== 200) continue
      const token = (await res.json()).data.token
      await request.delete(`${API}/me`, { headers: { Authorization: `Bearer ${token}` } })
    }
  }
})
