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

Ayarlar `.env.example` içinde: `API_TARGET` (geliştirme proxy hedefi), `VITE_API_BASE_URL` (üretimde API ayrı bir origin'deyse).

## Yapı

- `src/api/` — fetch istemcisi (`{success, data | error}` zarfı, Bearer token, 401 olayı), uç noktalar ve tipler
- `src/auth/` — oturum durumu; token `localStorage`'da (`covertgift.token`)
- `src/i18n/` — `tr.ts` ana sözlük, `en.ts` aynı tipte; dil `covertgift.lang` anahtarında saklanır
- `src/pages/` — Giriş, Kayıt, uygulama kabuğu, Gruplar, Grup (Anti-Spoiler sürpriz ekranı dahil)

## Tasarım

"Kobalt & hardal" stili (2026-10-21, kullanıcı seçimi): kobalt (`#2436ff`) tek dolu ana aksiyonda, seçili oyda ve işarette; hardal (`#ffc93c`) para taşıyan alanlarda (ortak bütçe, kişi başı, sürpriz ekranı); açık kobalt AI etiketi ve bilgi notlarında. Başlık ve rakamlar Unbounded, metin Onest. Koyu tema sistem ayarını (`prefers-color-scheme`) izler. Her kişiye kimliğinden türeyen sabit bir avatar tonu verilir (`src/lib/person.ts`, `.avatar`). Bileşenler yalnızca `src/index.css` token'larını kullanır; sabit renk yazılmaz.

Pazar yeri linkleri ileride `search_query` üzerinden `encodeURIComponent` ile kurulacak; Gemini URL üretmez.
