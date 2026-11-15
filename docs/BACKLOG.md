# Backlog — Hatalar ve Fikirler

Not: Bu liste henüz düzeltilmemiş bug'ları ve fikirleri toplar. Çözülenler silinir veya DEVLOG'a taşınır.

## Bug'lar

- [ ] **Ortaklıktan çıkınca alt yazı kalıyor.** Katılımcılar listesinde kişi "Katılmıyor" konumuna getirilse bile adının altındaki "Bu hediyeye ortağım" yazısı olduğu gibi duruyor. Alt yazı katılım durumuna göre güncellenmeli (veya katılmıyorsa gizlenmeli). Frontend, katılımcılar kartı.

- [ ] **Ana sayfada "Sıradaki sürpriz" mavi kartı "Senden bekleyenler" ile birlikte uzuyor.** İki kart aynı grid satırında olduğu için "Senden bekleyenler" listesi uzayınca mavi kart da onunla aynı yüksekliğe esniyor ve ekranda dev bir mavi alan oluşuyor. Mavi kartın yüksekliği içeriğine göre sabit kalmalı (ör. grid'de `items-start` / kendi yüksekliği), "Senden bekleyenler" ise kendi içinde uzamalı veya sınırlı yükseklikte kaydırılmalı. Frontend, ana sayfa (dashboard).

- [ ] **MULTI grup bildirimleri kontrol edilmeli.** Kod tarafında bildirim metinleri alıcı adını destekliyor (`lib/notifications.ts`: `recipient_name` varsa "… için …" metinleri; `pool_added` ve `pool_manager_assigned` de var), tıklayınca `?pool=` ile doğru sekmeye gidiyor. Ama bu yalnızca koddan okundu, tarayıcıda denenmedi ve seed'de havuz bildirimi yok (test DB'de `pool_id` dolu bildirim sayısı 0). Yapılacaklar: seed'e MULTI havuz bildirimleri ekle (fikir eklendi, tarih değişti, hatırlatma, süre doldu); Can, Elif ve Zeynep hesaplarında metinleri, alıcıya bildirim gitmediğini (Anti-Spoiler) ve tıklayınca doğru havuz sekmesine gidişi kontrol et; TR/EN metinleri gözden geçir.

## Fikirler

- [ ] **Profil fotoğrafı (pp).** Kişi sekmeleri ve avatarlar şu an baş harf ile kişi rengini gösteriyor. `AccountSummary`'ye `avatar_url` eklenmeli; `Avatar` bileşeni de foto varsa onu göstermeli.

