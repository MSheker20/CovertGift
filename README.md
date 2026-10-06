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
| **Faz 1 — Veritabanı Modelleri** | ⏳ Sırada |
| Backend API & Servisler | ⬜ Başlanmadı |
| Frontend | ⬜ Başlanmadı |

---

## 📚 Dokümantasyon Rehberi

| Dosya | Ne işe yarar? | Ne zaman bakılır? |
|---|---|---|
| [PROJECT_CONTEXT.md](PROJECT_CONTEXT.md) | **Tek doğruluk kaynağı.** Veri modelleri, iş kuralları, API sözleşmesi, mimari standartlar. Yapay zekâ asistanına da proje bağlamı olarak verilir. | Bir özelliği kodlamadan önce, "bu nasıl çalışmalı?" sorusunda |
| [ROADMAP.md](ROADMAP.md) | Faz faz, işaretlenebilir görev listesi ve her fazın kabul kriterleri. | Her çalışma oturumunun başında / sonunda |
| [docs/SETUP.md](docs/SETUP.md) | Faz 0 kurulum rehberi — her adımın **ne yaptığı ve neden yapıldığı** açıklamalı. | Ortamı kurarken / yeni makinede |
| [docs/DEVLOG.md](docs/DEVLOG.md) | Geliştirme günlüğü + **alınan kararlar ve gerekçeleri**. | "Bunu neden böyle yaptık?" sorusunda |
| [prototypes/linkpreview/README.md](prototypes/linkpreview/README.md) | Ürün linki önizleme denemesinin sonuçları ve öğrenilenler. | Faz 5'e başlarken |

---

## 🧱 Teknoloji Yığını

| Katman | Teknoloji |
|---|---|
| Backend | Go + Gin, GORM |
| Veritabanı | PostgreSQL (Docker Compose) |
| Kimlik doğrulama | JWT + özel Gin middleware |
| Frontend | React (Vite) + Tailwind CSS + shadcn/ui + Lucide |
| Yapay zekâ | Google Gemini 3 Flash (`response_schema` ile katı JSON) |

---

## 🗂️ Klasör Yapısı

```
CovertGift/
├── README.md               ← Bu dosya (giriş noktası)
├── PROJECT_CONTEXT.md      ← Teknik şartname
├── ROADMAP.md              ← Yol haritası
├── docs/
│   ├── SETUP.md            ← Kurulum rehberi
│   └── DEVLOG.md           ← Geliştirme günlüğü & kararlar
├── prototypes/
│   └── linkpreview/        ← Link önizleme deneme script'i
├── backend/                ← Go backend API (Faz 0 tamamlandı)
└── frontend/               ← (Faz 7'de oluşturulacak)
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
```

## 🔗 Bağlantılar
- GitHub: https://github.com/MSheker20/CovertGift
