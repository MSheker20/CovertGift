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
- **MULTI gruplar:** Hediye alınan kişi kendi havuzu için yalnızca kilitli kart görür. `/groups/:id/pools/:poolId/*` altındaki her rota `PoolAccess`'ten, `/ideas/:id/*` `IdeaGuard`'dan geçer; havuza özgü değişiklik grup anahtarını değil havuz anahtarını yayınlar. Kilit sırası: `groups` → `gift_pools` → diğerleri.
- **Davet linki:** Token loglanmaz; hedef kişi `/invites/:token` uçlarında da 403 alır.
- **Gemini URL üretmez.** Pazar yeri linkleri frontend'de `search_query` üzerinden `encodeURIComponent` ile oluşturulur.
- **Link önizleme** (`pkg/linkpreview`): domain allowlist, özel IP engeli, 2 MB limit ve en fazla 10 yönlendirme korunmalı.
- **Canlı güncellemeler (SSE):** `GET /groups/:id/events` olayları veri taşımaz (`event: changed`); istemci korumalı uçlardan yeniden çeker. Yeni bir değişiklik ucu `/groups/:id/*` veya `/ideas/:id/*` altına eklenirse yayın middleware'i otomatik çalışır. Hub bellek içidir; API tek replica.
- **Yanıt formatı:** `{"success": true, "data": ...}` veya `{"success": false, "error": "..."}`.
- **Hata kontrolü** açıkça yapılır (`if err != nil`). Şema değişiklikleri GORM `AutoMigrate` ile yapılır.

## Komutlar
- DB: `docker compose up -d`
- API: `cd backend && go run ./cmd/api`
- Frontend: `npm --prefix frontend run dev` (:5174, `/api` → :8080)
- Testler: `cd backend && go test ./...` (Anti-Spoiler entegrasyon testleri için `docker compose --profile test up -d --wait test-db` + `TEST_DATABASE_URL=...localhost:55432/covertgift_test go test -p 1 ./cmd/...`; adı `_test` ile bitmeyen DB reddedilir), `npm --prefix frontend test` (Vitest), `npm --prefix frontend run test:e2e` (Playwright; önce API çalışmalı)
- Seed: `cd backend && go run ./cmd/seed` (yalnızca ekler, idempotent). Konteynerde tüm uygulama: `docker compose --profile app up -d --build` (:8081)
- Loglama: `log/slog`; `LOG_FORMAT` (json/text, production'da json) ve `LOG_LEVEL`. 500 yanıtlarında `controllers.RespondInternal` kullanılır; iç hata istemciye değil loga gider.
- Swagger: `cd backend && swag init -g cmd/api/main.go --parseInternal`

## Notlar
- `.env`, `JWT_SECRET` ve `GEMINI_API_KEY` commit edilmez.
- `POST /api/v1/dev/reset-db` tüm tabloları siler. Varsayılan kapalıdır; yalnızca `ALLOW_DB_RESET=true` + `{"confirm":"RESET"}` ile çalışır. Yine de testlerde çalışan bir API'ye bu ucu asla çağırtma.
- Azure dağıtımı: `docs/AZURE.md` (imajlar yerel Docker ile derlenir, commit kısa kimliğiyle etiketlenir; `az acr build` kapalı). Canlıda yalnızca okuma kontrolleri yapılır; canlı veriye yazan istek atılmaz.
- Tarayıcıda denemek için `.claude/launch.json`'daki `api-testdb` + `frontend-testdb` geçici `test-db`'ye bağlanır (yerel veriye dokunmaz; demo verisi `go run ./cmd/seed` ile).
- Bu projede genel ECC plugin'i kapalıdır (`.claude/settings.json`). Gerekli skill, agent ve komutlar `.claude/` altında projeye özel olarak bulunur.

## Skill'ler (otomatik kullanım)
`.claude/skills/` altındaki skill'ler bu projede her oturumda yüklüdür. İlgili işe başlamadan önce ilgili skill'i `Skill` aracıyla çağır:
- **Frontend bileşen/sayfa yazarken veya değiştirirken:** `react-patterns`, `frontend-patterns`; görsel/UX işlerinde ayrıca `design-taste-frontend` ve `frontend-design-direction`
- **Form, modal, sekme, klavye/odak işleri:** `frontend-a11y`
- **Render/performans sorunları:** `react-performance`
- **Frontend testleri (Vitest/Playwright):** `react-testing`
- **Backend (Go) kodu:** `golang-patterns`, `backend-patterns`, `error-handling`; yeni uçlarda `api-design`; test yazarken `golang-testing`
- **Şema/sorgu/index değişikliği:** `postgres-patterns`, `database-migrations`
- **Auth, davet token'ı, girdi işleme, link önizleme gibi güvenlik yüzeyleri:** `security-review`
- **Yeni özellik/bug fix:** `tdd-workflow`
- **Docker/compose değişiklikleri:** `docker-patterns`
- **Commit/PR/branch işleri:** `git-workflow`
- **Playwright e2e testleri:** `e2e-testing`; tarayıcıda canlı doğrulama için `browser-qa`
- **Vite yapılandırması/proxy/build:** `vite-patterns`
- **TR/EN çeviri anahtarları:** `i18n-sync`
- **Erişilebilirlik denetimi (WCAG):** `accessibility`
- **Buton/akış bug'ı avı (state zincirini izleme):** `click-path-audit`
- **Büyük değişiklik sonrası build+tip+lint+test doğrulaması:** `verification-loop`
- **Yeni kütüphane/özellik eklemeden önce hazır çözüm arama:** `search-first`
- **Azure/CI-CD/dağıtım:** `deployment-patterns`
- **Tasarım token'ları tutarlılığı (`src/index.css`):** `design-system`
- **Gemini çağrıları (model seçimi, maliyet, bütçe):** `cost-aware-llm-pipeline`
