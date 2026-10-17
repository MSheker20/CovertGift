# Geçici backend test arayüzü

Son doğrulama: **2026-10-17**. Cloudflare üzerinden panel 200, `/health` 200 (`db: up`) ve oturumsuz `/api/v1/me` 401 döndü.

Paket kurulumu ve derleme gerektirmeyen HTML/CSS/JavaScript paneli. Kalıcı Faz 7 frontend'i değildir; sonradan bu klasör kaldırılabilir. Mobil tarayıcıda da kullanılabilir.

Proje kökünden, Node.js 22 veya üzeriyle:

```bash
node prototypes/test-ui/server.mjs
```

Tarayıcı: **http://127.0.0.1:5173**

Backend ayrı terminalde çalışmalı:

```bash
# Docker Desktop açıkken, proje kökünde
docker compose up -d --wait
cd backend
go run ./cmd/api
```

Panel sunucusu `/api/v1/*`, `/health`, `/swagger/*` ve `/docs` isteklerini `http://127.0.0.1:8080` adresine iletir. Böylece backend CORS ayarı değiştirilmez. `index.html` dosyasını doğrudan açmak yerine yukarıdaki sunucuyu kullanın.

API proxy'si yalnızca panelin kullandığı auth, profil, grup, hediye ve oy yollarını iletir. `/api/v1/dev/reset-db` dahil geliştirme uçları proxy'den 403 alır; veritabanını topluca sıfırlamak için kullanılamaz.

Geçici Cloudflare bağlantısı için panel çalışırken ayrı terminalde:

```bash
cloudflared tunnel --url http://127.0.0.1:5173 --no-autoupdate
```

Komutun verdiği HTTPS adresi, panel ve API'ye aynı adres üzerinden erişim sağlar. Adres her yeni tünelde değişebilir; kalıcı alan adı veya dağıtım değildir. Tünel açık olduğu ve bilgisayar/sunucular çalıştığı sürece kullanılabilir. Tüneli kapatmak için o terminalde `Ctrl+C` kullanın. Backend'in 8080 portunu doğrudan tünele bağlamayın; panelin kısıtlı proxy'sini kullanın. `cloudflared` ayrıca kurulu olmalıdır.

Yerel ve paylaşılan bağlantıda `/health` yanıtı `{"success":true,"data":{"db":"up","status":"ok"}}` olmalıdır. UI ve API aynı origin'i kullanır; ayrıca CORS değişikliği gerekmez. İlk `.env` kurulumu için [ana README](../../README.md#-hızlı-başlangıç) adımlarını izleyin.

Farklı backend veya UI portu:

```bash
API_TARGET=http://127.0.0.1:8081 UI_PORT=5174 node prototypes/test-ui/server.mjs
```

Telefondan aynı yerel ağda erişmek için:

```bash
UI_HOST=0.0.0.0 node prototypes/test-ui/server.mjs
```

Telefonda bilgisayarın yerel IP adresini `http://BILGISAYARIN_YEREL_IPSI:5173` biçiminde açın. Backend'e telefondan doğrudan erişmek gerekmez. Varsayılan bağlanma adresi yalnızca bu bilgisayara açıktır; bu araç yerel geliştirme içindir.

## Denenebilen işlemler

- Kayıt, giriş, profil, çıkış ve onaylı hesap silme.
- Grup listesi, yeni grup, ID ile grup açma, üyeler, e-posta ile üye ekleme ve onaylı grup silme.
- Grup detayında ilgi alanları listesi: tüm üyeler giriş alanı ve “Ekle” butonuyla tek tek ekler; yalnızca admin öğelerin yanındaki “Sil” butonunu görür ve silebilir. Üyeler kendi eklediklerini de silemez. Boş/tekrarlanan öğeler kabul edilmez. Mevcut virgül veya satır sonuyla ayrılmış ilgi alanları listeye dönüştürülerek gösterilir; yeni AI önerileri güncel listeyi kullanır.
- Toplam/kişi başı bütçe, manuel hediye ekleme, link önizleme, Gemini önerileri.
- Admin için toplam bütçe güncelleme formu. Kaydedince kişi başı tutar, grup listesi ve hediye bütçe aşımı uyarıları yenilenir. 0 bütçe aşımı uyarılarını kapatır.
- Fikirleri durumuna göre filtreleme, +1/−1 oy, skor/kendi oyu, admin vetosu ve onaylı hediye silme.
- API sağlık kontrolü ve son 15 yanıtın HTTP kodu/JSON gövdesi. Token gizlenir; şifre/istek gövdeleri günlüğe yazılmaz.

Oturum token'ı sekmeye özel `sessionStorage` içinde tutulur. Kullanıcı değişimi için çıkış yapabilir veya ayrı bir sekmede giriş yapabilirsiniz. Başka hesaptan açılan sayfadaki kullanıcı ID'sini grup oluştururken hedef ID olarak kullanıp Anti-Spoiler denemesi yapabilirsiniz. Hedef hesapta **ID ile grup aç** formuyla 403 yanıtını kontrol edin. Panel, UI görünürlüğünden bağımsız olarak yetkilendirmeyi API'ye bırakır; grup silme dahil yetkisiz işlemler sunucudan hata döner.

Önizleme kısmi dönerse formu elle doldurun. AI için backend'de geçerli Gemini yapılandırması gerekir. AI önce Türkiye mağazalarında Google Search araştırması yapar, ardından kaynakla desteklenen ürün/fiyatları seçer; en fazla 5, bazen 3'ten az öneri döner. API yanıtındaki `ai_sources` ve `ai_search_suggestions` alanları panel kartlarında henüz gösterilmez; JSON günlüğünden incelenebilir. Bütün kayıt işlemleri gerçek veritabanına gider. Veritabanını topluca sıfırlayan buton eklenmemiştir.
