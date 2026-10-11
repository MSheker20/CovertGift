# 📓 CovertGift — Geliştirme Günlüğü (DEVLOG)

> Bu dosya **ne yaptığımızı** ve **neden öyle yaptığımızı** kayıt altında tutar.
> Kural: Her anlamlı adım / karar sonrası en üste değil, **kronolojik olarak en alta** yeni bir kayıt eklenir.
> Kararlar ayrıca en alttaki **Karar Kaydı** tablosunda özetlenir.

---

## 2026-10-03 — Proje başlangıcı ve teknik şartname

**Yapılanlar**
- Projenin teknik şartnamesi yazıldı ve `PROJECT_CONTEXT.md` olarak kaydedildi.
- İçerik: 5 veritabanı varlığı (`User`, `Group`, `GroupMember`, `GiftIdea`, `Vote`), 4 temel iş kuralı (Anti-Spoiler, dinamik kişi başı bütçe, kırık link engelleme, state-based prompt), REST API sözleşmesi, kodlama standartları.
- Commit: `d502b74` — GitHub'a gönderildi.

**Ne işe yarar?**
`PROJECT_CONTEXT.md` projenin *tek doğruluk kaynağıdır*. Hem geliştirici hem de yapay zekâ asistanı her özelliği bu dosyaya göre uygular. Bir kural değişirse **önce bu dosya** güncellenir.

---

## 2026-10-06 — Ürün linki önizleme (link preview) prototipi

**Amaç:** Kullanıcı bir Trendyol paylaşım linki (`https://ty.gl/...`) yapıştırdığında ürünün adını, görselini ve fiyatını otomatik çekebilir miyiz?

**Ortam:** Go kuruldu → `go1.27.1 darwin/arm64` (`/opt/homebrew/bin/go`, Homebrew ile).

**Deneme 1 — Ham script (sadece yönlendirme takibi + OG meta etiketleri)**
- Test linki: `https://ty.gl/gkyyduf4wm8u3`
- Sonuç: Başlık geldi ✅ ama
  - Link Trendyol'a değil, **Adjust** takip sayfasına (`p8zh.adj.st`) gidip `200 OK` ile **durdu**.
  - Meta veriler Adjust'ın URL parametrelerinden (`adj_og_*`) üretilmiş jenerik veriydi.
  - Görsel bir **pazarlama popup'ı** idi (`.../mrktng/crm/Inapp/automationimages/...popup...jpg`).
  - Fiyat yoktu ❌.

**Deneme 2 — Düzeltilmiş script**
- Eklenenler:
  1. Son URL `adj.st` ise `adjust_redirect` parametresini decode edip **gerçek ürün sayfasına ikinci istek**.
  2. Fiyat için **JSON-LD** (`"price"`) yedeği.
  3. Jenerik görsel filtresine `mrktng`, `/crm/`, `popup`, `automationimages` eklendi.
- Sonuç: Trendyol ürün sayfasına ulaşıldı ✅, gerçek ürün görseli ✅, fiyat **699.00** ✅.
- Kalan hata 🐞: `og:description` "…ürününü Trendyol" diye kesiliyor — regex, içerikteki `'` karakterini (Trendyol**'**da) kapanış tırnağı sanıyor.

**Sonuç / Kararlar**
- Özellik uygulanabilir → `PROJECT_CONTEXT.md`'ye **İş Kuralı 5: Ürün Linki Önizleme** olarak eklendi.
- `GiftIdea` modeline opsiyonel `product_url` alanı eklendi (sadece MANUAL öneriler).
- Yeni endpoint: `POST /api/v1/groups/:id/ideas/preview-link`.
- Yeni paket: `backend/pkg/linkpreview/`.
- Backend, kullanıcının verdiği URL'ye istek atacağı için **SSRF koruması** zorunlu kılındı (domain allowlist + özel IP engeli + boyut/redirect limiti).
- Prototip kodu: [`prototypes/linkpreview/main.go`](../prototypes/linkpreview/main.go) — detaylar: [README](../prototypes/linkpreview/README.md).

---

## 2026-10-06 — Yol haritası

**Yapılanlar**
- `ROADMAP.md` oluşturuldu: 9 faz (Faz 0 Kurulum → Faz 8 Test & Cila), her fazda işaretlenebilir görevler ve **kabul kriterleri**.
- Link önizleme ayrı faz oldu (**Faz 5**); Gemini (Faz 6) ile paralel yapılabilir.
- Tespit edilen risk: `/api/v1/ideas/:id/*` uçları grup URL'si altında olmadığı için Anti-Spoiler kontrolü **orada ayrıca** uygulanmalı → Faz 4'e not düşüldü.

**Ne işe yarar?** Her oturumda "nerede kaldım, sırada ne var?" sorusunun cevabı. Bir faz kabul kriterlerini geçmeden sonrakine geçilmez.

---

## 2026-10-06 — İsim değişikliği: GiftlyAI → CovertGift

**Neden?** "GiftlyAI" adı telif/marka riski taşıyordu.

**Yapılanlar**
| Ne | Önce | Sonra |
|---|---|---|
| Yerel klasör | `~/Desktop/GiftlyAI` | `~/Desktop/CovertGift` |
| Doküman içerikleri | GiftlyAI | CovertGift |
| GitHub deposu | `MSheker20/GiftlyAI` | `MSheker20/CovertGift` (`gh repo rename`) |
| Git remote | `.../GiftlyAI.git` | `.../CovertGift.git` |

- Commit: `576a324` — GitHub'a gönderildi.
- Not: GitHub eski URL'yi yeni depoya otomatik yönlendirir.
- ⚠️ Açık konu: İlk commit mesajında (`d502b74`) hâlâ "GiftlyAI" geçiyor. Gerekirse geçmiş yeniden yazılıp force-push yapılabilir (geri alınamaz) — **şimdilik dokunulmadı.**

---

## 2026-10-06 — Docker kurulumu ve dokümantasyon

**Ortam**
- Docker `29.8.0`, Docker Compose `v5.5.1` kuruldu.

**Karar:** Faz 0 kurulumunu geliştirici **kendisi elle** yapacak (öğrenme amaçlı). Asistan dokümantasyonu hazırlar.

**Yapılanlar**
- `README.md` — giriş noktası, durum tablosu, doküman rehberi.
- `docs/SETUP.md` — Faz 0 adım adım kurulum rehberi; her adımda *ne yapar / neden*.
- `docs/DEVLOG.md` — bu dosya.
- Link önizleme prototipi `scratch` alanından repoya taşındı: `prototypes/linkpreview/`.
- Tespit: PostgreSQL 13+ sürümlerinde `gen_random_uuid()` çekirdekte yerleşik → `pgcrypto` eklentisi **gerekmiyor**. ROADMAP Faz 1 buna göre güncellendi.

---

## 2026-10-06 — Faz 0: Veritabanı ve Backend İskeletinin Kurulması

**Yapılanlar**
- `docker-compose.yml` ile PostgreSQL 17 servisi başlatıldı (`docker compose up -d`). Konteyner içi sağlık testi ve `gen_random_uuid()` doğrulaması başarıyla geçti (`118ebe5c-b1ad-4169-805b-...`).
- Katmanlı mimari klasörleri oluşturuldu (`cmd/api`, `internal/{config,controllers,middleware,models,repository,services}`, `pkg/{gemini,linkpreview}`).
- `go mod init github.com/MSheker20/CovertGift/backend` ile modül başlatıldı.
- Temel bağımlılıklar indirildi: `gin`, `gorm`, `driver/postgres`, `jwt/v5`, `bcrypt`, `uuid`, `godotenv`, `shopspring/decimal`.
- `.env.example` (şablon) ve `.env` (gizli) oluşturuldu; `openssl rand -hex 32` ile güçlü `JWT_SECRET` üretildi.
- `.gitignore` dosyası `.env` sızıntılarını, işletim sistemi kalıntılarını ve derleme çıktılarını engelleyecek şekilde genişletildi.

---

## 2026-10-11 — Faz 1: GORM Veritabanı Modellerinin Tanımlanması

**Yapılanlar**
- Teknik şartnamedeki 5 temel varlık (`User`, `Group`, `GroupMember`, `GiftIdea`, `Vote`) `backend/internal/models/` altında Go struct'ları olarak yazıldı:
  1. `models.User`: `uuid.UUID` primary key, `uniqueIndex` email, `PasswordHash` alanında `json:"-"` ile API yanıtı koruması.
  2. `models.Group`: `TargetUserID *uuid.UUID` (nullable pointer) ile Anti-Spoiler sürpriz altyapısı, `decimal.Decimal` bütçe, `Creator` foreign key ilişkisi.
  3. `models.GroupMember`: `uniqueIndex:idx_group_user` bileşik indeksi (bir kullanıcı gruba tek kez üye olabilir), varsayılan `role: 'member'`, cascade silme.
  4. `models.GiftIdea`: Pazar yeri linkleri için `SearchQuery`, manuel link önizleme için `ProductURL`, `decimal.Decimal` tahmini fiyat, cascade silme.
  5. `models.Vote`: `uniqueIndex:idx_idea_user` ile mükerrer oy engeli, `check:vote_type IN (-1, 1)` veritabanı kuralı.
- VS Code `gopls` linter uyarıları çözüldü: `go.work` oluşturuldu (`use ./backend`), `backend/go.mod` içine `shopspring/decimal` ve `golang-jwt/jwt/v5` kalıcı olarak kaydedildi. `go build ./...` ile sıfır hata doğrulandı.
- `main.go` içine GORM Bağlantı Havuzu (`SetMaxIdleConns(10)`, `SetMaxOpenConns(100)`) ve `AutoMigrate` eklendi.
- PostgreSQL 17 konteynerinde 5 tablonun (`gift_ideas`, `group_members`, `groups`, `users`, `votes`) ve Foreign Key kısıtlarının fiziksel olarak açıldığı `psql \dt` ile doğrulandı.
- Şartname standartlarına uygun JSON yanıt yardımcıları (`backend/internal/controllers/response.go`) oluşturuldu: `RespondOK`, `RespondCreated`, `RespondErr`. Faz 1 başarıyla tamamlandı.

---

## 🧭 Karar Kaydı (Özet)

| # | Tarih | Karar | Gerekçe |
|---|---|---|---|
| K1 | 10-03 | Gemini **asla URL üretmez**, sadece `search_query` üretir | Uydurma/kırık linkleri ve bot engellerini önlemek |
| K2 | 10-03 | Kişi başı bütçe DB'de tutulmaz, anlık hesaplanır | Üye sayısı değiştiğinde tutarsızlık olmasın |
| K3 | 10-03 | AI prompt'u sohbet geçmişi değil, DB durumundan üretilir | Token tasarrufu, deterministik bağlam |
| K4 | 10-06 | Link önizlemede `User-Agent: WhatsApp/...` kullanılır | Siteler sosyal medya botlarına OG etiketli sade HTML döner |
| K5 | 10-06 | Adjust tracker sayfaları `adjust_redirect` ile çözülür | Tracker sayfası `200` döndüğü için HTTP redirect takibi yetmiyor |
| K6 | 10-06 | `GiftIdea.product_url` sadece MANUAL önerilerde dolu | K1 ile çelişmemek: link kullanıcıdan gelir, AI'dan değil |
| K7 | 10-06 | Link önizlemede SSRF koruması zorunlu | Backend kullanıcı kontrollü URL'ye istek atıyor |
| K8 | 10-06 | Proje adı **CovertGift** | Telif/marka riski |
| K9 | 10-06 | `pgcrypto` kullanılmaz | PG13+ `gen_random_uuid()` yerleşik |
| K10 | 10-06 | Faz 0 elle kurulur, asistan dokümante eder | Geliştiricinin altyapıyı öğrenmesi |
| K11 | 10-11 | UUID için `google/uuid` paketi kullanılır | Go seviyesinde katı tip güvenliği (Type Safety) |
