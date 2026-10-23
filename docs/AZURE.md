# Azure demo dağıtımı

> **Güncel AI modu (2026-10-21):** Google Search kapalıdır (`GEMINI_SEARCH_ENABLED=false`).
> `GEMINI_MODEL=gemini-3.8-flash` ile tek çağrıda öneri üretilir; fiyatlar doğrulanmamış tahmindir.
> Model kimlikleri uygulama kodunda sabit değildir. `GEMINI_MODEL` anahtar varsa zorunludur.
> `GEMINI_FALLBACK_MODEL` boşsa otomatik model geçişi yapılmaz; doluysa yalnızca 503'te denenir.
> `GEMINI_RESEARCH_MODEL` yalnızca Search açıkken kullanılır; boşsa ana model kullanılır.
> Yerelde `backend/.env`, Azure'da Container App ortam değişkenleri düzenlenir ve uygulama yeniden başlatılır/yeni revision oluşur.
> Aşağıdaki eski Search araştırması açıklamaları yalnızca Search modu tekrar açılırsa geçerlidir.

Go API ve kalıcı React frontend ayrı imajlar olarak paketlenir. PostgreSQL ayrı bir yönetilen serviste çalışır.

> **2026-10-21 güncellemesi:** `covertgift-ui` uygulaması artık geçici panel yerine kalıcı React frontend'i
> (`covertgift-web:ff8bb3c`) çalıştırır. API `covertgift-api:ff8bb3c` imajındadır. Geri dönüş için eski imajlar
> registry'de durur: API `covertgift-api:search-off-20261021`, panel `covertgift-ui:demo`.
> Express ortamında `--revision-suffix` desteklenmez; `az containerapp update` yalnızca `--image` ile çalıştırılır.
>
> **2026-10-23 — ORTAM KAPALI.** Son dağıtım: frontend `covertgift-web:2e0d1e7` (Nötr grafit tasarımı), API `covertgift-api:ff8bb3c` (backend kodu `2e0d1e7` ile aynı). Ardından kullanıcı isteğiyle `covertgift-ui`, `covertgift-api` ve `covertgift-db-msheker` durduruldu; hiçbir kaynak silinmedi.
>
> ⚠️ **Azure, durdurulan PostgreSQL esnek sunucusunu 7 gün sonra kendiliğinden yeniden başlatır.** Kapalı tutmak için her hafta yeniden `stop` çalıştırılmalıdır (ya da uzun süreli kullanılmayacaksa yedek alınıp sunucu silinir). Durdurulmuş sunucuda depolama (32 GiB) ve yedekler, registry'de ise ACR ücreti işlemeye devam eder.
>
> Yeniden açmak (sıra önemli: önce veritabanı):
>
> ```sh
> az postgres flexible-server start -g covertgift-rg -n covertgift-db-msheker
> for a in covertgift-api covertgift-ui; do
>   az rest --method post --url "https://management.azure.com$(az containerapp show -g covertgift-rg -n $a --query id -o tsv)/start?api-version=2024-03-01"
> done
> ```
>
> Bu CLI sürümünde `az containerapp stop/start` yok; durdurma ve başlatma REST API'nin `stop`/`start` uçlarıyla yapılır (`.../stop?api-version=2024-03-01`).
> Yeniden açmadan önce ROADMAP Faz 8'deki güvenlik güncellemeleri (Go 1.27.3, nginx 1.30.6+) uygulanmalıdır; çalışan imajlar Go 1.27.1 ve nginx 1.27.5 içerir.

## Mevcut demo ortamı

- Bölge: Poland Central; kaynak grubu: `covertgift-rg`.
- Ortam: `covertgift-env`, Express modu.
- API: `covertgift-api`, yalnızca ortam içinden erişim, port 8080.
- Panel: https://covertgift-ui.yellowplant-0f3e00f1.polandcentral.azurecontainerapps.io
- Registry: `covertgiftmsheker.azurecr.io`; imajlar commit kısa kimliğiyle etiketlenir (ör. `ff8bb3c`). `demo` etiketi ilk dağıtımın imajıdır.
- PostgreSQL: `covertgift-db-msheker`, sürüm 17, B1ms, 32 GiB.
- İmaj çekme kimliği: `covertgift-acr-pull` (kullanıcı atanmış), registry kapsamında AcrPull.
- API: 0.5 vCPU / 1 GiB; panel: 0.25 vCPU / 0.5 GiB. İkisi de 0–1 replica.
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
az containerapp update -g covertgift-rg -n covertgift-api --image LOGIN_SERVER/covertgift-api:$TAG
az containerapp update -g covertgift-rg -n covertgift-ui --image LOGIN_SERVER/covertgift-web:$TAG
```

Yerelde smoke test yaparken frontend imajını `API_TARGET=http://127.0.0.1:9` gibi ölü bir hedefle çalıştırın;
engel kurallarını çalışan bir API'ye karşı denemeyin. Eski panel imajı `./prototypes/test-ui` klasöründen derlenir.

Yer tutucuları kendi değerlerinizle değiştirin. Container Apps'in yönetilen kimliğine,
kayıt defterinin izin modeline uygun imaj çekme yetkisini verin.

## API uygulaması

- İmaj: `LOGIN_SERVER/covertgift-api:demo`
- İş yükü profili: Consumption
- Başlangıç kaynağı: 0.25 vCPU / 0.5 GiB; bellek yetersizse artırın.
- Ingress: internal, target port 8080. Panel ile aynı ortamda çalıştırın.
- HTTP ölçeklendirme: minimum 0, maksimum 1 replica.
- Ortam değişkenleri: `PORT=8080`, `APP_ENV=production`, `GIN_MODE=release`.
- Secret üzerinden: `DATABASE_URL`, `JWT_SECRET`; AI için ayrıca `GEMINI_API_KEY`.
- `GEMINI_RESEARCH_MODEL`: Google Search araştırması; kod varsayılanı `gemini-3.8-flash`.
- `GEMINI_MODEL`: Search kullanmayan JSON seçimi; kod varsayılanı `gemini-3.5-flash-lite`.
- Kota/istek sınırı hataları API üzerinden HTTP 429 döner. Bu varsayılanlar yerel kaynak kodunu tarif eder; dağıtımda tanımlanan ortam değişkenlerini ayrıca kontrol edin.

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
az containerapp update -g covertgift-rg -n covertgift-ui --image LOGIN_SERVER/covertgift-web:KISA_COMMIT
```

- Uygulama adı `covertgift-ui` olarak kaldı; ingress external, target port 5173 (nginx bu portu dinler).
- `API_TARGET=https://API_INTERNAL_FQDN` mevcut ortam değişkeni olarak kullanılır. `UI_HOST`/`UI_PORT` artık kullanılmaz.
- nginx `/api/`, `/health`, `/swagger/`, `/docs` yollarını API'ye iletir; bilinmeyen yollar `index.html`'e düşer.
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

Panel HTTPS adresinde `/` açılmalı, `/health` 200 ve `db: up` dönmeli,
oturumsuz `/api/v1/me` 401 dönmeli. Kayıt, giriş, grup oluşturma ve AI önerisini
deneyin. Geliştirme sıfırlama endpoint'i panel proxy'sinde engellenir; API'de
`APP_ENV=production` bu işlemi kapatır.

Swagger şemasındaki localhost host değeri düzeltilmeden bulutta Try it out
özelliği çalışmaz. Gemini kota ve ücretleri Azure kredisinden bağımsızdır.
Ücretsiz servis uygunluğunu ve registry/log/depolama maliyetlerini portalda kontrol edin.

`GEMINI_THINKING_LEVEL=low` Search kapalı önerilerde düşünme süresini azaltmak için seçilir. Boş bırakılırsa modelin varsayılanı kullanılır; model değiştirilirken desteklenen seviyeler kontrol edilmelidir.
