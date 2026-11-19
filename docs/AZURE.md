# Azure demo dağıtımı

> **Güncel AI modu (2026-11-10'dan beri):** Google Search kapalıdır (`GEMINI_SEARCH_ENABLED=false`).
> `GEMINI_MODEL=gemini-3.6-flash` (yedek `GEMINI_FALLBACK_MODEL=gemini-3.5-flash-lite`, `GEMINI_THINKING_LEVEL=low`) ile tek çağrıda öneri üretilir; fiyatlar doğrulanmamış tahmindir. Model kalıcı 503 verirse API 503 + "servis yoğun" mesajı döner.
> Model kimlikleri uygulama kodunda sabit değildir. `GEMINI_MODEL` anahtar varsa zorunludur.
> `GEMINI_FALLBACK_MODEL` boşsa otomatik model geçişi yapılmaz; doluysa yalnızca 503'te denenir.
> `GEMINI_RESEARCH_MODEL` yalnızca Search açıkken kullanılır; boşsa ana model kullanılır.
> Yerelde `backend/.env`, Azure'da Container App ortam değişkenleri düzenlenir ve uygulama yeniden başlatılır/yeni revision oluşur.
> Aşağıdaki eski Search araştırması açıklamaları yalnızca Search modu tekrar açılırsa geçerlidir.

Go API ve kalıcı React frontend ayrı imajlar olarak paketlenir. PostgreSQL ayrı bir yönetilen serviste çalışır.

## Güncel durum (2026-11-19)

- **Açık.** Standart ortam `covertgift-apps` (WorkloadProfiles, yalnızca Consumption profili). API `covertgift-server` (`covertgift-api:1c0e188`, internal), arayüz `covertgift-web` (`covertgift-web:1c0e188`, external, `API_TARGET=http://covertgift-server`). `main` #33–#34. **Adres: https://covertgift.msheker.dev** (Azure'un kendi adresi `covertgift-web.gentlegrass-4f326f37.polandcentral.azurecontainerapps.io` da çalışır).
- **Express ortamından çıkıldı (2026-11-19).** Eski Express ortamı `covertgift-env` ve uygulamaları (`covertgift-api`, `covertgift-ui`, `covertgift-backend`) silindi. Eski adres (`covertgift-ui.yellowplant-0f3e00f1…`) artık çalışmıyor. Sebep aşağıda, "Olay: Express ortamında internal ingress" bölümünde.
- **Dağıtımdan önce** canlıda `/health`'e bakılır ve sonucu not edilir. Böylece bir sorun çıkarsa dağıtımdan önce de var olup olmadığı bilinir.
- Önceki canlı sürüm `a5c65ce` idi. Bu sürümde `users.avatar_url` sütunu elle silindi (aşağıda). Eski API'nin modeli bu sütunu içerdiği için `a5c65ce`'ye dönmek kullanıcı kaydını bozar. Geri dönmeden önce sütun yeniden eklenmeli: `ALTER TABLE users ADD COLUMN avatar_url varchar(255);`.
- 2026-11-15'te `users.avatar_url` (hiç kullanılmamıştı; 7 kullanıcının hepsinde boştu) dağıtımdan sonra elle silindi. Silmeden önce `pg_dump -Fc` ile iki tam yedek alındı (dağıtım öncesi ve silme öncesi); yedekler `~/CovertGift-backups/2026-11-15/` altında, Git dışında. Bağlantı için güvenlik duvarına geçici tek IP kuralı eklendi ve iş bitince silindi.
- `a5c65ce`'den önceki canlı sürüm `f883801` idi. Ona dönülürse davet linki ve MULTI gruplar kaybolur; eski API yeni tabloları ve sütunları yok sayar, ama o sırada kurulmuş MULTI gruplar eski arayüzde hedefsiz tekli grup gibi görünür ve havuzdaki fikirler (`pool_id` dolu) grup listesine karışır. Geri dönmeden önce bu gruplar değerlendirilmelidir.
- İmajlar **yerel Docker** ile `linux/amd64` için derlenip gönderilir; bu abonelikte `az acr build` kapalıdır (`TasksOperationsNotAllowed`).
- API **en fazla 1 replica** kalmalıdır: grup SSE'si (`/groups/:id/events`) ve kullanıcı bildirim SSE'si (`/notifications/stream`) bellek içi hub kullanır; birden çok kopyada olaylar kopyalar arasında taşınmaz.
- API başlarken AutoMigrate şemayı günceller. 2026-11-15 dağıtımında yeni tablo `user_avatars` (profil fotoğrafı, bytea) ve yeni sütun `users.avatar_version` eklendi. 2026-11-14 dağıtımında yeni tablolar `group_invites`, `gift_pools`, `pool_contributions`; yeni sütunlar `groups.mode` (varsayılan `SINGLE`, mevcut grupların hepsi tekli kaldı), `gift_ideas.pool_id`, `notifications.pool_id` eklendi. 2026-11-13 dağıtımında yeni tablolar `notifications` ve `group_covers`, `groups` tablosuna `cover_version` ve `cover_admin_only` sütunları eklendi (2026-11-09'da `groups.status`, `completed_at`, `event_date` eklenmişti).
- Hatırlatma zamanlayıcısı API içinde, açılışta bir kez ve sonra saatlik çalışır. API 0 replikaya inip uyanınca kaçan hatırlatmaları yakalar; `dedupe_key` her hatırlatmanın kullanıcı başına bir kez üretilmesini sağlar. İlk dağıtımda tarihi geçmiş her açık grup için yöneticilere birer "süre doldu" bildirimi üretilir.
- Bu CLI sürümünde `az containerapp logs show` `KeyError: 'eventStreamEndpoint'` verir; loglar portaldan okunur. Production'da loglar JSON'dur, her satırda `request_id` vardır.
- `az containerapp update` yalnızca `--image` ile çalıştırılır (revizyon soneki kullanılmıyor).
- **Yeni ortam kurarken modu açıkça verin:** `az containerapp env create … --environment-mode WorkloadProfiles`. Bu bölgede CLI, `--enable-workload-profiles` verilse bile sessizce **Express** ortamı kurar.
- Abonelik Poland Central'da yalnızca **1 standart ortama** izin verir (`MaxNumberOfRegionalEnvironmentsInSubExceeded`). Silinmekte olan (`ScheduledForDelete`) bir ortam da sınırı doldurur.

## Durdurma ve yeniden açma

Bu CLI sürümünde `az containerapp stop/start` yok; Container App'ler REST API'nin `stop`/`start` uçlarıyla yönetilir.

```sh
# Durdurma
for a in covertgift-web covertgift-server; do
  az rest --method post --url "https://management.azure.com$(az containerapp show -g covertgift-rg -n $a --query id -o tsv)/stop?api-version=2024-03-01"
done
az postgres flexible-server stop -g covertgift-rg -n covertgift-db-msheker

# Yeniden açma (sıra önemli: önce veritabanı)
az postgres flexible-server start -g covertgift-rg -n covertgift-db-msheker
for a in covertgift-server covertgift-web; do
  az rest --method post --url "https://management.azure.com$(az containerapp show -g covertgift-rg -n $a --query id -o tsv)/start?api-version=2024-03-01"
done
```

⚠️ **Azure, durdurulan PostgreSQL esnek sunucusunu 7 gün sonra kendiliğinden yeniden başlatır.** Kapalı tutmak için her hafta yeniden `stop` gerekir (ya da uzun süre kullanılmayacaksa yedek alınıp sunucu silinir). Durdurulmuş sunucuda depolama (32 GiB) ve yedekler, registry'de ACR ücreti işlemeye devam eder.

## Dağıtım geçmişi

Eski etiketler geri dönüş için registry'de durur.

| Tarih | API | Arayüz | Not |
|---|---|---|---|
| 2026-11-19 | `covertgift-api:1c0e188` (`covertgift-server`) | `covertgift-web:1c0e188` (`covertgift-web`) | Yeni standart ortam `covertgift-apps`; Express ortamı silindi. Şema değişikliği yok |
| 2026-11-16 | `covertgift-api:1c0e188` | `covertgift-web:1c0e188` | #33 görev paneli, #34 Go 1.27.4 + nginx avatar 3 MB. Express ortamında API'ye erişim kesildi, geri alma çözmedi (aşağıdaki olay) |
| 2026-11-15 | `covertgift-api:195b728` | `covertgift-web:195b728` | #25–#29: MULTI kişi sekmeleri (tarihe göre, havuz sayfasına bitişik), "Sıradaki sürpriz" kartında tarihin sahibi (`next_event_recipients`), havuz bildirimleri seed'i ve TR metin düzeltmesi, iki küçük bug, profil fotoğrafı (`user_avatars`, `users.avatar_version`; `PUT/DELETE /me/avatar`, `GET /users/:id/avatar`). Dağıtımdan sonra `users.avatar_url` elle silindi (yedekli) |
| 2026-11-14 | `covertgift-api:a5c65ce` | `covertgift-web:a5c65ce` | #23: davet linki (`group_invites`; `/groups/:id/invite`, `/invites/:token`) ve birden çok kişi için gruplar (`groups.mode`, `gift_pools`, `pool_contributions`, `gift_ideas.pool_id`, `notifications.pool_id`); web imajında davet yolları erişim/hata logundan çıkarıldı (`map $request_uri $loggable`) |
| 2026-11-13 | `covertgift-api:f883801` | `covertgift-web:f883801` | `24f64a2`'nin üzerine: #18 bildirimler, #19 grup kapak fotoğrafı, #20 açıklama + hesap durumu (`target_user` sızıntısı düzeltmesi), #21 küçük kapak yüklemesi; yeni tablolar `notifications`, `group_covers`, yeni sütunlar `groups.cover_version`, `cover_admin_only`; web imajında kapak rotası için `client_max_body_size 3m`. Önceki canlı sürüm `24f64a2` (#17 AI model geçişi: `gemini-3.6-flash`, yedek `gemini-3.5-flash-lite`, aşırı yükte 503) |
| 2026-11-10 | `covertgift-api:515aee6` | `covertgift-web:515aee6` | #15: yeniden tasarlanan ana ekran (`GET /groups` sayaçları), profil sayfası (`PATCH /me`, `PUT /me/password`); şema değişikliği yok |
| 2026-11-09 | `covertgift-api:208fa08` | `covertgift-web:208fa08` | #13: birden çok kazanan (`idea_ids`) |
| 2026-11-09 | `covertgift-api:7bb8416` | `covertgift-web:7bb8416` | #5–#10: reset-db koruması, Anti-Spoiler düzeltmesi, limitler ve doğrulama, slog, kazanan seçimi, oyu geri alma, hediye tarihi, SSE |
| 2026-11-09 | `covertgift-api:security-20261109` | `covertgift-web:security-20261109` | Go 1.27.3, nginx 1.30.6, Node 22.23.4; ortam yeniden açıldı |
| 2026-10-23 | `covertgift-api:ff8bb3c` | `covertgift-web:2e0d1e7` | "Nötr grafit" tasarımı; ardından ortam durduruldu |
| 2026-10-21 | `covertgift-api:ff8bb3c` | `covertgift-web:ff8bb3c` | Kalıcı React frontend geçici panelin yerini aldı |
| 2026-10-21 | `covertgift-api:search-off-20261021` | `covertgift-ui:demo` | Search kapalı AI modu; eski geçici panel |

## Mevcut demo ortamı

- Bölge: Poland Central; kaynak grubu: `covertgift-rg`.
- Ortam: `covertgift-apps`, WorkloadProfiles modu, yalnızca Consumption profili; loglar Log Analytics'e gider.
- API: `covertgift-server`, internal ingress (yalnızca ortam içinden erişim), port 8080. İç adres: `covertgift-server.internal.gentlegrass-4f326f37…`; arayüz kısa adı `http://covertgift-server` kullanır.
- Arayüz: `covertgift-web`, https://covertgift-web.gentlegrass-4f326f37.polandcentral.azurecontainerapps.io
- Özel alan adı: `covertgift.msheker.dev`. Alan adı kullanıcının Cloudflare hesabında (Cloudflare Registrar ve DNS). Kayıtlar:
  - `CNAME covertgift` → `covertgift-web.gentlegrass-4f326f37.polandcentral.azurecontainerapps.io`, **DNS only (gri bulut)**. Proxy (turuncu bulut) açılırsa Azure'un sertifika doğrulaması bozulabilir.
  - `TXT asuid.covertgift` → ortamın `customDomainVerificationId` değeri (`az containerapp env show … --query properties.customDomainConfiguration.customDomainVerificationId`).
  - Sertifika Azure'un ücretsiz yönetilen sertifikası (`mc-covertgift-app-covertgift-mshek-5237`, `SniEnabled`). Azure kendiliğinden yeniler; CNAME kaydı yerinde kalmalı.
  - Uygulama ya da ortam yeniden kurulursa: `az containerapp hostname add` ve ardından `az containerapp hostname bind --environment covertgift-apps --validation-method CNAME`. Yeni ortamın doğrulama kodu farklıysa TXT kaydı güncellenir.
- Registry: `covertgiftmsheker.azurecr.io`; imajlar commit kısa kimliğiyle etiketlenir (ör. `f883801`). `demo` etiketi ilk dağıtımın imajıdır.
- PostgreSQL: `covertgift-db-msheker`, sürüm 17, B1ms, 32 GiB.
- İmaj çekme kimliği: `covertgift-acr-pull` (kullanıcı atanmış), registry kapsamında AcrPull.
- API: 0.5 vCPU / 1 GiB; arayüz: 0.25 vCPU / 0.5 GiB. İkisi de 0–1 replica (API için 1'in üstüne çıkmayın; grup ve bildirim SSE hub'ları bellek içi).
- DATABASE_URL, JWT_SECRET ve mevcut GEMINI_API_KEY, API'de secret olarak tanımlıdır.

2026-10-19 tarihinde yerel yedek Azure'a aktarıldı. Aktarım sonrası ve API başladıktan
sonra kayıt sayıları yedekle karşılaştırıldı: 6 kullanıcı, 2 grup, 6 üyelik,
8 hediye fikri ve 2 oy; tümü eşleşti. Panel ana sayfası ve `/health` HTTP 200 döndü.

Express ortamında sistem atanmış ortam kimliği desteklenmez. Portalın varsayılan
kimliği yerine yukarıdaki kullanıcı atanmış kimliği kullanın. Kaynak oluşturma
formu yerine Azure CLI ve ARM şablonu ile dağıtım yapıldı.

İlk dağıtımda PostgreSQL güvenlik duvarına yerel istemci IP'si ve panel içinden
ölçülen tek çıkış IP'si (74.248.18.123) eklendi. Daha sonra yeni API kopyası
PostgreSQL bağlantısında zaman aşımına uğradı ve backend 503 vermeye başladı.
Panelin ölçülen IP'si değişmemişti; bu IP, diğer Express kopyalarının tüm çıkış
adreslerini kapsama garantisi sağlamaz.

Kesintiyi gidermek için `AllowAzureServices` adlı özel 0.0.0.0–0.0.0.0 kuralı
eklendi. Bu, tüm Azure veri merkezi IP'lerinden bağlantı denemesine izin verir;
diğer müşterilerin Azure aboneliklerini de kapsar. Tüm internete izin veren
0.0.0.0–255.255.255.255 kuralı eklenmedi. Sunucuda `require_secure_transport=on`
doğrulandı; kullanıcı/parola doğrulaması zorunlu kalır. Daha dar ağ yalıtımı
istenirse sabit çıkış veya özel ağ bağlantısı ayrıca tasarlanmalıdır.

API'ye `PGCONNECT_TIMEOUT=10` ortam değişkeni eklendi; pgx'in yeni veritabanı
bağlantıları için varsayılan bekleme süresini sınırlar. Bu ayar mevcut sorguların
çalışma süresini sınırlamaz. Minimum 0, maksimum 1 replica ayarı korundu.

Yerel aktarım yedeği ve parola giriş yardımcıları özel bir geçici klasörde tutuldu;
gizli ayarlar ve yedek dosyaları Git'e eklenmedi.

## İmajları hazırlama

Proje kökünden, Azure Container Apps için Linux/AMD64 imajları oluşturun. Etiket olarak commit kısa kimliğini kullanın;
eski etiketlerin üzerine yazmayın, geri dönüş için registry'de kalsınlar:

```sh
TAG=$(git rev-parse --short HEAD)
docker build --platform linux/amd64 -t LOGIN_SERVER/covertgift-api:$TAG ./backend
docker build --platform linux/amd64 -t LOGIN_SERVER/covertgift-web:$TAG ./frontend
```

İmajları Azure Container Registry'ye gönderin. Kayıt defterinin Overview ekranındaki
Login server değerini kullanın; adresi kayıt defteri adından türetmeyin:

```sh
az acr login --name KAYIT_DEFTERI_ADI
docker push LOGIN_SERVER/covertgift-api:$TAG
docker push LOGIN_SERVER/covertgift-web:$TAG
az containerapp update -g covertgift-rg -n covertgift-server --image LOGIN_SERVER/covertgift-api:$TAG
az containerapp update -g covertgift-rg -n covertgift-web --image LOGIN_SERVER/covertgift-web:$TAG
```

Yerelde smoke test yaparken frontend imajını `API_TARGET=http://127.0.0.1:9` gibi ölü bir hedefle çalıştırın;
engel kurallarını çalışan bir API'ye karşı denemeyin. Eski panel imajı `./prototypes/test-ui` klasöründen derlenir.

Yer tutucuları kendi değerlerinizle değiştirin. Container Apps'in yönetilen kimliğine,
kayıt defterinin izin modeline uygun imaj çekme yetkisini verin.

## API uygulaması

- İmaj: `LOGIN_SERVER/covertgift-api:KISA_COMMIT` (ilk dağıtımda `demo`; güncel etiket "Güncel durum" bölümünde)
- İş yükü profili: Consumption
- Kaynak: canlıda 0.5 vCPU / **1 GiB**. 0.5 GiB'a indirmeyin: kapak işleme bellek kullanır.
- Ingress: internal, target port 8080. Panel ile aynı ortamda çalıştırın.
- HTTP ölçeklendirme: minimum 0, maksimum **1** replica. Grup SSE'si ve kullanıcı bildirim SSE'si bellek içi hub kullanır; ikinci bir kopyada olaylar taşınmaz (gerekirse PostgreSQL LISTEN/NOTIFY eklenmeli). Hatırlatma zamanlayıcısı da her kopyada çalışırdı (`dedupe_key` çift satırı yine engeller).
- Kapak fotoğrafı (`PUT /groups/:id/cover`): sunucu yalnızca küçük görsel kabul eder: en çok 2 MiB, uzun kenar ≤ 2000 px ve ≤ 4 MP. Boyut başlıktan okunur, aşan görsel çözülmez (400); 2 MiB üstü 413. Büyük fotoğrafı tarayıcı küçültür (40 MB / 100 MP'ye kadar seçilebilir). Aynı anda en fazla 2 görsel işlenir. Kapaklar veritabanında (`group_covers.data`, bytea) durur; yedek boyutu grup başına ~150–400 KB artar.
- Hatırlatma zamanlayıcısı API sürecinin içindedir (ayrı cron/iş yok): açılışta bir kez, sonra saatlik. Ölçek 0'dayken çalışmaz; API uyandığında kaçanları yakalar.
- Ortam değişkenleri: `PORT=8080`, `APP_ENV=production`, `GIN_MODE=release`.
- Secret üzerinden: `DATABASE_URL`, `JWT_SECRET`; AI için ayrıca `GEMINI_API_KEY`.
- `GEMINI_MODEL`: öneri modeli; anahtar varsa **zorunludur**, kodda varsayılan yoktur (`gemini-3.6-flash`; 2026-11-10'da `gemini-3.7/3.8-flash` yanıt vermeyi bıraktığı için değişti).
- `GEMINI_RESEARCH_MODEL`: yalnızca Search açıkken kullanılır; boşsa `GEMINI_MODEL` kullanılır.
- `GEMINI_FALLBACK_MODEL`: canlıda `gemini-3.5-flash-lite`; yalnızca birincil model 503 verirse denenir, zaman aşımında denenmez.
- Kota/istek sınırı hataları API üzerinden HTTP 429, modelin kalıcı aşırı yükü 503 ("servis yoğun") döner.

Veritabanında önceden `covertgift` veritabanını oluşturun. API başlangıçta tabloları
AutoMigrate ile oluşturur. Azure PostgreSQL bağlantısında TLS kullanın:

```text
postgres://KULLANICI:URL_ENCODED_PAROLA@SUNUCU.postgres.database.azure.com:5432/covertgift?sslmode=require
```

PostgreSQL ağ erişimini API için yapılandırın. Secret değerlerini Docker build
argümanları olarak vermeyin. `.dockerignore` yerel `.env` dosyalarını dışlar.
API imajı yalnızca derlenen programı içerir ve ayrıcalıksız kullanıcıyla çalışır.

## Frontend uygulaması (kalıcı React frontend)

`frontend/Dockerfile` iki aşamalıdır: Node 22 ile `npm ci` + `npm run build`, ardından ayrıcalıksız
`nginx-unprivileged` ile `dist/` sunulur. `frontend/nginx.conf.template` başlangıçta `API_TARGET` ile doldurulur.

```sh
docker build --platform linux/amd64 -t LOGIN_SERVER/covertgift-web:KISA_COMMIT ./frontend
docker push LOGIN_SERVER/covertgift-web:KISA_COMMIT
az containerapp update -g covertgift-rg -n covertgift-web --image LOGIN_SERVER/covertgift-web:KISA_COMMIT
```

- Uygulama adı `covertgift-web`; ingress external, target port 5173 (nginx bu portu dinler).
- `API_TARGET=http://covertgift-server`: API'ye ortamın içinden kısa adla gidilir (standart ortamın servis keşfi). `UI_HOST`/`UI_PORT` artık kullanılmaz.
- nginx `/api/`, `/health`, `/swagger/`, `/docs` yollarını API'ye iletir; bilinmeyen yollar `index.html`'e düşer.
- Gövde sınırı: genel API bloğunda `client_max_body_size 1m`; yalnızca `^/api/v1/(groups/[^/]+/cover|me/avatar)$` için ayrı bir regex blok `client_max_body_size 3m` kullanır (2 MiB kapak ya da profil fotoğrafı + zarf payı). Bu blok genel API bloğundan **önce** durmalıdır.
- `/api/v1/dev/*` büyük/küçük harften bağımsız olarak nginx'te 403 ile engellenir. Bu kural regex olarak API kuralından **önce** durmalıdır; düz prefix location yazılırsa API regex'i önce eşleşir ve istek API'ye gider. Engeli yalnızca ölü bir hedefle (`API_TARGET=http://127.0.0.1:9`) deneyin, çalışan bir API'ye karşı denemeyin.
- Güvenlik başlıkları (`nosniff`, `Referrer-Policy`, `X-Frame-Options: DENY`) ve `Cache-Control` sunucu düzeyindedir; parmak izli `/assets/` bir yıl, HTML ve API yanıtları `no-cache`.

## Geçici panel uygulaması (eski, geri dönüş için)

- İmaj: `LOGIN_SERVER/covertgift-ui:demo`
- Aynı Container Apps Environment; Consumption.
- Ingress: external, HTTPS, target port 5173.
- HTTP ölçeklendirme: minimum 0, maksimum 1 replica.
- `API_TARGET=https://API_INTERNAL_FQDN`: Azure'un API için verdiği tam adres.
- `UI_HOST=0.0.0.0` ve `UI_PORT=5173` imajda tanımlıdır.

Panel API'ye sunucu üzerinden proxy yapar. Ayrı uygulamalarda API hedefi localhost
olamaz. Bu panel geçici demo içindir; kalıcı frontend yerine kullanılmamalıdır.

## Kontrol

Dağıtımdan sonra canlıda yalnızca **okuma** kontrolleri yapılır; canlı veriye yazan istek atılmaz:

- `/` 200; `/health` 200 ve `db: up` (AutoMigrate başarılı demektir).
- Oturumsuz `/api/v1/me`, `/api/v1/notifications`, `/api/v1/groups/<id>/cover`, `/api/v1/groups/<id>/pools` ve `/api/v1/invites/<herhangi>` 401 (son ikisi yeni API'nin yanıt verdiğini de gösterir; eski API 404 dönerdi). `/invite/<herhangi>` 200 (SPA).
- İki Container App de 0–1 replica ve `Running`.
- `/api/v1/dev/*` nginx'te 403 döner; bu engeli canlıda **istek atarak denemeyin**, yerelde ölü hedefle doğrulayın (yukarıdaki "İmajları hazırlama" ve frontend bölümleri). Hazır bir kontrol listesi kullanıyorsanız `/api/v1/dev/` ile başlayan hiçbir satır içermediğinden emin olun.

Kayıt, giriş, grup oluşturma, kapak yükleme ve AI önerisi gibi yazan akışları kullanıcı kendisi dener. Geliştirme sıfırlama endpoint'i frontend nginx'inde (ve eski panel proxy'sinde) engellenir; API'de
`APP_ENV=production` bu işlemi ayrıca kapatır.

Swagger şemasındaki localhost host değeri düzeltilmeden bulutta Try it out
özelliği çalışmaz. Gemini kota ve ücretleri Azure kredisinden bağımsızdır.
Ücretsiz servis uygunluğunu ve registry/log/depolama maliyetlerini portalda kontrol edin.

`GEMINI_THINKING_LEVEL=low` Search kapalı önerilerde düşünme süresini azaltmak için seçilir. Boş bırakılırsa modelin varsayılanı kullanılır; model değiştirilirken desteklenen seviyeler kontrol edilmelidir.

## Olay: Express ortamında internal ingress (2026-11-16 – 2026-11-19)

- **Belirti:** 2026-11-16 13:46 UTC'de API'ye yeni revizyon geldiği andan itibaren arayüzden API'ye giden her istek ağ geçidinden 404 `{"error":"Not found"}` aldı. Aynı yanıtı var olmayan bir uygulama adı da alıyordu. API sağlıklıydı ama hiç istek almadı (`ScaledToZero`).
- **Etkisi olmayanlar:** Eski imaja dönmek, yeniden başlatmak, ingress'i yeniden uygulamak ya da kapatıp açmak, `.internal.` adresi ve aynı ayarlarla yeni bir internal uygulama (`covertgift-backend`). Bunların hiçbiri sonucu değiştirmedi.
- **Ayırt edici test:** Sıfırdan kurulan bir Express ortamında Microsoft'un örnek imajı (`mcr.microsoft.com/k8se/quickstart`) internal ingress ile kuruldu. Diğer uygulamadan 404 aldı; hedef external yapılınca istek hemen ulaştı. Aynı test standart (WorkloadProfiles) ortamda internal ingress ile çalıştı.
- **Açıklama (tahmin, changelog ile doğrulanmadı):** Express belgesine göre uygulamalar birbirine "herkese açık adresleri üzerinden" ulaşır ve iç servis keşfi yoktur. Internal ingress'li bir uygulamanın ortamın giriş kapısında yolu olmadığı için bu çağrılar 404'e düşüyor. Kurulum iki gün çalıştı; büyük ihtimalle uygulama revizyon değişikliğiyle yeni Express davranışına geçti. Benzer kayıt: `microsoft/azure-container-apps#1714`.
- **Microsoft desteği:** Talep 2611160050004048. Yanıtlar yapay zekâ tarafından üretildi ("yönlendirme kaydı koptu, yeni uygulama kurun"), talebe mühendis atanmadı ve talep zaman aşımıyla kapatıldı.
- **Çözüm:** Standart ortam `covertgift-apps`'e taşındı. Kod ve imajlar değişmedi.
- **Ders:** Arayüz → internal API düzeni Express'te çalışmıyor; Express kullanılmaz. Dağıtımdan önce canlıda `/health` kontrol edilir.
