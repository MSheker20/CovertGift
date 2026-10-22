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

Uçtan uca testler kendi geçici `e2e-…@demo.test` kullanıcılarını oluşturur ve sonunda `DELETE /me` ile siler; Gemini çağrılmaz. İlk kurulumda tarayıcı için `npx playwright install chromium` gerekir.

Ayarlar `.env.example` içinde: `API_TARGET` (geliştirme proxy hedefi), `VITE_API_BASE_URL` (üretimde API ayrı bir origin'deyse).

## Yapı

- `src/api/` — fetch istemcisi (`{success, data | error}` zarfı, Bearer token, 401 olayı), uç noktalar ve tipler
- `src/auth/` — oturum durumu; token `localStorage`'da (`covertgift.token`)
- `src/i18n/` — `tr.ts` ana sözlük, `en.ts` aynı tipte; dil `covertgift.lang` anahtarında saklanır
- `src/pages/` — Giriş, Kayıt, uygulama kabuğu (yüzen üst bar + grup seçici), Gruplar, Yeni grup diyaloğu (hedef kişinin e-postası dahil), Grup (Anti-Spoiler sürpriz ekranı dahil)
- `src/pages/group/` — hediye havuzu ve oy kartları, manuel ekleme + link önizleme, üye ekleme, yerinde yönetim (ilgi alanı, bütçe, grubu silme, hedef kişiyi bağlama)
- `src/test/` — MSW sunucusu, örnek veriler ve render yardımcıları; `e2e/` — Playwright senaryoları

## Tasarım

"Kobalt & hardal" stili (2026-10-21), koyu tema 2026-10-22'de "Nötr grafit"e güncellendi (Discord benzerliği nedeniyle). Kobalt `#2f5bff` iki temada da tek dolu ana aksiyonda, seçili oyda ve işarette (beyaz metinle 5.17:1); hardal `#ffc93c` para taşıyan alanlarda (ortak bütçe, kişi başı, sürpriz ekranı); açık kobalt AI etiketi ve bilgi notlarında. Koyu temada yüzeyler renksiz grafittir (`#0c0c0d`, `#161618`, `#1e1e21`). Başlık ve rakamlar Unbounded, metin Onest. Koyu tema sistem ayarını (`prefers-color-scheme`) izler. Her kişiye kimliğinden türeyen sabit bir avatar tonu verilir (`src/lib/person.ts`, `.avatar`). Bileşenler yalnızca `src/index.css` token'larını kullanır; sabit renk yazılmaz. Metin kontrastı en az 4.5:1.

Pazar yeri linkleri `search_query` üzerinden `encodeURIComponent` ile kurulur (`src/lib/marketplace.ts`); Gemini URL üretmez.

## Üretim imajı

`Dockerfile` iki aşamalıdır (Node 22 derleme → `nginx-unprivileged`). `nginx.conf.template` başlangıçta `API_TARGET` ile doldurulur; `/api`, `/health`, `/swagger`, `/docs` yollarını API'ye iletir, bilinmeyen yolları `index.html`'e düşürür ve `/api/v1/dev/*` yolunu 403 ile engeller. Port 5173. Azure adımları: [docs/AZURE.md](../docs/AZURE.md).
