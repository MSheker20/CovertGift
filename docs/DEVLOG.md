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
