<div align="center">

# 🎁 CovertGift

**Arkadaşlarınla ortak hediye seç. Hediye alacağın kişi hiçbir şey görmesin.**

[![Canlı demo](https://img.shields.io/badge/Canl%C4%B1%20demo-covertgift.msheker.dev-2f5bff?style=flat-square)](https://covertgift.msheker.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-ffc93c?style=flat-square)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.27-00ADD8?style=flat-square&logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-19-61DAFB?style=flat-square&logo=react&logoColor=black)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-17-4169E1?style=flat-square&logo=postgresql&logoColor=white)
![Azure](https://img.shields.io/badge/Azure-Container%20Apps-0078D4?style=flat-square&logo=microsoftazure&logoColor=white)

[**Canlı demo**](https://covertgift.msheker.dev) · [Teknik şartname](PROJECT_CONTEXT.md) · [Yol haritası](ROADMAP.md) · [Geliştirme günlüğü](docs/DEVLOG.md)

<img src="docs/screenshots/home-dark.png" alt="CovertGift ana ekranı: sıradaki sürpriz, senden bekleyen görevler ve gruplar" width="900">

</div>

> *English:* CovertGift is a collaborative web app for planning group gifts. Friends pool a budget, collect ideas from product links or Gemini AI suggestions, vote, and pick a winner, while the person receiving the gift is kept out of everything (Anti-Spoiler). Go + Gin + PostgreSQL backend, React 19 + TypeScript frontend, deployed on Azure Container Apps. The UI is available in Turkish and English.

---

## ✨ Neler yapabilirsin?

**Grubu kur**
- Hediye alınacak kişi için bir grup aç. Kişi kayıtlıysa e-postasıyla bağla; o kişi grubu **hiçbir zaman** göremez.
- Arkadaşlarını e-postayla ya da davet linkiyle ekle. Toplam bütçe ve kişi başı tutar anında hesaplanır.
- Birden çok kişiye aynı anda hediye alınacaksa (ör. "Ofis Yılbaşı") her kişi için ayrı bir havuz açılır; herkes yalnızca başkalarının havuzunu görür.

**Fikir topla ve oyla**
- Trendyol, Amazon TR ya da Hepsiburada linki yapıştır; başlık, görsel ve fiyat otomatik gelir.
- Ya da **Gemini**'den ilgi alanlarına ve bütçeye göre öneri iste.
- Fikirleri oyla (+1 / −1), oyunu geri al. Yönetici veto edebilir.
- Başkalarının oyları ve yeni fikirler sayfa yenilenmeden anında görünür (SSE).

**Sonuçlandır**
- Hediye tarihini ekle, geri sayımı takip et. "Senden bekleyenler" paneli o an ne yapman gerektiğini öncelik sırasıyla gösterir.
- Bir ya da birden çok kazanan seç; toplam tutar bütçeyle karşılaştırılır.
- Yeni fikir, kazanan ve yaklaşan tarih gibi olaylar için bildirim al. Hediye alınan kişiye hiçbir bildirim gitmez.

**Kişiselleştir**
- Gruba kapak fotoğrafı ve açıklama ekle, profil fotoğrafını yükle (tarayıcıda kırpılır ve küçültülür).
- Türkçe / İngilizce, açık / koyu tema. Telefon için de tasarlandı.

<table>
  <tr>
    <td width="62%"><img src="docs/screenshots/group-dark.png" alt="Grup sayfası: bütçe, kişi başı tutar, ilgi alanları ve oylanan hediye fikirleri"></td>
    <td width="38%"><img src="docs/screenshots/home-mobile.png" alt="Telefonda ana ekran"></td>
  </tr>
  <tr>
    <td align="center"><sub>Grup sayfası: bütçe, ilgi alanları ve oylanan fikirler</sub></td>
    <td align="center"><sub>Telefonda ana ekran</sub></td>
  </tr>
  <tr>
    <td><img src="docs/screenshots/home-light.png" alt="Açık temada ana ekran"></td>
    <td><img src="docs/screenshots/login-dark.png" alt="Giriş ekranı"></td>
  </tr>
  <tr>
    <td align="center"><sub>Açık tema</sub></td>
    <td align="center"><sub>Giriş ekranı</sub></td>
  </tr>
</table>

<sub>Ekran görüntüleri demo verisiyle alınmıştır (`go run ./cmd/seed`).</sub>

---

## 🛡️ Anti-Spoiler: sürpriz nasıl korunuyor?

Sürprizi korumak arayüzde bir şeyleri gizlemekle değil, **sunucuda** yapılıyor:

- `/groups/:id/*` ve `/ideas/:id/*` altındaki her uç, isteği yapanın grubun hedef kişisi olup olmadığını kontrol eder. Hedef kişiyse **403** ve bir sürpriz mesajı döner.
- Birden çok kişilik gruplarda hediye alınan kişi kendi havuzu için yalnızca kilitli bir kart görür; havuz uçlarının hepsi ayrı bir erişim katmanından geçer.
- Davet linkleri de hedef kişiye 403 döner; token loglara yazılmaz.
- Bu kurallar gerçek router ve PostgreSQL ile çalışan entegrasyon testleriyle korunur.

Ayrıntılar: [PROJECT_CONTEXT.md](PROJECT_CONTEXT.md).

---

## 🏗️ Mimari

```mermaid
flowchart LR
    U[Tarayıcı] -->|HTTPS| W["covertgift-web<br/>nginx + React (external)"]
    W -->|"/api, /health<br/>(ortam içi)"| A["covertgift-server<br/>Go + Gin API (internal)"]
    A --> D[(PostgreSQL 17)]
    A -->|öneri| G[Gemini API]
    A -->|link önizleme| M[Trendyol / Amazon TR / Hepsiburada]
```

- **Tek giriş kapısı:** Dışarıya yalnızca nginx açık. API ortamın içinde kalır ve internetten doğrudan erişilemez. nginx, `/api/v1/dev/*` uçlarını kapatır, istek boyutlarını sınırlar ve güvenlik başlıklarını ekler.
- **Canlı güncellemeler:** Server-Sent Events. Olaylar veri taşımaz; istemci korumalı uçlardan yeniden çeker, böylece Anti-Spoiler kuralları burada da geçerli kalır.
- **Barındırma:** Azure Container Apps (standart ortam, Consumption profili; boştayken sıfıra ölçeklenir) ve Azure Database for PostgreSQL. Ayrıntılar: [docs/AZURE.md](docs/AZURE.md).

---

## 🧱 Teknoloji yığını

| Katman | Teknoloji |
|---|---|
| Backend | Go 1.27 · Gin · GORM · `log/slog` ile yapılandırılmış log · Swagger / OpenAPI |
| Veritabanı | PostgreSQL 17 (yerelde Docker Compose) |
| Kimlik doğrulama | JWT (HMAC-SHA256) · bcrypt |
| Yapay zekâ | `google.golang.org/genai`. Model adları ortam değişkenlerinden gelir (`GEMINI_MODEL`, isteğe bağlı `GEMINI_FALLBACK_MODEL`). Güncel modda Google Search kapalı; fiyatlar "tahmini" olarak etiketlenir. |
| Link önizleme | `pkg/linkpreview`: alan adı izin listesi, özel IP engeli (SSRF koruması), 2 MB ve 10 yönlendirme sınırı, JSON-LD ayrıştırma |
| Görsel işleme | `pkg/imageproc` (saf Go): tür tespiti, EXIF yönü, kırpma ve küçültme; görseller PostgreSQL'de |
| Canlı güncellemeler | Server-Sent Events: grup akışı ve kullanıcı bazlı bildirim akışı |
| Frontend | React 19 · Vite · TypeScript · Tailwind CSS v4 · Radix UI · TanStack Query · React Router · i18next (TR/EN) |
| Testler | `go test` (birim + Anti-Spoiler entegrasyon) · Vitest + Testing Library + MSW · Playwright |
| Dağıtım | Docker (çok aşamalı imajlar, ayrıcalıksız kullanıcı) · nginx · Azure Container Apps · Azure Container Registry |

---

## 🚀 Hızlı başlangıç

Ön koşullar: Go **1.27.4+**, Docker Desktop / Docker Compose, Node.js **22+**. Ayrıntılı kurulum: [docs/SETUP.md](docs/SETUP.md).

```bash
git clone https://github.com/MSheker20/CovertGift.git
cd CovertGift
cp backend/.env.example backend/.env
openssl rand -hex 32   # çıktıyı backend/.env içindeki JWT_SECRET alanına yaz
```

AI önerileri için `backend/.env`'e `GEMINI_API_KEY` ekle. Anahtar yoksa AI dışındaki her şey çalışır.

```bash
# Terminal 1: veritabanı ve API
docker compose up -d --wait
cd backend && go run ./cmd/api

# Terminal 2: frontend
npm --prefix frontend install
npm --prefix frontend run dev
```

Uygulama http://localhost:5174 adresinde açılır; Vite, `/api` ve `/health` isteklerini API'ye (`:8080`) iletir. Swagger: http://localhost:8080/swagger/index.html

**Demo verisi:** `(cd backend && go run ./cmd/seed)` dört demo kullanıcı ve örnek gruplar ekler (parola: `Demo1234!`, `SEED_PASSWORD` ile değiştirilebilir). Yalnızca ekleme yapar ve `APP_ENV=production` iken çalışmaz.

**Her şey konteynerde:** `docker compose --profile app up -d --build` ile arayüz http://localhost:8081 adresinde açılır.

> ⚠️ `POST /api/v1/dev/reset-db` **tüm tabloları siler**. Varsayılan olarak kapalıdır; yalnızca `ALLOW_DB_RESET=true` ve gövdede `{"confirm": "RESET"}` ile çalışır, üretimde her durumda kapalıdır.

---

## 🧪 Testler

```bash
go test ./backend/...                 # birim testleri (gerçek Gemini çağrılmaz)
npm --prefix frontend test            # Vitest
npm --prefix frontend run build       # tip kontrolü + üretim derlemesi
npm --prefix frontend run test:e2e    # Playwright (API ve DB çalışırken)
```

Anti-Spoiler entegrasyon testleri ayrı ve geçici bir test veritabanı kullanır. Veritabanı adı `_test` ile bitmiyorsa çalışmayı reddeder:

```bash
docker compose --profile test up -d --wait test-db
TEST_DATABASE_URL='postgres://covertgift:covertgift_test@localhost:55432/covertgift_test?sslmode=disable' \
  go test -p 1 -count=1 ./backend/cmd/...
```

---

## 🗂️ Proje yapısı

```
CovertGift/
├── backend/
│   ├── cmd/api/          ← HTTP sunucusu, router, Anti-Spoiler entegrasyon testleri
│   ├── cmd/seed/         ← Demo verisi
│   ├── internal/         ← controllers, services, repository, models, middleware, realtime (SSE)
│   └── pkg/              ← gemini, imageproc, linkpreview
├── frontend/             ← React uygulaması, testler, Dockerfile + nginx şablonu
├── docs/                 ← SETUP, AZURE, DEVLOG, BACKLOG, ekran görüntüleri
├── prototypes/           ← Link önizleme laboratuvarı ve eski test paneli
├── PROJECT_CONTEXT.md    ← Teknik şartname (tek doğruluk kaynağı)
└── ROADMAP.md            ← Fazlar ve kabul kriterleri
```

## 📚 Dokümantasyon

| Dosya | İçerik |
|---|---|
| [PROJECT_CONTEXT.md](PROJECT_CONTEXT.md) | Veri modelleri, iş kuralları, API sözleşmesi, mimari standartlar |
| [ROADMAP.md](ROADMAP.md) | Faz faz görev listesi ve kabul kriterleri |
| [docs/SETUP.md](docs/SETUP.md) | Adım adım kurulum; her adımın ne yaptığı ve neden yapıldığı |
| [docs/AZURE.md](docs/AZURE.md) | Docker imajları, Azure dağıtımı, işletim notları ve olay kayıtları |
| [docs/DEVLOG.md](docs/DEVLOG.md) | Geliştirme günlüğü ve alınan kararların gerekçeleri |
| [docs/BACKLOG.md](docs/BACKLOG.md) | Sıradaki işler, açık fikirler |
| [frontend/README.md](frontend/README.md) | Frontend'in yapısı, testleri ve tasarım token'ları |

## 📍 Durum

Faz 0–8 tamamlandı. Kimlik doğrulama, gruplar ve Anti-Spoiler, hediye havuzu ve oylama, link önizleme, Gemini önerileri, React frontend, bildirimler, canlı güncellemeler, kapak ve profil fotoğrafları, davet linkleri ve birden çok kişilik gruplar canlıda. Sıradaki işler [docs/BACKLOG.md](docs/BACKLOG.md)'de: e2e testlerinin güncellenmesi, CI, bireysel katkı takibi ve mobil uygulama fikri.

> Projenin eski adı *GiftlyAI* idi; telif riski nedeniyle 2026-10-06'da **CovertGift** olarak değiştirildi.

## 📄 Lisans

[MIT](LICENSE) © 2026 Mustafa Şeker. `.claude/` altındaki agent, komut ve skill dosyalarının çoğu [ECC](https://github.com/affaan-m/ECC) projesinden uyarlanmıştır (MIT); bkz. [.claude/THIRD_PARTY_NOTICES.md](.claude/THIRD_PARTY_NOTICES.md). Güvenlik açığı bildirmek için: [SECURITY.md](SECURITY.md).
