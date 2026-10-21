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

`design-taste-frontend` skill'inin "Editorial greeting" uygulama stili (seed 2035334530): serif selamlama başlığı (Instrument Serif), Instrument Sans gövde, tonal paneller, gölgesiz yüzeyler, teal vurgu (`#1b6e66`) yalnızca veride ve işarette. Her kişiye kimliğinden türeyen sabit bir pastel avatar tonu verilir (`src/lib/person.ts`). Token'lar `src/index.css` içindedir.

Pazar yeri linkleri ileride `search_query` üzerinden `encodeURIComponent` ile kurulacak; Gemini URL üretmez.
