# CovertGift — Claude Code Notları

Oturuma başlamadan önce oku:
- `PROJECT_CONTEXT.md` — teknik şartname (modeller, API sözleşmesi, iş kuralları)
- `ROADMAP.md` — faz durumu ve kabul kriterleri
- `docs/DEVLOG.md` — son değişiklikler

## Stack
- **Backend:** Go + Gin + GORM + PostgreSQL 17 (`backend/`, kök dizinde `go.work`)
- **AI:** `google.golang.org/genai`. Model adları `GEMINI_MODEL` / `GEMINI_RESEARCH_MODEL` ortam değişkenlerinden gelir; koda sabit yazılmaz.
- **Frontend:** `frontend/` — Vite + React 19 + TypeScript, Tailwind v4, shadcn tarzı bileşenler, TanStack Query, React Router (lazy rotalar), i18next (TR/EN). Tasarım token'ları yalnızca `src/index.css` içinde; bileşenlerde sabit renk yazılmaz. Koyu tema "Nötr grafit", ana renk iki temada `#2f5bff`, para alanları hardal.
- **Eski geçici panel:** `prototypes/test-ui/` (vanilla JS + Node 22+). Yerini React frontend aldı; `PATCH /groups/:id/target` gibi yeni uçları desteklemez.

## Kritik kurallar
- **Anti-Spoiler:** `/groups/:id/*` ve `/ideas/:id/*` uçlarında `target_user_id` kontrolü zorunlu. Eşleşirse 403 ve sürpriz mesajı döner.
- **Gemini URL üretmez.** Pazar yeri linkleri frontend'de `search_query` üzerinden `encodeURIComponent` ile oluşturulur.
- **Link önizleme** (`pkg/linkpreview`): domain allowlist, özel IP engeli, 2 MB limit ve en fazla 10 yönlendirme korunmalı.
- **Yanıt formatı:** `{"success": true, "data": ...}` veya `{"success": false, "error": "..."}`.
- **Hata kontrolü** açıkça yapılır (`if err != nil`). Şema değişiklikleri GORM `AutoMigrate` ile yapılır.

## Komutlar
- DB: `docker compose up -d`
- API: `cd backend && go run ./cmd/api`
- Frontend: `npm --prefix frontend run dev` (:5174, `/api` → :8080)
- Testler: `cd backend && go test ./...`, `npm --prefix frontend test` (Vitest), `npm --prefix frontend run test:e2e` (Playwright; önce API çalışmalı)
- Swagger: `cd backend && swag init -g cmd/api/main.go --parseInternal`

## Notlar
- `.env`, `JWT_SECRET` ve `GEMINI_API_KEY` commit edilmez.
- `POST /api/v1/dev/reset-db` tüm tabloları siler. Varsayılan kapalıdır; yalnızca `ALLOW_DB_RESET=true` + `{"confirm":"RESET"}` ile çalışır. Yine de testlerde çalışan bir API'ye bu ucu asla çağırtma.
- Azure dağıtımı: `docs/AZURE.md` (API ve frontend imajları commit kısa kimliğiyle etiketlenir).
- Bu projede genel ECC plugin'i kapalıdır (`.claude/settings.json`). Gerekli skill, agent ve komutlar `.claude/` altında projeye özel olarak bulunur.
