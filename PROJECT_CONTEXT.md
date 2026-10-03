# PROJE TEKNİK ŞARTNAMESİ: GiftlyAI

## 1. Proje Özeti ve Rolün
Sen **GiftlyAI** projesini geliştiren uzman bir full-stack yazılım mühendisisin.
Bu proje; arkadaş gruplarının ortak hediye grupları kurduğu, bütçe birleştirdiği, Gemini 3 Flash üzerinden yapay zekâ destekli semantik hediye önerileri aldığı, önerileri oyladığı (+1 / -1) ve hediye alınacak kişiden süreci gizlediği (Anti-Spoiler koruması) işbirlikçi bir web platformudur.

---

## 2. Teknoloji Yığını (Tech Stack)
- **Backend:** Go (Golang) + **Gin Framework** (`github.com/gin-gonic/gin`)
- **Veritabanı:** PostgreSQL (Docker üzerinde) + **GORM**
- **Kimlik Doğrulama:** JWT (JSON Web Tokens) ve özel Gin middleware'i
- **Frontend:** React (Vite) + Tailwind CSS + shadcn/ui + Lucide Icons
- **Yapay Zekâ Katmanı:** Google Gemini 3 Flash (Google AI Studio Free Tier, `response_schema` ile katı JSON formatı)
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
- `search_query`: VARCHAR(255), boş geçilemez -- Pazar yerleri için optimize edilmiş arama terimi
- `image_url`: VARCHAR(500), opsiyonel
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
   - Veritabanından grubun ilgi alanları, kalan bütçe limiti ve grupta daha önce negatif oy almış (`vote_type = -1`) ürünlerin başlıkları çekilir.
   - Tek bir JSON prompt'u ile Gemini'a iletilir ve 3-5 adet yeni öneri istenir.

---

## 5. REST API Uç Noktaları (REST Contract)

### Kimlik Doğrulama (Auth)
- `POST /api/v1/auth/register` -> İstek: `{ name, email, password }` -> `201 Created`
- `POST /api/v1/auth/login` -> İstek: `{ email, password }` -> `200 OK` + `{ token, user }`

### Grup İşlemleri
- `GET /api/v1/groups` -> Kullanıcının üye olduğu grupları listeler (hedef kişi olduğu gruplar filtrelenir)
- `POST /api/v1/groups` -> Yeni grup açar (oluşturan kişi `admin` üye yapılır)
- `GET /api/v1/groups/:id` -> Grup detayları, üye sayısı, kişi başı bütçe (Anti-Spoiler korumalı)
- `POST /api/v1/groups/:id/members` -> Gruba e-posta ile üye ekler

### Hediye Havuzu ve Oylama
- `GET /api/v1/groups/:id/ideas` -> Hediyeleri toplam oy skoruyla birlikte çeker (`SUM(votes.vote_type)`)
- `POST /api/v1/groups/:id/ideas` -> Manuel hediye önerisi ekler
- `POST /api/v1/groups/:id/ideas/generate-ai` -> Gemini API'yi tetikler, yeni önerileri tabloya kaydeder ve döner
- `POST /api/v1/ideas/:id/vote` -> Oy verir veya günceller `{ vote_type: 1 | -1 }` (Conflict durumunda güncelleme)
- `DELETE /api/v1/ideas/:id` -> Hediyeyi kaldırır (Sadece öneren kişi veya grup admini silebilir)

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
    └── go.mod
    ```
  - Hata kontrolleri açıkça yapılmalıdır (`if err != nil`).
  - Veritabanı ve migrasyonlar için GORM kullanılmalıdır.
  - JSON yanıtları standart formatta dönmelidir: `{"success": true, "data": ...}` veya `{"success": false, "error": "mesaj"}`.

- **React Frontend:**
  - Tek ekranlı modern panel tasarımı: Üst barda kullanıcı profili ve bütçe durumu; orta alanda oylama kartları; sağ yan panelde grup üyeleri ve Gemini öneri butonu.
  - API istekleri temiz bir `api/` katmanı üzerinden `fetch` veya `axios` ile yönetilmelidir.
