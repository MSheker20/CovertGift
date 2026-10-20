# Azure demo dağıtımı

Bu dosyalar mevcut Go API ve geçici test panelini ayrı imajlar olarak paketler.
Kalıcı frontend henüz hazır değildir. PostgreSQL ayrı bir yönetilen serviste çalışır.

## Mevcut demo ortamı

- Bölge: Poland Central; kaynak grubu: `covertgift-rg`.
- Ortam: `covertgift-env`, Express modu.
- API: `covertgift-api`, yalnızca ortam içinden erişim, port 8080.
- Panel: https://covertgift-ui.yellowplant-0f3e00f1.polandcentral.azurecontainerapps.io
- Registry: `covertgiftmsheker.azurecr.io`; imaj etiketi: `demo`.
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

Proje kökünden, Azure Container Apps için Linux/AMD64 imajları oluşturun:

```sh
docker build --platform linux/amd64 -t covertgift-api:demo ./backend
docker build --platform linux/amd64 -t covertgift-ui:demo ./prototypes/test-ui
```

İmajları Azure Container Registry'ye gönderin. Kayıt defterinin Overview ekranındaki
Login server değerini kullanın; adresi kayıt defteri adından türetmeyin:

```sh
az acr login --name KAYIT_DEFTERI_ADI
docker tag covertgift-api:demo LOGIN_SERVER/covertgift-api:demo
docker tag covertgift-ui:demo LOGIN_SERVER/covertgift-ui:demo
docker push LOGIN_SERVER/covertgift-api:demo
docker push LOGIN_SERVER/covertgift-ui:demo
```

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

## Geçici panel uygulaması

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
