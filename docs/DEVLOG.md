# 📓 CovertGift — Geliştirme Günlüğü (DEVLOG)

> Bu dosya **ne yaptığımızı** ve **neden öyle yaptığımızı** kayıt altında tutar.
> Kural: Her anlamlı adım / karar sonrası en üste değil, **kronolojik olarak en alta** yeni bir kayıt eklenir.
> Kararlar ayrıca **Karar Kaydı** tablosunda özetlenir.

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
| K32 | 10-13 | Grup silme (`DELETE /api/v1/groups/:id`) sadece grup yöneticisine (admin) aittir; bağlı tüm fikirler, oylar ve üyelikler atomik transaction ile temizlenir | Sahipsiz kayıt kalmasını önlemek ve grup kapatma/iptal hakkını kurucuya vermek |
| K33 | 10-13 | Test ve sunum amaçlı veritabanı sıfırlama (`POST /api/v1/dev/reset-db`) Swagger'a eklendi; `APP_ENV=production` ise devre dışıdır | Canlı sunumlarda ve otomatik testlerde veritabanını tek tıkla fabrika ayarlarına döndürebilmek |
| K34 | 10-16 | Geçici test paneli bağımlılıksız tutulur; kalıcı React frontend Faz 7'de ayrı geliştirilir | Backend'i erken deneyebilmek ve prototipi tamamlanmış frontend saymamak |
| K35 | 10-16 | Cloudflare tüneli 5173 portundaki sınırlı proxy'ye bağlanır | Panel/API aynı origin'i kullanır; geliştirme sıfırlama ucu ve proje dosyaları paylaşılmaz |
| K36 | 10-16 | İlgi alanlarına tüm üyeler tek tek ekler; yalnızca admin siler, yazımlar satır kilidiyle korunur | Üyelerin katkısını korumak ve eşzamanlı güncellemelerde veri kaybını engellemek |
| K37 | 10-16 | Bütçeyi yalnızca admin günceller; mevcut hediye uyarıları transaction'da ve listelemede güncellenir | Toplam/kişi başı bütçeyle hediye uyarılarının tutarlı kalması |
| K38 | 10-17 | AI ürün araştırması Google Search grounding ve ayrı JSON seçimiyle yapılır; kaynak adresleri metadata'dan alınır | Kaynaksız ürün/fiyatları elemek ve modelin URL üretmesini önlemek |
| K39 | 11-09 | `reset-db` varsayılan kapalı: `ALLOW_DB_RESET=true` + gövdede `{"confirm":"RESET"}` gerekir; production her durumda 403 | Yerel verinin yanlışlıkla atılan tek bir POST ile silinmesini önlemek |
| K40 | 11-09 | Entegrasyon testleri gerçek `newRouter` ile ayrı, geçici `test-db`ye (port 55432, tmpfs) karşı çalışır; `TEST_DATABASE_URL` yoksa atlanır, adı `_test` ile bitmeyen DB reddedilir | Gerçek middleware zincirini sınamak ve testlerin gerçek veriyi silmesini imkânsız kılmak |
| K41 | 11-09 | API CORS başlığı göndermez; frontend her ortamda aynı origin (Vite proxy / nginx) üzerinden erişir. Kullanılmayan `CORS_ORIGIN` kaldırıldı | Gereksiz bir izin yüzeyi açmamak; başka sitelerin tarayıcıdan API çağırmasını varsayılan olarak engelli tutmak |
| K42 | 11-09 | Loglama `log/slog` ile; 500 yanıtlarında iç hata yalnızca loga yazılır (`RespondInternal`) | Logları sorgulanabilir yapmak; veritabanı/sağlayıcı ayrıntılarının istemciye sızmasını önlemek |
| K43 | 11-09 | Seed yalnızca ekler, tek transaction'dır, demo yöneticisi varsa hiçbir şey yapmaz ve production'da çalışmaz | Seed komutunun gerçek veriyi silmesini veya yarım veri bırakmasını imkânsız kılmak |
| K44 | 11-09 | Kazanan seçimi grubu `COMPLETED` yapar ve fikir/oy işlemlerini 409 ile kapatır; admin yeniden açabilir | Kararı kesinleştirmek ama yanlış tıklamayı geri alınabilir tutmak |
| K45 | 11-09 | Hediye tarihi tek seferlik `DATE`; geçmiş tarih (1 yıla kadar) kabul; "bugün" UTC+3; bildirim yok | Basit tutmak; grup doğum gününden sonra da kapanmamış olabilir; konteynerde tz veritabanına bağımlı olmamak |
| K46 | 11-09 | Oyu geri almak çoğunluk vetosunu geri çevirmez | REJECTED admin vetosuyla da oluşabildiği için ayırt edilemez |
| K47 | 11-09 | Canlılık için önce 10 sn periyodik yenileme (TanStack Query); bir fikir işlemi sürerken durur; SSE sonraya | En az kodla çoğu ihtiyacı karşılamak; iyimser güncellemeyi eski yanıtla ezmemek |
| K48 | 11-09 | Canlı güncelleme SSE ile: olay yalnızca "changed" der, veri her zamanki korumalı uçlardan çekilir; akış 5 dk'da bir yenilenir; token başlıkta (fetch), URL'de değil | Olay kanalından veri sızmasını imkânsız kılmak; yetkiyi düzenli yeniden denetlemek; token'ı loglara/geçmişe düşürmemek |
| K49 | 11-09 | Bir grupta birden çok kazanan: admin 1–20 aktif fikri tek seferde seçer; seçim penceresi toplamı bütçeyle karşılaştırır, aşımda uyarır ama engellemez | Ortak hediye birden çok parçadan oluşabilir; bütçe aşımı bilinçli bir karar olabilir |
| K50 | 11-12 | Bildirimler kalıcı (`notifications` tablosu) ve kullanıcı bazlı SSE ile anlık; olay veri taşımaz, metin istemcide `type` + `params` ile üretilir | Grup akışıyla aynı güvenlik modeli: veri yalnızca Anti-Spoiler süzgeçli `GET /notifications` ucundan çıkar; TR/EN metin sunucuya gömülmez |
| K51 | 11-12 | Bildirim olayını handler bildirir (`RecordNotification`), üretimi rota grubuna bağlı `middleware.Notify` → `NotificationService.Dispatch` yapar; her değişiklik rotası testte "bildirir / bildirmez" olarak sınıflandırılır | Rota+metot eşlemesi olayın ayrıntısını (kim eklendi, değer değişti mi) bilemez; servis katmanı isteğin başarıyla bittiğini bilemez. Alıcı ve Anti-Spoiler kuralı tek yerde kalır, unutulan rota testi kırar |
| K52 | 11-12 | Hatırlatmalar API içi saatlik zamanlayıcıyla; kullanıcı + `dedupe_key` (tarihi içerir) benzersiz; gün sayısına aralıkla bakılır | Ayrı iş kuyruğu/cron gerektirmez; API uykudan uyanınca kaçan hatırlatma yakalanır, tarih değişince yeniden hatırlatılır, çift üretim olmaz |
| K53 | 11-13 | Kapak baytları ayrı `group_covers` tablosunda (bytea); `groups.cover_version` sürüm kopyası, aynı transaction'da güncellenir | Grup/liste sorguları ve `GroupAccess` kapak baytlarına dokunmaz; ek sorgu/JOIN olmadan sürüm yanıtta gelir |
| K54 | 11-13 | Kapağı varsayılan olarak tüm üyeler değiştirir; admin `cover_admin_only` ile kısıtlar; izin kayıt anında kilitli satırla yeniden denetlenir | Ortak grup hissi; politika değişikliğiyle yarışan yükleme eski izinle geçemez |
| K55 | 11-13 | Kapak işleme saf Go (`pkg/imageproc`): tür bayt imzasından, EXIF yönü, 4:3 kırpma, en çok 1600x1200 JPEG, eşzamanlılık 2 | Harici araç/cgo yok; metadata (konum dahil) atılır; bellek sınırlı |
| K56 | 11-13 | Kapak yanıtı `ETag` + `Cache-Control: private, max-age=0, must-revalidate`, `Vary: Authorization` | Süreli önbellek, aynı tarayıcıdaki başka kullanıcıya (ör. sonradan bağlanan hedef) görseli yetki sorulmadan verebilirdi |
| K57 | 11-13 | Kırpma arayüzü bağımlılıksız (`CoverCropDialog`, `coverCrop.ts`); görsel `fetch` + Bearer ile Blob olarak alınır | Yeni paket eklememek; token URL'ye yazılmasın |
| K58 | 11-13 | Grup açıklamasını yalnızca admin `PATCH /groups/:id/description` ile değiştirir; bildirim üretmez | Bütçe/tarih uçlarıyla aynı yetki deseni |
| K59 | 11-13 | Açıklama sınırı oluşturma ve düzenlemede ortak: kırpılmış en çok 1000 rune | İki uçta farklı kural (2000 / 1000) kalmasın |
| K60 | 11-13 | `Group.TargetUser` hiçbir yanıtta yok (`json:"-"`); detay yalnızca üyelere `target_account: {id, name}` döner | Grup detayı hedefin e-postasını üyelere sızdırıyordu (Anti-Spoiler / gizlilik) |
| K61 | 11-13 | Başlıkta hedef hesap durumu her zaman metinle yazılır ("{ad} hesabı bağlı" / "Hesap bağlı değil"); yeni renk token'ı yok | Küçük rozet gözden kaçıyordu; durum yalnızca renge bağlı olmasın |
| K62 | 11-13 | Kapak görselini sunucu yalnızca küçükse işler: en çok 2 MiB, uzun kenar 2000 piksel, 4 MP; sınır `DecodeConfig` ile çözmeden denetlenir | API 1 GiB bellekle çalışıyor; 50 MP görselin tam çözümü birkaç yüz MB tepe bellek demekti. Arayüz zaten en çok 1600x1200 gönderiyor |
| K63 | 11-13 | Kapak gövde sınırı 2 MiB + 64 KiB, nginx kapak bloğu 3m | Dosya sınırı küçüldü; büyük gövde okunmadan 413 alır |
| K64 | 11-13 | Büyük fotoğrafı tarayıcı küçültür (`createImageBitmap` resize, 3200 piksellik çalışma kopyası, çıktı ≤ 2 MiB için kalite düşürme); seçim sınırı 40 MB / 100 MP | Kullanıcı 48 MP telefon fotoğrafını yine seçebilir; işleme maliyeti istemcide kalır |
| K65 | 11-13 | Canlı dağıtım kontrolleri yalnızca okumadır ve `/api/v1/dev/*` canlıda hiçbir yöntemle istenmez; nginx engeli yalnızca yerelde ölü `API_TARGET` ile doğrulanır | 2026-11-13 kontrol listesine yanlışlıkla bir `/api/v1/DEV/reset-db` GET'i girdi; nginx 403 ile durdurdu, ama güvence bir katmana kalmamalı |


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

---

## 2026-10-13 — Hediye Grubu Silme Özelliği (`DELETE /api/v1/groups/:id`) 👥❌

**Gerekçe**
- Grup yöneticisinin (kurucusunun) organizasyondan vazgeçmesi, kutlamanın tamamlanması veya grubun iptal edilmesi durumunda grubu ve bağlı tüm verileri sistemden silebilmesi talep edildi.
- Anti-Spoiler ve Rol Güvenliği (RBAC): Sadece `admin` rolüne sahip grup kurucusu grubu silebilir. Normal üyeler veya hedef kişi silme isteği atarsa `403 Forbidden` ile engellenir.

**Yapılanlar**
- `repository.GroupRepository`: `DeleteGroup(groupID uuid.UUID) error` metodu eklendi.
  - GORM `Transaction` içinde:
    1. Gruba ait hediye fikirlerinin ID'leri toplanır.
    2. Fikirlere verilen tüm oylar (`votes`) silinir.
    3. Fikirler (`gift_ideas`) silinir.
    4. Grubun üyeleri (`group_members`) silinir.
    5. `groups` tablosundan grup kaydı silinir.
- `services.GroupService`: `DeleteGroup(groupID, userID uuid.UUID) error` metodu eklendi.
  - Anti-Spoiler denetimi: Hedef kişi grubu silemez (`403 Forbidden`).
  - Rol denetimi: İstek atan kullanıcı grupta `admin` değilse işlem engellenir (`403 Forbidden`).
- `controllers.GroupController`: `DeleteGroup(c *gin.Context)` handler'ı ve Swagger açıklamaları yazıldı.
- `main.go`: `groupSpecific.DELETE("", groupController.DeleteGroup)` rotası bağlandı (`GroupAccess` middleware'i altında).
- `swag init` ile Swagger şeması güncellendi (`DELETE /api/v1/groups/{id}`).
- Canlıda test edildi: Normal üye silmeyi denedi ➔ `403 Forbidden`; Hedef kişi denedi ➔ `403 Forbidden`; Admin sildi ➔ `200 OK`; ardından gruba erişilmeye çalışıldığında `404 Not Found` alındı.

---

## 2026-10-13 — Veritabanı Sıfırlama Butonu & Endpoint'i (`POST /api/v1/dev/reset-db`) 🔄💥

**Gerekçe**
- Swagger arayüzü ve demo sunumlarında test yaparken veritabanında kalan eski kayıtları (kullanıcılar, gruplar vb.) tek tek elle silmek yerine tek tıkla veritabanını fabrika ayarlarına döndürecek bir geliştirici butonu talep edildi.
- Güvenlik: Canlı üretim (`APP_ENV=production`) ortamında yanlışlıkla çalıştırılmasını engellemek için `403 Forbidden` koruması eklendi.

**Yapılanlar**
- `backend/internal/config`: `AppEnv` (`development`, `production`) alanı eklendi.
- `backend/internal/controllers/dev_controller.go`: `DevController` oluşturuldu. `ResetDB` metodu tek bir atomik transaction içinde `TRUNCATE TABLE votes, gift_ideas, group_members, groups, users RESTART IDENTITY CASCADE;` komutunu çalıştırır.
- `main.go`: `POST /api/v1/dev/reset-db` rotası bağlandı.
- `swag init` ile Swagger dokümantasyonuna **"Sistem & Test"** başlığı altında tek tıkla çalışan buton olarak eklendi.
- Canlıda test edildi: `curl -X POST http://localhost:8080/api/v1/dev/reset-db` çağrıldığında `200 OK` dönüp tüm tabloların milisaniyeler içinde sıfırlandığı doğrulandı.

---

## 2026-10-16 — Geçici Backend Test Arayüzü

**Gerekçe**
- Kalıcı, mobil web öncelikli Faz 7 arayüzünden önce API'yi tarayıcı üzerinden elle deneyebilmek istendi. Kalıcı frontend'e daha sonra sıfırdan başlanacak.

**Yapılanlar**
- `prototypes/test-ui/` altında bağımlılıksız HTML/CSS/JavaScript paneli ve Node.js standart kütüphanesiyle yerel statik sunucu/API proxy'si eklendi.
- Kayıt/giriş, profil, grup oluşturma/listeleme, ID ile grup erişimi, üye ekleme, bütçe özeti, manuel hediye/link önizleme, AI üretimi, oy, veto ve onaylı silme işlemleri bağlandı.
- HTTP durum kodu ve JSON yanıt günlüğü eklendi; token maskelenir, şifreler loglanmaz. Oturum sekmeye özel `sessionStorage` içinde tutulur.
- Backend CORS değişikliği gerektirmemek için panel sunucusu API isteklerini `127.0.0.1:8080` adresine iletir. Mevcut backend kaynak dosyaları değiştirilmedi.
- Çalıştırma: proje kökünde `node prototypes/test-ui/server.mjs`; tarayıcı: `http://127.0.0.1:5173`.
- Faz 7 görevleri tamamlandı olarak işaretlenmedi; geçici panel ayrı tutuldu.
- Doğrulama: JavaScript sözdizimi kontrolü; tarayıcıda kayıt/giriş, grup ve manuel hediye oluşturma, oy değiştirme, reddedilen filtresi ve kısmi önizleme; canlı API/proxy üzerinden üyeler, bütçe, Anti-Spoiler, oy upsert, veto ve silme yetkileri. 390 px mobil genişlikte yatay taşma görülmedi. Oluşturulan sentetik test hesapları ve grupları temizlendi. Gerçek Gemini üretim çağrısı yapılmadı.

---

## 2026-10-16 — Cloudflare ile Geçici Canlı Test Bağlantısı

- Kullanıcının isteğiyle `cloudflared` Homebrew üzerinden kuruldu ve `http://127.0.0.1:5173` için Quick Tunnel açıldı. Kalıcı alan adı veya sistem servisi oluşturulmadı.
- Tünel doğrudan backend'e değil, arayüzün API proxy'sine bağlandı. Proxy yalnızca panelin kullandığı API yollarını iletecek şekilde sınırlandı; giriş gerektirmeyen `/api/v1/dev/reset-db` ve diğer geliştirme uçları proxy'de engellendi.
- Kullanım komutu ve tünelin bilgisayar/süreçler açıkken çalışacağı `prototypes/test-ui/README.md` dosyasına eklendi.
- Canlı HTTPS adresinde sayfa 200, `/health` 200 (`db: up`), tokensız `/api/v1/me` 401 ve geliştirme sıfırlama yolu 403 ile doğrulandı. Tarayıcıda “API ve DB bağlı” durumu görüldü.

---

## 2026-10-16 — İlgi Alanlarını Sonradan Düzenleme

- `PATCH /api/v1/groups/:id/interests` eklendi. `{ "target_interests": "..." }` gövdesi yalnızca grubun ilgi alanlarını değiştirir. Boş metin alanı temizler; eksik/null veya metin dışı değer 400 döner.
- Kullanıcının isteğiyle şimdilik admin dahil tüm grup üyeleri düzenleyebilir. JWT, üyelik ve Anti-Spoiler kontrolleri korunur; hedef kişi ve grup dışı kullanıcı 403 alır.
- Geçici panelin grup detayına mevcut değeri gösteren düzenleme formu eklendi. Kaydetme sonrasında detay ve grup listesi yenilenir. Sonraki AI çağrısı veritabanındaki güncel ilgi alanlarını kullanır.
- Proxy allowlist'i, Swagger şemaları, teknik şartname ve roadmap güncellendi. API ve panel aynı Cloudflare bağlantısında yeniden başlatıldı.
- `go test ./...` geçti. Yeni servis testleri normal üye/admin, boşaltma, hedef kişi, grup dışı kullanıcı, bulunamayan grup ve veritabanı hatalarını kapsar. Canlı HTTPS üzerinden kalıcılık, diğer grup alanlarının korunması, 400/401/403/404 durumları ve Swagger doğrulandı; tarayıcıda düzenleme/kaydetme denendi. Sentetik test hesapları ve grupları temizlendi.

---

## 2026-10-16 — İlgi Alanları Listesi: Üyeler Ekler, Admin Siler

- Kullanıcının yeni isteğiyle toplu metin düzenleme yerine tek tek eklenen listeye geçildi. `POST /api/v1/groups/:id/interests` tüm üyelere ekleme, `DELETE` yalnızca admine silme yetkisi verir. Her iki istek de `{ "interest": "Kamp" }` gövdesi alır. Normal üye kendi eklediğini de silemez.
- Önceki `PATCH` endpoint'i kaldırıldı; üye listeyi değiştirip silme kuralını aşamaz. JWT, üyelik ve Anti-Spoiler korunur.
- Grup detayı `interests: string[]` döner. Mevcut `target_interests` TEXT sütunu korunur; eski virgül/noktalı virgül/satır sonu ayrımlı değerler okunur. Yeni yazımlar satır sonuyla ayrılır. Mevcut veriye şema migrasyonu uygulanmadı; Gemini güncel listeyi metin olarak kullanmaya devam eder.
- Ekleme tek öğe (1–100 karakter) kabul eder; boş ve tekrarlanan öğeler engellenir. Güncellemeler transaction içinde satır kilidiyle yapılır; eşzamanlı katkılar birbirini ezmez.
- Geçici panelde etiketler, tek giriş alanı ve “Ekle” butonu bulunur. Etiketlerdeki “Sil” butonu yalnızca admine gösterilir; yetki API'de ayrıca kontrol edilir.
- Doğrulama: `go test ./...`, JavaScript sözdizimi ve Swagger üretimi geçti. Canlı HTTPS testlerinde eski veri okuma, ekleme, admin/üye silme yetkileri, kaldırılan PATCH, 10 eşzamanlı eklemenin korunması, listeyi son öğeye kadar silme ve 400/401/403/404/409 durumları kontrol edildi. Tarayıcıda üye ekledi, admin sildi; üyede silme butonu yoktu. 390 px mobil genişlikte yatay taşma olmadı. Sentetik test verileri temizlendi.

---

## 2026-10-16 — Grup Bütçesini Güncelleme

- `PATCH /api/v1/groups/:id/budget` eklendi; `{ "total_budget": "6000.00" }` gövdesi alır. Düzenleme grup adminine aittir; normal üye, hedef kişi ve grup dışı kullanıcı 403 alır. JWT ve Anti-Spoiler kontrolleri korunur.
- Tutar 0–99999999.99 aralığında, en fazla iki ondalık basamaklı olmalıdır. Eksik/null, geçersiz, negatif ve veritabanı sınırını aşan tutarlar 400 döner. Para işlemleri `decimal.Decimal` ile yapılır.
- Grup bütçesi ve mevcut hediyelerin `is_over_budget` bayrakları tek transaction'da güncellenir. Listelemede de güncel bütçeden hesaplanır; eski bütçeyle başlamış eşzamanlı hediye eklemeleri arayüzde eski uyarı göstermez. Önceki kuralla tutarlı olarak 0 bütçe aşımı uyarılarını kapatır.
- Geçici panelde admin için bütçe formu eklendi. Kaydetme sonrası toplam/kişi başı bütçe, grup listesi ve hediye kartları yenilenir. Swagger, teknik şartname ve roadmap güncellendi. Aynı Cloudflare bağlantısı korunarak API ve proxy yeniden başlatıldı.
- Doğrulama: `go test ./...` geçti. Yetki, para sınırları ve güncel bütçeden hediye uyarıları için servis testleri eklendi. Canlı HTTPS testleri bütçe artırma/azaltma, kuruş hassasiyeti, sıfır ve eşit fiyat senaryolarını; kişi başı hesaplamayı; ilgi alanlarının korunmasını; 400/401/403/404 durumlarını kapsadı. Veritabanında ilgili sentetik hediyelerin saklanan bayrakları da SELECT ile doğrulandı. Tarayıcıdan 3000 → 6000 bütçe güncellemesiyle kişi başı tutar 1500 → 3000 oldu, 5000 ₺ hediyenin uyarısı kalktı. Üyede bütçe formu görünmedi; 390 px genişlikte taşma olmadı. Sentetik test verileri temizlendi.

---

## 2026-10-17 — Kaynaklı AI Akışının Kaydı ve Doküman Senkronizasyonu

**Mevcut kodda doğrulanan değişiklikler**
- Gemini istemcisi iki aşamalı çalışır: Google Search ile izinli Türkiye mağazalarında somut ürün/TL fiyatı araştırması, ardından `evidence_ids` ile desteklenen ürünlerin yapılandırılmış JSON'a dönüştürülmesi. Araştırma ile seçim toplam 150 saniyelik context kullanır; 503 durumunda arama/JSON ayarları korunarak yedek model denenir.
- Kaynakların grounding metadata'sından gelmesi; ürün adı, arama metni ve fiyatın destekleyen araştırmada bulunması; pozitif fiyat, iki ondalık basamak ve veritabanı sınırları kontrol edilir. Servis mevcut/reddedilmiş başlıkları ve aynı yanıttaki tekrarları normalize ederek eler; en fazla 5 öneri kaydeder. Daha az sonuç dönebilir.
- `GiftIdea` modelinde `ai_sources` (JSONB) ve `ai_search_suggestions` (TEXT) saklanır; AI önerilerinde `product_url` boş kalır. Grup oluşturma yanıtı kurucunun profilini yükler; yüklenmeyen `Creator` ilişkisi boş nesne yerine atlanabilir.
- README, ROADMAP, PROJECT_CONTEXT, SETUP ve panel README'si mevcut uygulamayla eşitlendi. İlk `.env` kurulumu, Node.js gereksinimi, ayrı terminallerde backend/panel/tünel çalıştırma ve doğrulama komutları eklendi. Geçici tünel adresi kalıcı proje bağlantısı olarak dokümanlara yazılmadı.
- Kaynakların panelde iframe içinde gösterildiğini söyleyen eski ifade düzeltildi: `ai_sources` ve Google arama önerisi HTML'i API'de mevcut, panel kartlarında gösterim henüz yok. Bu iş Faz 7'de açık tutuldu. Kaynaksız araştırmanın mevcut HTTP 500 yanıtı ile servis filtresi sonrası boş sonucun 422 yanıtı ayrıştırıldı; hata eşlemelerini iyileştirme işi roadmap'e eklendi. API davranışı bu dokümantasyon güncellemesinde değiştirilmedi.
- Swagger'ın AI açıklaması kaynak controller yorumunda güncellendi; `backend/` içinde `swag init -g cmd/api/main.go --parseInternal` ile üç şema dosyası yeniden üretildi.

**Çalıştırma ve doğrulama**
- `docker compose up -d --wait` ile PostgreSQL sağlıklı duruma geldi. Güncel kaynak `backend/` içinde `go run ./cmd/api`, panel proje kökünde `node prototypes/test-ui/server.mjs` ile başlatıldı. AutoMigrate tamamlandı.
- `cloudflared tunnel --url http://127.0.0.1:5173 --no-autoupdate` ile yeni Quick Tunnel açıldı. HTTPS üzerinden panel 200, `/health` 200 ve `db: up`, tokensız `/api/v1/me` 401 doğrulandı.
- `go test ./...` tüm backend paketlerinde geçti; Gemini testleri sahte HTTP taşıyıcısıyla araştırma/seçim, geçersiz kaynak/ürün/fiyat, 503 yedeği ve Türkçe UTF-8 kaynak aralıklarını kapsadı. İlgi alanı ve bütçe servis testleri de geçti. `node --check` ile panel sunucusu ve istemci JavaScript sözdizimleri doğrulandı.
- Kota kullanan gerçek Gemini testi bu oturumda çalıştırılmadı; `TestLiveTurkeyProducts` varsayılan olarak atlanır, `-args -live-ai` ile özellikle açılır. Önceki günün tarayıcı ve yazma senaryoları bu oturumda tekrar denenmedi. Proxy'nin geliştirme uçlarını engellediği kaynak allowlist'i üzerinden incelendi.

---

## 2026-10-19 — Sonraya Bırakılan AI ve Trendyol İyileştirmeleri

- Kullanıcı iki konunun daha sonra uygulanmak üzere kaydedilmesini istedi; ikisi de ROADMAP backlog'una açık görev olarak eklendi.
- **AI:** Mevcut önerilerin başlıkları Gemini'ye tekrar yasağı olarak gidiyor; manuel önerilerden olumlu tercih çıkarma ve oy skorlarıyla yönlendirme henüz yok. İleride beğenilen manuel fikirler olumlu örnek, veto edilenler olumsuz örnek olarak kullanılacak; yeni öneriler mevcut ürünleri tekrar etmemeli.
- **Trendyol:** Ürün `811478750` için yerel backend testi HTTP 200, `partial: false`, başlık/görsel/açıklama ve fiyat döndürdü. Azure'dan HTTP incelemesinde `/pl/` yönlendirmesi ve 403 görüldü; Türkiye mağazası sorgu parametreleri sonucu değiştirmedi. Bölgesel yönlendirme ve sunucu IP erişim kısıtları araştırılacak; kesin kök neden ayrıştırılmadan bir çözüm garantisi verilmedi. Amazon ve Hepsiburada'nın canlıda çalıştığı kullanıcı tarafından bildirildi.
- Türkiye çıkışlı ayrı servis ve frontend tarafına taşıma seçenekleri değerlendirme aşamasında. Tarayıcı seçeneğinde CORS kısıtları ayrıca doğrulanmalı. Bu kayıt sırasında uygulama kodu veya canlı dağıtım değiştirilmedi.

---

## 2026-10-20 — AI Model Ayrımı, Azure Paketleme ve Depo Senkronizasyonu

- Araştırma `GEMINI_RESEARCH_MODEL` (varsayılan `gemini-3.8-flash`), JSON seçimi `GEMINI_MODEL` (varsayılan `gemini-3.5-flash-lite`) ile ayrı yapılandırılır. Seçim aşamasında Google Search açılmaz. 503 yedeği çağrı ayarlarını korur; kullanılan model zaten yedekse aynı model tekrar çağrılmaz.
- Gemini 429 hatası hata zincirinde korunur ve controller HTTP 429 ile kullanıcıya kota/istek sınırı mesajı döner; provider ayrıntısı yanıta sızmaz. Swagger şemaları ve testler bu davranışı kapsar.
- API ve geçici panel için Dockerfile/`.dockerignore` dosyaları ve 19 Ekim Azure demo ortamının dağıtım/işletim kaydı depoya eklendi. Bu oturumda buluta yeniden dağıtım yapılmadı.
- README, teknik şartname, kurulum rehberi, panel rehberi ve roadmap kodla eşitlendi; Azure rehberi bağlantıları eklendi. 429 işi tamamlandı; kaynaksız araştırmanın 422 eşlemesi, geçici servis hatasının 503 eşlemesi ve yerel compose entegrasyonu açık tutuldu.
- `.gitignore` günlükleri, geçici dosyaları, veritabanı yedeklerini ve özel anahtar/kimlik dosyalarını dışlayacak şekilde genişletildi; `.env.example` takipte kalır.
- Doğrulama: `go test ./backend/...`, `go vet ./backend/...`, panelin iki JavaScript dosyasında `node --check`, `git diff --check`, yerel doküman bağlantıları ve yaygın gizli anahtar kalıpları kontrolü geçti. `.env`, örnek yedek/anahtar/günlük yollarının dışlandığı ve `.env.example` dosyasının dışlanmadığı doğrulandı. Canlı Gemini çağrısı, Docker imaj derlemesi ve Azure canlı sağlık kontrolü bu oturumda yapılmadı.


## 2026-10-21 — Search kapalı öneriler ve dışarıdan model yapılandırması

- Gemini Search varsayılan olarak kapalıdır. Tek yapılandırılmış JSON çağrısı kullanılır; kaynak ve arama HTML'i üretilmez.
- Kaynaksız mod açıkça işaretlenir; servis açıklamaya doğrulanmamış fiyat/stok notu ekler. Bütçe, tekrar, veto ve Anti-Spoiler kontrolleri korunur.
- Model kimlikleri üretim Go kodundan kaldırıldı. `GEMINI_MODEL` zorunlu; araştırma modeli boşsa ana model kullanılır. İsteğe bağlı `GEMINI_FALLBACK_MODEL` yalnızca 503 için geçerlidir. 429'da ek çağrı yapılmaz.
- Yerel ve örnek ayarlar: `GEMINI_MODEL=gemini-3.8-flash`, `GEMINI_SEARCH_ENABLED=false`, araştırma/yedek model boş.

### Oturum durumu — kullanıcı kapatmak için durdurdu

- Branch: `codex/gemini-search-429`; değişiklikler çalışma dizininde, henüz commit edilmedi.
- Tüm backend testleri ve `go vet ./backend/...` geçti; `git diff --check` temiz.
- Search kapalı 3.8 canlı testi yaklaşık 149 saniyede Google 504 döndürdü. Ardından `GEMINI_THINKING_LEVEL=low` dış yapılandırması eklendi; ikinci canlı test kullanıcı kapatmak istediği için 75. saniyede durduruldu; başarı doğrulanmadı.
- Güncel Docker imajı yerelde başarıyla derlendi: `covertgiftmsheker.azurecr.io/covertgift-api:search-off-20261021`.
- İmaj henüz registry'ye gönderilmedi, Azure uygulaması güncellenmedi. Canlıda eski `covertgift-api:demo` çalışıyor; model ortam değişkenleri tanımlı değil.
- ACR Tasks `TasksOperationsNotAllowed` nedeniyle kullanılamıyor; yerel Docker açıldı ve `az acr login` başarılı oldu.
- Devam: canlı model testini sonuçlandır; imajı push et; Container App'e imaj ve `GEMINI_MODEL=gemini-3.8-flash`, `GEMINI_SEARCH_ENABLED=false`, `GEMINI_THINKING_LEVEL=low` ayarlarını ver; yedek modeli boş bırak; sağlık ve öneri akışını doğrula. Secret değerlerini değiştirme veya yazdırma. Eski `demo` imajını geri dönüş için koru.

---

## 2026-10-21 — Kalıcı frontend iskeleti: Auth + Gruplar (Faz 7, ilk iterasyon)

- `frontend/` kuruldu: Vite + React 19 + TypeScript, Tailwind CSS v4, React Router, TanStack Query, i18next (TR varsayılan, EN), Lucide. Kararlar kullanıcıyla soru-cevapla alındı: TanStack Query, sıcak/oyunbaz görünüm, mobil öncelik + TR/EN, token `localStorage`'da, geliştirmede Vite proxy (`:5174` → `:8080`).
- shadcn CLI'nin bağımlılık kurulumu npm'in `EALLOWSCRIPTS` kuralına takıldı; `components.json` yazıldı, Button/Input/Label/Dialog/Sonner shadcn kalıbıyla (`radix-ui`, `cva`, `clsx`, `tailwind-merge`) elle eklendi.
- Tasarım `design-taste-frontend` skill'inin uygulama stiliyle seçildi: "Editorial greeting" (seed 2035334530). Yüz: veriden kurulan serif selamlama + kişi başına sabit pastel avatar tonu.
- Ekranlar: Giriş, Kayıt (sonrasında otomatik giriş), uygulama kabuğu (masaüstü kenar çubuğu / telefonda üst bar + menü), Gruplar (selamlama, sayaçlar, grup listesi, sürpriz yapılanlar, boş/yükleniyor/hata durumları), Yeni grup diyaloğu, Grup sayfasının ilk sürümü (bütçe, kişi başı, ilgi alanları, üyeler) ve Anti-Spoiler "Bu bir sürpriz!" ekranı.
- Doğrulama: `npm run build` ve skill lint'i (0 FAIL; tek uyarı diyaloğun gölgesi) geçti. Yerel API ile tarayıcıda kayıt → boş durum → grup kurma (`3200,50` → ₺3.200,50, ilgi alanları ayrıştırıldı) → grup sayfası; hedef kullanıcıyla gerçek 403'te sürpriz ekranı ve listede grubun görünmemesi; 375 px'te İngilizce, yatay taşma yok, fontlar yüklü. Sentetik test hesapları `DELETE /me` ile silindi.
- Açık: hediye havuzu ve oylama kartları, AI öneri, link önizleme, üye/ilgi alanı/bütçe düzenleme, `target_user_id` seçimi (arayüzde henüz yok), skill inspector'ının tam turu.

---

## 2026-10-21 — Hediye havuzu ve oylama kartları (Faz 7, ikinci iterasyon)

- Kullanıcı kararları: satır kartlar (solda görsel, sağda dikey oy sütunu); elenenler altta katlanır "Elenenler" bölümünde; kapsamda manuel ekleme + link önizleme, "Gemini ile öner", admin vetosu + silme ve e-postayla üye ekleme. Oyu geri alma backend'de desteklenmediği için ROADMAP backlog'una eklendi; arayüzde oy yalnızca değiştirilebilir.
- Grup sayfasına `GiftPool` (iyimser oy + yeniden çekme, Gemini yükleme durumu ve 429/422/503 mesajları, onaylı veto/silme), `IdeaCard` (fiyat, AI/Manuel ve bütçe üstü etiketleri, Trendyol/Amazon arama linkleri, MANUAL için "Ürüne git", AI kaynakları ve sandbox iframe'de Google önerileri), `AddIdeaDialog` (preview-link ile doldurma, kısmi önizleme uyarısı) ve `AddMemberForm` eklendi. Dış linkler yalnızca http(s) ise gösterilir.
- Doğrulama: build, oxlint ve skill lint'i (0 FAIL; uyarılar diyalog/menü gölgeleri) geçti. Yerel API'de iki test kullanıcısıyla tarayıcıda: oy verme ve −1→+1 değiştirme (sıralama sunucu skoruyla güncellendi), menüden admin vetosu ve Elenenler bölümü, hepsiburada.com ana sayfasıyla kısmi önizleme → formu düzeltip ekleme ("Ürüne git" ve kodlanmış Trendyol linki), kayıtlı olmayan e-postayla üye eklemede 404 mesajı, 375 px'te taşmasız düzen. Test kullanıcıları `DELETE /me` ile silindi.
- Denenmedi: canlı Gemini çağrısı (kota harcamamak için), gerçek bir ürün sayfasıyla tam önizleme, üye rolündeki kullanıcının menüsü, AI kaynak/öneri gösteriminin gerçek veriyle görünümü.

### Canlı Gemini denemesi (aynı gün)

- Arayüzden tek bir "Gemini ile öner" çağrısı yapıldı (`GEMINI_MODEL=gemini-3.8-flash`, Search kapalı, `GEMINI_THINKING_LEVEL=low`). Yükleme durumu doğru göründü: buton kilitli ve dönen simgeli, 1–2 dakika notu görünür. Yanıt yaklaşık 26 saniyede geldi; 4 öneri eklendi ve toast çıktı. Öneriler ilgi alanlarına uygundu (kahve değirmeni, kafa lambası, trekking bastonu, AeroPress Go), hepsi ₺3.000 bütçenin altındaydı. Görsel yok, AI rozeti ve Trendyol/Amazon arama linkleri var.
- Search kapalı olduğu için kaynak ve Google öneri HTML'i dönmedi; bu bölüm kartta doğru biçimde gizli kaldı. Kaynaklı gösterim gerçek veriyle hâlâ görülmedi.
- Bulgu ve düzeltme: backend'in "Web araması yapılmadı. Fiyat tahminidir…" notu açıklamanın son paragrafında olduğu için iki satırlık kırpmada görünmüyordu. Fiyatlar doğrulanmış gibi okunuyordu. Kaynaksız AI fikirlerinde fiyatın yanına "Tahmini fiyat" etiketi eklendi (üzerine gelince not görünür); kartta açıklamanın yalnızca ilk paragrafı gösteriliyor.
- AI fikirlerinde "… önerdi" satırı butona basan kullanıcıyı gösteriyor (backend `created_by`); bu davranış değiştirilmedi.
- Test kullanıcısı `DELETE /me` ile silindi.

---

## 2026-10-21 — Frontend görsel yenileme: Kobalt & hardal

- Kullanıcı arayüzü soluk ve fontları sıkıcı buldu. Dört yön görsel olarak karşılaştırıldı; seçim "B · Kobalt & hardal", koyu tema sistem ayarını izleyecek, doz "renkli ama düzenli".
- `src/index.css` baştan yazıldı: açık ve koyu token setleri (`prefers-color-scheme`), kobalt ana aksiyon, hardal para alanları, açık kobalt etiket/not tonu, uyarı/olumsuz pill çiftleri, overlay ve seçim renkleri. Fontlar Instrument Sans/Serif yerine Unbounded (başlık, rakam) ve Onest (metin).
- Bileşenlerdeki sabit renkler (`#e3f0ee`, `#fffbeb`, `text-white`, `bg-black/35` vb.) token'lara bağlandı. Avatar tonları CSS değişkeniyle iki temaya uyarlandı; ürün görselleri iki temada da beyaz kutuda gösteriliyor. Sonner bildirimleri sistem temasını izliyor.
- Ekranlar: giriş/kayıt sağ paneli kobalt alan; ana sayfada ortak bütçe ve grup sayfasında kişi başı tutar hardal; ilgi alanları, AI etiketi ve sayaçlar açık kobalt; sürpriz ekranı hardal kart. Seçili beğeni kobalt dolgu; oy verilen buton bir kez zıplar (azaltılmış hareket tercihinde kapalı).
- Doğrulama: `npm run build`, oxlint ve skill lint'i (0 FAIL; uyarılar diyalog/menü gölgeleri) geçti. Tarayıcıda koyu ve açık temada giriş, ana sayfa, grup sayfası ve oylama; 375 px'te sürpriz ekranı ve yatay taşma kontrolü yapıldı. Fontların yüklendiği doğrulandı. Test kullanıcıları silindi.

### Yüzen üst bar (aynı gün)

- Kullanıcı isteğiyle masaüstündeki sol kenar çubuğu ve telefondaki menü sayfası kaldırıldı; yerine sayfanın üstünde kenarlardan içeride duran, yuvarlatılmış ve opak tek bir bar geldi (bulanıklık yok).
- Solda işaret (telefonda yalnızca kutu, geniş ekranda kelime işaretiyle), yanında grup seçici: ana sayfada "Tüm gruplar", grup sayfasında o grubun adı; menüde tüm gruplar ve hediye alınacak kişi. Sağda dil anahtarı (telefonda hesap menüsünde) ve hesap menüsü (isim, e-posta, çıkış).
- Doğrulama: tsc, build, oxlint ve skill lint'i (0 FAIL) geçti; tarayıcıda dar ve 1360 px genişlikte bar, grup seçici ve hesap alanı görüldü, yatay taşma yok.

---

## 2026-10-21 — Grup yönetimi, hayalet oturum, kod bölme ve frontend testleri

- Kullanıcı kararları: hedef kişiyi e-postayla seçme backend kararı gerektirdiği için roadmap'e eklendi (admin sonradan bağlayabilecek); kazanan seçimi, davet linki, gerçek zamanlı oy ve frontend cilaları roadmap backlog'unda detaylandırıldı. Grup yönetimi grup sayfasında yerinde; testler Vitest + Testing Library + Playwright.
- **Grup yönetimi:** İlgi alanı kutusunda "+ Ekle" (tüm üyeler) ve etiketlerde × (admin); toplam bütçe kutusunda kalemle yerinde düzenleme (admin, iki ondalık ve aralık denetimi); başlıkta admin menüsünden grubu adıyla onaylanan silme. Bütçe değişince grup, liste ve fikirlerin bütçe üstü etiketleri yenilenir. İlgi alanı ekleme butonunun erişilebilir adı üye ekleme butonundan ayrıldı.
- **Hayalet oturum:** Token geçerli ama kullanıcı silinmişse `/me` 404 döner; artık token silinip giriş ekranına "oturumun sona erdi" mesajıyla dönülür. 4xx yanıtlarda `/me` yeniden denenmez.
- **Kod bölme:** Sayfalar React Router `lazy` rotalarıyla ayrı parçalara bölündü; ana paket 619 kB → 446 kB, Vite uyarısı kalktı. Koruma bileşenleri `src/routes/guards.tsx`'e taşındı, oxlint uyarıları kalmadı.
- **Testler:** Vitest (jsdom) + Testing Library + MSW 3 ile 30 test: yardımcılar, API istemcisi (zarf, 401, ağ hatası), kimlik akışı (giriş/kayıt hataları, hayalet oturum), grup sayfası (sürpriz ekranı, kodlanmış pazar yeri linkleri, tahmini fiyat etiketi, iyimser oy ve geri alma, üye/admin yetkileri), yönetim (ilgi alanı, bütçe, grup silme) ve link önizlemeli fikir ekleme. Tanımsız istekler testi durdurur (`onUnhandledFrame: 'error'`). Kapsam yaklaşık %78 satır. Playwright kabul testi gerçek API'ye karşı masaüstü ve telefonda 4/4 geçti; test kullanıcılarının silindiği veritabanından doğrulandı.
- Doğrulama: tsc, oxlint, build ve skill lint'i (0 FAIL) geçti; yönetim kontrolleri tarayıcıda görüldü.

---

## 2026-10-21 — Hedef kişiyi e-postayla seçme ve sonradan bağlama

- Dal: `feat/target-email` (PR #1 henüz birleşmediği için `feat/frontend` üzerinden açıldı).
- Kullanıcı kararları: `POST /groups` gövdesinde isteğe bağlı `target_email`, ayrı kullanıcı arama ucu yok; `target_user_id` geriye dönük uyumluluk için kalır, ikisi birlikte gönderilirse 400 döner. `PATCH /groups/:id/target` yalnızca bağlama yapar: hedef bir kez bağlanır, değiştirme ve kaldırma yok. Kayıtlı olmayan e-postada yeni grup diyaloğu durur, isim yedeği için alan boşaltılır. `target_person_name` bağlamada değişmez.
- **Backend:** Servis e-postayı `FindByEmail` ile (büyük/küçük harf duyarsız) çözer; kayıtlı değilse `ErrTargetUserNotFound` (404), kendini seçme `ErrSelfTarget` (400) döner. `LinkTarget` admin, Anti-Spoiler ve üyelik kontrollerinden sonra `SetTargetUser` ile grup satırını `FOR UPDATE` kilitler ve mevcut hedef (409) ile üyelik (409) kontrollerini kilit altında tekrarlar. `AddMember` de aynı satırı kilitleyip hedef kişiyi reddediyor; böylece eşzamanlı bağlama ve üye ekleme hedef kişiyi üye yapamaz. Şema değişmedi.
- **Frontend:** Yeni grup diyaloğuna "Hedef kişinin e-postası (isteğe bağlı)" alanı eklendi. 404'te alanın altında uyarı çıkar; kişi kendi e-postasını girerse istek gönderilmeden uyarılır. Admin menüsünde hedef bağlı değilken "Hedef kişiyi bağla" diyaloğu var; bağlantının değiştirilemeyeceği yazılı. Başlıkta "hesabı bağlı" etiketi görünür. TR/EN metinleri eklendi.
- **Testler:** Go servis testleri (oluşturma 9 durum, bağlama 14 durum, kilitli üye ekleme eşlemesi) ve controller testleri (oluşturma 7, bağlama 13 durum) table-driven yazıldı. Swagger `swag init -g cmd/api/main.go --parseInternal` ile yenilendi. Vitest 37/37 (7 yeni); Playwright 6/6 masaüstü + telefon. Anti-Spoiler e2e grubu artık arayüzden e-postayla kuruluyor; ikinci senaryo kayıtsız e-posta uyarısını, isimle kurmayı ve sonradan menüden bağlamayı doğruluyor.
- Gerçek API'de curl ile: üye bağlama 403, üyeyi hedef yapma 409, kendini bağlama 400, ikinci bağlama 409, bağlı hedefi üye ekleme 400, hedefin grup ve bağlama uçlarında 403. Test hesapları silindi, `@demo.test` kullanıcısı kalmadı. Telefon genişliğinde diyalog görsel olarak kontrol edildi.
- Açık: geçici panel (`prototypes/test-ui`) hâlâ UUID alanını kullanıyor; geriye dönük uyumluluk sayesinde çalışıyor. Kayıtlı olmayan e-postada dönen 404, `POST /members` ile aynı biçimde e-postanın kayıtlı olup olmadığını belli ediyor; bu davranış önceden de vardı.

---

## 2026-10-21 — Azure: yeni API ve kalıcı React frontend dağıtımı

- Kullanıcı kararları: API ve yeni frontend birlikte dağıtılır; frontend aynı `covertgift-ui` adresinde geçici panelin yerini alır; kaynak `feat/target-email` (`ff8bb3c`, PR #1 henüz birleşmedi). İmajlar commit kimliğiyle etiketlendi, `demo` etiketinin üzerine yazılmadı.
- Başlangıç durumu: canlı API zaten `search-off-20261021` imajında ve `GEMINI_MODEL=gemini-3.8-flash`, `GEMINI_SEARCH_ENABLED=false`, `GEMINI_THINKING_LEVEL=low` ayarlarıyla çalışıyordu (önceki kayıttaki "push edilmedi" notu artık geçerli değil).
- `frontend/Dockerfile` (Node 22 derleme + `nginx-unprivileged`), `frontend/nginx.conf.template` ve `frontend/.dockerignore` eklendi. nginx 5173'ü dinler, API'yi aynı origin'den proxy'ler, `/api/v1/dev/*` yolunu 403 ile engeller.
- **Olay:** Yerel smoke testinde engel kuralı ilk sürümde çalışmadı (düz prefix location, regex API location'ından sonra değerlendirildi) ve test isteği yerel API'deki `POST /api/v1/dev/reset-db` ucunu tetikleyerek **yerel veritabanını sıfırladı**. Azure etkilenmedi. Kural, API kuralından önce gelen büyük/küçük harf duyarsız regex'e çevrildi ve ölü bir hedefe karşı `dev`, `DEV`, `//dev`, `x/../dev`, `%64ev`, `%44EV` varyantlarında 403 doğrulandı. Kullanıcı onayıyla yerel veri 2026-10-19 tarihli aktarım yedeğinden (`pg_restore --data-only`, tek transaction) geri yüklendi: 6 kullanıcı, 2 grup, 6 üyelik, 8 fikir, 2 oy — yedekle eşleşti. 19 Ekim sonrası yerel değişiklikler geri getirilemedi.
- Dağıtım: `covertgift-api:ff8bb3c` ve `covertgift-web:ff8bb3c` ACR'ye gönderildi; `az containerapp update --image` ile iki uygulama güncellendi (Express ortamı `--revision-suffix` desteklemiyor). Şema değişikliği yok.
- Canlı doğrulama: `/` 200 ve React paketi, SPA yolu 200, `/health` 200 `db: up`, oturumsuz `/me` 401, `/api/v1/dev/x` nginx'te 403, güvenlik başlıkları mevcut. Geçici hesaplarla: kayıtsız `target_email` 404; e-postayla kurulan grupta hedef 403 alır ve listede 0 grup görür; sonradan bağlama 200, ikinci bağlama 409, bağlanan hedef 403. Hesaplar `DELETE /me` ile silindi. Tarayıcıda canlı giriş ekranı görüldü. Gemini çağrısı yapılmadı.
- Geri dönüş: `covertgift-api:search-off-20261021` ve `covertgift-ui:demo` registry'de duruyor.

---

## 2026-10-22 — Koyu tema "Nötr grafit" ve giriş ekranı düzeni

- Geri bildirim: arayüz koyu temada Discord'a çok benziyordu. Ölçüm: eski koyu ana renk `#4e5cff`, Discord blurple'ı (`#5865F2`) ile aynı tonda (235°); mavi-gri yüzeyler (`#151726` vb.) benzerliği artırıyordu.
- Üç alternatif (A · Sıcak kömür, B · Hardal öncü, C · Nötr grafit) koddan önce gerçek ekranlarda (ana sayfa, grup sayfası) geçici CSS ile karşılaştırıldı. Kullanıcı seçimi: **C · Nötr grafit**.
- `src/index.css` koyu token'ları: renksiz grafit yüzeyler (`--ground #0c0c0d`, `--surface #161618`, `--raised #1e1e21` …), saf kobalt `--primary #2f5bff` (227°), `--accent #86a0ff`, `--soft #1b2550`. Hardal alanlar ve açık tema değişmedi. Kontrastlar: beyaz/ana renk 5.17, accent/yüzey 7.30, muted/yüzey 7.65.
- Giriş/kayıt çerçevesi: sol form bloğu 400 → 340 px, başlık 44 → 36 px. Sağ ürün paneli artık tam yükseklik değil; içeriği kadar uzar ve dikeyde ortalanır, içerideki boşluk kalktı.
- Doğrulama: Vitest 37/37, build ve oxlint geçti. 1280×860'ta koyu ve açık temada giriş ekranı Playwright ekran görüntüsüyle kontrol edildi. Karşılaştırma için açılan geçici demo hesapları silindi.
- Kullanıcı isteğiyle açık tema da aynı kobalta geçti: `--primary`/`--accent #2f5bff`, `--primary-hover #1f45e0`, `--soft #e6ecff`, `--on-soft #1636b0`, `--selection #c9d6ff`; favicon da güncellendi. Kontrastlar: beyaz/ana renk 5.17, ana renk metni beyaz zeminde 5.17 ve yüzeyde 4.67, on-soft/soft 8.14. `--active` üzerinde 4.24 kalan accent yalnızca menü onay simgelerinde (simge için 3:1 yeterli). Giriş panelindeki sabit `#e3e6ff` açıklama metni yeni mavide 4.19'a düşüyordu; iki temada da `text-on-primary` (beyaz, 5.17) yapıldı.
- Yerel: `.claude/launch.json` API için `"autoPort": false` aldı (API'nin 8081'e kayıp Vite proxy'sinin boşa düşmesini önler).

---

## 2026-10-22 — Doküman güncellemesi ve yeni fikir

- Kullanıcı isteğiyle yeni işe başlanmadı; tüm proje dokümanları güncel duruma çekildi: `CLAUDE.md` (frontend artık var, frontend komutları, reset-db uyarısı), `README.md` (Faz 7 tamam, Faz 8 sürüyor, frontend hızlı başlangıcı, güncel AI modu, teknoloji ve klasör yapısı), `PROJECT_CONTEXT.md` (frontend yığını, nginx proxy kuralları, React frontend standartları), `ROADMAP.md`, `docs/SETUP.md` (Adım 7a artık React frontend, yedek komutu), `docs/AZURE.md` (commit etiketli derleme/dağıtım komutları, bekleyen tasarım dağıtımı), `frontend/README.md` (yapı, Nötr grafit tasarımı, üretim imajı) ve `prototypes/test-ui/README.md` (eski olarak işaretlendi).
- Backlog'a **hediye tarihi / doğum günü** fikri eklendi: isteğe bağlı `event_date`, geri sayım, yaklaşan tarihe göre sıralama, hatırlatma ve tarih geçince grubu tamamlama önerisi; Anti-Spoiler korunur. Tekrar eden mi tek seferlik mi, saat dilimi ve bildirim kanalı kararları sonraya bırakıldı.
- Faz 8'e yerel veri koruması maddesi eklendi (yerel `reset-db` ucu Vite proxy'sinde engellenmiyor); karar verilmedi.
- `.claude/` altındaki genel agent/skill dosyaları projeye özel olmadığı için değiştirilmedi.

---

## 2026-10-23 — Güvenlik taraması (bilinen açıklar)

- Kullanıcı gündemdeki açıkları sordu; `govulncheck` (Go vuln DB 2026-10-21), `npm audit` ve web/birincil kaynak araştırması yapıldı. Kullanıcı kararıyla güncellemeler şimdilik yapılmadı; ayrıntılı yapılacaklar ROADMAP Faz 8'in başında.
- **Go:** Kodun çağırdığı yollarda 16 açık: Go 1.27.1 standart kütüphanesi (12; eMudhra ara CA engeli GO-2026-6809 / CVE-2026-111943 Go 1.27.3'te, diğerleri 1.27.2'de), `x/net` HTTP/2 açıkları, Gin üzerinden `grpc` ve `quic-go`. API iç ağda ve nginx arkasında olduğu için doğrudan risk düşük; CA engeli giden TLS bağlantıları (Gemini, link önizleme, PostgreSQL) için önemli.
- **nginx:** Canlı frontend imajı nginx 1.27.5 (desteksiz hat; Alpine 3.21.3, OpenSSL 3.3.3). 2026 bildirimlerinin 22'si 1.27'yi kapsıyor; yapılandırmamızla kesişen CVE-2026-42533 (map + regex, yakalama yok) ve CVE-2026-1642 (TLS upstream). Hedef 1.30.6+ stable. İmaj seçimi bu dağıtımda yapılmıştı.
- **Node:** 21 Ekim güvenlik sürümü (22.23.4 / 26.12.1); Node yalnızca derleme aşamasında.
- **Temiz/etkisiz:** `npm audit` 0 açık; `@tanstack/react-query` 5.104.5 Mayıs tedarik zinciri saldırısında etkilenmeyen paketlerden. Yerel PostgreSQL 17.11 güncel. PHP2Shell, BeyondTrust, Fortinet ve Microsoft Patch Tuesday yığınımızda yok.

---

## 2026-10-23 — Son dağıtım ve Azure ortamının kapatılması

- Kullanıcı isteğiyle projenin son hali dağıtıldı, ardından ortam bir süreliğine kapatıldı.
- Dağıtım: yalnızca frontend değişmişti (Nötr grafit, giriş ekranı, favicon); `covertgift-web:2e0d1e7` derlendi, ölü hedefle yerelde denendi (sayfa ve SPA 200, `/api/v1/DEV/...` 403, CSS'te `#2f5bff`), ACR'ye gönderildi ve `covertgift-ui` güncellendi. Backend kodu `ff8bb3c` ile aynı olduğu için API yeniden dağıtılmadı. Canlıda CSS'te yeni ana renk, `/health` 200 (`db: up`), oturumsuz `/me` 401, `/api/v1/dev/x` 403 ve koyu temada giriş ekranı görüntüsüyle doğrulandı.
- Kapatma: `covertgift-ui` ve `covertgift-api` REST API `stop` ucuyla (CLI'de `containerapp stop` yok), `covertgift-db-msheker` `az postgres flexible-server stop` ile durduruldu; üçü de `Stopped`, canlı adres 502. Hiçbir kaynak, imaj veya veri silinmedi.
- Dikkat: Azure durdurulmuş PostgreSQL esnek sunucusunu 7 gün sonra kendiliğinden başlatır; kapalı tutmak için haftalık `stop` gerekir. Depolama, yedek ve ACR ücretleri sürer. Yeniden açma adımları `docs/AZURE.md` başında.

---

## 2026-11-09 — Güvenlik güncellemeleri ve Azure ortamının yeniden açılması

- Faz 8'deki güvenlik güncellemeleri uygulandı: Go 1.27.1 → 1.27.3 (`go.mod`, `go.work`, `backend/Dockerfile`); `golang.org/x/net` v0.62.0, `google.golang.org/grpc` v1.85.0, `github.com/quic-go/quic-go` v0.65.0. `go mod tidy`, doğrudan kullanılan `swaggo/*` ve `genai` modüllerini doğrudan bağımlılık bölümüne taşıdı.
- Frontend: derleme imajı `node:22.23.4-alpine`, sunucu `nginx-unprivileged:1.30-alpine` (imajda nginx 1.30.6).
- Doğrulama: `go vet` ve `go test ./...` geçti, `govulncheck` kodda 0 açık, `npm audit` 0, Vitest 37/37, `npm run build` başarılı. Frontend imajı ölü hedefle yerelde denendi: `/api/v1/dev/*` (büyük/küçük harf) 403, güvenlik başlıkları mevcut.
- `az acr build` bu abonelikte kullanılamıyor (`TasksOperationsNotAllowed`); imajlar yerel Docker ile `linux/amd64` için derlendi. Değişiklikler henüz commit edilmediği için etiket `security-20261109`.
- Azure: PostgreSQL 7 gün kuralıyla zaten `Ready` durumdaydı. `covertgift-api` ve `covertgift-ui` REST `start` ucuyla başlatıldı ve yeni imajlara güncellendi. Canlıda `/health` 200 (`db: up`), `/` 200, oturumsuz `/api/v1/me` 401, `/swagger/index.html` 200.

---

## 2026-11-09 — Yerel veri koruması: reset-db varsayılan kapalı

- Sorun: `POST /api/v1/dev/reset-db` yalnızca `APP_ENV=production` ile kapanıyordu; yerelde varsayılan açıktı, oturum istemiyordu ve Vite proxy'si iletiyordu. 2026-10-21'de bir smoke testi yerel veritabanını bu uçla silmişti.
- Karar (K39): İki katmanlı koruma. (1) `ALLOW_DB_RESET` ortam değişkeni; boş veya `false` ise uç 403 döner, geçersiz değerde API başlamaz. (2) Bayrak açıkken bile gövdede `{"confirm": "RESET"}` (büyük harf, birebir) yoksa 400. Production her durumda 403. Bayrak açıkken API başlangıçta uyarı loglar.
- Neden: Varsayılanın kapalı olması, ayarı unutan birinin veriyi kaybetmemesini sağlar; onay ifadesi, bayrak açıkken yanlışlıkla atılan çıplak bir POST'u durdurur. Vite proxy engeli eklenmedi: `:8080`'e doğrudan istekleri durdurmadığı için asıl korumayı backend'de tutmak tercih edildi.
- Testler: reddedilen tüm durumların veritabanına dokunmadan döndüğü (`db` nil) tablo testi ve bayrağın varsayılan kapalı olduğunu doğrulayan config testi. Swagger şeması `ResetDBRequest` gövdesiyle yeniden üretildi.

---

## 2026-11-09 — Anti-Spoiler entegrasyon testleri

- Router kurulumu `cmd/api/router.go` içinde `newRouter(db, cfg, geminiClient)` ve `migrate(db)` olarak `main`'den ayrıldı; uygulama ve testler aynı kurulumu kullanır.
- `docker-compose.yml`'a `test` profiliyle `test-db` servisi eklendi: ayrı konteyner, port 55432, veri tmpfs'te (konteyner durunca silinir). Asıl `db` servisine ve verisine dokunulmaz.
- `cmd/api/antispoiler_integration_test.go` (K40): hedef kişi grup listesinde grubu göremez; 14 korumalı uçta (grup detayı/silme, üye, ilgi alanı, bütçe, hedef, fikirler, önizleme, AI, oy, veto, fikir silme) 403 + sürpriz mesajı alır ve yanıt veri içermez; denemeleri hiçbir şeyi değiştirmez. Üye olmayan 403 alır ama sürpriz mesajı almaz. Hedef üye olarak eklenemez (400). Sonradan bağlama: üye hedef yapılamaz (409), yalnızca admin bağlar, bağlanan hedef anında dışlanır, hedef değiştirilemez (409).
- **Bulunan açık:** `POST /ideas/:id/reject` ve `DELETE /ideas/:id` hedef kontrolü yapmıyordu; hedef yalnızca üye olamadığı için "bu grubun üyesi değilsiniz" ile engelleniyordu. Kural gereği `ideaService.checkNotTarget` eklendi; iki uç da artık üyelikten önce hedefi kontrol edip 403 + sürpriz mesajı döner.

---

## 2026-11-09 — İstek boyutu limiti ve giriş doğrulaması

- **Gövde limiti:** `middleware.BodyLimit` tüm rotalarda 1 MiB sınırı uygular. `Content-Length` sınırı aşarsa istek okunmadan 413 döner; uzunluğu bildirilmeyen gövdeler `http.MaxBytesReader` ile kesilir ve 400 döner. Önceden sınır yoktu.
- **Bulunan açıklar:** Grup oluştururken bütçe hiç doğrulanmıyordu (yalnızca `PATCH /budget` doğruluyordu); negatif bütçe kabul ediliyor, `numeric(10,2)`'ye sığmayan değer 500 dönüyordu. Manuel fikirde fiyat da doğrulanmıyordu. İkisi de artık ortak `validMoney` ile 0–99.999.999,99 ve en fazla iki ondalık ister; aksi halde 400.
- **Binding kuralları:** Grup açıklaması ve ilgi alanları `max=2000`, fikir açıklaması `max=5000`, `search_query` `max=255` (önceden 255 üstü 500 dönüyordu), `product_url` `max=1000,http_url`, `image_url` `max=500,http_url` (`javascript:` gibi şemalar reddedilir), önizleme URL'si `max=2048`, üye e-postası `max=150`, giriş parolası `max=100`, oy `oneof=1 -1`.
- **CORS (K41):** Eklenmedi. Frontend her ortamda aynı origin'den eriştiği için API CORS başlığı göndermez; okunup hiç kullanılmayan `CORS_ORIGIN` kaldırıldı.
- Testler: `BodyLimit` birim testi, `validMoney` ile oluşturma/fiyat servis testleri ve gerçek router üzerinden 413 + 10 doğrulama senaryosu içeren entegrasyon testi (`cmd/api/validation_integration_test.go`).

---

## 2026-11-09 — Yapılandırılmış loglama, seed komutu ve Docker Compose

- **Loglama (K42):** `internal/logging` stdlib `log/slog`'u kurar; `LOG_FORMAT` boşsa production'da JSON, diğer ortamlarda text. `slog.SetDefault` eski `log.Printf` çağrılarını da aynı işleyiciye yönlendirir. `gin.Default()` yerine `middleware.Recovery` (paniği loglar, standart 500 zarfı döner) ve `middleware.RequestLogger` (istek başına tek satır; `request_id`, yöntem, yol, rota, durum, süre, IP; sorgu dizesi, gövde ve Authorization loglanmaz; 5xx ERROR, 4xx WARN, `/health` DEBUG). Yanıtlara `X-Request-ID` eklendi. Link önizlemesinin istek başına logları DEBUG'a indi.
- **Bulunan açık:** Bazı 500 yanıtları iç hatayı istemciye gösteriyordu (hesap silme, grup silme, AI üretimi, DB sıfırlama: `"...: "+err.Error()`); diğerleri hatayı hiçbir yere yazmıyordu. Tüm 500'ler artık `controllers.RespondInternal` ile genel mesaj döner, iç hata `c.Error` ile loga gider.
- **Seed (K43):** `cmd/seed` — 4 demo kullanıcı (`@demo.covertgift.test`), hedefi e-postayla bağlı bir grup, 2 üye, 3 manuel fikir, 4 oy. Gerçek servisler üzerinden yazılır (Anti-Spoiler ve doğrulama geçerli), tek transaction'dır, demo yöneticisi varsa çıkar, `APP_ENV=production` iken reddeder. Şema için `models.Migrate` API ile ortak.
- **Compose:** `app` profiliyle `api` (backend/.env'den okur; `DATABASE_URL` compose ağına göre ezilir, `ALLOW_DB_RESET` her zaman `false`) ve `web` (nginx, `API_TARGET=http://api:8080`, http://localhost:8081). API dışarıya port açmaz.
- Entegrasyon testleri artık iki pakette (`cmd/api`, `cmd/seed`) ve aynı test DB'sini boşalttığı için `go test -p 1` ile çalıştırılır.
- Testler: slog kurulumu, istek logu (sızıntı yok, `X-Request-ID`), `RespondInternal` (istemciye sızmaz, loga yazılır), panik kurtarma; seed için yalnızca ekleme, idempotentlik, mevcut kullanıcının korunması ve hedefin grubu görmemesi.

---

## 2026-11-09 — Backlog: kazanan seçimi, oyu geri alma, hediye tarihi ve canlı oylar

- **Kazanan seçimi (K44):** `Group.status` (`ACTIVE`/`COMPLETED`) ve `completed_at` eklendi. `POST /groups/:id/winner` (yalnızca admin) aktif bir fikri `SELECTED`, grubu `COMPLETED` yapar; grup satırı kilitlenir. Tamamlanan grupta fikir ekleme, AI, oy, oyu geri alma, veto ve fikir silme 409 döner; Anti-Spoiler kontrolü her zaman bundan önce gelir. `DELETE /groups/:id/winner` grubu yeniden açar. Arayüz: fikir menüsünde "Kazanan olarak seç" (onaylı), kazanan kart en üstte ve vurgulu, oylar salt okunur, ekleme butonları gizli, grup listesinde "Tamamlandı" etiketi, admin menüsünde "Grubu yeniden aç".
- **Oyu geri alma (K46):** `DELETE /ideas/:id/vote` (idempotent). Arayüzde seçili oya tekrar basmak oyu kaldırır; iyimser güncelleme ve hata halinde geri alma korunur.
- **Hediye tarihi (K45):** `Group.event_date` (`models.Date`, JSON `YYYY-MM-DD`, PostgreSQL `DATE`). Grup kurarken veya admin menüsünden ayarlanır/kaldırılır (`PATCH /groups/:id/event-date`). Grup sayfasında tarih + geri sayım, listede geri sayım etiketi (7 gün ve altı vurgulu), sıralama: yaklaşan tarihli açık gruplar en yakın önce, sonra diğer açık gruplar, en sonda tamamlananlar. Tarih geçmiş ve grup açıksa admin'e kazananı seçme hatırlatması. Seed grubu 3 hafta sonrası tarihle kurulur.
- **Canlı oylar (K47):** Grup detayı ve fikir listesi sayfa açıkken 10 sn'de bir çekilir; bir oy/fikir işlemi sürerken (ortak `mutationKey`) ve grup tamamlanınca fikir yenilemesi durur. Hedef kişi 403 aldığı için yenileme de durur.
- **Testler:** Backend — `models.Date`, entegrasyon: oyu geri alma (başkasının oyu korunur, üye olmayan 403), kazanan seçimi (yetki, başka grubun fikri 400, reddedilmiş fikir 409, kapalı grupta 7 işlem 409, hedef yine sürpriz mesajı, yeniden açma), hediye tarihi (oluşturma, biçim/aralık 400, admin dışı 403, geçmiş tarih, kaldırma); Anti-Spoiler listesine 4 yeni uç eklendi (toplam 18). Frontend — tarih yardımcıları ve sıralama; oyu geri alma, kazanan seçimi, tamamlanmış grup görünümü, yeniden açma, tarih kaydetme/kaldırma.
- **Tarayıcıda doğrulama:** Geçici `test-db` + seed verisiyle (gerçek yerel veriye dokunmadan): oyu geri alma, kazanan seçimi ve yeniden açma, başka üyenin API'den verdiği oyun sayfa yenilenmeden yansıması, geçmiş tarihte "5 gün geçti" ve hatırlatma, telefon genişliğinde yatay kaydırma olmaması. Dar ekranda durum etiketi başlığı sıkıştırdığı için alt satıra alındı.

---

## 2026-11-09 — Canlı güncellemeler: Server-Sent Events

- **Backend:** `internal/realtime.Hub` (bellek içi, grup başına abone kümesi; sinyaller birleşir, yayın hiç bloklamaz; en fazla 2000 açık akış). `GET /groups/:id/events` `GroupAccess` arkasında (Anti-Spoiler + üyelik): `text/event-stream`, `X-Accel-Buffering: no` (nginx tamponlamasın), 25 sn'de bir `: ping`, en fazla 5 dk ömür. `middleware.PublishGroupChanges` (/groups/:id/* altında başarılı her değişiklik) ve `middleware.PublishIdeaChanges` (/ideas/:id/* altında; grup işlemden önce bulunur, silinen fikirde de olay gider) yayın yapar. Başarısız istekler olay üretmez.
- **Frontend:** `useGroupEvents` akışı fetch ile okur (EventSource başlık gönderemez; token URL'ye yazılmaz). "changed" gelince grup detayı ve fikirler 250 ms birleştirmeyle yeniden çekilir; kendi oy işlemimiz sürerken atlanır. Akış kapanınca 1 sn'de, hatada artan aralıkla (en çok 30 sn) yeniden bağlanır; 401/403/404'te durur; yeniden bağlanınca kaçanlar için bir kez yeniler. Hedef kişi grup detayında 403 aldığı için akış hiç açılmaz. Bağlantı açıkken periyodik yenileme 60 sn'ye iner, kopukken 10 sn'ye döner.
- **Testler:** Hub (teslim, birleştirme, sınır, abonelik sızıntısı), akış ucu (başlıklar, ping, olay, ömür dolunca kapanma, 503), entegrasyon (hedef ve üye olmayan 403, oturumsuz 401, oy ve bütçe değişikliği olay üretir, başarısız istek üretmez), frontend (Bearer başlığı, açık akıştaki olayla skorun güncellenmesi, hedef için akışın açılmaması).
- **Doğrulama:** Tarayıcıda (test-db) başka üyenin oyu ~1 sn'de yansıdı. Canlıdaki nginx imajı üzerinden ölçüm: olay ~0,2 sn'de geldi, tamponlama yok.
- **Sınırlama:** Hub tek kopya içindir. API birden çok replica'ya ölçeklenirse olaylar kopyalar arasında taşınmaz; o zaman PostgreSQL LISTEN/NOTIFY eklenmeli.

---

## 2026-11-09 — Azure dağıtımı `7bb8416`

- `main` (`7bb8416`, #5–#10) yerel Docker ile `linux/amd64` için derlendi, ACR'ye `covertgift-api:7bb8416` ve `covertgift-web:7bb8416` olarak gönderildi, iki Container App güncellendi.
- Canlı kontrol (yalnızca okuma): `/health` 200 (`db: up`, yani AutoMigrate başarılı), `/` 200, oturumsuz `/api/v1/me` 401, Swagger 200 ve yeni uçlar (`/groups/{id}/events`, `/groups/{id}/winner`) şemada, oturumsuz olay akışı 401, yanıtlarda `X-Request-ID`. Canlı veriye yazan hiçbir istek atılmadı; kullanıcı testi canlıda kendisi yapacak.
- `az containerapp logs show` bu CLI sürümünde `KeyError: 'eventStreamEndpoint'` ile çalışmıyor.

---

## 2026-11-09 — Birden çok kazanan

- **API:** `POST /groups/:id/winner` artık `{ "idea_ids": [...] }` alır (1–20, tekrarlar ayıklanır). Repo, grup satırı kilitliyken gruba ait bulunan fikirleri guard'a verir; kimliklerden biri gruba ait değilse 400, biri aktif değilse 409 ve hiçbir fikir değişmez. Yeniden açma tüm `SELECTED` fikirleri aktife döndürür. Eski `idea_id` alanı kaldırıldı (tek tüketici frontend).
- **Arayüz:** "Kazanan olarak seç" artık aktif fikirlerin işaretlenebilir listesini açar; tıklanan fikir önceden işaretli gelir. Seçilenlerin toplamı bütçeyle birlikte gösterilir, aşımda uyarılır (K49). Onay düğmesi seçim sayısını söyler; hiçbir şey seçili değilse kapalıdır; en fazla 20. Tamamlanan grupta tüm kazananlar üstte, bant hepsini sayar.
- **Testler:** Entegrasyon — iki kazanan, tekrarlanan kimlik, başka grubun fikri (hiçbiri değişmez), 21 kimlik 400, yeniden açınca hepsi aktif. Frontend — önceden işaretli fikir, toplam ve bütçe aşımı uyarısı, `idea_ids` gövdesi, boş seçimde kapalı düğme, birden çok kazananlı tamamlanmış görünüm.
- **Tarayıcıda (test-db):** iki kazanan seçildi; toplam ₺2.550 / bütçe ₺3.000, üçüncüyle ₺3.400 ve uyarı; tamamlandıktan sonra iki kart "Kazanan" etiketli ve üstte.

---

## 2026-11-10 — Ana ekran yeniden tasarımı: sıradaki sürpriz ve öncelikli görevler

- **API:** `GET /groups` her grupta `member_count`, `idea_count` (reddedilmemiş fikirler) ve çağıran kullanıcıya özel `pending_votes` döner: açık gruptaki, kullanıcının eklemediği ve henüz oy vermediği `ACTIVE` fikirler (tamamlanan grupta 0). Sayaçlar tüm gruplar için tek ek sorguyla (`GroupRepository.GetGroupListStats`, alt sorgular) hesaplanır; yanıt tipi `services.GroupListItem` (`models.Group` gömülü), şema değişmedi. Anti-Spoiler filtresi aynı. Swagger yeniden üretildi.
- **Arayüz:** "Gruplar / Yönettiğin / Ortak bütçe" kutucukları, "Sürpriz yapılanlar" paneli ve başlıktaki bütçe özeti kaldırıldı; başlık "N açık sürprizin var." der. Yeni düzen (8 + 4, iki satır): kobalt "Sıradaki sürpriz" kartı (en yakın bugün/sonrası tarihli açık grup, büyük gün sayısı, ≤1 günde "Bugün"/"Yarın"; tarih yoksa tarihsiz ilk grubuna "tarih ekle" bağlantısı), "Senden bekleyenler" (Acil/Yüksek/Normal/Düşük etiketli, en fazla 6 satır; kurallar `src/lib/priority.ts`), "Gruplarım" (yalnızca açık gruplar; satırda bütçe yerine "N üye · M fikir" ve "N oy bekliyor" rozeti), "Geçmiş sürprizler" (tamamlananlar, yeni → eski). Yüksek öncelik için hardaldan ayrı turuncu `--pill-high-bg/ink` token'ları eklendi (açık/koyu). Oylar grup listesini geçersiz kılmadığı için ana ekran her açılışta listeyi yeniden çeker.
- **Testler:** Entegrasyon `TestGroupListCounts` (sayaçlar, kendi fikri, reddedilen fikir, beğenmeme oyu da sayılır, başkasının oyu etkilemez, tamamlanınca 0, hedef listede görmez). Vitest `priority.test.ts` (16 senaryo: seviyeler, sıralama, sınır, sıradaki sürpriz).
- **Tarayıcıda (test-db + seed):** dört öncelik seviyesi, sıradaki sürpriz kartı, geçmiş sürprizler açık/koyu temada ve 375px genişlikte (yatay kaydırma yok) kontrol edildi. Playwright, test API'sine yönlendirilmiş geçici bir :5174 sunucusuyla 6/6 geçti.

---

## 2026-11-10 — Profil düzenleme: API ve veri katmanı

- **API:** `PATCH /me` isim ve e-postayı birlikte günceller. Doğrulama kayıtla aynı (`controllers.ProfileFields` iki DTO'ya gömülü); e-posta kayıttaki gibi olduğu gibi saklanır, benzersizlik büyük/küçük harf duyarsız denetlenir (kendi e-postasının harf düzenini değiştirmek serbest), başka kullanıcıdaysa 409; eşzamanlı yarışta DB benzersizlik ihlali (23505) de 409'a çevrilir. JWT `email` claim'i taşıdığı için yanıt `{user, token?}`: token yalnızca e-posta değiştiğinde döner. `PUT /me/password` (`current_password`, `new_password`): yanlış mevcut şifre ve eskisiyle aynı yeni şifre 400 (401 istemcide oturumu kapattığı için bilerek değil); açık oturumlar geçerli kalır, token iptali yok. Swagger yeniden üretildi.
- **Frontend (yalnızca veri katmanı):** `authApi.updateProfile`, `authApi.changePassword`, `UpdateProfileResponse` tipi; `useAuth().updateUser(res)` kullanıcıyı `me` önbelleğine yazar, token geldiyse saklı token'ı değiştirir ve gömülü üye adları için grup sorgularını geçersiz kılar. Profil sayfası arayüzü ve i18n anahtarları sonraki adımda.
- **Testler:** Entegrasyon `TestUpdateProfile` (yalnız isim token'sız, yeni e-posta yeni token + yeni e-postayla giriş, eski token geçerli, harf düzeni, 409 büyük/küçük harf duyarsız, 6 geçersiz giriş 400, oturumsuz 401, başarısız denemeler profili değiştirmez) ve `TestChangePassword` (yanlış/aynı/kısa/uzun/eksik 400, yanlış şifre oturumu kapatmaz, oturumsuz 401, yeni şifreyle giriş olur eskiyle olmaz, mevcut oturum açık). Vitest `profile.test.tsx` (5 senaryo).

## 2026-11-10 — Profil sayfası arayüzü

- **Giriş:** Hesap menüsüne (çıkıştan önce) "Profil"; `/profile` korumalı, lazy rota. Sayfada "Ana ekran" geri bağlantısı.
- **Yerleşim:** Tek sütun, en çok 640px, ortalı. Üstte avatar + isim + e-posta; altında "Hesap bilgileri", "Şifre değiştir" ve `pill-negative` tonlu "Hesabı sil" kartları (`Panel`, `FormField`, `Input`, `Button`; yeni renk yok).
- **Etkileşim:** Her kartın kendi düğmesi; değişiklik yoksa veya form geçersizse pasif, kaydederken "Kaydediliyor…". 409 e-posta alanının altında, yanlış mevcut şifre mevcut şifre alanının altında (`role="alert"`, `aria-invalid`, `aria-describedby`); diğer hatalar toast. Şifre alanlarında göster/gizle düğmesi (`aria-label`, `aria-pressed`); yeni şifre 6–100 karakter, mevcutla aynı olamaz, tekrar eşleşmeli; başarıda alanlar temizlenir. Hesap silme `ConfirmDialog` ile; başarıda çıkış ve `/login`. `authApi.deleteMe` eklendi. TR/EN `profile.*` ve `nav.profile` anahtarları.
- **Testler:** Vitest `pages/profile.test.tsx` (10 senaryo: menüden giriş, kaydet pasif/aktif, başarılı güncelleme + yeni token, 409, istemci doğrulaması, göster/gizle, yanlış şifre, başarılı şifre değişimi, silme onayı ve vazgeç, silme hatası). Playwright `e2e/profile.spec.ts` (masaüstü + telefon), test API'sine yönlendirilmiş geçici :5174 sunucusuyla; tüm e2e 8/8.
- **Tarayıcıda (test-db):** 1280px ve 375px, açık/koyu temada kontrol edildi; 375px'te yatay kaydırma yok. İpucu ("En az 6 karakter.") hata mesajının üstüne alındı.
- **Bilinen kararsız test:** Vitest arada bir "1 error" bildiriyor: `group.test.tsx` "deletes the group…" sonrasında MSW interceptor'ının soket kapanışında Node undici `assert(!this.aborted)` hatası. Büyük olasılıkla sayfadan çıkılınca iptal edilen, hiç kapanmayan SSE (`/events`) isteği (Node 26, @mswjs/interceptors 0.45.9). Testler geçiyor; profil testleriyle ilgisi yok.

## 2026-11-10 — AI önerileri çalışmıyordu: model değişimi ve 503 mesajı

- Belirti: "Gemini ile öner" 500 dönüyordu. `gemini-3.8-flash` ve `gemini-3.7-flash` küçük istekte bile 45 sn içinde yanıt vermedi; arada 503 "high demand" geldi. `gemini-3.5-flash` 503 verdi. `gemini-3.6-flash` ve `gemini-3.5-flash-lite` 1–8 sn'de yanıt verdi. `gemini-2.5-flash` yeni kullanıcılara kapalı (404).
- Düzeltme: birincil model `gemini-3.6-flash`, yedek `gemini-3.5-flash-lite` (ikisi de `GEMINI_THINKING_LEVEL=low` ile çalışıyor). Kod yalnızca 503'te yedeğe geçer; zaman aşımında geçmez.
- Kod: `pkg/gemini` kalıcı 503'ü `ErrOverloaded` olarak sarar; `POST /groups/:id/ideas/generate-ai` bunu 503 ve "Yapay zekâ servisi şu an yoğun. Birkaç dakika sonra tekrar deneyin." mesajıyla döner (önceden genel 500). İki yeni test eklendi.
- Doğrulama: test DB'ye bağlı yerel API ile tek istek, HTTP 201, 4 öneri, ~6 sn. Canlıya AI isteği atılmadı (fikirleri DB'ye yazar).

## 2026-11-12 — Bildirim sistemi: kalıcı bildirimler, anlık zil ve hatırlatmalar

- **Model:** `notifications` tablosu (`user_id`, `group_id`, `type`, `params` JSONB, `dedupe_key`, `read_at`, `created_at`); `(user_id, dedupe_key)` benzersiz, `(user_id, created_at)` indeksli; kullanıcı ve grup silinince CASCADE. `models.Migrate` ile AutoMigrate. Metin sunucuda tutulmaz; `type` + `params` saklanır, frontend TR/EN üretir.
- **Olaylar:** yeni fikir (manuel ve AI), gruba eklenme (eklenene "gruba eklendin", diğerlerine "yeni üye"), kazanan seçimi, yeniden açma, hediye tarihi ve bütçe değişikliği. Aynı bütçe/tarih yeniden kaydedilirse bildirim üretilmez. Alıcılar: grup üyeleri; işlemi yapan ve hedef kişi hariç.
- **Tetikleme (K51) ve gerekçe:** Üç seçenek değerlendirildi. (1) Middleware'de rota+metot eşlemesi: yeni rota kendiliğinden kapsanır ama olayın ayrıntısını bilemez (eklenen üye kim, bütçe gerçekten değişti mi, AI kaç fikir üretti) ve eşleme rota yolu değişince sessizce bozulur. (2) Servis katmanında üretim: ayrıntıyı bilir ama isteğin sonunda başarılı olup olmadığını bilemez ve her servis metoduna alıcı/Anti-Spoiler mantığı dağılır. (3) Seçilen: handler başarılı işlemin ardından yalnızca "ne oldu"yu `controllers.RecordNotification` ile bildirir; `PublishGroupChanges`/`PublishIdeaChanges` yanında rota grubuna bağlı `middleware.Notify`, yalnızca yanıt < 400 ise olayları `NotificationService.Dispatch`'e verir. Alıcı seçimi, Anti-Spoiler, kayıt ve SSE sinyali tek fonksiyondadır. Yeni bir değişiklik ucu zincirden otomatik geçer; bildirim kararı unutulmasın diye `TestEveryMutationRouteHasANotificationDecision` router'daki her `/groups/:id/*` ve `/ideas/:id/*` değişiklik rotasının "bildirir / bildirmez" tablosunda olmasını ister.
- **Anti-Spoiler:** (a) `Dispatch` grubu her olayda yeniden okur ve `target_user_id`'yi alıcılardan çıkarır; zamanlayıcı da aynı kontrolü yapar. (b) `PATCH /groups/:id/target` hedefin o gruba ait bildirimlerini bağlama transaction'ında siler (`GroupRepository.SetTargetUser`) ve hedefin açık ziline veri taşımayan yenileme sinyali gönderir. (c) Listeleme, okunmamış sayacı, okundu ve tümünü okundu sorguları `group_id NOT IN (hedefi olduğum gruplar)` ile süzülür; yarış sonucu tabloda satır kalsa bile hedef göremez, sayamaz, işaretleyemez. `params` yalnızca üyelere giden satırlarda bulunur.
- **Teslim:** `realtime.Hub` anahtar bazlı hale getirildi (kod aynı; grup hub'ı grup kimliğiyle, yeni kullanıcı hub'ı kullanıcı kimliğiyle). `EventsController` iki akışı aynı çerçeveyle sunar (25 sn ping, 5 dk ömür, 2000 sınır). Uçlar: `GET /notifications`, `GET /notifications/stream`, `POST /notifications/:id/read`, `POST /notifications/read-all` (JWT korumalı). Başkasının bildirimi 404 döner (varlığı sızmaz). Okundu işlemleri kullanıcının diğer sekmelerine de sinyal gönderir. Swagger yeniden üretildi.
- **Hatırlatmalar (K52):** `services.ReminderScheduler` API içinde goroutine; başlangıçta bir kez, sonra saatlik. 3 gün kala ve 1 gün kala tüm üyelere, tarih geçmiş ve grup `ACTIVE` ise yöneticilere bir kez. Aralıkla bakar (2–3 gün → 3 günlük anahtar, 0–1 gün → 1 günlük anahtar): API kapalıyken kaçan hatırlatma açılışta kalan gün sayısıyla üretilir. Anahtar tarihi içerir; tarih değişirse yeni tarih için yeniden hatırlatılır. "Bugün" UTC+3. Saat `Now` ile enjekte edilir. `main.go` artık `signal.NotifyContext` kullanır: SIGINT/SIGTERM'de zamanlayıcı durur, sunucu 5 sn içinde kapanır (SSE akışları kendiliğinden bitmediği için süre dolunca bağlantılar kesilir).
- **Frontend:** Üst barda dil anahtarı ile hesap menüsü arasında zil (`components/NotificationBell.tsx`): okunmamış rozeti (9'dan fazlası "9+"), açılır liste (mevcut `DropdownMenu`), okunmamışlar vurgulu ve noktalı, tıklayınca okundu + gruba gider, "Tümünü okundu yap" (liste açık kalır), boş/yükleniyor/hata durumu. Okundu işlemleri iyimser güncellenir. `useNotificationEvents` akışı dinler ve `['notifications']` ile grup listesini geçersiz kılar; kopunca yeniden bağlanır ve bir kez yeniler. Ortak akış okuma mantığı `api/eventStream.ts`'e taşındı; `useGroupEvents` onu kullanır (davranış aynı). a11y: düğme adı sayıyı içerir ("Bildirimler, 3 okunmamış"), rozet `aria-hidden`, sayaç ayrıca `role="status"` ile duyurulur, okunmamış satırda gizli "Okunmamış" metni. Yeni renk yok; yalnızca mevcut token'lar. TR/EN `notifications.*` anahtarları.
- **Testler:** Go — hub (kullanıcı bazlı yayılım, eşzamanlı kullanım, `-race`); entegrasyon `notifications_integration_test.go`: alıcı seçimi ve tüm olay türleri, hedefe hiç satır üretilmemesi, sonradan bağlanınca silme + gizleme + sinyal, başkasının bildirimi 404 / oturumsuz 401 / bozuk kimlik 400, okundu ve tümünü okundu, zamanlayıcı (enjekte saatle 3g/1g/doldu, tekilleştirme, yakalama, tamamlanan grup, tarih değişimi, UTC+3 sınırı), `Run`'ın iptalde durması, bildirim akışı (işlemi yapan ve hedef olay almaz), rota sınıflandırması. Vitest `components/notifications.test.tsx` (10 senaryo: sayaç ve liste, tıklama, okunmuşa tekrar istek atılmaması, tümünü okundu, boş durum, SSE ile yenileme, yeniden bağlanma, metinler TR/EN, rozet). Playwright `e2e/notifications.spec.ts` (masaüstü + telefon).
- **Bilinen sınırlar:** Eski bildirimler silinmiyor (liste en yeni 50'yi gösterir; tablo büyürse temizlik işi eklenmeli). Kullanıcı hub'ı da bellek içi; birden çok replica'da LISTEN/NOTIFY gerekir ve zamanlayıcı her kopyada çalışır (`dedupe_key` çift satırı yine engeller). İlk dağıtımda tarihi geçmiş her açık grup için yöneticilere birer "süre doldu" bildirimi üretilir.

## 2026-11-13 — Grup kapak fotoğrafı

- **Model (K53):** Yeni `group_covers` tablosu (`group_id` birincil anahtar + FK ON DELETE CASCADE, `data` bytea, `content_type`, `etag`, `width`, `height`, `updated_at`, `updated_by` FK ON DELETE SET NULL). `groups` tablosuna `cover_version` (etag kopyası) ve `cover_admin_only` eklendi; hepsi `models.Migrate` (AutoMigrate) ile. Gerekçe: baytlar ayrı tabloda olduğu için grup/liste sorguları ve `GroupAccess` middleware'i (her istekte grubu okur) kapağa hiç dokunmaz. Sürüm için iki seçenek vardı: her grup sorgusunda `group_covers`'a ek sorgu/JOIN ya da `groups`'ta kopya sütun. Kopya seçildi: ek sorgu yok, liste ve detay yanıtı `cover_version`'ı kendiliğinden taşır; tutarlılık, kapak yazan/silen tek transaction'ın (grup satırı kilitli) iki tabloyu birlikte güncellemesiyle sağlanır. Sürüm, işlenmiş görselin SHA-256 özetinin ilk 16 baytıdır; aynı görsel aynı sürümü üretir.
- **Yetki (K54):** Varsayılan olarak tüm üyeler kapağı ekler/değiştirir/kaldırır; admin `PATCH /groups/:id/cover-policy {cover_admin_only}` ile kısıtlar (üye 403). İzin önce servis katmanında (görsel boşuna işlenmesin), sonra kayıt anında grup satırı kilitliyken güncel politikayla yeniden denetlenir; politika değişikliğiyle yarışan yükleme eski izinle geçemez.
- **Uçlar:** `GET` / `PUT` / `DELETE /groups/:id/cover` ve `PATCH /groups/:id/cover-policy`; hepsi `/groups/:id/*` altında, yani Anti-Spoiler + üyelik (`GroupAccess`), grup SSE yayını ve `Notify` zincirinden geçer. `DELETE` idempotenttir (kapak yoksa da 200). Swagger yeniden üretildi.
- **Görsel işleme (K55, `pkg/imageproc`, saf Go):** 10 MiB'a kadar JPEG/PNG/WebP (`golang.org/x/image` v0.47.0: WebP çözücü ve CatmullRom ölçekleme; bu sürüm mevcut `x/text` ile uyumlu olduğu için başka modül yükseltilmedi). Tür bayt imzasından belirlenir, `Content-Type` ve dosya adı dikkate alınmaz; HEIC/HEIF açık mesajla 415. Çözmeden önce `image.DecodeConfig`: kenar > 12000 ya da > 50 MP ise görsel çözülmez. EXIF yönü elle okunur (JPEG APP1 ve WebP EXIF parçası; bozuk EXIF yüklemeyi düşürmez, yön 1 sayılır), görsel ortadan 4:3 kırpılır, en çok 1600x1200'e küçültülür (büyütme yok; çıktı genişliği 4'ün katına yuvarlanır ki oran tam 4:3 olsun), şeffaflık beyaza yaylandırılır, JPEG kalite 80. Ölçekleme kaynak yönünde yapılır, yön küçük çıktıya uygulanır (daha az bellek). Yön uygulandıktan sonra 64x48'den küçük görsel 400 döner (4:3 kırpma anlamsızlaşmasın diye eklenen alt sınır). Aynı anda en fazla 2 görsel işlenir.
- **Gövde limiti:** `middleware.BodyLimitWithRoutes` yalnızca `PUT /api/v1/groups/:id/cover` için 10 MiB + 64 KiB sınır uygular; aynı yolun başka metotları ve diğer tüm rotalar 1 MiB'da kalır. Sınırı aşan `Content-Length` okunmadan, uzunluğu bildirilmeyen gövde okunurken 413 döner.
- **Üretim nginx'i:** `frontend/nginx.conf.template` API bloğunda `client_max_body_size 1m` olduğu için kapak yüklemesi üretimde 413 alırdı. Yalnızca `^/api/v1/groups/[^/]+/cover$` için 11m sınırlı ayrı bir blok eklendi (genel API bloğundan önce); diğer rotalar 1m'de kalır. Mevcut `covertgift-web:24f64a2` imajına yeni şablon bağlanarak yerelde denendi: 4 MB yükleme 200, `/ideas`'a 2 MB gövde 413, kapağa 12 MB 413. nginx tüm yanıtlara `Cache-Control: no-cache` eklediği için üretimde kapak yanıtı zaten her kullanımda doğrulanır.
- **Önbellek (K56):** `ETag` = `cover_version`, `If-None-Match` → 304, `Cache-Control: private, max-age=0, must-revalidate`, `Vary: Authorization`. `max-age` bilerek 0: tarayıcı önbelleği URL ile anahtarlandığı için süreli bir kopya, aynı tarayıcıda oturum açan başka bir kullanıcıya (ör. sonradan bağlanan hedef kişi) sunucuya sorulmadan verilebilirdi. Doğrulama her seferinde yetkiyi yeniden denetler; değişmeyen görselde baytlar veritabanından okunmadan 304 döner. İstemci ayrıca görseli TanStack Query'de sürümle önbelleğe alır.
- **Bildirim:** Kapak ve politika değişiklikleri bildirim üretmez; üç yeni rota `TestEveryMutationRouteHasANotificationDecision` tablosuna "bildirmez" olarak eklendi. Grup akışına `changed` olayı gider.
- **Frontend (K57):** Bağımlılık eklenmedi. `components/CoverCropDialog.tsx`: dosya seçimi (tür ve 10 MB istemci denetimi, HEIC için ayrı mesaj, sürükle-bırak), sabit 4:3 çerçeve, sürükleme (fare/dokunma), yakınlaştırma (tekerlek, iki parmak, kaydırıcı, `+`/`−`), ok tuşlarıyla kaydırma; çerçevede görünen alan kaydedilecek alanın kendisidir. Çıktı canvas ile en çok 1600x1200 JPEG (0.85). Hesaplar `src/lib/coverCrop.ts`'te (saf fonksiyonlar). `src/api/useGroupCover.ts`: görsel `fetch` + Bearer ile Blob alınır (token URL'de yok), sorgu anahtarı `['cover', grup, sürüm]` (grup listesi geçersiz kılınınca yeniden indirilmez), blob başına tek object URL kullanıcı sayısıyla tutulur ve son kullanan ayrılınca `URL.revokeObjectURL` çağrılır. `components/GroupCover.tsx`: `GroupCoverImage` (sabit oranlı kutu, fallback) ve `GroupThumb` (kare küçük görsel; yoksa `Avatar`).
- **Ekranlar:** Ana ekrandaki "Sıradaki sürpriz" kartı fotoğraf varsa solda metin / sağda 4:3 kutu (1280px'te kart 728x232, fotoğraf 278x209), 768px'te gün sayısı başlığın altına iner, telefonda fotoğraf kartın üstünde 16:9 şerit; fotoğraf yoksa kart birebir eski hali (sarmalayıcı `display: contents`). Grup sayfası başlığında 4:3 kutu + başlık; "Gruplarım" satırlarında ve grup seçicide kare küçük görsel. Yönetici "Grup işlemleri" menüsünde "Fotoğraf ekle / değiştir / kaldır" ve "Fotoğrafı yalnızca yönetici değiştirsin" anahtarını (menuitemcheckbox; menü açık kalır) görür. Üye, kısıt kapalıyken yalnızca fotoğraf işlemlerini içeren ayrı bir "Kapak fotoğrafı" düğmesi görür; kısıt açıkken hiçbir düğme görmez. `NewGroupDialog`'a isteğe bağlı fotoğraf alanı eklendi: grup kurulduktan sonra yüklenir; yükleme başarısız olursa grup kurulu kalır ve uyarı gösterilir.
- **Testler:** Go birim (`pkg/imageproc`): boyut ve oran (7 durum), çıktı boyutu, ortadan kırpma, 8 EXIF yönü × iki bayt sırası, bozuk EXIF, metadata'nın atılması (APP1–APP15/COM yok), çözünürlük bombası, bozuk/yanlış tür/HEIC/10 MB üstü/çok küçük, PNG şeffaflığı, WebP (şeffaf ve EXIF yönlü). `BodyLimitWithRoutes` birim testi. Entegrasyon (`cmd/api/cover_integration_test.go`): hedef kişi 4 uçta 403 + sürpriz mesajı (doğru ETag ile de; yanıt görsel/ETag/sürüm sızdırmaz, hiçbir şey değişmez), sonradan bağlanan hedef, üye kısıt kapalıyken 200 / açıkken 403, admin her zaman, politika ucunu üye çağıramaz, üye olmayan 403 / oturumsuz 401 / olmayan grup 404, sürüm değişimi, ETag 304, silince 404, liste/detay bayt taşımaz, saklanan görselin yeniden kodlanması, 11 hatalı yükleme + gövde limiti, grup/kurucu silinince CASCADE ve yükleyen silinince `updated_by` NULL, SSE olayı var / bildirim satırı yok. Anti-Spoiler rota tablosu 22 uca çıktı. Vitest: `lib/coverCrop.test.ts`, `components/cover.test.tsx` (kırpma diyaloğu, görsel bileşenleri, object URL temizliği), `pages/cover.test.tsx` (görünürlük kuralları, kısıt anahtarı, yükleme/kaldırma, ana ekran ve fallback, yeni grup akışı), `api/client.test.ts` (multipart gövde, Blob okuma). Playwright `e2e/cover.spec.ts` (masaüstü + telefon).
- **Tarayıcıda (test-db + seed):** 1280 / 768 / 375 px, açık ve koyu tema; 16:9, dikey ve 4:3 örnek görsellerle kırpma (fare, klavye, tekerlek, kaydırıcı), yükleme, API'den yapılan değişikliğin sayfa yenilenmeden yansıması, üye görünümü (kısıt açık/kapalı), yeni grup akışı; yatay taşma yok.
- **Bilinen sınırlar:** jsdom'un `FormData`'sı Node `fetch` üzerinden okunamadığı için multipart gövdenin içeriği sayfa testlerinde değil `client.test.ts`'te (fetch taklidiyle) ve Playwright'ta denenir. PNG `eXIf` yönü okunmaz (nadir). Kapaklar veritabanında durduğu için yedek boyutu grup başına ~150–400 KB artar. 50 MP sınırındaki bir görsel çözülürken birkaç yüz MB bellek kullanabilir; eşzamanlı işleme 2 ile sınırlıdır.
- **Doğrulama:** `go build`, `go vet`, `go test ./...` geçti; entegrasyon (`-p 1 ./cmd/...`) normal ve `-race` ile geçti; Vitest 133/133; `tsc -b`, `oxlint`, `npm run build` temiz; Playwright 12/12 (test API'sine yönlendirilmiş geçici :5174 sunucusuyla). `govulncheck`: yeni bağımlılıkta açık yok; Go 1.27.3 standart kütüphanesinde GO-2026-6859 (`net/http` MaxBytesReader, 1.27.4'te düzeltilmiş) raporlanıyor. Bu, kapaktan bağımsız olarak mevcut `BodyLimit` yolunu da kapsar; Go sürümü bu işte yükseltilmedi.

## 2026-11-13 — Grup açıklaması ve hedef hesap durumu

- **API (K58):** `PATCH /groups/:id/description {description}`; yalnızca admin (`requireAdmin`, bütçe/tarih uçlarıyla aynı desen). Metin kırpılır; kırpıldıktan sonra en fazla 1000 karakter (rune; Türkçe harfler tek sayılır); boş ya da yalnızca boşluk açıklamayı siler; alan eksik/`null`/yanlış tür 400. Üye 403, hedef kişi `GroupAccess` zinciriyle 403 + sürpriz mesajı. Yanıt güncel grup. Swagger yeniden üretildi.
- **Sınır kararı (K59):** Oluşturma ucunda `binding:"max=2000"` vardı; istenen sınır 1000 olduğu için iki uç aynı kurala (`services.normalizeDescription`, `MaxGroupDescription = 1000`) bağlandı ve oluşturmada da metin artık kırpılıyor. Sonuç: oluştururken 1001+ karakter artık 400. Veritabanındaki eski, 1000'den uzun açıklamalar olduğu gibi gösterilir; düzenlerken kısaltılması gerekir. `NewGroupDialog` alanı `maxLength=1000` ve etiketi "Not" yerine "Açıklama" oldu (anahtar `newGroup.notes` korunarak).
- **Bildirim ve canlılık:** Açıklama değişikliği bildirim üretmez; rota `TestEveryMutationRouteHasANotificationDecision` tablosuna "bildirmez" olarak eklendi. `/groups/:id/*` altında olduğu için grup akışına `changed` olayı gider (entegrasyon testiyle doğrulandı).
- **Hedef hesap adı (K60, Anti-Spoiler):** İnceleme sırasında bir sızıntı bulundu: `GetGroupByID` `TargetUser`'ı preload ettiği için `GET /groups/:id` her üyeye hedef kişinin **tam kullanıcı nesnesini (e-posta dahil)** `group.target_user` olarak dönüyordu. `models.Group.TargetUser` artık `json:"-"`; hiçbir yanıtta yer almaz. Grup detayı bunun yerine `target_account: {id, name}` (`services.AccountSummary`) döner; yalnızca üyeler alır (hedef 403). `GET /groups` ve `PATCH /target` yanıtları hedef hesap bilgisini hiç taşımaz. Ad yüklenemezse istemci `target_person_name`'e düşer.
- **Arayüz (K61):** Başlıktaki küçük "hesabı bağlı" rozeti kalktı. Başlık sütunu: başlık + "X için sürpriz · N üye · tarih" satırı (menü yanında), altında açıklama ve hesap durumu; kapak düzeni değişmedi. `pages/group/GroupHeaderInfo.tsx`:
  - `GroupDescription`: `whitespace-pre-line`, kapalıyken `line-clamp-3`; `scrollHeight > clientHeight` ölçülür (ResizeObserver + `document.fonts.ready`), taşmıyorsa düğme çizilmez. Düğme `aria-expanded` + `aria-controls`. Açıklama değişince yeniden kısalır. Açıklama yoksa yöneticiye kesik çizgili "Açıklama ekle", üyeye hiçbir şey.
  - `TargetStatus`: bağlıysa `bg-soft`/`text-on-soft` hap ve kobalt (`text-accent`) `ShieldCheck`, "{ad} hesabı bağlı", `title` + gizli metin "{ad} bu grubu göremez."; değilse mevcut `--pill-high-bg/ink` turuncu tonunda `ShieldAlert` ve "Hesap bağlı değil", yöneticiye yanında "Hesabı bağla" (menüdeki `LinkTargetDialog`'un aynısı; menü öğesi korunur), üyeye yalnızca bilgi. İkon `aria-hidden`; durum her zaman yazılı. Yeni renk token'ı eklenmedi.
  - Yönetici menüsünde "Açıklamayı düzenle" → `DescriptionDialog` (Textarea, `n / 1000 karakter` sayacı `aria-describedby` ile bağlı, aşınca kırmızı + `aria-invalid`, kaydetmede istemci hatası; sunucu hatası `role="alert"`; kaydet/vazgeç). Pencere her açılışta güncel açıklamayla başlar.
- **Testler:** Go birim — `normalizeDescription` (7 durum), `UpdateDescription` yetkileri, `targetAccount` en az alan. Entegrasyon (`cmd/api/description_integration_test.go`) — admin 200 + kırpma + güncel grup, detay/liste açıklamayı döner, üye 403 (sürpriz mesajı değil), hedef 403 + sürpriz mesajı, üye olmayan 403, oturumsuz 401, 1001 rune 400 / 1000 rune 200, eksik/`null`/yanlış tür 400, boş ve boşluk siler, oluşturmada 1001 → 400 ve kırpma, SSE olayı var / bildirim satırı yok / başarısız istek olay üretmez, `target_account` yalnızca `{id,name}` ve detayda hedefin e-postası yok, liste ve bağlama yanıtı hedef adını/e-postasını taşımaz, hedef detay/açıklama ucunda veri almaz, listesinde ve bildirimlerinde grup yok. Anti-Spoiler rota tablosu 23 uca çıktı. Vitest `pages/description.test.tsx` (14 senaryo: kes/aç ve aria, sığan metinde düğme yok, yönetici ipucu / üye boş, menüden düzenleme + kaldırma, 1001 karakter istemcide engellenir, sunucu hatası, üyede düzenleme yok, yeni grup alanı, bağlı durum + ad + title + aria-hidden, ad yoksa yedek, durumdan bağlama, üye yalnızca bilgi, İngilizce). `target.test.tsx` yeni metinlere uyarlandı. Playwright `e2e/description.spec.ts` (masaüstü + telefon: gerçek 3 satır kesme, aç/kapa, durumdan bağlama, düzenleme, üye ve hedef görünümü, yatay taşma yok).
- **Doğrulama:** `go build`, `go vet`, `go test ./...` geçti; entegrasyon (`-p 1 ./cmd/...`) normal ve `-race` ile geçti; Vitest 147/147; `tsc -b`, `oxlint`, `npm run build` temiz; Playwright 14/14 (test API'sine yönlendirilmiş geçici :5174 sunucusuyla). Tarayıcıda (test-db + seed): 1280 açık, 768 açık, 375 koyu/açık; uzun açıklama (kesik ve açık), açıklamasız grup (yönetici ipucu → kaydet), bağlı (seed: "Ahmet Şahin hesabı bağlı") ve bağlı olmayan grup; yatay taşma yok.

## 2026-11-13 — Kapak fotoğrafı: küçültme tarayıcıya, sunucu yalnızca küçük görsel kabul eder

- **Gerekçe:** Sunucu 12000 piksel kenara / 50 MP'ye kadar görseli tam çözüp işliyordu (K55). Tek bir 50 MP görselin RGBA tamponu ~200 MB, ölçekleme ara tamponlarıyla birkaç yüz MB eder; eşzamanlılık 2 olduğu için tepe bellek bunun iki katına çıkabilir. Azure'da API 1 GiB bellekle çalıştığından bu, konteyneri OOM ile öldürebilir. Arayüz zaten 4:3 kırpıp en çok 1600x1200 JPEG gönderdiği için büyük görseli sunucuda işlemenin meşru bir kullanıcısı yok.
- **Sunucu sınırları (K62, `pkg/imageproc`):** `MaxInputBytes` 10 MiB → **2 MiB**; `MaxDimension` 12000 → **2000** (uzun kenar), `MaxPixels` 50 MP → **4 MP** (2000x2000 tam sınırda kabul). Sınır yine `image.DecodeConfig` ile yalnızca başlıktan okunur; aşan görsel hiç çözülmez ve 400 + "Fotoğraf çok büyük. Lütfen tarayıcıda küçültülmüş (en çok 2000 piksel) bir görsel yükleyin." döner (`ErrDimensionsTooBig`). 2 MiB üstü 413 + "Fotoğraf en fazla 2 MB olabilir. Lütfen tarayıcıda küçültülmüş bir görsel yükleyin." (`ErrTooLarge`). Değişmeyenler: bayt imzası, HEIC 415, 64x48 alt sınır, EXIF yönü, metadata silme, 4:3 kırpma, 1600x1200'e küçültme, JPEG 80, eşzamanlılık 2. 2000 piksellik girdinin en kötü durumu ~16 MB RGBA + çıktı tamponu; iki eşzamanlı işlemle birkaç on MB.
- **Gövde sınırı (K63):** `middleware.CoverUploadMaxBodyBytes` = `imageproc.MaxInputBytes + 64 KiB` = **2 MiB + 64 KiB** (yalnızca `PUT /groups/:id/cover`; diğer rotalar 1 MiB). 10 MB'lık kullanıcı yükleme sınırı artık yok: sunucu 2 MiB üstünü okumadan 413 ile reddeder. `frontend/nginx.conf.template` kapak bloğu `client_max_body_size 11m` → **3m** (2 MiB + zarf için küçük pay).
- **Tarayıcı sorumluluğu (K64, `src/lib/coverCrop.ts`):** Büyük orijinal fotoğraf seçilebilir; istemci sınırı 10 MB → **40 MB** (`COVER_MAX_BYTES`) ve en çok 100 MP (`COVER_MAX_PIXELS`, tarayıcı belleği için). `prepareImage`: önce DOM'a eklenmemiş `<img>` ile yalnızca boyut okunur (piksel çözülmez); uzun kenar 3200'ü (`COVER_WORK_MAX_EDGE`) aşıyorsa `createImageBitmap(file, {imageOrientation: 'from-image', resizeWidth, resizeHeight, resizeQuality: 'high'})` ile küçültülmüş çözülür ve 3200 piksellik bir JPEG çalışma kopyasına (0.92) çevrilir; önizleme ve son çizim bu kopyadan yapılır, tam çözünürlüklü fotoğraf bellekte tutulmaz. Tarayıcı resize seçeneklerini desteklemiyor ya da boyutu farklı yorumluyorsa (bit eşlem boyutu beklenenle uyuşmazsa) eski yönteme (`<img>` → canvas) düşülür. `renderCrop` çıktıyı 0.85 ile kodlar; 2 MiB'ı (`COVER_UPLOAD_MAX_BYTES`) aşarsa 0.75 / 0.65 / 0.5 ile yeniden dener, yine aşarsa anlaşılır hata (`outputTooLarge`). HEIC artık seçimde reddedilmez: tarayıcı açabiliyorsa (Safari) JPEG'e çevrilip gönderilir, açamıyorsa (Chrome) "Bu tarayıcı HEIC fotoğrafları açamıyor…" mesajı. Yeni metinler TR/EN: `cover.error.dimensions`, `cover.error.outputTooLarge`; `tooLarge`, `heic`, `read` ve `pickHint` güncellendi.
- **Ödünleşim:** Çalışma kopyası 3200 piksel olduğundan büyük fotoğrafta yakınlaştırma ~2 katı geçince çıktı 1600 pikselden dar olur (büyütme yapılmadığı için yine keskin, yalnızca daha küçük). Sunucuya doğrudan (arayüz dışı) büyük görsel gönderen istemciler artık 400/413 alır; bilinen böyle bir istemci yok (eski `prototypes/test-ui` kapak desteklemez).
- **Testler:** Go birim — 4 MP sınırında 2000x2000 kabul, uzun kenar 2001 (yatay ve dikey) red, sınırların tutarlılığı (`2000² = 4 MP`), tam 2 MiB kabul / +1 bayt `ErrTooLarge`, mevcut boyut/oran durumları yeni sınırlara uyarlandı. `TestProcessCoverRejectsBeforeDecoding`: 64x64 veri taşıyıp başlığında başka boyut yazan PNG'ler; sınır üstü `ErrDimensionsTooBig` alır (tam çözme denenseydi `ErrCorrupt` olurdu), sınırdaki başlık boyut denetimini geçip çözmede `ErrCorrupt` alır — reddin çözmeden yapıldığını bellek ölçümü varsaymadan gösterir. `TestCoverUploadMaxBodyBytes`: değer 2 MiB + 64 KiB, sınırda 200, +1 bayt 413. Entegrasyon: 1600x1200 200; başlığı 6000x4000 diyen PNG, 2001 piksellik ve 2400x1800 JPEG 400 (mesaj kontrolü); 2 MiB üstü dosya, 3 MiB gövde ve uzunluğu bildirilmeyen büyük gövde 413; tam 2 MiB dosya 200. Vitest: 40 MB / 48 MP seçimi, HEIC'in çözücüye bırakılması, `workingSize`, HEIC/çözünürlük/çıktı hataları; dialog testleri `prepareImage` taklidine geçti. Playwright: yeni senaryo — 6000x4000 PNG doğrudan API'ye 400; arayüzde seçilince çalışma kopyası 3200x2133, kırpma 2844x2133, gönderilen dosya JPEG ve 2 MiB altında, yanıt 1600x1200.

## 2026-11-13 — Azure dağıtımı `f883801`

- **Kapsam:** `main` (`f883801`). Önceki canlı sürüm `24f64a2`'ydi (`515aee6` + #16 dağıtım dokümanı + #17 AI model geçişi: `gemini-3.6-flash`, yedek `gemini-3.5-flash-lite`, aşırı yükte 503). Yeni gelenler: #18 bildirim sistemi, #19 grup kapak fotoğrafı, #20 grup açıklaması + hedef hesap durumu (`target_user` sızıntısı düzeltmesi dahil), #21 küçük kapak yüklemesi ve tarayıcıda küçültme.
- **İmajlar:** yerel Docker ile `linux/amd64` için derlendi, ACR'ye `covertgift-api:f883801` ve `covertgift-web:f883801` olarak gönderildi, iki Container App `az containerapp update --image` ile güncellendi. Web imajındaki tek yapılandırma farkı `frontend/nginx.conf.template`'teki kapak bloğu: yalnızca `^/api/v1/groups/[^/]+/cover$` için `client_max_body_size 3m` (diğer API rotaları 1m).
- **Şema:** API başlarken AutoMigrate yeni `notifications` ve `group_covers` tablolarını, `groups.cover_version` ve `groups.cover_admin_only` sütunlarını ekledi. Silinen tablo/sütun yok.
- **Yerel nginx smoke (dağıtımdan önce):** web imajı ölü `API_TARGET` ile çalıştırıldı: `/` 200, `/api/v1/DEV/` 403, kapak bloğunda `client_max_body_size 3m`.
- **Canlı kontroller (yalnızca okuma):** `/health` 200 (`db: up`, yani AutoMigrate başarılı); oturumsuz `/api/v1/me`, `/api/v1/notifications` ve `/api/v1/groups/<id>/cover` 401; `/api/v1/dev/*` nginx'te 403; iki uygulama da 0–1 replica ve `Running`.
- **Dürüstlük notu (K65):** Canlıya veri yazan istek atılmadı. Ancak kontrol listesine yanlışlıkla bir `GET /api/v1/DEV/reset-db` dahil edildi. İstek nginx'in `/api/v1/dev/*` (büyük/küçük harf duyarsız) engeline takılıp 403 aldı ve API'ye ulaşmadı; ulaşsaydı da uç yalnızca `POST` kabul eder ve production'da (`APP_ENV=production`) her durumda kapalıdır. Yine de `docs/AZURE.md`'deki kural geçerlidir: engel kuralları çalışan bir API'ye karşı denenmez, yalnızca yerelde ölü hedefle denenir.
- **İşletim notları:** API 0.5 vCPU / 1 GiB, kapak işleme eşzamanlılığı 2; tek replica kuralı artık hem grup SSE'si hem kullanıcı bildirim SSE'si için geçerli. Hatırlatma zamanlayıcısı API içinde açılışta ve saatlik çalışır; API 0 replikadan uyanınca kaçanları yakalar (`dedupe_key` ile bir kez). İlk dağıtımda tarihi geçmiş açık gruplar için yöneticilere birer "süre doldu" bildirimi üretilmesi beklenir.

## 2026-11-13 — Birden çok kişi için gruplar (MULTI): backend

- **Karar:** SINGLE gruplar grup sütunlarını kullanmaya devam eder; örtük havuz ve veri taşıma yok. Kod kapsamla parametrelenir: servislerde `giftScope`, depoda `repository.IdeaScope` (SINGLE filtresi `group_id = ? AND pool_id IS NULL`). Bütçe/ilgi alanı/tarih/kazanan/yeniden açma gövdeleri `scope_repo.go`'daki ortak yardımcılara taşındı; grup deposu ince sarmalayıcı oldu. Refaktörden sonra mevcut testler değişmeden geçti.
- **Şema:** `groups.mode` (CHECK), `gift_pools`, `pool_contributions`, `gift_ideas.pool_id`, `notifications.pool_id` (hepsi AutoMigrate; FK'lar entegrasyon testinde doğrulanır).
- **Anti-Spoiler:** `PoolAccess` (gövde okunmadan önce), `IdeaGuard` (`/ideas/:id/*`, gövde doğrulamasından önce), tüm `/ideas/:id/*` işlemleri tek `resolveIdea` yardımcısından geçer. Kilitli kart ayrı tiptir (`LockedPool`). Liste sayaçları, durum ve `next_event_date` yalnızca görünen havuzlardan; bildirim süzgeci `pool_id`'ye göre; `Dispatch` alıcıyı dışlar; havuz değişiklikleri SSE'de havuz anahtarına yayınlanır ve akış yalnızca görünen havuzlara abone olur (`Hub.SubscribeMany`).
- **Yarışlar:** fikir ekleme/AI kaydı kapsam satırı `FOR SHARE` kilitliyken durumu yeniden denetler; oy + çoğunluk vetosu tek transaction'da (kapsam `FOR SHARE`, fikir `FOR UPDATE`), hatalar artık yutulmaz. Alıcı ekleme ile üye ekleme/davetle katılma grup satırı kilidiyle sıralanır; ikisi de katkı satırı yazar.
- **Testler:** `cmd/api/pools_integration_test.go` (rota tablosu Anti-Spoiler, anlık görüntü değişmezliği, SSE, bildirim/hatırlatma, katılma yolları, katkı, yönetici yetkileri, alıcı kuralları, hesap silme, eşzamanlılık, şema); servis birim testleri; seed'e "Ofis Yılbaşı" MULTI grubu.

## 2026-11-13 — Davet linki ve MULTI frontend

- **Davet linki:** `group_invites` (yalnızca token özeti saklanır), `GET/POST/DELETE /groups/:id/invite` (admin), `GET /invites/:token`, `POST /invites/:token/accept`. Katılma, üye ekleme ve hedef bağlama ile aynı grup satırı kilidini kullanır; hedef kişi önizleme ve katılmada 403 alır. Token API logunda rota şablonuyla, nginx'te `access_log off` ile gizlenir (nginx yalnızca `nginx -t` ile doğrulandı). Frontend: üyeler panelinde "Davet linki" bölümü, `/invite/:token` karşılama sayfası, giriş/kayıt sonrası davete dönüş (`lib/returnPath.ts`, açık yönlendirme korumalı). Hedef hesabı bağlı değilken uyarı gösterilir.
- **MULTI frontend:** yeni grup penceresinde tip seçimi; grup sayfasında kişi sekmeleri (`?pool=`), kilitli kart, katılımcılar paneli ve ortaklık anahtarı, üyeler panelinde hediye alınacak kişi / sorumlu yönetimi. Mevcut bileşenler `scope` + `canManage` ile parametrelendi (`pages/group/scope.ts`); `PoolRef` tipi kilitli havuz için istek kurmayı derleme hatası yapar. Canlı güncelleme, sürmekte olan değişiklik bitene kadar ertelenir (yutulmaz).
- **İnceleme sonucu düzeltilenler:** önizleme isteğinin grup akışına olay göndermesi (zamanlama sızıntısı), fikir ekleme ile kişi çıkarma arasındaki kilitlenme (tek kilit sırası: grup → havuz), veto/silme/oy geri almanın kazanan seçimiyle yarışı, `next_event_date`'in yaklaşan tarihi öncelemesi.
- **Lint:** `react/exhaustive-deps` hata düzeyinde açıldı; ihlal yok.
- **Doğrulama:** Go birim + entegrasyon testleri, Vitest 218/218, Playwright 18/18 (test veritabanı), tarayıcıda masaüstü ve 375 px kontrolü.

## 2026-11-14 — Davet linki ve MULTI: açık kalanların kapatılması

- **nginx davet logu (gerçek açık):** `location ^~ /invite/ { access_log off; }` işlemiyordu: `try_files` isteği `/index.html`'e iç yönlendirdiği için log, sonlandıran location'ın ayarıyla yazılıyor ve token erişim loguna düşüyordu. Karar artık özgün yola göre veriliyor (`map $request_uri $loggable` + `access_log … if=$loggable`); `/api/v1/invites/` rotasında hata logu `crit` düzeyine çekildi (upstream hatası istek satırını token'la yazıyordu). Yerel konteynerde ölü `API_TARGET` ile doğrulandı: davet yolları için log satırı yok, diğer yollar loglanıyor, güvenlik başlıkları ve `/api/v1/dev` engeli duruyor.
- **Hesap silme kilit sırası:** `userRepo.Delete` kullanıcının dokunduğu grupları, sonra o grupların havuzlarını kimlik sırasıyla kilitliyor (grup → havuz). `TestConcurrentAccountDeleteVsPoolWrites` düzeltme olmadan `deadlock detected` ile 500 veriyor, düzeltmeyle `-race -count=5` geçiyor.
- **Gemini sırasında canlı güncelleme:** AI isteği ayrı mutation anahtarına (`mutationKeys.ai`) alındı; öneri üretilirken grubun SSE yenilemeleri artık ertelenmiyor.
- **Etkisiz sorumlu değişikliği:** `PATCH /recipients/:userId/manager` hiçbir şey değiştirmediyse grup akışına olay göndermiyor (`controllers.SkipRealtimePublish`).
- **Havuzda gerçek Gemini çağrısı:** test veritabanına bağlı geçici API ile bir kez denendi: 201, 4 öneri, hepsi `pool_id` taşıyor ve `product_url` boş; hediye alınan kişi 403 aldı, bildirim listesi değişmedi; yönetici `ai_ideas_added` bildirimini `pool_id` ile aldı.
- **Oynaklık:** entegrasyon paketi tek başına 4/4 geçti; önceki tek başarısızlık aynı test veritabanını kullanan eşzamanlı bir koşuya denk gelmişti.
- **Kilitli kartta sorumlu (yalnızca yönetici):** `LockedPool.manager` yalnızca çağıran grup yöneticisiyse ve kendi havuzuna sorumlu atanmışsa döner (sorumluyu kendisi atadığı için yeni bilgi değildir); diğer alıcıların kilitli kartında alan hiç yoktur ve test bunu ayrı ayrı doğrular. Sorumlu değiştirme penceresi mevcut sorumluyu önceden seçer.

