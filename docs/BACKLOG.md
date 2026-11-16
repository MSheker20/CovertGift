# Backlog — Hatalar ve Fikirler

Not: Bu liste henüz düzeltilmemiş bug'ları ve fikirleri toplar. Çözülenler silinir veya DEVLOG'a taşınır.

## Sıradaki oturum planı (2026-11-15'te hazırlandı)

Öneri sırası (2026-11-16'da güncellendi): önce ACİL madde (0) ile 4–5 tek dağıtımda, sonra güvence (1–3), sonra ürün işleri arasından seçim.

### 0. ACİL — "Senden bekleyenler" paneli ve aynı dağıtım (2026-11-16)
Panel düzeltmesi `fix/tasks-panel` PR'ında (ayrıntı DEVLOG'da). Kalan sıra:
1. **PR 2:** `chore/go-1.27.4-avatar-limit` (madde 4 ve 5).
2. İki PR merge edilince tek Azure dağıtımı. Dağıtımdan önce kullanıcı onayı alınır; canlıda yalnızca okuma kontrolü yapılır ve panelin görünümüne bakılır.

Playwright e2e (1) ve CI (2) bu turun dışında kaldı, sonraki adımdır.

### A. Güvence ve altyapı (önce)
1. **Playwright e2e'yi çalıştır ve düzelt.** Bu oturumda hiç çalıştırılmadı; #25 (sekme yapısı), #28 (ana sayfa düzeni) ve #29 (avatar) `multi.spec`, `profile.spec` ve `acceptance.spec`'i bozmuş olabilir. `npm --prefix frontend run test:e2e`; API ve test DB açık olmalı.
2. **CI kur (GitHub Actions).** Şu an hiçbir PR'da otomatik kontrol yok; #25–#30 CI'sız merge edildi. Backend için `go vet` ve `go test ./...`, ayrıca Postgres servisiyle `-p 1 ./cmd/...` entegrasyon testleri. Frontend için `tsc -b`, `oxlint` ve Vitest. Playwright ayrı bir iş olarak, istenirse.
3. **Test DB'yi ayır.** Açık dev sunucusu (`api-testdb`) ile entegrasyon testleri aynı `covertgift_test`'i kullanınca testler bir kez düştü, seed testi de oturumları düşürdü. Önizleme için ayrı bir veritabanı (ör. `covertgift_preview_test`) ya da ayrı bir konteyner.

### B. Küçük teknik borç (tek dağıtımda toplanabilir)
4. **Go 1.27.4'e yükselt.** GO-2026-6859, `net/http` `MaxBytesReader`'daki bir açık; kapak ve avatar yüklemesinin gövde sınırı tam bunu kullanıyor. (2026-11-16: ACİL panelle aynı dağıtıma alındı, PR 2.) Güncellenecek yerler: `go.work`, `backend/go.mod`, `backend/Dockerfile` (`golang:1.27.4-bookworm`), `.claude/launch.json` (`GOTOOLCHAIN`). Doğrulama: `go vet`, `go test ./...` ve `-p 1 ./cmd/...` entegrasyon testleri.
5. **nginx'te avatar yükleme sınırı.** `PUT /api/v1/me/avatar` genel 1 MB sınırına takılıyor, sunucu ise 2 MB kabul ediyor. (2026-11-16: PR 2'ye dahil.) `frontend/nginx.conf.template`'e `^/api/v1/me/avatar$` için `client_max_body_size 3m` bloğu eklenecek. Bu blok genel API bloğundan önce durmalı ve `proxy_*` ayarlarını kopyalamalı, çünkü miras alınmıyorlar. Yerel web imajında kontrol: ~1.5 MB avatar 413 almamalı, `/ideas`'a 2 MB gövde 413 almalı. Güncellenecek belgeler: `docs/AZURE.md` (nginx bölümü ve sürüm tablosu), `PROJECT_CONTEXT.md` (gövde sınırı satırı).
6. **graphify grafiğini güncelle** (`/graphify --update`); grafik #25–#30'dan önceki koddan.

### C. Ürün işleri (seçilecek)
7. **Bireysel katkı takibi (kim ne kadar ödedi).** MULTI katılımcı listesinin doğal devamı; şema ve arayüz işi, en büyük madde.
8. **Frontend cilaları** (ROADMAP): oy sonrası kartların zıplamaması, Gemini önerisi gelince yeni kartların vurgulanması, daha yönlendirici bir boş havuz durumu. Azaltılmış hareket tercihine uyulacak.
9. **Profil fotoğrafının canlı güncellenmesi.** Biri fotoğrafını değiştirince başkalarının açık sayfası ancak veri yenilenince görüyor. Kullanıcının gruplarına SSE `changed` olayı gönderilebilir.
10. **Link önizleme önbelleği** (aynı URL için 24 saat) ve **Trendyol önizlemesinin Azure'da çalışması** (araştırma; ROADMAP'te ayrıntı var).
11. **AI tarafı (ertelendi):** manuel ve beğenilen önerilerden tercih çıkarımı, kalan AI hata yanıtları (422/503).

### D. Operasyon ve güvenlik
12. **Azure güvenlik duvarı.** `ClientIPAddress_2026-10-19` kuralındaki IP (212.154.121.233) artık senin IP'n değil; kaldırılmalı mı karar ver. `AllowAzureServices` tüm Azure IP'lerine açık; daha dar bir yalıtım ileride düşünülebilir.
13. **Eski bildirim temizliği** (tablo büyürse) ve `~/CovertGift-backups/2026-11-15/` yedeklerinin ne kadar saklanacağı.
14. **Maliyet.** Kullanılmayacaksa ortamı durdur (belge: `docs/AZURE.md`, "Durdurma ve yeniden açma"); PostgreSQL 7 gün sonra kendiliğinden açılır.

## Bug'lar

Şu an açık bug yok.

## Fikirler

Şu an açık fikir yok.
