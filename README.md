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

Doküman güncellemesi: **2026-10-22**. Azure demo dağıtımı ve işletim notları: [Azure rehberi](docs/AZURE.md).

Faz 0–7 tamamlandı; Faz 8 (test, güvenlik ve cila) sürüyor. Kalıcı React frontend 2026-10-21'de Azure'a dağıtıldı ve [canlı demo adresinde](https://covertgift-ui.yellowplant-0f3e00f1.polandcentral.azurecontainerapps.io) geçici panelin yerini aldı. 2026-10-22 tasarım güncellemesi ("Nötr grafit") henüz canlıya alınmadı.

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
| **Faz 6 — Gemini AI Hediye Önerileri** | ✅ Tamamlandı (Search kapalı tek çağrı modu; veto, tekrar ve bütçe filtreleri; 429 kota yanıtı) |
| **API Dokümantasyonu (Swagger / OpenAPI)** | ✅ Tamamlandı (Canlı UI: `/swagger/index.html` veya `/docs`) |
| **Faz 7 — Frontend** | ✅ Tamamlandı (`frontend/`: React 19 + Vite + TypeScript, TR/EN, hediye havuzu, oylama, Gemini, link önizleme, grup yönetimi, hedef kişiyi e-postayla bağlama; Vitest 37 + Playwright 6 test) |
| **Faz 8 — Test, güvenlik ve cila** | ⏳ Sürüyor (entegrasyon testleri, CORS/limitler, yapılandırılmış log, seed script açık) |
| Geçici test paneli | 🗄️ Eski (`prototypes/test-ui/`; React frontend yerini aldı) |

---

## 📚 Dokümantasyon Rehberi

| Dosya | Ne işe yarar? | Ne zaman bakılır? |
|---|---|---|
| [PROJECT_CONTEXT.md](PROJECT_CONTEXT.md) | **Tek doğruluk kaynağı.** Veri modelleri, iş kuralları, API sözleşmesi, mimari standartlar. Yapay zekâ asistanına da proje bağlamı olarak verilir. | Bir özelliği kodlamadan önce, "bu nasıl çalışmalı?" sorusunda |
| [ROADMAP.md](ROADMAP.md) | Faz faz, işaretlenebilir görev listesi ve her fazın kabul kriterleri. | Her çalışma oturumunun başında / sonunda |
| [docs/SETUP.md](docs/SETUP.md) | Faz 0 kurulum rehberi — her adımın **ne yaptığı ve neden yapıldığı** açıklamalı. | Ortamı kurarken / yeni makinede |
| [docs/AZURE.md](docs/AZURE.md) | API ve frontend Docker imajları, Azure demo dağıtımı, geri dönüş imajları ve işletim notları. | Bulut ortamını kurarken / güncellerken |
| [frontend/README.md](frontend/README.md) | React frontend'in çalıştırılması, testleri, yapısı ve tasarım token'ları. | Arayüz üzerinde çalışırken |
| [docs/DEVLOG.md](docs/DEVLOG.md) | Geliştirme günlüğü + **alınan kararlar ve gerekçeleri**. | "Bunu neden böyle yaptık?" sorusunda |
| [prototypes/linkpreview/README.md](prototypes/linkpreview/README.md) | Ürün linki önizleme prototip bulguları, pazar yeri testleri ve mimari kararlar. | Pazar yeri ayrıştırıcılarını incelerken |
| [prototypes/test-ui/README.md](prototypes/test-ui/README.md) | Eski geçici test paneli, API proxy'si ve Cloudflare tüneli. | Yalnızca eski paneli çalıştırmak gerekirse |

---

## 🧱 Teknoloji Yığını

| Katman | Teknoloji |
|---|---|
| Backend | Go 1.27 + Gin Framework, GORM |
| Veritabanı | PostgreSQL 17 (Docker Compose) |
| Kimlik doğrulama | JWT (HMAC-SHA256, 24 saat) + Bcrypt + Gin Middleware |
| Link Önizleme | WhatsApp Bot UA, Cookiejar, Dial-seviyesi SSRF koruması, JSON-LD Scraper |
| Yapay zekâ | `google.golang.org/genai`; model adları ortam değişkenlerinden (`GEMINI_MODEL=gemini-3.8-flash`, Search kapalı, tek yapılandırılmış JSON çağrısı). Yedek model yalnızca `GEMINI_FALLBACK_MODEL` doluysa 503'te denenir |
| API Dokümantasyonu | Swagger / OpenAPI 2.0 (`swaggo/swag`, `swaggo/gin-swagger`) |
| Frontend | React 19 + Vite + TypeScript, Tailwind CSS v4, shadcn tarzı bileşenler (`radix-ui`), Lucide, React Router, TanStack Query, i18next (TR/EN); üretimde nginx |
| Frontend testleri | Vitest + Testing Library + MSW; Playwright (gerçek API'ye karşı) |
| Tasarım | Unbounded + Onest; ana renk kobalt `#2f5bff`, para alanları hardal `#ffc93c`; koyu tema "Nötr grafit" (sistem ayarını izler) |
| Eski geçici panel | HTML/CSS/JavaScript + Node.js 22+ (ek paket gerektirmez) |

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
│   └── test-ui/            ← Eski geçici tarayıcı paneli + sınırlı API proxy'si
├── backend/
│   ├── cmd/api/            ← HTTP sunucu giriş noktası
│   ├── docs/               ← Swagger / OpenAPI 2.0 şemaları (docs.go, swagger.json, yaml)
│   ├── internal/           ← Controllers, services, repository, models, middleware
│   ├── pkg/
│   │   ├── gemini/         ← Gemini 3.8 Flash istemcisi & resilient fallback
│   │   └── linkpreview/    ← Ürün linki önizleme & SSRF kalkanı (Trendyol, Amazon, HB)
│   └── bin/api             ← Yerel derleme çıktısı (Git'e dahil edilmez)
└── frontend/               ← Kalıcı React arayüzü (Vite), Vitest/Playwright testleri, Dockerfile + nginx
```

---

## 🚀 Hızlı Başlangıç

Ön koşullar: Go **1.27.1+**, çalışan Docker Desktop / Docker Compose, frontend için Node.js **22+**. Ayrıntılı ortam kurulumu: [docs/SETUP.md](docs/SETUP.md).

```bash
# İlk kurulum: depoyu indir ve proje köküne geç
git clone https://github.com/MSheker20/CovertGift.git
cd CovertGift

# Yalnızca ilk kurulumda: yerel ayar dosyasını oluştur
cp backend/.env.example backend/.env
openssl rand -hex 32
```

Üretilen değeri `backend/.env` içindeki `JWT_SECRET` alanına yazın. AI önerilerini kullanmak için `GEMINI_API_KEY` ve `GEMINI_MODEL` (ör. `gemini-3.8-flash`) ekleyin; `GEMINI_SEARCH_ENABLED=false` güncel moddur. `.env` dosyası Git'e dahil edilmez. Anahtar yokken AI dışındaki özellikler çalışır.

**Terminal 1 — veritabanı ve backend (proje kökünden):**

```bash
docker compose up -d --wait
cd backend
go run ./cmd/api
```

**Terminal 2 — frontend (proje kökünden):**

```bash
npm --prefix frontend install
npm --prefix frontend run dev
```

[Uygulama](http://localhost:5174), [sağlık kontrolü](http://localhost:5174/health), [Swagger](http://localhost:8080/swagger/index.html). Vite, `/api` ve `/health` isteklerini `127.0.0.1:8080`'deki API'ye iletir. Sağlık kontrolü `{"success":true,"data":{"db":"up","status":"ok"}}` döndürmelidir.

> ⚠️ `POST /api/v1/dev/reset-db` **tüm tabloları siler**. Uç varsayılan olarak kapalıdır (403); yalnızca API `ALLOW_DB_RESET=true` ile başlatılırsa ve istek gövdesinde `{"confirm": "RESET"}` gönderilirse çalışır. Üretimde (`APP_ENV=production`) her durumda kapalıdır ve frontend'in nginx'i ayrıca 403 döner. Yerel veriniz için yine de düzenli `pg_dump` yedeği alın.

Eski geçici paneli ve Cloudflare tünelini çalıştırmak gerekirse: [test paneli rehberi](prototypes/test-ui/README.md).

## 🧪 Doğrulama ve Demo

```bash
# Proje kökünden; normal testler gerçek Gemini API'sini çağırmaz
go test ./backend/...
npm --prefix frontend test
npm --prefix frontend run build
npm --prefix frontend run test:e2e   # API ve DB çalışırken; geçici kullanıcıları kendisi siler
curl --fail http://localhost:5174/health

# API çalışırken, veritabanına demo verileri yazan sunum script'i
./scripts/demo_presentation.sh
./scripts/demo_presentation.sh --auto
```

Güncel AI modunda Google Search kapalıdır: tek yapılandırılmış çağrı en fazla 5 öneri üretir, fiyatlar doğrulanmamış tahmindir ve arayüzde "Tahmini fiyat" olarak etiketlenir. Search yeniden açılırsa `ai_sources` kaynak listesi ve `ai_search_suggestions` HTML'i frontend'de (kaynaklar katlanır listede, öneri HTML'i betiksiz sandbox iframe'de) gösterilir.

## 🔗 Bağlantılar
- GitHub: https://github.com/MSheker20/CovertGift
