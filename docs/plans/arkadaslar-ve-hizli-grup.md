# Arkadaşlar ve hızlı grup kurma — plan

> Durum: planlandı (2026-11-24). Hedef: 2026-11-25 sunumu (yerelde). Canlıya kod dağıtılır, seed **eklenmez**.

## Kararlar

| Konu | Karar |
| --- | --- |
| Arkadaşlık modeli | İstek + onay. Kabul edilince iki taraf da birbirinin listesinde. |
| Ekleme yolları | 1) Tam e-posta ile istek (kullanıcı arama/listeleme yok). 2) Kişisel arkadaşlık linki: açan kişi onaylayınca **doğrudan** arkadaş olur; sahibi linki sıfırlayabilir. |
| Hızlı grup | Mevcut "Yeni Grup" diyaloğuna arkadaş seçici. Yeni kavram (çevre vb.) yok. |
| Hedef seçimi | "Kime hediye?" arkadaş listesinden seçilebilir; e-posta alanı da kalır. |
| Gruba giriş | Seçilen arkadaşlara **grup daveti** gider; kabul edince üye olur. Ret → davet kalkar, admin tekrar davet edebilir. Süresiz; admin iptal edebilir. |
| Kapsam | Çekirdek + demo verisi (seed). QR, çevreler, Playwright sonraya. |

## Backend

### Modeller (AutoMigrate)
- `Friendship`: `requester_id`, `addressee_id`, `status` (`PENDING`/`ACCEPTED`), `created_at`, `accepted_at`.
  Çift başına tek satır: `LEAST/GREATEST` üzerinde benzersiz index (A→B ve B→A aynı anda olamaz). Kullanıcı silinince CASCADE.
- `User.FriendCode`: kişisel link için rastgele token (≥128 bit, base64url). Sıfırlanabilir. Loglanmaz (davet linki kuralı).
- `GroupMemberInvite`: `group_id`, `invitee_id`, `inviter_id`, `created_at`. (`group_id, invitee_id`) benzersiz. Grup silinince CASCADE.
- `Notification.GroupID` nullable olur (arkadaşlık bildirimleri gruba bağlı değil). Liste/okundu sorguları ve Anti-Spoiler süzmesi `group_id IS NULL` satırları da doğru ele almalı.
  Yeni türler: `friend_request`, `friend_accepted`, `group_invite` (params: actor_name, group_title).

### Uçlar
Arkadaşlık (`/api/v1/friends`, yalnızca giriş yapmış kullanıcı):
- `GET /friends` → `{ friends: [{id,name,avatar}], incoming: [...], outgoing: [...] }`
- `POST /friends/requests` `{email}` → kendine 400; kayıtlı değil 404 (mevcut `target_email` davranışıyla tutarlı); zaten arkadaş/istek var 409; karşı taraf zaten istek atmışsa doğrudan kabul.
- `POST /friends/requests/:id/accept`, `DELETE /friends/requests/:id` (ret ya da geri çekme)
- `DELETE /friends/:userId` (arkadaşlıktan çıkar; grup üyeliklerine dokunmaz)
- `GET /friends/link`, `POST /friends/link/reset`
- `GET /friend-links/:code` → sahibin adı/avatarı (önizleme), `POST /friend-links/:code/accept` → doğrudan arkadaşlık

Grup daveti:
- `POST /groups` isteği `member_ids: uuid[]` (≤ 30) ve `target_user_id` kabul eder (zaten alan var; arkadaş olma şartı eklenir). Grup oluşturma transaction'ında davetler yazılır. Yalnızca arkadaşlar davet edilebilir; hedef davet edilemez.
- `POST /groups/:id/member-invites` `{user_ids}` (admin; sonradan arkadaş davet etme), `DELETE /groups/:id/member-invites/:userId` (admin iptal). Grup detayında `pending_invites` (yalnızca admin/üyeler).
- `GET /me/group-invites`, `POST /me/group-invites/:id/accept`, `DELETE /me/group-invites/:id` (ret).
  Bunlar `/groups/:id/*` dışında olduğundan Anti-Spoiler kontrolü **elle** yapılır: davetlinin hedef olduğu grup davetleri listelenmez, kabul 403 döner.

### Anti-Spoiler ve güvenlik
- Hedef bağlanırken (`POST /groups`, `PATCH /groups/:id/target`) o kişiye ait bekleyen davet silinir; bekleyen daveti olan kişi hedef seçilirse daveti kalkar.
- MULTI: davet kabul edilince normal üye olur; havuz kuralları değişmez.
- Kilit sırası korunur: `groups` → `group_member_invites` → `group_members`. Kabul işlemi grup satırını kilitler.
- Arkadaş listesi hiçbir grup bilgisi taşımaz. E-posta yalnızca kişinin kendisine döner.
- Davet ve arkadaşlık değişiklikleri SSE ile ilgili grup anahtarını yayınlar (davet kabulü → `member_joined`).
- Hız sınırı: arkadaşlık isteği ve link kabulü mevcut rate limiter'a eklenir (e-posta tarama).

## Frontend
- **Arkadaşlar sayfası** (`/friends`, üst bar profil menüsünden): arkadaş listesi, gelen/giden istekler, "E-postayla ekle", kişisel link (kopyala ✓, sıfırla).
- **Arkadaşlık linki karşılama** (`/f/:code`): giriş yoksa giriş/kayıt sonrası geri dönüş (InvitePage deseni).
- **Yeni Grup diyaloğu**: "Kime hediye?" arkadaş seç / e-posta yaz; "Kimlerle?" arkadaş çoklu seçici (hedef listeden düşer). Arkadaş yoksa sayfaya yönlendiren boş durum.
- **Grup sayfası üyeler paneli**: bekleyen davetler ("bekliyor", admin için iptal) + "Arkadaş davet et".
- **Bildirimler**: `friend_request` ve `group_invite` satırlarında Kabul / Reddet butonları; ana sayfada "Senden bekleyenler"e grup davetleri.
- i18n: TR/EN anahtarları (`i18n-sync`). Renkler token'lardan.

## Seed (yalnızca yerel; canlıda seed çalıştırılmaz)
- `seedFriends` adımı: demo kullanıcılar arasında kabul edilmiş arkadaşlıklar, bir gelen istek, bir bekleyen grup daveti. İşaret varsa atlanır (idempotent).

## Testler
- Go birim: arkadaşlık durum geçişleri, çift yönlü tekillik, link kodu.
- Go entegrasyon (`test-db`): hedef kişi davetle/kabulle gruba giremez, davet listesinde hedefi olduğu grup görünmez, hedef bağlanınca davet silinir, arkadaş olmayan davet edilemez, bildirim süzmesi.
- Vitest: Arkadaşlar sayfası, diyalogda seçici (hedef üyelerden düşer), bildirim kabul/ret.
- Tarayıcıda `api-testdb` + `frontend-testdb` ile uçtan uca deneme.

## Sıra
1. Modeller + migration + servis + uçlar (arkadaşlık) → testler
2. Grup davetleri + Anti-Spoiler + bildirimler → entegrasyon testleri
3. Frontend: Arkadaşlar sayfası + link karşılama
4. Frontend: Yeni Grup seçici + üyeler paneli + bildirim aksiyonları
5. Seed + tarayıcıda demo provası (yerel)
6. PR → Azure dağıtımı (onayınla; yalnızca kod, seed yok) → salt-okunur kontroller
7. DEVLOG / ROADMAP güncellemesi

## Sonraya
QR kod, arkadaş çevreleri, "Ona hediye grubu kur" kısayolu, eski grup üyelerinden öneri, Playwright akışı.
