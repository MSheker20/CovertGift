import { defineConfig, devices } from '@playwright/test'

// Uçtan uca testler gerçek Go API'ye (varsayılan 127.0.0.1:8080) ve PostgreSQL'e karşı koşar.
// API'yi önceden başlat: `cd backend && go run ./cmd/api`. Vite geliştirme sunucusu otomatik açılır.
// Testler kendi geçici kullanıcılarını oluşturur ve sonunda `DELETE /me` ile siler. Gemini çağrılmaz.
export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  retries: 0,
  reporter: [['list']],
  use: {
    baseURL: 'http://localhost:5174',
    locale: 'tr-TR',
    trace: 'retain-on-failure',
  },
  projects: [
    { name: 'desktop', use: { ...devices['Desktop Chrome'] } },
    { name: 'phone', use: { ...devices['Pixel 7'] } },
  ],
  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:5174',
    reuseExistingServer: true,
    timeout: 60_000,
  },
})
