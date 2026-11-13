# CovertGift — Frontend

Kalıcı React arayüzü (Faz 7). Vite + React 19 + TypeScript, Tailwind CSS v4, shadcn/ui bileşenleri (`src/components/ui`), Lucide, React Router, TanStack Query, i18next (TR/EN).

## Çalıştırma

Önce veritabanı ve API (proje kökünden): `docker compose up -d`, ardından `cd backend && go run ./cmd/api`.

```bash
cd frontend
npm install
npm run dev        # http://localhost:5174 — /api ve /health istekleri 127.0.0.1:8080'e iletilir
npm run build      # tsc + üretim derlemesi
```

Testler:

```bash
npm test               # Vitest + Testing Library + MSW (API taklit edilir)
npm run test:coverage  # kapsam raporu (coverage/)
npm run test:e2e       # Playwright; önce Go API'yi başlat, Vite sunucusunu kendisi açar
```

Durum (2026-11-13): Vitest 14 dosyada 151 test; Playwright 5 dosyada 8 test, `desktop` ve `phone` (Pixel 7) projelerinde = 16 senaryo (`e2e/acceptance`, `profile`, `notifications`, `cover`, `description`).

Uçtan uca testler kendi geçici `e2e-…@demo.test` kullanıcılarını oluşturur ve sonunda `DELETE /me` ile siler; Gemini çağrılmaz. İlk kurulumda tarayıcı için `npx playwright install chromium` gerekir.

Ayarlar `.env.example` içinde: `API_TARGET` (geliştirme proxy hedefi), `VITE_API_BASE_URL` (üretimde API ayrı bir origin'deyse).

## Yapı

- `src/api/` — fetch istemcisi (`{success, data | error}` zarfı, Bearer token, 401 olayı), uç noktalar ve tipler; `eventStream.ts` (ortak SSE okuyucu, fetch ile; token URL'de değil), `useGroupEvents` (grup akışı; bağlantı yoksa 10 sn, varken 60 sn yenileme), `useNotificationEvents` (kullanıcı bildirim akışı), `useGroupCover` (kapak görseli Bearer ile Blob olarak alınır, `['cover', grup, sürüm]` anahtarıyla önbelleğe alınır, object URL son kullanan ayrılınca serbest bırakılır)
- `src/auth/` — oturum durumu; token `localStorage`'da (`covertgift.token`); `updateUser` profil değişikliğinden sonra önbelleği ve gerekirse token'ı günceller
- `src/i18n/locales/` — `tr.ts` ana sözlük, `en.ts` aynı tipte; dil `covertgift.lang` anahtarında saklanır
- `src/components/` — `NotificationBell` (üst bar zili: okunmamış rozeti, açılır liste, tümünü okundu), `CoverCropDialog` (bağımlılıksız 4:3 kırpma: sürükleme, tekerlek/iki parmak/kaydırıcı/klavye ile yakınlaştırma), `GroupCover` (`GroupCoverImage`, `GroupThumb`), `ConfirmDialog`, `Avatar`, `Panel`, `FormField`; `ui/` shadcn tarzı temel bileşenler
- `src/pages/` — Giriş, Kayıt, uygulama kabuğu (yüzen üst bar: grup seçici, dil, bildirim zili, hesap menüsü), ana ekran (`GroupsPage`: sıradaki sürpriz, senden bekleyenler, gruplarım, geçmiş sürprizler), Yeni grup diyaloğu (hedef kişinin e-postası, açıklama, isteğe bağlı kapak), Grup (Anti-Spoiler sürpriz ekranı dahil), Profil (`ProfilePage`: ad/e-posta, şifre, hesap silme)
- `src/pages/group/` — hediye havuzu ve oy kartları (oyu geri alma), birden çok kazanan seçimi (`WinnersDialog`), manuel ekleme + link önizleme, üye ekleme, yerinde yönetim (`GroupManage`: ilgi alanı, bütçe, hediye tarihi, açıklama, kapak ve kapak yetkisi, grubu yeniden açma, grubu silme, hedef kişiyi bağlama), `GroupHeaderInfo` (açıklama: 3 satırda kesilir, "Devamını gör"; hedef hesap durumu: "{ad} hesabı bağlı" / "Hesap bağlı değil")
- `src/lib/` — `eventDate.ts` (geri sayım, sıralama), `priority.ts` (ana ekran öncelikleri), `coverCrop.ts` (kırpma hesapları, tarayıcıda küçültme: 40 MB / 100 MP'ye kadar seçim, 3200 px çalışma kopyası, çıktı ≤ 2 MiB), `notifications.ts` (bildirim metinleri), `description.ts`, `marketplace.ts`
- `src/test/` — MSW sunucusu, örnek veriler ve render yardımcıları; `e2e/` — Playwright senaryoları

## Tasarım

"Kobalt & hardal" stili (2026-10-21), koyu tema 2026-10-22'de "Nötr grafit"e güncellendi (Discord benzerliği nedeniyle). Kobalt `#2f5bff` iki temada da tek dolu ana aksiyonda, seçili oyda ve işarette (beyaz metinle 5.17:1); hardal `#ffc93c` para taşıyan alanlarda (ortak bütçe, kişi başı, sürpriz ekranı); açık kobalt AI etiketi ve bilgi notlarında. Koyu temada yüzeyler renksiz grafittir (`#0c0c0d`, `#161618`, `#1e1e21`). Başlık ve rakamlar Unbounded, metin Onest. Koyu tema sistem ayarını (`prefers-color-scheme`) izler. Her kişiye kimliğinden türeyen sabit bir avatar tonu verilir (`src/lib/person.ts`, `.avatar`). Bileşenler yalnızca `src/index.css` token'larını kullanır; sabit renk yazılmaz. Metin kontrastı en az 4.5:1.

Pazar yeri linkleri `search_query` üzerinden `encodeURIComponent` ile kurulur (`src/lib/marketplace.ts`); Gemini URL üretmez.

## Üretim imajı

`Dockerfile` iki aşamalıdır (Node 22 derleme → `nginx-unprivileged`). `nginx.conf.template` başlangıçta `API_TARGET` ile doldurulur; `/api`, `/health`, `/swagger`, `/docs` yollarını API'ye iletir, bilinmeyen yolları `index.html`'e düşürür ve `/api/v1/dev/*` yolunu 403 ile engeller. Gövde sınırı API rotalarında 1m, yalnızca kapak yükleme rotasında (`/api/v1/groups/<id>/cover`) 3m. Port 5173. Azure adımları: [docs/AZURE.md](../docs/AZURE.md).
