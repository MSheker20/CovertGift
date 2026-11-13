# PROJE TEKNİK ŞARTNAMESİ: CovertGift

> **Güncel AI modu (2026-10-21):** Google Search kapalıdır (`GEMINI_SEARCH_ENABLED=false`).
> `GEMINI_MODEL=gemini-3.8-flash` ile tek çağrıda öneri üretilir; fiyatlar doğrulanmamış tahmindir.
> Model kimlikleri uygulama kodunda sabit değildir. `GEMINI_MODEL` anahtar varsa zorunludur.
> `GEMINI_FALLBACK_MODEL` boşsa otomatik model geçişi yapılmaz; doluysa yalnızca 503'te denenir.
> `GEMINI_RESEARCH_MODEL` yalnızca Search açıkken kullanılır; boşsa ana model kullanılır.
> Yerelde `backend/.env`, Azure'da Container App ortam değişkenleri düzenlenir ve uygulama yeniden başlatılır/yeni revision oluşur.
> Aşağıdaki eski Search araştırması açıklamaları yalnızca Search modu tekrar açılırsa geçerlidir.

## 1. Proje Özeti ve Rolün
Sen **CovertGift** projesini geliştiren uzman bir full-stack yazılım mühendisisin.
Bu proje; arkadaş gruplarının ortak hediye grupları kurduğu, bütçe birleştirdiği, Gemini 3 Flash üzerinden yapay zekâ destekli semantik hediye önerileri aldığı, önerileri oyladığı (+1 / -1) ve hediye alınacak kişiden süreci gizlediği (Anti-Spoiler koruması) işbirlikçi bir web platformudur.

---

## 2. Teknoloji Yığını (Tech Stack)
- **Backend:** Go (Golang) + **Gin Framework** (`github.com/gin-gonic/gin`)
- **Veritabanı:** PostgreSQL (Docker üzerinde) + **GORM**
- **Kimlik Doğrulama:** JWT (JSON Web Tokens) ve özel Gin middleware'i
- **Frontend:** `frontend/` — React 19 (Vite) + TypeScript + Tailwind CSS v4 + shadcn tarzı bileşenler (`radix-ui`) + Lucide Icons; React Router (lazy rotalar), TanStack Query, i18next (TR/EN). Testler: Vitest + Testing Library + MSW, Playwright. Üretimde ayrıcalıksız nginx statik dosyaları sunar ve `/api`'yi API'ye iletir.
- **Eski geçici test paneli:** `prototypes/test-ui/` altında HTML/CSS/JavaScript + Node.js 22+ statik sunucu ve API proxy'si; React frontend yerini aldı, yeni uçları (ör. `PATCH /groups/:id/target`) desteklemez.
- **Yapay Zekâ Katmanı:** `google.golang.org/genai`. Güncel mod üstteki nottadır (Search kapalı, `GEMINI_MODEL` ile tek yapılandırılmış JSON çağrısı). Search açıkken araştırma `GEMINI_RESEARCH_MODEL`, JSON seçimi `GEMINI_MODEL` ile yapılır; `GEMINI_FALLBACK_MODEL` yalnızca doluysa 503'te denenir. Kota/istek sınırı hatası 429 döner.
- **Konteynerizasyon:** Yerelde PostgreSQL için Docker Compose; API (`backend/Dockerfile`) ve frontend (`frontend/Dockerfile`, nginx) için ayrı imajlar. Azure demo dağıtımı: [docs/AZURE.md](docs/AZURE.md)

---

## 3. Veritabanı Modelleri (5 Temel Varlık / Entity + Bildirimler + Grup Kapakları)

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
- `target_user_id`: UUID (Opsiyonel, Foreign Key -> `users.id`) -- Anti-Spoiler kontrolü için kullanılır. Grup kurulurken `target_email` ile ya da sonradan admin tarafından `PATCH /groups/:id/target` ile doldurulur; bir kez bağlanır, değiştirilmez ve kaldırılmaz. Boşsa yalnızca `target_person_name` kullanılır ve Anti-Spoiler devreye girmez.
- `total_budget`: NUMERIC(10, 2), varsayılan 0.00
- `creator_id`: UUID (Foreign Key -> `users.id`), boş geçilemez
- `created_at`: TIMESTAMP, varsayılan now()
- `event_date`: DATE (Opsiyonel) -- Hediyenin verileceği gün (doğum günü vb.), tek seferlik. JSON'da `"YYYY-MM-DD"`. Bugünden en fazla 1 yıl önce, en fazla 5 yıl sonra olabilir ("bugün" UTC+3'e göre). Frontend geri sayım gösterir ve listeyi yaklaşan tarihe göre sıralar.
- `status`: VARCHAR(20), varsayılan 'ACTIVE' ('ACTIVE', 'COMPLETED') -- Admin kazananı seçince 'COMPLETED' olur; tamamlanan grupta fikir ekleme, AI önerisi, oy verme/geri alma, veto ve fikir silme 409 döner.
- `completed_at`: TIMESTAMP (Opsiyonel) -- Grubun tamamlandığı an; yeniden açılınca boşalır.
- `cover_version`: VARCHAR(64), varsayılan '' -- Kapak fotoğrafının sürümü (`group_covers.etag` kopyası); fotoğraf yoksa boştur ve JSON'da yer almaz. Kapak değişirken aynı transaction'da güncellenir. Grup ve liste sorguları kapak baytlarına hiç dokunmaz; istemci bu değeri önbellek anahtarı olarak kullanır.
- `cover_admin_only`: BOOLEAN, varsayılan false -- true ise kapak fotoğrafını yalnızca admin ekler/değiştirir/kaldırır; false iken tüm üyeler yapabilir.

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
- `status`: VARCHAR(20), varsayılan 'ACTIVE' ('ACTIVE', 'REJECTED', 'SELECTED') -- Admin vetosu veya çoğunluk downvote ile 'REJECTED' olur; kazanan seçilince 'SELECTED' olur (bir grupta birden çok kazanan olabilir)
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

### 6. `Notification` (Bildirimler)
- `id`: UUID (Primary Key)
- `user_id`: UUID (Foreign Key -> `users.id`, ON DELETE CASCADE), boş geçilemez -- bildirimin sahibi
- `group_id`: UUID (Foreign Key -> `groups.id`, ON DELETE CASCADE), boş geçilemez
- `type`: VARCHAR(40), boş geçilemez -- `idea_added`, `ai_ideas_added`, `added_to_group`, `member_joined`, `winner_selected`, `group_reopened`, `event_date_changed`, `budget_changed`, `event_reminder`, `winner_overdue`
- `params`: JSONB, boş geçilemez -- metni oluşturmak için küçük veri (`actor_name`, `idea_title`, `member_name`, `count`, `event_date`, `total_budget`, `days_left`). Metin sunucuda üretilmez; frontend `type` + `params` ile TR/EN oluşturur.
- `dedupe_key`: VARCHAR(200), opsiyonel -- yalnızca hatırlatmalarda dolu (`reminder:3d:<grup>:<tarih>`, `reminder:1d:<grup>:<tarih>`, `overdue:<grup>:<tarih>`); anlık olaylarda NULL
- `read_at`: TIMESTAMP, opsiyonel -- NULL ise okunmamış
- `created_at`: TIMESTAMP, varsayılan now()
- *Kısıtlama:* UNIQUE(`user_id`, `dedupe_key`) -- her hatırlatma kullanıcı başına bir kez (NULL anahtarlar birbirinden farklı sayılır). İndeks: (`user_id`, `created_at`).

### 7. `GroupCover` (Grup Kapak Fotoğrafları)
- `group_id`: UUID (Primary Key, Foreign Key -> `groups.id`, ON DELETE CASCADE) -- grup başına en fazla bir kapak
- `data`: BYTEA, boş geçilemez -- sunucuda işlenmiş JPEG (4:3, en çok 1600x1200, metadata'sız). Yüklenen özgün dosya saklanmaz.
- `content_type`: VARCHAR(50), boş geçilemez -- daima `image/jpeg`
- `etag`: VARCHAR(64), boş geçilemez -- içeriğin SHA-256 özetinin ilk 16 baytı (hex); `groups.cover_version` ile aynıdır
- `width`, `height`: INTEGER, boş geçilemez
- `updated_at`: TIMESTAMP, varsayılan now()
- `updated_by`: UUID (Opsiyonel, Foreign Key -> `users.id`, ON DELETE SET NULL) -- kapağı son yükleyen
- Baytlar hiçbir JSON yanıtında yer almaz; yalnızca `GET /groups/:id/cover` döner.

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

   **Türkiye ürün araştırması (2026-10-16):** AI üretimi iki aşamalıdır: Google Search grounding ile izinli Türkiye mağazalarındaki somut ürünler ve TL fiyatları araştırılır; yalnızca API kaynaklarıyla desteklenen araştırma parçaları yapılandırılmış önerilere dönüştürülür. Seçim aşamasında `evidence_ids` ile kaynak eşleştirilir; ürün adı, arama metni ve fiyatın destekleyen araştırmada geçmesi doğrulanır. Kaynaksız, eksik, fiyatı geçersiz ve tekrar eden kayıtlar elenir; gerekirse 3'ten az öneri döner. Arama sonuçları anlık stok garantisi değildir. `ai_sources` alanı Google'ın döndürdüğü kaynak başlığı ve adresini taşır; `product_url` AI için boş kalır. `ai_search_suggestions` Google'ın arama önerisi HTML'idir; mevcut geçici panel bunu henüz göstermez. Kalıcı frontend'de izole gösterim planlanır. Kaynak bulunamaması şu an controller'ın genel hata yolundan 500 döner; servis filtresinden sonra hiç öneri kalmaması 422, API anahtarı eksikliği 503, Gemini kota/istek sınırı 429 döner. Diğer Gemini hataları genel 500 yolundan döner.

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

6. **Bildirimler (kalıcı + anlık):**
   - **Olaylar:** yeni fikir (manuel / AI), gruba eklenme ("gruba eklendin" eklenen kişiye, "yeni üye" diğerlerine), kazanan seçimi, grubun yeniden açılması, hediye tarihi ve bütçe değişikliği (değer gerçekten değiştiyse). Oy, veto, ilgi alanı ve fikir silme bildirim üretmez.
   - **Alıcılar:** grubun üyeleri; işlemi yapan **hariç**, hedef kişi **hariç**.
   - **Anti-Spoiler (kritik):** Bir grubun `target_user_id`'si olan kullanıcıya o gruba ait hiçbir bildirim üretilmez (alıcı seçiminde grup her olayda yeniden okunur). Hedef sonradan bağlanırsa (`PATCH /groups/:id/target`) o kullanıcının o gruba ait bildirimleri aynı transaction'da silinir. Ayrıca listeleme, sayaç ve okundu işaretleme sorguları `groups.target_user_id` ile süzülür (savunma derinliği): tabloda bir satır kalsa bile hedef onu göremez, sayaçta sayılmaz, okundu yapamaz.
   - **Üretim noktası:** Handler başarılı işlemden sonra `controllers.RecordNotification` ile olayı bildirir; `/groups/:id/*` ve `/ideas/:id/*` rota gruplarına bağlı `middleware.Notify` yalnızca istek başarılıysa (< 400) `NotificationService.Dispatch`'i çağırır (alıcılar, Anti-Spoiler, kayıt, SSE sinyali tek yerde). Yeni bir değişiklik ucu eklendiğinde entegrasyon testi (`TestEveryMutationRouteHasANotificationDecision`) bildirim kararı verilene kadar kırılır.
   - **Hatırlatmalar:** API içindeki zamanlayıcı (`services.ReminderScheduler`) başlangıçta bir kez, sonra saatte bir çalışır. Hediye tarihine 3 gün kala ve 1 gün kala tüm üyelere (`event_reminder`; grup tamamlanmış olsa da), tarih geçtiyse ve grup hâlâ `ACTIVE` ise yalnızca yöneticilere bir kez (`winner_overdue`). "Bugün" UTC+3'e göredir. Gün sayısına aralıkla bakılır (2–3 gün → 3 günlük, 0–1 gün → 1 günlük), böylece API kapalıyken kaçan hatırlatma açılınca kalan gün sayısıyla üretilir; `dedupe_key` tarihi içerdiği için tarih değişirse yeni tarih için yeniden hatırlatılır. Saat enjekte edilebilir (`Now`), `context` iptalinde durur.
   - **Teslim:** Kullanıcı bazlı SSE (`GET /notifications/stream`); olay veri taşımaz, istemci `GET /notifications` ucunu yeniden çeker. Kullanıcı hub'ı da bellek içidir (grup hub'ıyla aynı `realtime.Hub` tipi, kullanıcı kimliğiyle anahtarlı); API tek replica kuralı geçerlidir.

7. **Grup Kapak Fotoğrafı:**
   - **Yetki:** Varsayılan olarak tüm grup üyeleri kapağı ekler, değiştirir ve kaldırır. Admin `cover_admin_only` ile bunu kısıtlar; açıkken yalnızca admin değiştirir, üye 403 alır. Politikayı yalnızca admin değiştirir. Görüntüleme her üyeye açıktır. İzin, kayıt anında grup satırı kilitliyken yeniden denetlenir.
   - **Anti-Spoiler (kritik):** Hedef kişi kapak uçlarının hiçbirine (görüntüleme dahil) erişemez; `GroupAccess` zinciri 403 + sürpriz mesajı döner. Doğru `ETag`'i bilse bile 304 alamaz. Grup listesinde grup görünmediği için `cover_version` da görünmez.
   - **Sunucu işleme hattı (`pkg/imageproc`, saf Go):** En fazla 10 MiB JPEG, PNG veya WebP. Tür, `Content-Type` başlığına ve dosya adına değil bayt imzasına göre belirlenir; HEIC/HEIF açık bir mesajla, GIF/SVG ve diğerleri genel mesajla 415 döner. Çözmeden **önce** `image.DecodeConfig` ile boyut okunur: kenar 12000 pikselden ya da toplam 50 megapikselden büyükse görsel hiç çözülmez (decompression bomb). EXIF yönü (JPEG APP1 ve WebP EXIF parçası) uygulanır; görsel ortadan 4:3 kırpılır; uzun kenar en çok 1600 olacak şekilde CatmullRom ile küçültülür (büyütme yapılmaz); şeffaflık beyaz zemine yaylandırılır; JPEG kalite 80 ile yeniden kodlanır. Yeniden kodlama tüm EXIF/konum/ICC/yorum verisini atar. Yön uygulandıktan sonra 64x48'den küçük görsel reddedilir. Aynı anda en fazla 2 görsel işlenir (bellek koruması).
   - **Hata kodları:** bozuk dosya, çok yüksek çözünürlük, çok küçük görsel, eksik `file` alanı 400; 10 MiB üstü 413; desteklenmeyen tür 415.
   - **Gövde limiti:** Yalnızca `PUT /groups/:id/cover` 10 MiB + 64 KiB kabul eder; diğer tüm rotalar 1 MiB ile sınırlı kalır (`middleware.BodyLimitWithRoutes`).
   - **Bildirim ve canlı güncelleme:** Kapak ve politika değişiklikleri bildirim üretmez. `/groups/:id/*` altında oldukları için grup akışına `changed` olayı gider ve açık grup sayfaları yenilenir.
   - **Önbellek:** `GET /groups/:id/cover` `ETag` (= `cover_version`) ve `Cache-Control: private, max-age=0, must-revalidate` döner. Tarayıcı kopyayı saklar ama her kullanımda doğrular; böylece yetki her istekte yeniden denetlenir, değişmeyen görsel 304 ile gövdesiz (ve baytlar veritabanından okunmadan) döner.

---

## 5. REST API Uç Noktaları (REST Contract)

### Kimlik Doğrulama (Auth)
- `POST /api/v1/auth/register` -> İstek: `{ name, email, password }` -> `201 Created`
- `POST /api/v1/auth/login` -> İstek: `{ email, password }` -> `200 OK` + `{ token, user }`
- `GET /api/v1/me` -> Giriş yapan kullanıcının kendi profilini döner (Bearer token ile)
- `DELETE /api/v1/me` -> Giriş yapan kullanıcının kendi hesabını, oluşturduğu grupları, fikirleri ve oylarını siler (Cascade & atomik temizlik)

### Grup İşlemleri
- `GET /api/v1/groups` -> Kullanıcının üye olduğu grupları listeler (hedef kişi olduğu gruplar filtrelenir). Her grupta varsa `cover_version` ve `cover_admin_only` bulunur; kapak baytları yanıtta yer almaz.
- `POST /api/v1/groups` -> Yeni grup açar (oluşturan kişi `admin` üye yapılır). İsteğe bağlı `target_email` ile hedef kişi seçilir: sunucu kayıtlı kullanıcıyı büyük/küçük harf duyarsız bulur ve `target_user_id` alanını doldurur. Kayıtlı değilse 404 döner ve grup kurulmaz; kişi kendini seçerse 400 döner. Ayrı kullanıcı arama ucu yoktur. Eski `target_user_id` (UUID) alanı geriye dönük uyumluluk için kabul edilir; iki alan birlikte gönderilirse 400 döner. `target_person_name` her durumda kullanıcının yazdığı isim olarak kalır.
- `GET /api/v1/groups/:id` -> Grup detayları, üye sayısı, kişi başı bütçe (Anti-Spoiler korumalı). `group.cover_version` (varsa) ve `group.cover_admin_only` dahildir.
- `POST /api/v1/groups/:id/members` -> Gruba e-posta ile üye ekler
- `POST /api/v1/groups/:id/interests` -> `{ interest: string }` ile listeye tek ilgi alanı ekler; tüm grup üyeleri ekleyebilir. Başındaki/sonundaki boşluklar temizlenir; 1–100 karakter, virgül/noktalı virgül/satır sonu içermez. Tekrar ekleme 409 döner. Yanıt `{ interests: string[] }` olur.
- `DELETE /api/v1/groups/:id/interests` -> `{ interest: string }` ile yalnızca **admin** bir öğeyi silebilir; normal üye kendi eklediğini de silemez (403). Anti-Spoiler ve üyelik kontrolü her iki uçta da zorunludur. Eski toplu `PATCH` yolu kaldırılmıştır; tüm listeyi değiştirerek silme yetkisi aşılamaz. Eşzamanlı eklemeler transaction + satır kilidiyle korunur.
- `PATCH /api/v1/groups/:id/budget` -> `{ total_budget: decimal }` ile yalnızca **admin** toplam bütçeyi günceller. Aralık 0–99999999.99, en fazla iki ondalık basamaktır. Kişi başı tutar anlık hesaplanır; mevcut hediye bütçe aşımı bayrakları aynı transaction'da güncellenir ve listelemede güncel bütçeye göre hesaplanır. Mevcut kurala uygun olarak 0 bütçe aşımı uyarılarını kapatır. Anti-Spoiler ve üyelik kontrolü zorunludur.
- `PATCH /api/v1/groups/:id/event-date` -> `{ event_date: "YYYY-MM-DD" | null }` ile yalnızca **admin** hediye tarihini ayarlar veya kaldırır. Aralık dışı veya geçersiz tarih 400.
- `GET /api/v1/groups/:id/events` -> Server-Sent Events akışı. Grupta başarılı bir değişiklik olunca (fikir, oy, veto, bütçe, üye, ilgi alanı, tarih, kazanan) `event: changed` gönderir; olay veri taşımaz, istemci grubu ve fikirleri yeniden çeker. `GroupAccess` (Anti-Spoiler + üyelik) bağlantı açılırken uygulanır; akış en fazla 5 dakika açık kalır, sonra istemci yeniden bağlanır (yetki yeniden denetlenir). 25 sn'de bir `: ping`; `X-Accel-Buffering: no`. Aynı anda en fazla 2000 akış (aşılırsa 503). Hub bellek içidir; birden çok API kopyasında LISTEN/NOTIFY gibi bir aracı gerekir. Token `Authorization` başlığıyla gönderilir (frontend fetch ile okur; URL'de token yok).
- `POST /api/v1/groups/:id/winner` -> `{ idea_ids: [uuid, ...] }` ile yalnızca **admin** grubun 1–20 aktif fikrini kazanan seçer (tekrarlar ayıklanır): fikirler `SELECTED`, grup `COMPLETED`. Fikirlerden biri başka gruba aitse ya da liste boş/20'den uzunsa 400; biri aktif değilse veya grup zaten tamamlandıysa 409; hata durumunda hiçbir fikir değişmez. Grup satırı kilitlenerek transaction içinde yapılır.
- `DELETE /api/v1/groups/:id/winner` -> Yalnızca **admin**, yanlış seçimi geri almak için grubu yeniden açar: tüm kazanan fikirler `ACTIVE`, grup `ACTIVE`. Grup zaten açıksa 409.
- `PATCH /api/v1/groups/:id/target` -> `{ email }` ile yalnızca **admin**, hedefi boş olan gruba kayıtlı bir kullanıcıyı hedef olarak bağlar. Kayıtlı değilse 404; hedef zaten bağlıysa veya kişi grubun üyesiyse 409; admin kendini seçerse 400; admin olmayan üye 403 alır. Kontroller grup satırı kilitlenerek transaction içinde tekrarlanır; eşzamanlı üye ekleme de aynı kilidi kullandığı için hedef kişi üye yapılamaz. `target_person_name` değişmez. Bağlandıktan sonra hedef kişi grubu listede görmez ve tüm `/groups/:id/*` uçlarında 403 alır.
- `GET /api/v1/groups/:id/cover` -> Kapak fotoğrafının baytları (`image/jpeg`; JSON değil). `ETag` + `If-None-Match` → 304; `Cache-Control: private, max-age=0, must-revalidate`, `Vary: Authorization`. Her üye görebilir; hedef kişi ve üye olmayan 403, kapak yoksa 404. Token `Authorization` başlığıyla gönderilir (frontend fetch ile Blob alır; URL'de token yok).
- `PUT /api/v1/groups/:id/cover` -> `multipart/form-data`, alan `file` (JPEG/PNG/WebP, en fazla 10 MiB). Kapağı ekler veya değiştirir; sunucu görseli İş Kuralı 7'deki gibi işler. Yanıt `{ cover_version, width, height }`. Varsayılan olarak tüm üyeler; `cover_admin_only` açıksa yalnızca admin (üye 403). 400 / 413 / 415 hata kodları İş Kuralı 7'de.
- `DELETE /api/v1/groups/:id/cover` -> Kapağı kaldırır; yetki yüklemeyle aynıdır. Kapak yoksa da 200 (idempotent).
- `PATCH /api/v1/groups/:id/cover-policy` -> `{ cover_admin_only: boolean }` ile yalnızca **admin** kapağı kimin değiştirebileceğini ayarlar; üye 403. Yanıt `{ cover_admin_only }`.
- `DELETE /api/v1/groups/:id` -> Grubu ve bağlı tüm verileri siler (Sadece grup admini silebilir; Anti-Spoiler korumalı). Kapak fotoğrafı CASCADE ile silinir.

### Hediye Havuzu ve Oylama
- `GET /api/v1/groups/:id/ideas` -> Hediyeleri toplam oy skoruyla birlikte çeker (`SUM(votes.vote_type)`)
- `POST /api/v1/groups/:id/ideas` -> Manuel hediye önerisi ekler (opsiyonel `product_url` ile; link önizlemeden gelen veriler formu doldurur)
- `POST /api/v1/groups/:id/ideas/preview-link` -> İstek: `{ url }` -> `200 OK` + `{ title, description, image_url, price, final_url, search_query }` (Veritabanına yazmaz, sadece önizleme döner; Anti-Spoiler korumalı)
- `POST /api/v1/groups/:id/ideas/generate-ai` -> Google Search araştırması ve JSON seçimiyle en fazla 5 kaynaklı öneriyi kaydeder; `201` + `GiftIdea[]` döner. `ai_sources` ve `ai_search_suggestions` yanıta dahildir. 3'ten az sonuç dönebilir; mevcut hata eşlemeleri İş Kuralı 4'te açıklanır.
- `POST /api/v1/ideas/:id/vote` -> Oy verir veya günceller `{ vote_type: 1 | -1 }` (Conflict durumunda güncelleme; downvote sayısı çoğunluğu aşarsa fikri otomatik REJECTED yapar)
- `DELETE /api/v1/ideas/:id/vote` -> Kullanıcının bu fikre verdiği oyu kaldırır; oy yoksa da 200 (idempotent). Çoğunluk vetosuyla reddedilmiş fikir kendiliğinden geri açılmaz. Anti-Spoiler ve üyelik kontrolü zorunludur.
- `POST /api/v1/ideas/:id/reject` -> Grup admini doğrudan hediye fikrini veto eder (`status = REJECTED`)
- `DELETE /api/v1/ideas/:id` -> Hediyeyi kaldırır (Sadece öneren kişi veya grup admini silebilir)

### Bildirimler
- `GET /api/v1/notifications` -> `{ items: Notification[], unread_count }`. Giriş yapan kullanıcının en yeni 50 bildirimi (yeniden eskiye); öğe: `{ id, group_id, group_title, type, params, read_at?, created_at }`. `unread_count` tüm okunmamışları sayar. Kullanıcının hedef kişisi olduğu gruplara ait satırlar hiçbir zaman dönmez (Anti-Spoiler).
- `GET /api/v1/notifications/stream` -> Server-Sent Events akışı. Kullanıcıya yeni bildirim geldiğinde ya da bildirimleri okundu yapıldığında (diğer sekmeler için) `event: changed` gönderir; olay veri taşımaz. Grup akışıyla aynı çerçeve: 25 sn'de bir `: ping`, en fazla 5 dakika ömür, `X-Accel-Buffering: no`, en fazla 2000 açık akış (aşılırsa 503), token `Authorization` başlığıyla.
- `POST /api/v1/notifications/:id/read` -> Kullanıcının kendi bildirimini okundu yapar (idempotent; ilk okunma anı korunur). Bildirim yoksa, başka kullanıcıya aitse ya da Anti-Spoiler nedeniyle gizliyse 404; geçersiz kimlik 400.
- `POST /api/v1/notifications/read-all` -> Kullanıcının okunmamış tüm bildirimlerini okundu yapar.

### Sistem & Test
- `POST /api/v1/dev/reset-db` -> Tüm tabloları TRUNCATE CASCADE ile sıfırlar. Yalnızca `ALLOW_DB_RESET=true` ile açılır ve gövdede `{"confirm": "RESET"}` ister (yoksa 403 / 400). Production ortamında her durumda devre dışıdır
- Üretim frontend'inin nginx'i `/api/`, `/health`, `/swagger/`, `/docs` yollarını API'ye iletir; `/api/v1/dev/*` büyük/küçük harften bağımsız olarak 403 ile engellenir. Geliştirmede Vite proxy'si bu ucu **engellemez**; yerel API'de çağrılırsa yerel veritabanı sıfırlanır.
- Eski geçici panel proxy'si yalnızca auth, profil, grup, hediye ve oy yollarıyla `/health`, `/swagger/*`, `/docs` yollarını iletir; geliştirme uçlarını 403 ile engeller.

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
    ├── pkg/imageproc/         # Kapak görseli işleme (tür doğrulama, EXIF yönü, 4:3 kırpma, küçültme, JPEG)
    └── go.mod
    ```
  - Hata kontrolleri açıkça yapılmalıdır (`if err != nil`).
  - Veritabanı ve migrasyonlar için GORM kullanılmalıdır.
  - JSON yanıtları standart formatta dönmelidir: `{"success": true, "data": ...}` veya `{"success": false, "error": "mesaj"}`.

- **React Frontend:**
  - Düzen: üstte yüzen bar (işaret, grup seçici, dil, bildirim zili, hesap menüsü); grup sayfasında bütçe/kişi başı/ilgi alanı kutuları, ortada oylama kartları, sağda üyeler. Mobil öncelikli.
  - API istekleri `src/api/` katmanında `fetch` ile yönetilir (`{success, data | error}` zarfı, Bearer token, 401'de oturum kapanır). Sunucu durumu TanStack Query ile tutulur.
  - Renkler yalnızca `src/index.css` token'larından gelir; bileşenlerde sabit renk yazılmaz. Ana renk iki temada kobalt `#2f5bff`, para alanları hardal `#ffc93c`, koyu tema "Nötr grafit". Metin kontrastı en az 4.5:1 tutulur.
  - Tüm metinler i18next sözlüklerinde (TR varsayılan, EN). Anti-Spoiler 403'ünde özel "Bu bir sürpriz!" ekranı gösterilir.
  - Kapak fotoğrafı: `<img>` Authorization başlığı gönderemediği için görsel `fetch` ile Blob alınır ve object URL ile gösterilir (`src/api/useGroupCover.ts`, `components/GroupCover.tsx`); TanStack Query anahtarı `['cover', grup, cover_version]`, object URL son kullanan bileşen ayrılınca serbest bırakılır. Kırpma ekranı (`components/CoverCropDialog.tsx`, hesaplar `src/lib/coverCrop.ts`) bağımlılıksızdır: sabit 4:3 çerçeve, sürükleme, tekerlek/iki parmak/kaydırıcı/klavye ile yakınlaştırma; çıktı canvas ile en çok 1600x1200 JPEG (kalite 0.85). Sunucu aynı kuralları yeniden uygular.
