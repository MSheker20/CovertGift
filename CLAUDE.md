# CovertGift — Claude Code Notları

Oturuma başlamadan önce oku:
- `PROJECT_CONTEXT.md` — teknik şartname (modeller, API sözleşmesi, iş kuralları)
- `ROADMAP.md` — faz durumu ve kabul kriterleri
- `docs/DEVLOG.md` — son değişiklikler

## Stack
- **Backend:** Go + Gin + GORM + PostgreSQL 17 (`backend/`, kök dizinde `go.work`)
- **AI:** `google.golang.org/genai`. Model adları `GEMINI_MODEL` / `GEMINI_RESEARCH_MODEL` ortam değişkenlerinden gelir; koda sabit yazılmaz.
- **Geçici panel:** `prototypes/test-ui/` (vanilla JS + Node 22+). Kalıcı React frontend (Faz 7) henüz başlamadı.

## Kritik kurallar
- **Anti-Spoiler:** `/groups/:id/*` ve `/ideas/:id/*` uçlarında `target_user_id` kontrolü zorunlu. Eşleşirse 403 ve sürpriz mesajı döner.
- **Gemini URL üretmez.** Pazar yeri linkleri frontend'de `search_query` üzerinden `encodeURIComponent` ile oluşturulur.
- **Link önizleme** (`pkg/linkpreview`): domain allowlist, özel IP engeli, 2 MB limit ve en fazla 10 yönlendirme korunmalı.
- **Yanıt formatı:** `{"success": true, "data": ...}` veya `{"success": false, "error": "..."}`.
- **Hata kontrolü** açıkça yapılır (`if err != nil`). Şema değişiklikleri GORM `AutoMigrate` ile yapılır.

## Komutlar
- DB: `docker compose up -d`
- API: `cd backend && go run ./cmd/api`
- Testler: `cd backend && go test ./...`

## Notlar
- `.env`, `JWT_SECRET` ve `GEMINI_API_KEY` commit edilmez.
- Bu projede genel ECC plugin'i kapalıdır (`.claude/settings.json`). Gerekli skill, agent ve komutlar `.claude/` altında projeye özel olarak bulunur.
