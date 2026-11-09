# 🛠️ CovertGift — Faz 0 Kurulum Rehberi

> **Güncel AI modu (2026-10-21):** Google Search kapalıdır (`GEMINI_SEARCH_ENABLED=false`).
> `GEMINI_MODEL=gemini-3.8-flash` ile tek çağrıda öneri üretilir; fiyatlar doğrulanmamış tahmindir.
> Model kimlikleri uygulama kodunda sabit değildir. `GEMINI_MODEL` anahtar varsa zorunludur.
> `GEMINI_FALLBACK_MODEL` boşsa otomatik model geçişi yapılmaz; doluysa yalnızca 503'te denenir.
> `GEMINI_RESEARCH_MODEL` yalnızca Search açıkken kullanılır; boşsa ana model kullanılır.
> Yerelde `backend/.env`, Azure'da Container App ortam değişkenleri düzenlenir ve uygulama yeniden başlatılır/yeni revision oluşur.
> Aşağıdaki eski Search araştırması açıklamaları yalnızca Search modu tekrar açılırsa geçerlidir.

> Bu rehber [ROADMAP.md → Faz 0](../ROADMAP.md) görevlerini **elle** yapman için hazırlandı.
> Her adımda: **📌 Ne yapıyoruz → 💡 Neden → ⌨️ Komut/Dosya → ✅ Kontrol.**
> Bir adımı tamamladıkça ROADMAP'teki kutucuğu `[x]` yap, önemli bir şey öğrendiysen [DEVLOG](DEVLOG.md)'a yaz.
> Bu dosyadaki config/API örnekleri Faz 0 iskeletini açıklar; güncel çalışan kaynaklar `backend/` altındadır. Hazır projeyi başlatmak için [README → Hızlı Başlangıç](../README.md#-hızlı-başlangıç) adımlarını izleyin. Geçici panel ve Cloudflare kullanımı Adım 7a'dadır (2026-10-17).

**Hedef (Kabul Kriteri):** `docker compose up -d` + `go run ./cmd/api` → `GET http://localhost:8080/health` → `200 OK`.

---

## Adım 0 — Ön koşulları doğrula

📌 Gerekli araçların kurulu olduğunu kontrol ediyoruz.
💡 Sürüm uyumsuzlukları sonradan anlaşılması zor hatalara yol açar.

```bash
go version              # beklenen: go1.27.x darwin/arm64
docker --version        # beklenen: 29.x
docker compose version  # beklenen: v5.x
git --version
node --version          # geçici test paneli için 22+
cloudflared --version   # isteğe bağlı HTTPS tüneli için
```

✅ Hepsi sürüm yazdırıyor. Docker Desktop uygulaması **açık** olmalı (menü çubuğunda balina ikonu).

| Araç | Bu projede ne işe yarar? |
|---|---|
| Go | Backend API'sini yazıp çalıştırmak |
| Docker | PostgreSQL'i bilgisayarına kurmadan izole bir konteynerde çalıştırmak |
| Docker Compose | Konteyner ayarlarını (port, şifre, disk) tek bir YAML dosyasında tanımlamak |
| Git | Sürüm kontrolü + GitHub'a gönderme |
| Node.js 22+ | Geçici test panelini ve API proxy'sini çalıştırmak |
| cloudflared (isteğe bağlı) | Paneli geçici HTTPS adresiyle internetten açmak |

---

## Adım 1 — `.gitignore`

📌 Proje kök dizininde (`CovertGift/`) `.gitignore` dosyası oluştur.
💡 Gizli bilgiler (`.env` içindeki şifreler, Gemini API anahtarı) ve üretilen dosyalar **asla** GitHub'a gitmemeli.

```gitignore
# Ortam değişkenleri (şifreler, API anahtarları)
.env
*.env.local

# Go
backend/bin/
*.exe
*.test
*.out

# Node / Frontend
node_modules/
dist/

# İşletim sistemi / editör
.DS_Store
.idea/
.vscode/
```

✅ `git status` çıktısında ileride `.env` dosyası **görünmemeli**.

---

## Adım 2 — `docker-compose.yml` (PostgreSQL)

📌 Kök dizinde `docker-compose.yml` oluştur.
💡 Veritabanını Docker'da çalıştırmak: bilgisayarına Postgres kurmazsın, herkes aynı sürümü kullanır, istediğin an silip sıfırdan başlatabilirsin.

```yaml
services:
  db:
    image: postgres:17-alpine          # Resmi Postgres imajı, alpine = küçük boyut
    container_name: covertgift-db      # docker komutlarında bu isimle erişeceğiz
    restart: unless-stopped            # Docker açılınca otomatik başlasın
    environment:
      POSTGRES_USER: covertgift
      POSTGRES_PASSWORD: covertgift_dev   # Sadece yerel geliştirme şifresi!
      POSTGRES_DB: covertgift
    ports:
      - "5432:5432"                    # bilgisayar:konteyner
    volumes:
      - covertgift_pgdata:/var/lib/postgresql/data   # Veriler konteyner silinse de kalır
    healthcheck:                       # Veritabanı bağlantı kabul ediyor mu?
      test: ["CMD-SHELL", "pg_isready -U covertgift -d covertgift"]
      interval: 5s
      timeout: 3s
      retries: 10

volumes:
  covertgift_pgdata:
```

| Alan | Ne işe yarar? |
|---|---|
| `image` | Hangi yazılımın hangi sürümü çalışacak |
| `environment` | Postgres ilk açılışta bu kullanıcı/şifre/veritabanını oluşturur |
| `ports` | Go uygulaması `localhost:5432` üzerinden konteynere bağlanır |
| `volumes` | Veriler Docker'ın yönettiği kalıcı bir diske yazılır |
| `healthcheck` | `docker compose ps` çıktısında `healthy` görerek DB'nin hazır olduğunu anlarız |

⌨️ Başlat ve kontrol et:
```bash
docker compose up -d        # -d = arka planda çalıştır
docker compose ps           # STATUS sütununda "healthy" görmelisin
docker exec -it covertgift-db psql -U covertgift -d covertgift -c "SELECT version();"
docker exec -it covertgift-db psql -U covertgift -d covertgift -c "SELECT gen_random_uuid();"
```

✅ Son komut bir UUID döndürüyor. (Postgres 13+ ile `gen_random_uuid()` yerleşik — `pgcrypto` eklentisine gerek yok, bkz. DEVLOG K9.)

---

## Adım 3 — Backend klasör yapısı

📌 Şartnamedeki standart Go yapısını oluştur.
💡 Her katmanın tek bir sorumluluğu var; kod büyüdükçe neyin nerede olduğu belli olur.

```bash
mkdir -p backend/cmd/api
mkdir -p backend/internal/config backend/internal/controllers backend/internal/middleware
mkdir -p backend/internal/models backend/internal/repository backend/internal/services
mkdir -p backend/pkg/gemini backend/pkg/linkpreview
```

| Klasör | Sorumluluğu | Örnek |
|---|---|---|
| `cmd/api/` | Uygulamanın giriş noktası (`main.go`): her şeyi birbirine bağlar, sunucuyu başlatır | `main.go` |
| `internal/config/` | Ortam değişkenlerini okur | `config.go` |
| `internal/models/` | GORM veritabanı yapıları (tablolar) | `user.go`, `group.go` |
| `internal/repository/` | **Sadece** veritabanı sorguları | `user_repo.go` |
| `internal/services/` | İş mantığı (Anti-Spoiler, bütçe hesabı, şifre hash) | `auth_service.go` |
| `internal/controllers/` | HTTP isteğini alır → servisi çağırır → JSON döner | `group_controller.go` |
| `internal/middleware/` | Her istekten önce çalışan kontroller (JWT, Anti-Spoiler) | `auth.go` |
| `pkg/gemini/` | Gemini API istemcisi (projeden bağımsız, tekrar kullanılabilir) | `client.go` |
| `pkg/linkpreview/` | Link önizleme scraper'ı (prototipten taşınacak) | `preview.go` |

💡 `internal/` Go'da özeldir: bu klasördeki paketler modül **dışından** import edilemez. `pkg/` ise başka projelerde de kullanılabilecek kodlar içindir.

Akış: `İstek → middleware → controller → service → repository → PostgreSQL`

---

## Adım 4 — Go modülü ve bağımlılıklar

📌 Go modülünü başlat, kütüphaneleri indir.
💡 `go.mod` projenin kimliği ve bağımlılık listesidir; `go.sum` indirilen paketlerin doğrulama özetleridir. **İkisi de commit'lenir.**

```bash
cd backend
go mod init github.com/MSheker20/CovertGift/backend

go get github.com/gin-gonic/gin
go get gorm.io/gorm
go get gorm.io/driver/postgres
go get github.com/golang-jwt/jwt/v5
go get golang.org/x/crypto/bcrypt
go get github.com/google/uuid
go get github.com/joho/godotenv
go get github.com/shopspring/decimal
```

| Paket | Ne işe yarar? | Hangi fazda? |
|---|---|---|
| `gin-gonic/gin` | HTTP router + JSON yanıtlar | Faz 0 |
| `gorm.io/gorm` + `driver/postgres` | ORM: Go struct'ları ↔ tablolar, AutoMigrate | Faz 1 |
| `golang-jwt/jwt/v5` | Giriş token'ı üretme/doğrulama | Faz 2 |
| `x/crypto/bcrypt` | Şifreleri güvenli hash'leme | Faz 2 |
| `google/uuid` | UUID tipi | Faz 1 |
| `joho/godotenv` | `.env` dosyasını okuma | Faz 0 |
| `shopspring/decimal` | Para tutarları (`NUMERIC(10,2)`) — `float` yuvarlama hatası yapar | Faz 1 |

> Gemini SDK'sı (`google.golang.org/genai`) Faz 6'da eklenecek.

✅ `backend/go.mod` dosyasında yukarıdaki paketler `require` bloğunda görünüyor.

---

## Adım 5 — Ortam değişkenleri (`.env`)

📌 `backend/.env.example` (şablon, **commit'lenir**) ve `backend/.env` (gerçek değerler, **commit'lenmez**) oluştur.
💡 Şifreler kodun içine yazılmaz; ortama göre (yerel / sunucu) değişen ayarlar buradan okunur.

`backend/.env.example`:
```dotenv
PORT=8080
DATABASE_URL=postgres://covertgift:covertgift_dev@localhost:5432/covertgift?sslmode=disable
JWT_SECRET=buraya-uzun-rastgele-bir-deger-yaz
GEMINI_API_KEY=
GEMINI_RESEARCH_MODEL=gemini-3.8-flash
GEMINI_MODEL=gemini-3.8-flash
ALLOW_DB_RESET=false
```

```bash
cp .env.example .env
openssl rand -hex 32     # çıkan değeri .env içindeki JWT_SECRET'e yapıştır
```

| Değişken | Ne işe yarar? |
|---|---|
| `PORT` | API'nin dinleyeceği port |
| `DATABASE_URL` | Adım 2'deki compose ayarlarıyla **birebir** aynı kullanıcı/şifre/DB |
| `JWT_SECRET` | Token imzalama anahtarı — sızarsa herkes sahte token üretebilir |
| `GEMINI_API_KEY` | Google AI Studio'dan alınacak (Faz 6) |
| `GEMINI_MODEL` | Öneri modeli (ör. `gemini-3.8-flash`); anahtar varsa zorunlu, kodda varsayılan yok |
| `GEMINI_RESEARCH_MODEL` | Yalnızca Search açıkken; boşsa `GEMINI_MODEL` kullanılır |
| `LOG_FORMAT` / `LOG_LEVEL` | `json` ya da `text` (boşsa production'da json) / `debug`, `info`, `warn`, `error` |
| `APP_ENV` | Varsayılan `development`; `production` geliştirme veritabanı sıfırlama ucunu kapatır |
| `ALLOW_DB_RESET` | Varsayılan `false`; `true` olursa `POST /api/v1/dev/reset-db` açılır ve gövdede `{"confirm": "RESET"}` ister |

CORS ayarı yoktur: frontend API'ye her ortamda aynı origin'den (Vite proxy'si ya da nginx) erişir, bu yüzden API CORS başlığı göndermez ve tarayıcılar başka sitelerden gelen istekleri engeller. Eski `CORS_ORIGIN` değişkeni 2026-11-09'da kaldırıldı; `.env` dosyanızda kaldıysa yok sayılır.

✅ `git status` → `.env.example` görünüyor, `.env` **görünmüyor**.

---

## Adım 6 — Config + `/health` endpoint'i

📌 Ortam değişkenlerini okuyan config paketi ve sağlık kontrolü yapan ilk endpoint.
💡 `/health` hem "sunucu ayakta mı?" hem "veritabanına bağlanabiliyor mu?" sorusunu cevaplar — Faz 0'ın kabul testi budur.

**Referans** (kendin yazmayı dene, takılırsan bak):

`backend/internal/config/config.go`
```go
package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	GeminiKey   string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  .env bulunamadı, sistem ortam değişkenleri kullanılacak")
	}
	cfg := &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		GeminiKey:   os.Getenv("GEMINI_API_KEY"),
	}
	if cfg.DatabaseURL == "" || cfg.JWTSecret == "" {
		log.Fatal("❌ DATABASE_URL ve JWT_SECRET zorunludur")
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

`backend/cmd/api/main.go`
```go
package main

import (
	"log"
	"net/http"

	"github.com/MSheker20/CovertGift/backend/internal/config"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	cfg := config.Load()

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("❌ Veritabanına bağlanılamadı: %v", err)
	}

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil || sqlDB.Ping() != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "veritabanı erişilemez"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "ok", "db": "up"}})
	})

	log.Printf("🚀 CovertGift API :%s portunda çalışıyor", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("❌ Sunucu başlatılamadı: %v", err)
	}
}
```

💡 Yanıt formatı şartnamedeki standarda uygun: `{"success": true, "data": ...}` / `{"success": false, "error": "..."}`.

---

## Adım 7 — Kabul testi ✅

```bash
# Kök dizinde
docker compose up -d

# backend/ içinde
go mod tidy          # kullanılmayan paketleri temizler, eksikleri ekler
go run ./cmd/api

# Yeni bir terminalde
curl -i http://localhost:8080/health
```

Beklenen:
```
HTTP/1.1 200 OK
{"success":true,"data":{"db":"up","status":"ok"}}
```

Ek test — DB kapalıyken davranış:
```bash
docker compose stop db
curl -i http://localhost:8080/health    # 503 beklenir
docker compose start db
```

---

## Adım 7a — React Frontend (Güncel Çalıştırma)

Docker Desktop açıkken proje kökünde `docker compose up -d --wait`, ardından `backend/` içinde `go run ./cmd/api` çalıştırın. Yeni terminalde proje kökünden (Node.js 22+):

```bash
npm --prefix frontend install
npm --prefix frontend run dev
```

[Uygulamayı açın](http://localhost:5174). Vite, `/api` ve `/health` isteklerini `127.0.0.1:8080`'e iletir:

```bash
curl --fail http://localhost:5174/health
```

> ⚠️ `POST /api/v1/dev/reset-db` tüm tabloları siler. Varsayılan olarak kapalıdır; yalnızca `ALLOW_DB_RESET=true` ve gövdede `{"confirm": "RESET"}` ile çalışır. Bayrağı açtığınız bir API'ye test veya script'lerde bu ucu çağırtmayın; yerel veri için düzenli yedek alın:
>
> ```bash
> docker exec covertgift-db pg_dump -U covertgift -d covertgift -Fc > ~/covertgift-$(date +%F).dump
> ```

Doğrulama (proje kökünden):

```bash
go test ./backend/...
npm --prefix frontend test
npm --prefix frontend run build
npm --prefix frontend run test:e2e   # API çalışırken; ilk kurulumda: npx --prefix frontend playwright install chromium
```

Anti-Spoiler entegrasyon testleri gerçek router ve PostgreSQL ile çalışır. Ayrı ve geçici (tmpfs) bir test veritabanı kullanır; `TEST_DATABASE_URL` yoksa atlanır, veritabanı adı `_test` ile bitmiyorsa çalışmayı reddeder:

```bash
docker compose --profile test up -d --wait test-db
TEST_DATABASE_URL='postgres://covertgift:covertgift_test@localhost:55432/covertgift_test?sslmode=disable' go test -p 1 -count=1 ./backend/cmd/...   # -p 1: paketler aynı test DB'sini paylaşır
docker compose --profile test stop test-db
```

Seed (demo verisi) ve tüm uygulamayı konteynerde çalıştırma:

```bash
# backend/ içinden: yerel DB'ye 4 demo kullanıcı, hedefi bağlı bir grup, 3 fikir ve oylar ekler.
# Yalnızca ekler; demo yöneticisi varsa hiçbir şey yapmaz; APP_ENV=production iken çalışmaz.
(cd backend && go run ./cmd/seed)          # parola: Demo1234! (SEED_PASSWORD ile değiştirilebilir)

# API + React frontend + DB konteynerde (ayarlar backend/.env'den; JWT_SECRET zorunlu)
docker compose --profile app up -d --build  # arayüz: http://localhost:8081
docker compose --profile app stop api web
```

Eski geçici paneli (`prototypes/test-ui/`) ve Cloudflare tünelini çalıştırmak gerekirse: [panel rehberi](../prototypes/test-ui/README.md). Panel yeni uçları desteklemez.

Swagger'ı modeller ve controller yorumlarından yeniden üretmek için `swag` kurulu ortamda `backend/` içinden `swag init -g cmd/api/main.go --parseInternal` çalıştırın.

Normal Go testleri gerçek Gemini API'sini çağırmaz. Kota kullanan gerçek API testi özellikle istenirse `backend/` içinden `go test ./pkg/gemini -run TestLiveTurkeyProducts -v -args -live-ai` ile açılır.

---

## Adım 8 — Commit

```bash
git add .
git status            # .env listede OLMAMALI!
git commit -m "Faz 0: Docker Compose, backend iskeleti, config ve /health endpoint"
git push
```

Sonra: ROADMAP'te Faz 0 kutucuklarını işaretle, DEVLOG'a kısa bir kayıt ekle.

---

## 🧰 Sık Kullanılan Komutlar

| Komut | Ne yapar? |
|---|---|
| `docker compose up -d` | Veritabanını arka planda başlatır |
| `docker compose ps` | Konteyner durumu (`healthy`?) |
| `docker compose logs -f db` | Postgres loglarını canlı izler |
| `docker compose stop` | Durdurur (veriler kalır) |
| `docker compose down` | Konteyneri siler (veriler **kalır**, volume'de) |
| `docker compose down -v` | ⚠️ Konteyner **ve verileri** siler — sıfırdan başlamak için |
| `docker exec -it covertgift-db psql -U covertgift -d covertgift` | Veritabanı konsoluna girer (`\dt` tablolar, `\q` çıkış) |
| `go run ./cmd/api` | API'yi çalıştırır |
| `go mod tidy` | Bağımlılıkları düzenler |
| `go build -o bin/api ./cmd/api` | Çalıştırılabilir dosya üretir |
| `go test -v ./...` | Tüm birim testlerini çalıştırır (link önizleme, parser, SSRF kalkanı) |
| `npm --prefix frontend run dev` | React frontend'i :5174'te başlatır |
| `npm --prefix frontend test` | Frontend bileşen/akış testleri (Vitest) |
| `npm --prefix frontend run test:e2e` | Gerçek API'ye karşı uçtan uca testler (Playwright) |
| `./scripts/demo_presentation.sh` | 🎓 Tüm API uç noktalarını canlı sergileyen akademik interaktif sunum script'i |
| `./scripts/demo_presentation.sh --auto` | Otomatik demo / test modu |

---

## 🩺 Sorun Giderme

| Belirti | Olası neden | Çözüm |
|---|---|---|
| `Cannot connect to the Docker daemon` | Docker Desktop kapalı | Uygulamayı aç, balina ikonunun sabitlenmesini bekle |
| `port 5432 is already allocated` / `address already in use` | Bilgisayarda başka bir Postgres çalışıyor (ör. Homebrew) | `brew services stop postgresql` **veya** compose'da `"5433:5432"` yapıp `DATABASE_URL`'de portu `5433` olarak değiştir |
| `password authentication failed` | `.env` ile compose bilgileri uyuşmuyor **veya** volume eski şifreyle oluşturulmuş | Bilgileri eşitle; gerekirse `docker compose down -v` ile volume'ü sıfırla (şifre sadece **ilk** açılışta uygulanır) |
| `connection refused` (API açılırken) | DB henüz hazır değil | `docker compose ps` → `healthy` olana kadar bekle |
| `.env bulunamadı` uyarısı | `go run` komutu `backend/` dışından çalıştırıldı | `backend/` klasörüne geçip çalıştır |
| `package ... is not in std` | Modül yolu import ile uyuşmuyor | `go.mod`'daki `module` satırı ile import yolları aynı olmalı |
