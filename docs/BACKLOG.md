# Backlog — Hatalar ve Fikirler

Not: Bu liste henüz düzeltilmemiş bug'ları ve fikirleri toplar. Çözülenler silinir veya DEVLOG'a taşınır.

## Sıradaki oturum planı (2026-11-15'te hazırlandı)

Öneri sırası: önce ACİL madde (0), sonra güvence (1–3), sonra küçük bir dağıtım (4–5), sonra ürün işleri arasından seçim.

### 0. ACİL — "Senden bekleyenler" paneli (kullanıcı isteği, 2026-11-15)
#28'den sonra panel masaüstünde kartın yüksekliğine sabitlendi ve listesi kendi içinde kayıyor. Canlıda (koyu tema) görünüm kötü:
- Tarayıcının varsayılan, kalın gri kaydırma çubuğu ve izi görünüyor.
- Üçüncü satır yarıdan kesiliyor ("Normal" etiketi ve başlık yarım kalıyor); kaydırılabildiğine dair başka bir ipucu yok.

Ya kaydırma çubuğu değiştirilecek ya da alan baştan tasarlanacak. Seçenekler:
- **(a) Küçük düzeltme:** İnce, token renkli bir kaydırma çubuğu; altta yumuşak bir solma ipucu; satırlar yarım kesilmeyecek şekilde yükseklik ayarı ya da scroll-snap.
- **(b) Yeniden tasarım (önerilen):** İç kaydırma kaldırılır. Panelde en önemli 2–3 görev ve altında "Tümünü gör (4)" ile genişleyen liste ya da ayrı bir görünüm olur. Mavi kart yine kendi yüksekliğinde kalır (#28'in amacı korunur), paneldeki hiçbir satır kesilmez. Telefon düzeni de birlikte gözden geçirilir.

Karar oturum başında, birkaç örnekle birlikte verilecek. Dosyalar: `frontend/src/pages/GroupsPage.tsx` (`TasksPanel`) ve `components/Panel.tsx`.

### A. Güvence ve altyapı (önce)
1. **Playwright e2e'yi çalıştır ve düzelt.** Bu oturumda hiç çalıştırılmadı; #25 (sekme yapısı), #28 (ana sayfa düzeni) ve #29 (avatar) `multi.spec`, `profile.spec` ve `acceptance.spec`'i bozmuş olabilir. `npm --prefix frontend run test:e2e`; API ve test DB açık olmalı.
2. **CI kur (GitHub Actions).** Şu an hiçbir PR'da otomatik kontrol yok; #25–#30 CI'sız merge edildi. Backend için `go vet` ve `go test ./...`, ayrıca Postgres servisiyle `-p 1 ./cmd/...` entegrasyon testleri. Frontend için `tsc -b`, `oxlint` ve Vitest. Playwright ayrı bir iş olarak, istenirse.
3. **Test DB'yi ayır.** Açık dev sunucusu (`api-testdb`) ile entegrasyon testleri aynı `covertgift_test`'i kullanınca testler bir kez düştü, seed testi de oturumları düşürdü. Önizleme için ayrı bir veritabanı (ör. `covertgift_preview_test`) ya da ayrı bir konteyner.

### B. Küçük teknik borç (tek dağıtımda toplanabilir)
4. **Go 1.27.4'e yükselt.** GO-2026-6859, `net/http` `MaxBytesReader`'daki bir açık; kapak ve avatar yüklemesinin gövde sınırı tam bunu kullanıyor.
5. **nginx'te avatar yükleme sınırı.** `PUT /api/v1/me/avatar` genel 1 MB sınırına takılıyor, sunucu ise 2 MB kabul ediyor. Kapak gibi ayrı bir 3 MB bloğu ekle. Ardından Azure'a dağıt.
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
