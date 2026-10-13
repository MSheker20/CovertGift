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

## 2026-10-11 — Faz 2: Kimlik Doğrulama (Auth) ve JWT Altyapısı

**Yapılanlar**
- Katmanlı mimari desenine uygun olarak kimlik doğrulama modülü geliştirildi:
  1. `repository/user_repo.go`: `Create`, `FindByEmail`, `FindByID` sorguları. E-posta aramalarında büyük/küçük harf çakışmalarını önlemek için `LOWER(email)` kullanıldı. Kayıt bulunamadığında `nil, nil` dönülerek temiz servis kontrolü sağlandı.
  2. `services/auth_service.go`: Şifreler `bcrypt` (maliyet: 10) ile güvenli biçimde hash'lendi. Kullanıcı sayımı (User Enumeration) saldırılarına karşı giriş hatalarında yeknesak hata (`ErrInvalidCredentials`) fırlatıldı. 24 saat geçerli imzalı JWT token üretildi (`HS256`, özel `JWTClaims` ile `user_id` ve `email`).
  3. `middleware/auth.go`: `Authorization: Bearer <token>` başlığını doğrulayan Gin middleware'i yazıldı. Algorithm Confusion saldırılarına karşı `SigningMethodHMAC` kontrolü yapıldı, geçerli kullanıcının UUID'si context'e (`userID`) yazıldı.
  4. `controllers/auth_controller.go`: `POST /api/v1/auth/register`, `POST /api/v1/auth/login` ve `GET /api/v1/me` uç noktaları eklendi. Gelen istekler `validator/v10` (`binding:"required,email,min=6"`) ile doğrulandı. E-posta çakışmasında `409 Conflict`, şifre/kullanıcı hatalarında `401 Unauthorized`, başarılı kayıtta `201 Created` dönüldü.
  5. `cmd/api/main.go`: Dependency Injection ile Repository -> Service -> Controller katmanları birbirine bağlandı, rotalar açık (public) ve korumalı (protected) gruplara ayrıldı.
- `curl` testleriyle 5 kabul kriteri doğrulandı: Kayıt (201), mükerrer kayıt (409), giriş ve JWT üretimi (200), tokensız erişim reddi (401), tokenlı profil sorgusu (200). `models.User` içindeki `json:"-"` sayesinde şifre hash'inin API yanıtlarında asla sızmadığı teyit edildi.

---

## 2026-10-11 — Faz 3: Grup İşlemleri ve Anti-Spoiler Koruması 🛡️

**Yapılanlar**
- İşbirlikçi hediye grupları ve şartnamedeki Anti-Spoiler koruması geliştirildi:
  1. `repository/group_repo.go`: Atomik grup oluşturma ve kurucuyu `admin` yapma işlemi GORM Transaction (`db.Transaction`) ile bağlandı. `GetUserGroups` sorgusunda SQL seviyesinde `groups.target_user_id IS NULL OR groups.target_user_id != userID` filtresi uygulandı. `GetMemberCount` ile veritabanı seviyesinde `COUNT(*)` sağlandı.
  2. `services/group_service.go`: Dinamik kişi başı bütçe formülü (Toplam Bütçe / Üye Sayısı) kuruş hassasiyetiyle (`decimal.Round(2)`) ve sıfıra bölme korumasıyla uygulandı. Hedef kişinin kendisi için grup açması engellendi. Gruba e-posta ile üye eklerken hedef kişinin üye yapılması engellendi (`ErrCannotAddTargetAsMember`).
  3. `middleware/group_access.go`: Gruba özel tüm alt rotalar için (`/api/v1/groups/:id/*`) merkezi yetkilendirme ve Anti-Spoiler koruması yazıldı. İstek yapan kişi hedef kişiyse anında `403 Forbidden` (`{"error": "Erişim engellendi: Bu hediye grubu senin için oluşturulmuş bir sürprizdir."}`) fırlatıldı; grup üyesi olmayanlar engellendi; doğrulanan grup nesnesi context'e yazıldı.
  4. `controllers/group_controller.go`: `POST /api/v1/groups`, `GET /api/v1/groups`, `GET /api/v1/groups/:id` ve `POST /api/v1/groups/:id/members` uç noktaları bağlandı. Boş grup listesi için `[]` dizi dönüşü garanti edildi.
  5. `cmd/api/main.go`: İç içe rota grubu (`groupSpecific := protected.Group("/groups/:id")`) mimarisi ile `GroupAccess` middleware'i bağlandı.
- `curl` testleriyle Faz 3 kabul kriterleri doğrulandı:
  - Grup açan kullanıcı otomatik `admin` oldu.
  - Hedef kişi (`target_user_id`) kendi gruplarını listelediğinde sürpriz grubu görmedi (`data: []`).
  - Hedef kişi doğrudan grup ID'sine istek attığında `403 Forbidden` ile engellendi.
  - Dinamik kişi başı bütçe başarıyla hesaplandı.

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
| K12 | 10-11 | E-posta sorgularında `LOWER(email)` kullanılır | Büyük/küçük harf kaynaklı mükerrer kayıtları ve giriş hatalarını önlemek |
| K13 | 10-11 | Giriş hatalarında şifre/kullanıcı ayrımı yapılmaz (`401`) | Kullanıcı sayımı (User Enumeration) saldırılarını engellemek |
| K14 | 10-11 | Token süresi 24 saat, imzalama HMAC-SHA256 (`HS256`) | Güvenli ve taşınabilir kimlik doğrulama oturumu |
| K15 | 10-11 | Grup oluşturma ve admin üyelik tek transaction içinde yapılır | Veritabanında sahipsiz/admini olmayan grup kalmasını engellemek |
| K16 | 10-11 | Anti-Spoiler iki katmanlıdır: SQL filtreleme + `GroupAccess` Middleware | Hedef kişinin grubu ne listede ne de direkt linkle görmesini engellemek |
| K17 | 10-11 | Hedef kişi gruba üye olarak eklenemez (`400 Bad Request`) | Sürpriz gizliliğini korumak |

---

## 2026-10-11 — Faz 4: Hediye Havuzu ve Oylama (+1 / -1) 🎁

**Yapılanlar**
- Hediye havuzu yönetimi, anlık skor hesaplama ve oy verme modülü geliştirildi:
  1. `repository/idea_repo.go`: `CreateIdea`, `GetIdeasWithScores`, `GetIdeaByID`, `Vote` ve `DeleteIdea` metodları yazıldı. Gruptaki hediyelerin anlık skoru `COALESCE(SUM(votes.vote_type), 0)` ve kullanıcının kendi oyu `my_vote` tek bir optimize SQL sorgusuyla çekildi. Mükerrer oyları engellemek ve mevcut oyu güncellemek için PostgreSQL Upsert (`clause.OnConflict`) uygulandı.
  2. `services/idea_service.go`: Manuel öneri ekleme (`AddManualIdea`), skorlu liste getirme (`GetGroupIdeas`), oylama (`VoteIdea`) ve yetkili silme (`DeleteIdea`) iş kuralları yazıldı. Oy tipinin sadece `+1` ve `-1` olabileceği doğrulandı. Hediyeyi sadece öneren kullanıcının veya grup admininin silebilmesi kuralı (`ErrUnauthorizedDelete`) uygulandı. `/ideas/:id/*` uç noktalarında hediyenin grubuna ulaşılarak hedef kişi için Anti-Spoiler koruması (`ErrAntiSpoilerTriggered`) sağlandı.
  3. `controllers/idea_controller.go`: `POST /api/v1/groups/:id/ideas`, `GET /api/v1/groups/:id/ideas`, `POST /api/v1/ideas/:id/vote` ve `DELETE /api/v1/ideas/:id` uç noktaları bağlandı.
  4. `cmd/api/main.go`: Hediye ve oylama rotaları korumalı rota ağacına entegre edildi.
- `curl` testleriyle Faz 4 kabul kriterleri uçtan uca doğrulandı:
  - Manuel hediye başarıyla eklendi (`201 Created`).
  - İlk oyda (`+1`) skor `1` ve `my_vote: 1` oldu.
  - İkinci oyda (`-1`) kayıt çoğalmadı; var olan oy güncellendi ve skor anında `-1`, `my_vote: -1` oldu (Upsert doğrulandı).
  - Hedef kişi hediye havuzunu listelemeye veya hediyeye oy vermeye çalıştığında `403 Forbidden` aldı.
  - Yetkisiz silme denemesi `403 Forbidden` ile engellendi; kurucu/admin başarıyla sildi (`200 OK`).

---

## 2026-10-12 — Faz 6: Gemini AI Hediye Önerileri, Admin & Demokrasi Vetosu 🤖

**Yapılanlar**
- Google GenAI SDK (`google.golang.org/genai v1.73.0`) projeye entegre edildi:
  1. `models.GiftIdea`: `Status string gorm:"type:varchar(20);not null;default:'ACTIVE'"` alanı eklendi (`'ACTIVE'`, `'REJECTED'`, `'SELECTED'`).
  2. `pkg/gemini/client.go`: `gemini-3.8-flash` modeli ile Structured JSON Schema çıktısı yapılandırıldı (`title`, `description`, `estimated_price`, `search_query`). Kural 3 (Sıfır Kırık Link Kuralı) ile URL üretimi yasaklandı; sadece e-ticaret siteleri için arama terimi alındı. Geçici 503 yüksek talep hatalarında `gemini-3.5-flash-lite` modeline esnek geri çekilme (Resilient Fallback) mekanizması kuruldu.
  3. `repository/idea_repo.go`: `CreateIdeasBatch`, `UpdateStatus`, `GetGroupIdeaTitlesByStatus` ve `GetDownvoteCount` metodları eklendi.
  4. `services/idea_service.go`:
     - **Admin Vetosu (`RejectIdea`):** Sadece grup yöneticisi tek tıkla hediyeyi veto edebilir (`status = 'REJECTED'`). Normal üyeler engellenir (`403`).
     - **Demokrasi Vetosu (`VoteIdea`):** Grup üyelerinin çoğunluğu bir hediyeye downvote verirse ($downvotes > N/2$), sistem hediyeyi otomatik olarak `REJECTED` durumuna çeker.
     - **State-Based Context Prompt (`GenerateAIIdeas`):** Hedef kişinin adı, hobileri (`target_interests`), bütçesi, havuzdaki mevcut hediyeler ve **reddedilen/veto edilen hediyeler** dinamik prompt'a dönüştürülür. Gemini'ye *"Şu hediyeleri grup reddetti, kesinlikle bunlara benzer bir şey önerme"* uyarısı verilir.
     - **Anti-Spoiler Kalkanı:** Hedef kişi `/generate-ai` isteği attığında `403 Forbidden` ile sürpriz gizliliği korunur.
     - **Sunucu Tarafı Bütçe Filtresi:** Toplam bütçeyi aşan öneriler backend seviyesinde elenir.
  5. `controllers/idea_controller.go`: `POST /api/v1/groups/:id/ideas/generate-ai` ve `POST /api/v1/ideas/:id/reject` uç noktaları bağlandı.
  6. `cmd/api/main.go`: Gemini istemcisi Fail-Safe yaklaşımla (`GEMINI_API_KEY` kontrolü ile) bağımlılık enjeksiyonuna bağlandı.
- Otomatik test scripti (`scratch/test_faz6.sh`) ile tüm uç noktalar test edildi:
  - Anti-spoiler kalkanı hedef kişinin AI öneri isteğini başarıyla 403 ile engelledi.
  - Gemini 3.8 Flash hedef kişinin ilgi alanlarına (kamp, kahve demleme, trekking) nokta atışı uygun 3 hediye üretti.
  - Üyenin veto denemesi 403 ile engellendi; admin vetosuyla hediye başarıyla `REJECTED` oldu.
  - Demokrasi vetosuyla 2 downvote alan hediye anında `REJECTED` oldu.
  - İkinci AI çağrısında reddedilen hediye türleri ve mevcut havuzdaki hediyeler prompt negatif kısıtı sayesinde asla tekrarlanmadı; tamamen yeni ve orijinal öneriler üretildi.

---

## 2026-10-12 — Faz 5: Ürün Linki Önizleme (`pkg/linkpreview`) & Anti-Spoiler Koruması 🔗

**Gerekçe**
- Kullanıcı hediye havuzuna manuel bir ürün önerisi eklerken pazar yeri bağlantısını (Trendyol, Amazon TR, Hepsiburada) yapıştırdığında; ürün başlığını, yüksek çözünürlüklü görselini ve anlık fiyatını otomatik çekerek form doldurma zahmetini ortadan kaldırmak (Kural 5).

**Yapılanlar**
1. **Modüler Extractor Mimarisi (`backend/pkg/linkpreview/`):**
   - `Extractor` arayüzü (`Name`, `Hosts`, `Refine`) ile site ayrıştırıcıları bağımsız dosyalara bölündü:
     - `preview.go`: `Preview` modeli, `Extractor` arayüzü, `Fetch(ctx, url)` ve `finalize` normalizasyon akışı.
     - `fetch.go`: SSRF korumalı HTTP istemcisi (Allowlist + dial-seviyesi `blockPrivateIPs` ile loopback/yerel IP engeli), WhatsApp User-Agent, çerez kavanozu (`cookiejar`), 2 MB limit, 10 redirect ve Adjust (`adj.st`) takip çözümlemesi (`adjust_redirect`).
     - `extract.go`: Attribute-bazlı meta parser (çift/tek tırnak ve `&#39;` çözümü), HTML entity decode (`html.UnescapeString`), çok katmanlı JSON-LD parser (`@graph`, Product, Offer), fiyat normalizasyonu, jenerik reklam görseli filtresi ve `buildSearchQuery`.
     - `generic.go`: Temel katman fallback (OG etiketleri → JSON-LD → `<title>`).
     - `trendyol.go`: `WebPage.name` temiz başlık, gömülü state JSON'undan (`discountedPrice`) indirimli fiyat tespiti.
     - `amazon.go`: Görsel galerisindeki `hiRes` 1500px fotoğrafı çekme (puan basılmış og:image kartını eleme), buybox JSON'undan `priceAmount` tespiti.
     - `hepsiburada.go`: Akamai koruması için otomatik gzip/Accept-Encoding uyumu, JSON-LD @graph Product/Offer verisi ve 1500px JPG görsel yükseltmesi (`upgradeHBImage`).
2. **Birim Testleri (`linkpreview_test.go`):**
   - Ağ gerektirmeyen offline testler: SSRF URL doğrulaması, private IP engeli (AWS metadata 169.254, 127.0.0.1, IPv6 loopback), fiyat normalizasyonları, search query temizleme, HTML entity decode ve site refine akışları %100 başarıyla geçti.
3. **API Entegrasyonu & Anti-Spoiler Güvenliği:**
   - `POST /api/v1/groups/:id/ideas/preview-link` uç noktası `groupSpecific` grubuna eklendi (`middleware.GroupAccess`).
   - Anti-Spoiler Kalkanı: Hedef kişi link önizleme isteği attığında anında `403 Forbidden` aldı.
   - Hata Toleransı: Geçersiz URL veya SSRF engellerinde backend çökmeyip `partial: true` dönerek kullanıcının formu manuel doldurmasına olanak sağladı.

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
| K12 | 10-11 | E-posta sorgularında `LOWER(email)` kullanılır | Büyük/küçük harf kaynaklı mükerrer kayıtları ve giriş hatalarını önlemek |
| K13 | 10-11 | Giriş hatalarında şifre/kullanıcı ayrımı yapılmaz (`401`) | Kullanıcı sayımı (User Enumeration) saldırılarını engellemek |
| K14 | 10-11 | Token süresi 24 saat, imzalama HMAC-SHA256 (`HS256`) | Güvenli ve taşınabilir kimlik doğrulama oturumu |
| K15 | 10-11 | Grup oluşturma ve admin üyelik tek transaction içinde yapılır | Veritabanında sahipsiz/admini olmayan grup kalmasını engellemek |
| K16 | 10-11 | Anti-Spoiler iki katmanlıdır: SQL filtreleme + `GroupAccess` Middleware | Hedef kişinin grubu ne listede ne de direkt linkle görmesini engellemek |
| K17 | 10-11 | Hedef kişi gruba üye olarak eklenemez (`400 Bad Request`) | Sürpriz gizliliğini korumak |
| K18 | 10-11 | Oylamada GORM `clause.OnConflict` (Upsert) kullanılır | Aynı kullanıcının mükerrer oy atmasını önlemek ve oyunu güncellemesini sağlamak |
| K19 | 10-11 | Hediye silme yetkisi sadece öneren kişi ve grup adminine aittir | Hediye havuzunun bütünlüğünü korumak |
| K20 | 10-11 | `/ideas/:id/*` uçlarında grubun hedef kişisi izole edilir | Grup dışındaki bağımsız uçlarda da Anti-Spoiler garantisi |
| K21 | 10-12 | `Group.target_interests` alanı eklendi | Gemini AI hediye üretiminde hedef kişinin hobilerini ve ilgi alanlarını semantik girdi olarak kullanmak |
| K22 | 10-12 | `GiftIdea.status` alanı (`ACTIVE`, `REJECTED`, `SELECTED`) eklendi | Reddedilen fikirleri fiziksel olarak silmeyip AI prompt'una negatif kısıt olarak beslemek |
| K23 | 10-12 | Model olarak `gemini-3.8-flash` + `gemini-3.5-flash-lite` fallback kullanıldı | Güncel Google GenAI mimarisi ve yüksek talep (503) anlarında kesintisiz çalışma |
| K24 | 10-12 | Admin Veto (`POST /ideas/:id/reject`) ve Demokrasi Vetosu ($downvotes > N/2$) ayrımı yapıldı | Grup yöneticisinin karar hakkı ile topluluk oylaması dengesi |
| K25 | 10-12 | Bütçe üstü öneriler sunucu tarafında çöpe atılmaz; `is_over_budget: true` bayrağı ile işaretlenir | AI piyasa fiyatını tam bilmeyebilir veya grup bütçe artışını tolere edebilir; nihai karar gruba bırakılır |
| K26 | 10-12 | SSRF denetimi DNS çözümleme anında (dial seviyesinde) yapılır | DNS Rebinding saldırılarını ve yerel IP aralıklarını (Loopback, AWS metadata vb.) engellemek |
| K27 | 10-12 | Hepsiburada Akamai engeli için `Accept-Encoding` elle set edilmez | Go'nun şeffaf gzip çözümlemesini korumak ve 403 engeline takılmamak |
| K28 | 10-12 | Amazon için `og:image` yerine `hiRes` ürün galerisi görseli alınır | `og:image` puan/yıldız basılmış sosyal medya kartı olduğu için gerçek ürün fotoğrafı değildir |
| K29 | 10-12 | Önizleme hatalarında HTTP 500 dönülmez, `partial: true` ile 200 dönülür | Frontend'in çökmesini engellemek ve kullanıcının formu elle doldurmasına izin vermek |
| K30 | 10-13 | Swagger/OpenAPI dokümantasyonu kod içi bildirimlerle (`swaggo/swag`) üretilir ve `/swagger/index.html` üzerinden sunulur | Kod ve dokümantasyon senkronizasyonunu sağlamak, API sözleşmesini canlı interaktif arayüzle test edilebilir kılmak |
| K31 | 10-13 | Kullanıcı hesap silme (`DELETE /api/v1/me`) atomik transaction ile bağlı verileri (oylar, oluşturulan gruplar, fikirler) temizler | Veritabanı tutarlılığı sağlamak, sahipsiz grup kalmasını önlemek ve KVKK/GDPR uyumlu hesap kapatma sunmak |


---

## 2026-10-12 — Grup Modeline `target_interests` Alanının Eklenmesi 🎁

**Gerekçe**
- Faz 6'da Gemini AI ile semantik hediye önerileri üretirken, sadece grup başlığı ve açıklaması hedef kişinin zevklerini yansıtmakta yetersiz kalıyordu. Kural 4 gereğince yapay zekânın isabetli öneriler yapabilmesi için hedef kişinin ilgi alanları ve hobilerinin modele dahil edilmesi kararlaştırıldı.

**Yapılanlar**
- `models.Group`: `TargetInterests string gorm:"type:text" json:"target_interests,omitempty"` alanı eklendi. GORM AutoMigrate veriyi kaybetmeden PostgreSQL tablosuna sütunu ekledi.
- `services.GroupService`: `CreateGroup` fonksiyonuna `targetInterests` parametresi eklendi.
- `controllers.GroupController`: `CreateGroupRequest` DTO'suna `target_interests` alanı eklendi.
- `PROJECT_CONTEXT.md` ve `README.md` güncellendi.
- `backend` sıfır hatayla derlendi (`go build ./...`).

---

## 2026-10-13 — Swagger / OpenAPI 2.0 Entegrasyonu 📖

**Gerekçe**
- Projenin akademik jüri ve hocaya sunumunda, tüm REST API uç noktalarının, DTO modellerinin, hata/başarı durumlarının ve JWT Bearer kimlik doğrulamasının interaktif bir arayüz üzerinden incelenebilmesi ve test edilebilmesi talep edildi.

**Yapılanlar**
- `swaggo/swag` ve `swaggo/gin-swagger` paketleri projeye dahil edildi (`backend/go.mod`).
- `backend/cmd/api/main.go` içine genel API meta bilgileri (`@title`, `@version`, `@BasePath /`, `@securityDefinitions.apikey BearerAuth`) ve `/health` dökümantasyonu eklendi.
- Kontrolör katmanındaki tüm handler fonksiyonlarına Swagger bildirimleri (Summary, Description, Tags, Param, Success, Failure, Router) eklendi:
  - `AuthController`: `/api/v1/auth/register`, `/api/v1/auth/login`, `/api/v1/me`
  - `GroupController`: `/api/v1/groups` (POST, GET), `/api/v1/groups/{id}` (GET), `/api/v1/groups/{id}/members` (POST)
  - `IdeaController`: `/api/v1/groups/{id}/ideas` (POST, GET), `/api/v1/groups/{id}/ideas/preview-link` (POST), `/api/v1/groups/{id}/ideas/generate-ai` (POST), `/api/v1/ideas/{id}/vote` (POST), `/api/v1/ideas/{id}/reject` (POST), `/api/v1/ideas/{id}` (DELETE)
- Standart JSON yanıt formatını (`controllers.Response`) Swagger ile şematize etmek için `controllers/response.go` içinde generic ve data tipleri modellendi.
- `swag init` çalıştırılarak `backend/docs/` dizini altında `docs.go`, `swagger.json` ve `swagger.yaml` üretildi.
- Gin üzerinde `/swagger/*any` rotası ile Swagger UI bağlandı; `/swagger` ve `/docs` için kalıcı 301 yönlendirmesi eklendi.
- `scripts/demo_presentation.sh` ve `README.md` dosyalarına Swagger UI bağlantıları eklendi.
- Tüm birim testleri başarıyla geçti (`go test -v ./...`), canlıda `http://localhost:8080/swagger/index.html` üzerinden 200 OK ile doğrulandı.

---

## 2026-10-13 — Kullanıcı Profil & Hesap Silme Özelliği (`DELETE /api/v1/me`) 👤❌

**Gerekçe**
- Kullanıcıların kendi profillerini kalıcı olarak silebilmesi (KVKK / GDPR "unutulma hakkı" ve hesap kapatma) talep edildi.
- Bir kullanıcının silinmesi durumunda sahipsiz grup kalmaması, yabancı anahtar (foreign key `RESTRICT`) kısıtlarına takılınmaması ve hedef kişi olduğu sürpriz grupların bozulmaması için çok katmanlı atomik temizlik kurgulandı.

**Yapılanlar**
- `repository.UserRepository`: `Delete(id uuid.UUID) error` metodu eklendi.
  - GORM `Transaction` içinde:
    1. Kullanıcının sürpriz hedefi olduğu gruplarda `target_user_id = NULL` yapılır.
    2. Kullanıcının verdiği tüm oylar (`votes`) silinir.
    3. Kullanıcının önerdiği tüm fikirler (`gift_ideas`) ve üzerindeki oylar silinir.
    4. Kullanıcının kurucusu olduğu gruplar (`groups`), bağlı tüm fikirler, oylar ve üyelikler silinir.
    5. Kullanıcının diğer gruplardaki üyelikleri (`group_members`) silinir.
    6. `users` tablosundan kullanıcı kaydı silinir.
- `services.AuthService`: `DeleteUser(id uuid.UUID) error` metodu tanımlandı ve implemente edildi.
- `controllers.AuthController`: `DeleteMe(c *gin.Context)` fonksiyonu eklendi; Swagger bildirimleri yazıldı.
- `main.go`: `protected.DELETE("/me", authController.DeleteMe)` rotası kaydedildi.
- `swag init` ile Swagger dokümantasyonu güncellendi (`DELETE /api/v1/me`).
- Canlıda test edildi: Kullanıcı kaydoldu ➔ grup açtı ➔ profilini sildi (200 OK) ➔ tekrar `/me` sorguladığında `404 Not Found`, giriş yapmayı denediğinde `401 Unauthorized` aldığı doğrulandı.

