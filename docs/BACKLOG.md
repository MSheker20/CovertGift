# Backlog — Hatalar ve Fikirler

Not: Bu liste henüz düzeltilmemiş bug'ları ve fikirleri toplar. Çözülenler silinir veya DEVLOG'a taşınır.

## Bug'lar

- [ ] **Ortaklıktan çıkınca alt yazı kalıyor.** Katılımcılar listesinde kişi "Katılmıyor" konumuna getirilse bile adının altındaki "Bu hediyeye ortağım" yazısı olduğu gibi duruyor. Alt yazı katılım durumuna göre güncellenmeli (veya katılmıyorsa gizlenmeli). Frontend, katılımcılar kartı.

- [ ] **Ana sayfada "Sıradaki sürpriz" mavi kartı "Senden bekleyenler" ile birlikte uzuyor.** İki kart aynı grid satırında olduğu için "Senden bekleyenler" listesi uzayınca mavi kart da onunla aynı yüksekliğe esniyor ve ekranda dev bir mavi alan oluşuyor. Mavi kartın yüksekliği içeriğine göre sabit kalmalı (ör. grid'de `items-start` / kendi yüksekliği), "Senden bekleyenler" ise kendi içinde uzamalı veya sınırlı yükseklikte kaydırılmalı. Frontend, ana sayfa (dashboard).

- [ ] **MULTI grup: ana sayfadaki "Sıradaki sürpriz" kartında tarih kime ait belli değil.** Kartın alt satırı grubun tüm alıcılarını sayıyor ("Zeynep ve senin için", `recipientsLine`), tarih ise yalnızca en yakın havuzun tarihi (`next_event_date`). Yani "Ofis Yılbaşı · Zeynep ve senin için · 14 Aralık" gibi okunuyor; 14 Aralık'ın hangi kişinin hediyesi olduğu anlaşılmıyor ve API'de de yok. Gerekenler: backend `GroupListItem`'a en yakın havuzun alıcı adı (Anti-Spoiler: yalnızca çağıranın görebildiği havuzlardan; kendi havuzu görünmez) ve frontend'de kartta "Zeynep için · 14 Aralık" gösterilmesi. Aynı grupta iki havuz aynı tarihliyse nasıl gösterileceği kararlaştırılmalı. `GroupsPage.tsx` (`NextSurpriseCard`, `forLine`), `lib/priority.ts` (`nextSurprise`).
- [ ] **MULTI grup bildirimleri kontrol edilmeli.** Kod tarafında bildirim metinleri alıcı adını destekliyor (`lib/notifications.ts`: `recipient_name` varsa "… için …" metinleri; `pool_added` ve `pool_manager_assigned` de var), tıklayınca `?pool=` ile doğru sekmeye gidiyor. Ama bu yalnızca koddan okundu, tarayıcıda denenmedi ve seed'de havuz bildirimi yok (test DB'de `pool_id` dolu bildirim sayısı 0). Yapılacaklar: seed'e MULTI havuz bildirimleri ekle (fikir eklendi, tarih değişti, hatırlatma, süre doldu); Can, Elif ve Zeynep hesaplarında metinleri, alıcıya bildirim gitmediğini (Anti-Spoiler) ve tıklayınca doğru havuz sekmesine gidişi kontrol et; TR/EN metinleri gözden geçir.

## Fikirler

- [ ] **MULTI grup: kişi seçici sekmeleri (Mustafa / Eren Y. / Selinnur Y. ...) daha göze hoş ve ayırt edilebilir olmalı.** Şu an düz yazılı hap (pill) butonlar; seçili olan mavi, diğerleri soluk, kilitli olan (sen) kilit ikonlu. UX açısından kişiler zor ayrışıyor.
  - Profil fotoğrafı (pp) için ön hazırlık yapılabilir: üstte pp, altında isim düzeni (avatar kartı / story tarzı).
  - Karar yöntemi: birkaç örnek tasarım çalışması (soru-cevap ile) üretip birlikte seçeceğiz.
  - **Karar (2026-11-15, son hâli, uygulandı):** Rehber tarzı çıkıntı sekme ve sade alt çizgili satır denendi, beğenilmedi. Son tasarımda havuz içeriği tek bir çerçeveli sayfada duruyor. Kişi seçici bu sayfanın üst kenarını boydan boya kaplayan bitişik bir şerit: kişiler eşit genişlikte, içerik ortalı ve aralarında tek bir dikey çizgi var. Şeridin köşeleri sayfanınkiyle aynı yuvarlaklıkta. Seçili kişi sayfayla aynı zeminde ve alt çizgisiz, yani havuza açılıyor; diğer kişiler gri zeminde ve alt çizgiyle ayrılıyor. Sekmede küçük avatar, isim, oy sayısı ve kupa var; tarih panel başlığında. "Üyeler" paneli sayfanın dışında. Mobilde şerit satır atlamaz, kendi içinde kayar. Profil fotoğrafı (`avatar_url`) henüz yok.
- **Karar (2026-11-15), "Sıradaki sürpriz" kartında aynı tarih:** İki havuz aynı tarihteyse iki isim birlikte gösterilir ("Zeynep ve Eren için"). İkiden fazla kişi varsa "Zeynep ve 2 kişi için" yazılır.
- [ ] **Kişi sekmeleri hediye tarihine göre sıralansın.** Yaklaşan tarih öne gelsin. Tarihi olmayanlar sona gider; kullanıcının kendi (kilitli "sen") kartı her zaman en sonda.
