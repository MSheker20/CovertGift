# Backlog — Hatalar ve Fikirler

Not: Bu liste henüz düzeltilmemiş bug'ları ve fikirleri toplar. Çözülenler silinir veya DEVLOG'a taşınır.

## Bug'lar

- [ ] **Ortaklıktan çıkınca alt yazı kalıyor.** Katılımcılar listesinde kişi "Katılmıyor" konumuna getirilse bile adının altındaki "Bu hediyeye ortağım" yazısı olduğu gibi duruyor. Alt yazı katılım durumuna göre güncellenmeli (veya katılmıyorsa gizlenmeli). Frontend, katılımcılar kartı.

- [ ] **Ana sayfada "Sıradaki sürpriz" mavi kartı "Senden bekleyenler" ile birlikte uzuyor.** İki kart aynı grid satırında olduğu için "Senden bekleyenler" listesi uzayınca mavi kart da onunla aynı yüksekliğe esniyor ve ekranda dev bir mavi alan oluşuyor. Mavi kartın yüksekliği içeriğine göre sabit kalmalı (ör. grid'de `items-start` / kendi yüksekliği), "Senden bekleyenler" ise kendi içinde uzamalı veya sınırlı yükseklikte kaydırılmalı. Frontend, ana sayfa (dashboard).

## Fikirler

- [ ] **Profil fotoğrafı (pp).** Kişi sekmeleri ve avatarlar şu an baş harf ile kişi rengini gösteriyor. `AccountSummary`'ye `avatar_url` eklenmeli; `Avatar` bileşeni de foto varsa onu göstermeli.

