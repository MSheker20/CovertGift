# 🎁 CovertGift

> **Güncel AI modu (2026-10-21):** Google Search kapalıdır (`GEMINI_SEARCH_ENABLED=false`).
> `GEMINI_MODEL=gemini-3.8-flash` ile tek çağrıda öneri üretilir; fiyatlar doğrulanmamış tahmindir.
> Model kimlikleri uygulama kodunda sabit değildir. `GEMINI_MODEL` anahtar varsa zorunludur.
> `GEMINI_FALLBACK_MODEL` boşsa otomatik model geçişi yapılmaz; doluysa yalnızca 503'te denenir.
> `GEMINI_RESEARCH_MODEL` yalnızca Search açıkken kullanılır; boşsa ana model kullanılır.
> Yerelde `backend/.env`, Azure'da Container App ortam değişkenleri düzenlenir ve uygulama yeniden başlatılır/yeni revision oluşur.
> Aşağıdaki eski Search araştırması açıklamaları yalnızca Search modu tekrar açılırsa geçerlidir.

Arkadaş gruplarının **ortak hediye** organize ettiği, bütçeyi birleştirdiği, **Gemini 3 Flash** ile yapay zekâ destekli hediye önerileri aldığı, önerileri **oyladığı** (+1 / -1) ve tüm süreci hediye alınacak kişiden **gizlediği** (Anti-Spoiler) işbirlikçi web platformu.

> Eski adı: *GiftlyAI* — telif riski nedeniyle 2026-10-06'da **CovertGift** olarak değiştirildi. Bkz. [DEVLOG](docs/DEVLOG.md).

---

## 📍 Mevcut Durum

Doküman güncellemesi: **2026-10-20**. Azure demo dağıtımı ve işletim notları: [Azure rehberi](docs/AZURE.md).

Son yerel/tünel doğrulaması: **2026-10-17**. PostgreSQL, güncel backend ve geçici test paneli çalıştırıldı; Cloudflare üzerinden arayüz 200, `/health` 200 (`db: up`) ve oturumsuz `/api/v1/me` 401 döndü.

| Alan | Durum |
|---|---|
| Teknik şartname | ✅ Hazır — [PROJECT_CONTEXT.md](PROJECT_CONTEXT.md) |
| Yol haritası | ✅ Hazır — [ROADMAP.md](ROADMAP.md) |
| Link önizleme prototipi | ✅ Çalışıyor — [prototypes/linkpreview](prototypes/linkpreview/) |
| Geliştirme ortamı (Go, Docker) | ✅ Kurulu |
| **Faz 0 — Proje iskeleti** | ✅ Tamamlandı (`/health` 200 OK) |
| **Faz 1 — Veritabanı Modelleri** | ✅ Tamamlandı (AutoMigrate 5 tablo aktif) |
| **Faz 2 — Kimlik Doğrulama (Auth)** | ✅ Tamamlandı (JWT, bcrypt, `GET /me`, `DELETE /me` profil silme) |
| **Faz 3 — Gruplar & Anti-Spoiler** | ✅ Tamamlandı (Üyeler ilgi alanı ekler; admin ilgi alanı siler, bütçeyi günceller ve grubu siler) |
| **Faz 4 — Hediye Havuzu & Oylama** | ✅ Tamamlandı (Upsert oylama, yetkili silme) |
| **Faz 5 — Link Önizleme** | ✅ Tamamlandı (`pkg/linkpreview`, Trendyol, Amazon TR, Hepsiburada, SSRF Kalkanı) |
| **Faz 6 — Gemini AI Hediye Önerileri** | ✅ Backend hazır (Türkiye mağazalarında Google Search araştırması, kaynaklı ürün/fiyat seçimi, veto ve tekrar filtresi) |
| **API Dokümantasyonu (Swagger / OpenAPI)** | ✅ Tamamlandı (Canlı UI: `/swagger/index.html` veya `/docs`) |
| **Geçici test paneli** | ✅ Hazır (`prototypes/test-ui/`, Node.js proxy, Cloudflare Quick Tunnel) |
| **Faz 7 — Frontend** | ⏳ Sırada (Vite, React, TypeScript, Tailwind, shadcn/ui) |

---

## 📚 Dokümantasyon Rehberi

| Dosya | Ne işe yarar? | Ne zaman bakılır? |
|---|---|---|
| [PROJECT_CONTEXT.md](PROJECT_CONTEXT.md) | **Tek doğruluk kaynağı.** Veri modelleri, iş kuralları, API sözleşmesi, mimari standartlar. Yapay zekâ asistanına da proje bağlamı olarak verilir. | Bir özelliği kodlamadan önce, "bu nasıl çalışmalı?" sorusunda |
| [ROADMAP.md](ROADMAP.md) | Faz faz, işaretlenebilir görev listesi ve her fazın kabul kriterleri. | Her çalışma oturumunun başında / sonunda |
| [docs/SETUP.md](docs/SETUP.md) | Faz 0 kurulum rehberi — her adımın **ne yaptığı ve neden yapıldığı** açıklamalı. | Ortamı kurarken / yeni makinede |
| [docs/AZURE.md](docs/AZURE.md) | API ve panel Docker imajları, Azure demo dağıtımı ve işletim notları. | Bulut ortamını kurarken / güncellerken |
| [docs/DEVLOG.md](docs/DEVLOG.md) | Geliştirme günlüğü + **alınan kararlar ve gerekçeleri**. | "Bunu neden böyle yaptık?" sorusunda |
| [prototypes/linkpreview/README.md](prototypes/linkpreview/README.md) | Ürün linki önizleme prototip bulguları, pazar yeri testleri ve mimari kararlar. | Pazar yeri ayrıştırıcılarını incelerken |
| [prototypes/test-ui/README.md](prototypes/test-ui/README.md) | Geçici test paneli, API proxy'si ve Cloudflare tüneli. | Backend'i tarayıcıdan veya telefondan denemek için |

---

## 🧱 Teknoloji Yığını

| Katman | Teknoloji |
|---|---|
| Backend | Go 1.27 + Gin Framework, GORM |
| Veritabanı | PostgreSQL 17 (Docker Compose) |
| Kimlik doğrulama | JWT (HMAC-SHA256, 24 saat) + Bcrypt + Gin Middleware |
| Link Önizleme | WhatsApp Bot UA, Cookiejar, Dial-seviyesi SSRF koruması, JSON-LD Scraper |
| Yapay zekâ | `google.golang.org/genai`: Google Search araştırması + yapılandırılmış JSON seçimi; araştırma `gemini-3.8-flash`, JSON seçimi `gemini-3.5-flash-lite`; 503 durumunda farklı modelden `gemini-3.5-flash-lite` yedeği |
| API Dokümantasyonu | Swagger / OpenAPI 2.0 (`swaggo/swag`, `swaggo/gin-swagger`) |
| Geçici panel | HTML/CSS/JavaScript + Node.js 22+ (ek paket gerektirmez) |
| Planlanan frontend | React (Vite) + TypeScript + Tailwind CSS + shadcn/ui + Lucide |

---

## 🗂️ Klasör Yapısı

```
CovertGift/
├── README.md               ← Bu dosya (giriş noktası)
├── PROJECT_CONTEXT.md      ← Teknik şartname
├── ROADMAP.md              ← Yol haritası
├── docs/
│   ├── SETUP.md            ← Kurulum rehberi
│   ├── AZURE.md            ← Azure demo dağıtımı
│   └── DEVLOG.md           ← Geliştirme günlüğü & mimarî karar kaydı
├── scripts/
│   └── demo_presentation.sh← 🎓 Hocaya canlı sunum & interaktif API test script'i (ANSI HTTP kutuları)
├── prototypes/
│   ├── linkpreview/        ← Link önizleme prototip ve inceleme laboratuvarı
│   └── test-ui/            ← Geçici tarayıcı paneli + sınırlı API proxy'si
├── backend/
│   ├── cmd/api/            ← HTTP sunucu giriş noktası
│   ├── docs/               ← Swagger / OpenAPI 2.0 şemaları (docs.go, swagger.json, yaml)
│   ├── internal/           ← Controllers, services, repository, models, middleware
│   ├── pkg/
│   │   ├── gemini/         ← Gemini 3.8 Flash istemcisi & resilient fallback
│   │   └── linkpreview/    ← Ürün linki önizleme & SSRF kalkanı (Trendyol, Amazon, HB)
│   └── bin/api             ← Yerel derleme çıktısı (Git'e dahil edilmez)
└── frontend/               ← (Faz 7'de oluşturulacak: Vite + React + Tailwind)
```

---

## 🚀 Hızlı Başlangıç

Ön koşullar: Go **1.27.1+**, çalışan Docker Desktop / Docker Compose, panel için Node.js **22+**. Ayrıntılı ortam kurulumu: [docs/SETUP.md](docs/SETUP.md).

```bash
# İlk kurulum: depoyu indir ve proje köküne geç
git clone https://github.com/MSheker20/CovertGift.git
cd CovertGift

# Yalnızca ilk kurulumda: yerel ayar dosyasını oluştur
cp backend/.env.example backend/.env
openssl rand -hex 32
```

Üretilen değeri `backend/.env` içindeki `JWT_SECRET` alanına yazın. AI önerilerini kullanmak için `GEMINI_API_KEY` ekleyin; `GEMINI_RESEARCH_MODEL` araştırma modelini (varsayılan `gemini-3.8-flash`), `GEMINI_MODEL` JSON seçim modelini (varsayılan `gemini-3.5-flash-lite`) belirler. `.env` dosyası Git'e dahil edilmez. Anahtar yokken AI dışındaki özellikler çalışır.

**Terminal 1 — veritabanı ve backend (proje kökünden):**

```bash
docker compose up -d --wait
cd backend
go run ./cmd/api
```

**Terminal 2 — geçici panel (proje kökünden):**

```bash
node prototypes/test-ui/server.mjs
```

[Test paneli](http://127.0.0.1:5173), [sağlık kontrolü](http://127.0.0.1:5173/health), [Swagger](http://127.0.0.1:5173/swagger/index.html). Sağlık kontrolü `{"success":true,"data":{"db":"up","status":"ok"}}` döndürmelidir. Panel kalıcı Faz 7 frontend'inden bağımsızdır.

**Terminal 3 — isteğe bağlı internet erişimi:**

```bash
cloudflared tunnel --url http://127.0.0.1:5173 --no-autoupdate
```

Komutun verdiği `https://….trycloudflare.com` adresi paneli ve API'yi birlikte açar. Adres geçicidir ve yeniden başlatıldığında değişebilir; bilgisayar, backend, panel ve tünel açık kalmalıdır. Tüneli **5173** portuna bağlayın: panelin proxy'si `/api/v1/dev/reset-db` dahil geliştirme uçlarını engeller. Kullanım ayrıntıları: [test paneli rehberi](prototypes/test-ui/README.md).

## 🧪 Doğrulama ve Demo

```bash
# Proje kökünden; normal testler gerçek Gemini API'sini çağırmaz
go test ./backend/...
node --check prototypes/test-ui/server.mjs
node --check prototypes/test-ui/app.js
curl --fail http://127.0.0.1:5173/health

# API çalışırken, veritabanına demo verileri yazan sunum script'i
./scripts/demo_presentation.sh
./scripts/demo_presentation.sh --auto
```

AI akışı önce Google Search ile Türkiye mağazalarında ürün ve TL fiyatı araştırır, ardından desteklenen ürünleri JSON'a dönüştürür. Kaynak ve kalite kontrollerinden sonra en fazla 5 öneri kaydedilir; yeterli ürün yoksa 3'ten az sonuç veya hata dönebilir. Kaynaklar anlık stok garantisi vermez. `ai_sources` ve `ai_search_suggestions` API'de bulunur; panelde kaynak/arama önerisi gösterimi henüz uygulanmadı.

## 🔗 Bağlantılar
- GitHub: https://github.com/MSheker20/CovertGift
