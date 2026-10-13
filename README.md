# 🎁 CovertGift

Arkadaş gruplarının **ortak hediye** organize ettiği, bütçeyi birleştirdiği, **Gemini 3 Flash** ile yapay zekâ destekli hediye önerileri aldığı, önerileri **oyladığı** (+1 / -1) ve tüm süreci hediye alınacak kişiden **gizlediği** (Anti-Spoiler) işbirlikçi web platformu.

> Eski adı: *GiftlyAI* — telif riski nedeniyle 2026-10-06'da **CovertGift** olarak değiştirildi. Bkz. [DEVLOG](docs/DEVLOG.md).

---

## 📍 Mevcut Durum

| Alan | Durum |
|---|---|
| Teknik şartname | ✅ Hazır — [PROJECT_CONTEXT.md](PROJECT_CONTEXT.md) |
| Yol haritası | ✅ Hazır — [ROADMAP.md](ROADMAP.md) |
| Link önizleme prototipi | ✅ Çalışıyor — [prototypes/linkpreview](prototypes/linkpreview/) |
| Geliştirme ortamı (Go, Docker) | ✅ Kurulu |
| **Faz 0 — Proje iskeleti** | ✅ Tamamlandı (`/health` 200 OK) |
| **Faz 1 — Veritabanı Modelleri** | ✅ Tamamlandı (AutoMigrate 5 tablo aktif) |
| **Faz 2 — Kimlik Doğrulama (Auth)** | ✅ Tamamlandı (JWT, bcrypt, `/me`) |
| **Faz 3 — Gruplar & Anti-Spoiler** | ✅ Tamamlandı (Dinamik bütçe, 403 kalkanı) |
| **Faz 4 — Hediye Havuzu & Oylama** | ✅ Tamamlandı (Upsert oylama, yetkili silme) |
| **Faz 5 — Link Önizleme** | ✅ Tamamlandı (`pkg/linkpreview`, Trendyol, Amazon TR, Hepsiburada, SSRF Kalkanı) |
| **Faz 6 — Gemini AI Hediye Önerileri** | ✅ Tamamlandı (Gemini 3.8 Flash, Admin Veto, Demokrasi Vetosu) |
| **API Dokümantasyonu (Swagger / OpenAPI)** | ✅ Tamamlandı (Canlı UI: `/swagger/index.html` veya `/docs`) |
| **Faz 7 — Frontend** | ⏳ Sırada (Vite, React, TypeScript, Tailwind, shadcn/ui) |

---

## 📚 Dokümantasyon Rehberi

| Dosya | Ne işe yarar? | Ne zaman bakılır? |
|---|---|---|
| [PROJECT_CONTEXT.md](PROJECT_CONTEXT.md) | **Tek doğruluk kaynağı.** Veri modelleri, iş kuralları, API sözleşmesi, mimari standartlar. Yapay zekâ asistanına da proje bağlamı olarak verilir. | Bir özelliği kodlamadan önce, "bu nasıl çalışmalı?" sorusunda |
| [ROADMAP.md](ROADMAP.md) | Faz faz, işaretlenebilir görev listesi ve her fazın kabul kriterleri. | Her çalışma oturumunun başında / sonunda |
| [docs/SETUP.md](docs/SETUP.md) | Faz 0 kurulum rehberi — her adımın **ne yaptığı ve neden yapıldığı** açıklamalı. | Ortamı kurarken / yeni makinede |
| [docs/DEVLOG.md](docs/DEVLOG.md) | Geliştirme günlüğü + **alınan kararlar ve gerekçeleri**. | "Bunu neden böyle yaptık?" sorusunda |
| [prototypes/linkpreview/README.md](prototypes/linkpreview/README.md) | Ürün linki önizleme prototip bulguları, pazar yeri testleri ve mimari kararlar. | Pazar yeri ayrıştırıcılarını incelerken |

---

## 🧱 Teknoloji Yığını

| Katman | Teknoloji |
|---|---|
| Backend | Go 1.27 + Gin Framework, GORM |
| Veritabanı | PostgreSQL 17 (Docker Compose) |
| Kimlik doğrulama | JWT (HMAC-SHA256, 24 saat) + Bcrypt + Gin Middleware |
| Link Önizleme | WhatsApp Bot UA, Cookiejar, Dial-seviyesi SSRF koruması, JSON-LD Scraper |
| Yapay zekâ | Google Gemini 3.8 Flash (`google.golang.org/genai`, Structured Output, State-Based Prompt) |
| API Dokümantasyonu | Swagger / OpenAPI 2.0 (`swaggo/swag`, `swaggo/gin-swagger`) |
| Frontend | React (Vite) + TypeScript + Tailwind CSS + shadcn/ui + Lucide |

---

## 🗂️ Klasör Yapısı

```
CovertGift/
├── README.md               ← Bu dosya (giriş noktası)
├── PROJECT_CONTEXT.md      ← Teknik şartname
├── ROADMAP.md              ← Yol haritası
├── docs/
│   ├── SETUP.md            ← Kurulum rehberi
│   └── DEVLOG.md           ← Geliştirme günlüğü & mimarî karar kaydı
├── scripts/
│   └── demo_presentation.sh← 🎓 Hocaya canlı sunum & interaktif API test script'i (ANSI HTTP kutuları)
├── prototypes/
│   └── linkpreview/        ← Link önizleme prototip ve inceleme laboratuvarı
├── backend/
│   ├── cmd/api/            ← HTTP sunucu giriş noktası
│   ├── docs/               ← Swagger / OpenAPI 2.0 şemaları (docs.go, swagger.json, yaml)
│   ├── internal/           ← Controllers, services, repository, models, middleware
│   ├── pkg/
│   │   ├── gemini/         ← Gemini 3.8 Flash istemcisi & resilient fallback
│   │   └── linkpreview/    ← Ürün linki önizleme & SSRF kalkanı (Trendyol, Amazon, HB)
│   └── bin/api             ← Derlenmiş API ikili dosyası
└── frontend/               ← (Faz 7'de oluşturulacak: Vite + React + Tailwind)
```

---

## 🚀 Hızlı Başlangıç

```bash
# 1. Veritabanını arka planda başlat
docker compose up -d

# 2. Backend dizinine geçip sunucuyu çalıştır
cd backend
go run ./cmd/api

# 3. Sağlık kontrolü yap (ayrı bir terminalde veya tarayıcıda)
curl http://localhost:8080/health
# Yanıt: {"success":true,"data":{"db":"up","status":"ok"}}

# 4. 📖 Canlı Swagger API Dokümantasyonunu Aç (Tarayıcıda)
# http://localhost:8080/swagger/index.html  (veya http://localhost:8080/docs)

# 5. 🎓 Akademik Sunum Script'ini Çalıştır (Tüm API uç noktaları kutular içinde sergilenir)
./scripts/demo_presentation.sh          # Adım adım klavye kontrollü
./scripts/demo_presentation.sh --auto   # Otomatik test modu
```

## 🔗 Bağlantılar
- GitHub: https://github.com/MSheker20/CovertGift
