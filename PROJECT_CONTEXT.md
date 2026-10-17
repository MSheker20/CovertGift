# PROJE TEKNİK ŞARTNAMESİ: CovertGift

## 1. Proje Özeti ve Rolün
Sen **CovertGift** projesini geliştiren uzman bir full-stack yazılım mühendisisin.
Bu proje; arkadaş gruplarının ortak hediye grupları kurduğu, bütçe birleştirdiği, Gemini 3 Flash üzerinden yapay zekâ destekli semantik hediye önerileri aldığı, önerileri oyladığı (+1 / -1) ve hediye alınacak kişiden süreci gizlediği (Anti-Spoiler koruması) işbirlikçi bir web platformudur.

---

## 2. Teknoloji Yığını (Tech Stack)
- **Backend:** Go (Golang) + **Gin Framework** (`github.com/gin-gonic/gin`)
- **Veritabanı:** PostgreSQL (Docker üzerinde) + **GORM**
- **Kimlik Doğrulama:** JWT (JSON Web Tokens) ve özel Gin middleware'i
- **Frontend:** React (Vite) + Tailwind CSS + shadcn/ui + Lucide Icons
- **Geçici test paneli:** `prototypes/test-ui/` altında HTML/CSS/JavaScript + Node.js 22+ statik sunucu ve API proxy'si; kalıcı frontend Faz 7'de geliştirilecek
- **Yapay Zekâ Katmanı:** `google.golang.org/genai` ile Google Search araştırması + `response_schema` ile JSON seçimi. Kod varsayılanı `gemini-3.8-flash`; `GEMINI_MODEL` ile değiştirilebilir. 503 hatasında `gemini-3.5-flash-lite` yedeği denenir; kota ve erişim kullanılan API hesabına bağlıdır.
- **Konteynerizasyon:** PostgreSQL için Docker Compose

---

## 3. Veritabanı Modelleri (5 Temel Varlık / Entity)

### 1. `User` (Kullanıcılar)
- `id`: UUID (Primary Key, varsayılan gen_random_uuid())
- `name`: VARCHAR(100), boş geçilemez
- `email`: VARCHAR(150), benzersiz (unique), boş geçilemez
- `password_hash`: VARCHAR(255), boş geçilemez
- `avatar_url`: VARCHAR(255), opsiyonel
- `created_at`: TIMESTAMP, varsayılan now()

### 2. `Group` (Hediye Grupları)
- `id`: UUID (Primary Key)
- `title`: VARCHAR(150), boş geçilemez
- `description`: TEXT
- `target_person_name`: VARCHAR(100), boş geçilemez
- `target_interests`: TEXT -- Hedef kişinin ilgi alanları. Liste öğeleri satır sonuyla saklanır; eski virgül/noktalı virgül ayrımlı metinler de okunur. Grup detayındaki `interests: string[]` alanı arayüz için ayrıştırılmış listedir. Gemini bağlamı bu metinden üretilir.
- `target_user_id`: UUID (Opsiyonel, Foreign Key -> `users.id`) -- Anti-Spoiler kontrolü için kullanılır
- `total_budget`: NUMERIC(10, 2), varsayılan 0.00
- `creator_id`: UUID (Foreign Key -> `users.id`), boş geçilemez
- `created_at`: TIMESTAMP, varsayılan now()

### 3. `GroupMember` (Grup Üyelikleri)
- `id`: UUID (Primary Key)
- `group_id`: UUID (Foreign Key -> `groups.id`, ON DELETE CASCADE), boş geçilemez
- `user_id`: UUID (Foreign Key -> `users.id`, ON DELETE CASCADE), boş geçilemez
- `role`: VARCHAR(20), varsayılan 'member' ('admin' veya 'member')
- `joined_at`: TIMESTAMP, varsayılan now()
- *Kısıtlama:* UNIQUE(`group_id`, `user_id`)

### 4. `GiftIdea` (Hediye Önerileri / Havuz)
- `id`: UUID (Primary Key)
- `group_id`: UUID (Foreign Key -> `groups.id`, ON DELETE CASCADE), boş geçilemez
- `title`: VARCHAR(200), boş geçilemez
- `description`: TEXT
- `estimated_price`: NUMERIC(10, 2), varsayılan 0.00
- `source_type`: VARCHAR(20), varsayılan 'AI' ('AI' veya 'MANUAL')
- `status`: VARCHAR(20), varsayılan 'ACTIVE' ('ACTIVE', 'REJECTED', 'SELECTED') -- Admin vetosu veya çoğunluk downvote ile 'REJECTED' olur
- `is_over_budget`: BOOLEAN, varsayılan false -- Bütçeyi aşan fikirlerde true olur (fikir elenmez, grubun toleransına bırakılır)
- `search_query`: VARCHAR(255), boş geçilemez -- Pazar yerleri için optimize edilmiş arama terimi
- `image_url`: VARCHAR(500), opsiyonel
- `product_url`: VARCHAR(1000), opsiyonel -- Sadece `MANUAL` önerilerde, kullanıcının yapıştırdığı linkin çözümlenmiş (final) ürün adresi. AI önerilerinde daima boştur.
- `ai_sources`: JSONB, opsiyonel -- Google grounding metadata'sından alınan `{ title, url }` kaynak listesi; modelin ürettiği URL'ler kullanılmaz
- `ai_search_suggestions`: TEXT, opsiyonel -- Google'ın döndürdüğü arama önerisi HTML'i; mevcut panelde gösterilmez
- `created_by`: UUID (Foreign Key -> `users.id`), boş geçilemez
- `created_at`: TIMESTAMP, varsayılan now()

### 5. `Vote` (Oylama Verileri)
- `id`: UUID (Primary Key)
- `gift_idea_id`: UUID (Foreign Key -> `gift_ideas.id`, ON DELETE CASCADE), boş geçilemez
- `user_id`: UUID (Foreign Key -> `users.id`, ON DELETE CASCADE), boş geçilemez
- `vote_type`: SMALLINT, boş geçilemez -- (+1 Upvote, -1 Downvote)
- `created_at`: TIMESTAMP, varsayılan now()
- *Kısıtlama:* UNIQUE(`gift_idea_id`, `user_id`)

---

## 4. Temel İş Mantığı ve Mimari Kurallar (Business Logic)

1. **Anti-Spoiler Koruması (Kritik Kural):**
   - Gruba ait tüm API uç noktalarında (`/api/v1/groups/:id/*`), giriş yapmış kullanıcının `authenticated_user.id` değeri grubun `target_user_id` değerine eşitse işlem anında kesilmelidir.
   - Durum kodu: `403 Forbidden`. Hata mesajı: `{"error": "Erişim engellendi: Bu hediye grubu senin için oluşturulmuş bir sürprizdir."}`.
   - Hedef kişi hediye havuzunu, bütçeyi veya oyları kesinlikle göremez.

2. **Dinamik Kişi Başı Bütçe Dağılımı:**
   - Kişi başı tutar veritabanında statik tutulmaz, anlık hesaplanır:
     $$\text{Kişi Başı Bütçe} = \frac{\text{group.total\_budget}}{\text{count(group\_members)}}$$
   - Grup detay cevabı dönerken bu alan otomatik eklenir.

3. **Kırık Link Engelleme (Dinamik Arama Bağlantısı Modeli):**
   - Yapay zekâ (Gemini) kesinlikle doğrudan URL üretmeyecek (kırık link ve bot engellerini önlemek için).
   - Gemini yalnızca temiz `search_query` metni üretir (Örn: `"Logitech G29 Direksiyon Seti"`).
   - Frontend bu arama metnini pazar yeri linklerine dinamik olarak dönüştürür:
     - Trendyol: `https://www.trendyol.com/sr?q=${encodeURIComponent(idea.search_query)}`
     - Amazon TR: `https://www.amazon.com.tr/s?k=${encodeURIComponent(idea.search_query)}`

4. **Token Tasarrufu Sağlayan Prompt Yapısı (State-Based Context):**
   - Sohbet geçmişi biriktirilmez.
   - Veritabanından grubun güncel ilgi alanları, toplam bütçesi, mevcut ve `REJECTED` durumundaki ürün başlıkları çekilir. Tek bir negatif oy, çoğunluk vetosu eşiğine ulaşmadıkça fikri reddedilmiş yapmaz.
   - Bu bağlam araştırma ve JSON seçimi aşamalarında kullanılır. Hedef 3–5 farklı öneridir; doğrulamadan geçen daha az ürün varsa sonuç sayısı azalır.

   **Türkiye ürün araştırması (2026-10-16):** AI üretimi iki aşamalıdır: Google Search grounding ile izinli Türkiye mağazalarındaki somut ürünler ve TL fiyatları araştırılır; yalnızca API kaynaklarıyla desteklenen araştırma parçaları yapılandırılmış önerilere dönüştürülür. Seçim aşamasında `evidence_ids` ile kaynak eşleştirilir; ürün adı, arama metni ve fiyatın destekleyen araştırmada geçmesi doğrulanır. Kaynaksız, eksik, fiyatı geçersiz ve tekrar eden kayıtlar elenir; gerekirse 3'ten az öneri döner. Arama sonuçları anlık stok garantisi değildir. `ai_sources` alanı Google'ın döndürdüğü kaynak başlığı ve adresini taşır; `product_url` AI için boş kalır. `ai_search_suggestions` Google'ın arama önerisi HTML'idir; mevcut geçici panel bunu henüz göstermez. Kalıcı frontend'de izole gösterim planlanır. Kaynak bulunamaması şu an controller'ın genel hata yolundan 500 döner; servis filtresinden sonra hiç öneri kalmaması 422, API anahtarı eksikliği 503 döner.

5. **Ürün Linki Önizleme (Link Preview / Manuel Öneri Zenginleştirme):**
   - Kullanıcı manuel öneri eklerken bir ürün linki yapıştırabilir (Örn: Trendyol paylaşım linki `https://ty.gl/...`). Backend sayfayı çekip başlık, açıklama, görsel ve fiyatı otomatik doldurur.
   - Bu link **kullanıcı tarafından sağlanır**; Gemini hâlâ URL üretmez (Kural 3 geçerliliğini korur).
   - **İstek kimliği:** `User-Agent: WhatsApp/2.21.12.21 A` (sosyal medya önizleme botu) + `Accept-Language: tr-TR`. Çerez kavanozu (cookiejar) aktif, yönlendirmeler takip edilir, zaman aşımı 10 sn.
   - **Takip (tracker) sayfası çözümleme:** Kısa linkler Adjust gibi ara sayfalarda (`*.adj.st`, `*.adjust.com`) `200 OK` ile durabilir. Bu sayfalardaki meta etiketleri (`adj_og_*`) jenerik pazarlama verisidir. Son URL bir tracker ise `adjust_redirect` sorgu parametresi decode edilip gerçek ürün sayfasına ikinci istek atılır.
   - **Veri çıkarma önceliği:**
     - Başlık: `og:title` -> `<title>`
     - Görsel: `og:image`
     - Açıklama: `og:description`
     - Fiyat: `product:price:amount` -> JSON-LD (`schema.org/Offer`) `"price"` alanı
   - **Jenerik görsel filtresi:** Görsel URL'si `discovery-web`, `banner`, `mrktng`, `/crm/`, `popup`, `automationimages` içeriyorsa ürün resmi sayılmaz ve `image_url` boş bırakılır.
   - **Meta regex kuralı:** `content` değeri çift ve tek tırnak için ayrı kalıplarla ayrıştırılmalıdır (`content="..."` içinde `'` karakteri bulunabilir, Örn: "Trendyol'da").
   - **`search_query` türetme:** Başlıktan marka/fiyat ekleri ("- Fiyatı", satıcı adı vb.) temizlenerek `search_query` üretilir; böylece manuel öneriler de Trendyol/Amazon dinamik arama linklerini destekler.
   - **Güvenlik (SSRF koruması):** Sadece `http/https` şemaları kabul edilir; izinli alan adı listesi (allowlist: `ty.gl`, `trendyol.com`, `adj.st`, `amazon.com.tr`, `amzn.eu`, `hepsiburada.com` vb.) dışındaki hostlar ve özel/yerel IP aralıkları (`127.0.0.0/8`, `10.0.0.0/8`, `192.168.0.0/16`, `169.254.0.0/16`) reddedilir. Yanıt gövdesi en fazla 2 MB okunur, en fazla 10 yönlendirme takip edilir.
   - Önizleme başarısız olursa hata fırlatılmaz; kullanıcı formu elle doldurmaya devam edebilir (`{"success": true, "data": {"partial": true, ...}}`).

---

## 5. REST API Uç Noktaları (REST Contract)

### Kimlik Doğrulama (Auth)
- `POST /api/v1/auth/register` -> İstek: `{ name, email, password }` -> `201 Created`
- `POST /api/v1/auth/login` -> İstek: `{ email, password }` -> `200 OK` + `{ token, user }`
- `GET /api/v1/me` -> Giriş yapan kullanıcının kendi profilini döner (Bearer token ile)
- `DELETE /api/v1/me` -> Giriş yapan kullanıcının kendi hesabını, oluşturduğu grupları, fikirleri ve oylarını siler (Cascade & atomik temizlik)

### Grup İşlemleri
- `GET /api/v1/groups` -> Kullanıcının üye olduğu grupları listeler (hedef kişi olduğu gruplar filtrelenir)
- `POST /api/v1/groups` -> Yeni grup açar (oluşturan kişi `admin` üye yapılır)
- `GET /api/v1/groups/:id` -> Grup detayları, üye sayısı, kişi başı bütçe (Anti-Spoiler korumalı)
- `POST /api/v1/groups/:id/members` -> Gruba e-posta ile üye ekler
- `POST /api/v1/groups/:id/interests` -> `{ interest: string }` ile listeye tek ilgi alanı ekler; tüm grup üyeleri ekleyebilir. Başındaki/sonundaki boşluklar temizlenir; 1–100 karakter, virgül/noktalı virgül/satır sonu içermez. Tekrar ekleme 409 döner. Yanıt `{ interests: string[] }` olur.
- `DELETE /api/v1/groups/:id/interests` -> `{ interest: string }` ile yalnızca **admin** bir öğeyi silebilir; normal üye kendi eklediğini de silemez (403). Anti-Spoiler ve üyelik kontrolü her iki uçta da zorunludur. Eski toplu `PATCH` yolu kaldırılmıştır; tüm listeyi değiştirerek silme yetkisi aşılamaz. Eşzamanlı eklemeler transaction + satır kilidiyle korunur.
- `PATCH /api/v1/groups/:id/budget` -> `{ total_budget: decimal }` ile yalnızca **admin** toplam bütçeyi günceller. Aralık 0–99999999.99, en fazla iki ondalık basamaktır. Kişi başı tutar anlık hesaplanır; mevcut hediye bütçe aşımı bayrakları aynı transaction'da güncellenir ve listelemede güncel bütçeye göre hesaplanır. Mevcut kurala uygun olarak 0 bütçe aşımı uyarılarını kapatır. Anti-Spoiler ve üyelik kontrolü zorunludur.
- `DELETE /api/v1/groups/:id` -> Grubu ve bağlı tüm verileri siler (Sadece grup admini silebilir; Anti-Spoiler korumalı)

### Hediye Havuzu ve Oylama
- `GET /api/v1/groups/:id/ideas` -> Hediyeleri toplam oy skoruyla birlikte çeker (`SUM(votes.vote_type)`)
- `POST /api/v1/groups/:id/ideas` -> Manuel hediye önerisi ekler (opsiyonel `product_url` ile; link önizlemeden gelen veriler formu doldurur)
- `POST /api/v1/groups/:id/ideas/preview-link` -> İstek: `{ url }` -> `200 OK` + `{ title, description, image_url, price, final_url, search_query }` (Veritabanına yazmaz, sadece önizleme döner; Anti-Spoiler korumalı)
- `POST /api/v1/groups/:id/ideas/generate-ai` -> Google Search araştırması ve JSON seçimiyle en fazla 5 kaynaklı öneriyi kaydeder; `201` + `GiftIdea[]` döner. `ai_sources` ve `ai_search_suggestions` yanıta dahildir. 3'ten az sonuç dönebilir; mevcut hata eşlemeleri İş Kuralı 4'te açıklanır.
- `POST /api/v1/ideas/:id/vote` -> Oy verir veya günceller `{ vote_type: 1 | -1 }` (Conflict durumunda güncelleme; downvote sayısı çoğunluğu aşarsa fikri otomatik REJECTED yapar)
- `POST /api/v1/ideas/:id/reject` -> Grup admini doğrudan hediye fikrini veto eder (`status = REJECTED`)
- `DELETE /api/v1/ideas/:id` -> Hediyeyi kaldırır (Sadece öneren kişi veya grup admini silebilir)

### Sistem & Test
- `POST /api/v1/dev/reset-db` -> Test ve sunum ortamında tüm tabloları TRUNCATE CASCADE ile sıfırlar (Production ortamında güvenlik için devre dışıdır)
- Geçici panel proxy'si yalnızca auth, profil, grup, hediye ve oy yollarıyla `/health`, `/swagger/*`, `/docs` yollarını iletir. Geliştirme uçları 403 ile engellenir. Cloudflare tüneli bu proxy'nin 5173 portuna bağlanır.

---

## 6. Kodlama ve Mimari Standartlar
- **Go Backend:**
  - Standart Go paket mimarisi:
    ```
    backend/
    ├── cmd/api/main.go
    ├── internal/
    │   ├── config/
    │   ├── controllers/
    │   ├── middleware/
    │   ├── models/
    │   ├── repository/
    │   └── services/
    ├── pkg/gemini/
    ├── pkg/linkpreview/       # Ürün linki önizleme (OG / JSON-LD scraper)
    └── go.mod
    ```
  - Hata kontrolleri açıkça yapılmalıdır (`if err != nil`).
  - Veritabanı ve migrasyonlar için GORM kullanılmalıdır.
  - JSON yanıtları standart formatta dönmelidir: `{"success": true, "data": ...}` veya `{"success": false, "error": "mesaj"}`.

- **React Frontend:**
  - Tek ekranlı modern panel tasarımı: Üst barda kullanıcı profili ve bütçe durumu; orta alanda oylama kartları; sağ yan panelde grup üyeleri ve Gemini öneri butonu.
  - API istekleri temiz bir `api/` katmanı üzerinden `fetch` veya `axios` ile yönetilmelidir.
