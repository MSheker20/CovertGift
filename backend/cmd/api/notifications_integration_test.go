package main

// Bildirim entegrasyon testleri: gerçek router + middleware zinciri + PostgreSQL.
// Alıcı seçimi, Anti-Spoiler (hedefe hiç bildirim gitmez; sonradan bağlanınca silinir ve
// gizlenir), okundu işaretleme yetkisi, hatırlatma zamanlayıcısı (enjekte saatle) ve akış.

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/google/uuid"
)

type notificationItem struct {
	ID         string         `json:"id"`
	GroupID    string         `json:"group_id"`
	GroupTitle string         `json:"group_title"`
	Type       string         `json:"type"`
	Params     map[string]any `json:"params"`
	ReadAt     *time.Time     `json:"read_at"`
}

type notificationList struct {
	Items       []notificationItem `json:"items"`
	UnreadCount int                `json:"unread_count"`
}

func (a *testAPI) notifications(u user) notificationList {
	a.t.Helper()
	res := a.mustOK(http.MethodGet, "/notifications", u.token, nil, http.StatusOK)
	var list notificationList
	if err := json.Unmarshal(res.Data, &list); err != nil {
		a.t.Fatalf("bildirim listesi okunamadı: %s", res.Data)
	}
	return list
}

// types kullanıcının bildirim türlerini yeniden eskiye döner.
func (a *testAPI) types(u user) []string {
	a.t.Helper()
	out := []string{}
	for _, n := range a.notifications(u).Items {
		out = append(out, n.Type)
	}
	return out
}

func (a *testAPI) userID(u user) uuid.UUID {
	a.t.Helper()
	return uuid.MustParse(idOf(a.t, a.mustOK(http.MethodGet, "/me", u.token, nil, http.StatusOK)))
}

// rowCount kullanıcının bir gruba ait bildirim satırı sayısını doğrudan tablodan okur
// (listeleme süzgecinden bağımsız).
func (a *testAPI) rowCount(groupID string, userID uuid.UUID) int64 {
	a.t.Helper()
	var count int64
	if err := a.db.Model(&models.Notification{}).Where("group_id = ? AND user_id = ?", groupID, userID).Count(&count).Error; err != nil {
		a.t.Fatal(err)
	}
	return count
}

func sameTypes(got []string, want ...string) bool {
	return strings.Join(got, ",") == strings.Join(want, ",")
}

func TestNotificationRecipientsAndAntiSpoiler(t *testing.T) {
	s := setupScenario(t)
	api := s.api
	group := "/groups/" + s.groupID

	// Kurulum: admin üyeyi ekledi, üye bir fikir ekledi.
	if got := api.types(s.member); !sameTypes(got, models.NotificationAddedToGroup) {
		t.Fatalf("üye: %v", got)
	}
	adminList := api.notifications(s.admin)
	if len(adminList.Items) != 1 || adminList.UnreadCount != 1 {
		t.Fatalf("admin: %+v", adminList)
	}
	idea := adminList.Items[0]
	if idea.Type != models.NotificationIdeaAdded || idea.Params["idea_title"] != "Kamp sandalyesi" ||
		idea.Params["actor_name"] != "Member" || idea.GroupID != s.groupID || idea.GroupTitle != "Doğum günü sürprizi" {
		t.Fatalf("fikir bildirimi yanlış: %+v", idea)
	}

	// Yeni üye: eklenen "gruba eklendin", diğer üye "yeni üye katıldı" alır; ekleyen almaz.
	extra := api.newUser("Extra")
	api.mustOK(http.MethodPost, group+"/members", s.admin.token, map[string]string{"email": extra.email}, http.StatusCreated)
	if got := api.types(extra); !sameTypes(got, models.NotificationAddedToGroup) {
		t.Fatalf("eklenen üye: %v", got)
	}
	memberList := api.notifications(s.member)
	if memberList.Items[0].Type != models.NotificationMemberJoined || memberList.Items[0].Params["member_name"] != "Extra" {
		t.Fatalf("üye katıldı bildirimi yanlış: %+v", memberList.Items[0])
	}
	if got := api.types(s.admin); !sameTypes(got, models.NotificationIdeaAdded) {
		t.Fatalf("işlemi yapan admin kendi işleminden bildirim aldı: %v", got)
	}

	// Bütçe: gerçek değişiklik bildirilir, aynı tutar bildirilmez.
	api.mustOK(http.MethodPatch, group+"/budget", s.admin.token, map[string]string{"total_budget": "4000"}, http.StatusOK)
	api.mustOK(http.MethodPatch, group+"/budget", s.admin.token, map[string]string{"total_budget": "4000.00"}, http.StatusOK)
	budget := api.notifications(s.member).Items[0]
	if budget.Type != models.NotificationBudgetChanged || budget.Params["total_budget"] != "4000.00" {
		t.Fatalf("bütçe bildirimi yanlış: %+v", budget)
	}

	// Tarih: ayarlama, aynı tarih (bildirim yok), kaldırma.
	date := time.Now().AddDate(0, 0, 30).Format(models.DateLayout)
	api.mustOK(http.MethodPatch, group+"/event-date", s.admin.token, map[string]any{"event_date": date}, http.StatusOK)
	api.mustOK(http.MethodPatch, group+"/event-date", s.admin.token, map[string]any{"event_date": date}, http.StatusOK)
	if n := api.notifications(s.member).Items[0]; n.Type != models.NotificationEventDateChanged || n.Params["event_date"] != date {
		t.Fatalf("tarih bildirimi yanlış: %+v", n)
	}
	api.mustOK(http.MethodPatch, group+"/event-date", s.admin.token, map[string]any{"event_date": nil}, http.StatusOK)
	if n := api.notifications(s.member).Items[0]; n.Type != models.NotificationEventDateChanged || n.Params["event_date"] != nil {
		t.Fatalf("tarih kaldırma bildirimi yanlış: %+v", n)
	}

	// Kazanan ve yeniden açma.
	api.mustOK(http.MethodPost, group+"/winner", s.admin.token, map[string]any{"idea_ids": []string{s.ideaID, s.ideaID}}, http.StatusOK)
	if n := api.notifications(s.member).Items[0]; n.Type != models.NotificationWinnerSelected || n.Params["count"] != float64(1) {
		t.Fatalf("kazanan bildirimi yanlış: %+v", n)
	}
	api.mustOK(http.MethodDelete, group+"/winner", s.admin.token, nil, http.StatusOK)

	want := []string{
		models.NotificationGroupReopened, models.NotificationWinnerSelected,
		models.NotificationEventDateChanged, models.NotificationEventDateChanged,
		models.NotificationBudgetChanged, models.NotificationMemberJoined, models.NotificationAddedToGroup,
	}
	if got := api.types(s.member); !sameTypes(got, want...) {
		t.Fatalf("üye bildirimleri:\n got %v\nwant %v", got, want)
	}
	// Sonradan eklenen üye kendi "katıldı" bildirimini almaz; sonraki her şeyi alır.
	wantExtra := append(append([]string{}, want[:5]...), models.NotificationAddedToGroup)
	if got := api.types(extra); !sameTypes(got, wantExtra...) {
		t.Fatalf("sonradan eklenen üye:\n got %v\nwant %v", got, wantExtra)
	}

	// Bildirim üretmeyen işlemler: başarısız istek, oy, ilgi alanı.
	before := len(api.notifications(s.admin).Items) + len(api.notifications(s.member).Items) + len(api.notifications(extra).Items)
	if code, _ := api.call(http.MethodPatch, group+"/budget", s.member.token, map[string]string{"total_budget": "1"}); code != http.StatusForbidden {
		t.Fatalf("üye bütçe değiştirebildi: HTTP %d", code)
	}
	if code, _ := api.call(http.MethodPost, group+"/ideas", s.member.token, map[string]any{"title": "x"}); code != http.StatusBadRequest {
		t.Fatalf("geçersiz fikir kabul edildi: HTTP %d", code)
	}
	api.mustOK(http.MethodPost, "/ideas/"+s.ideaID+"/vote", s.admin.token, map[string]int{"vote_type": 1}, http.StatusOK)
	api.mustOK(http.MethodPost, group+"/interests", s.member.token, map[string]string{"interest": "Kahve"}, http.StatusOK)
	after := len(api.notifications(s.admin).Items) + len(api.notifications(s.member).Items) + len(api.notifications(extra).Items)
	if after != before {
		t.Fatalf("bildirim üretmemesi gereken işlemler %d bildirim üretti", after-before)
	}

	// ANTİ-SPOILER: hedef kişi ve grup dışındaki kullanıcı hiçbir şey görmez; hedef için
	// tabloda tek satır bile yoktur.
	for name, u := range map[string]user{"target": s.target, "outsider": s.outside} {
		if list := api.notifications(u); len(list.Items) != 0 || list.UnreadCount != 0 {
			t.Fatalf("%s bildirim gördü: %+v", name, list)
		}
	}
	if n := api.rowCount(s.groupID, api.userID(s.target)); n != 0 {
		t.Fatalf("hedef kişi için %d bildirim satırı üretildi", n)
	}
}

func TestNotificationsHiddenWhenTargetLinkedLater(t *testing.T) {
	api := newTestAPI(t)
	admin, member, future := api.newUser("Admin"), api.newUser("Member"), api.newUser("Future")
	futureID := api.userID(future)

	newGroup := func(title string) string {
		return idOf(t, api.mustOK(http.MethodPost, "/groups", admin.token, map[string]any{
			"title": title, "target_person_name": "Hedef", "total_budget": "1000",
		}, http.StatusCreated))
	}
	surprise, other := newGroup("Sürpriz"), newGroup("Başka grup")
	api.mustOK(http.MethodPost, "/groups/"+surprise+"/members", admin.token, map[string]string{"email": member.email}, http.StatusCreated)
	// Hedef olacak kişi başka bir grubun sıradan üyesi; o grubun bildirimleri kalmalı.
	api.mustOK(http.MethodPost, "/groups/"+other+"/members", admin.token, map[string]string{"email": future.email}, http.StatusCreated)

	// Sürpriz grubuna ait, hedef olacak kişide duran eski bir satır (tabloya doğrudan yazılır).
	insert := func() uuid.UUID {
		t.Helper()
		n := models.Notification{
			UserID: futureID, GroupID: uuid.MustParse(surprise), Type: models.NotificationIdeaAdded,
			Params: models.NotificationParams{"actor_name": "Admin", "idea_title": "Gizli hediye"},
		}
		if err := api.db.Create(&n).Error; err != nil {
			t.Fatal(err)
		}
		return n.ID
	}
	insert()
	if list := api.notifications(future); len(list.Items) != 2 || list.UnreadCount != 2 {
		t.Fatalf("bağlanmadan önce 2 bildirim beklenirdi: %+v", list)
	}

	signal, cancel, err := api.userHub.Subscribe(futureID)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	api.mustOK(http.MethodPatch, "/groups/"+surprise+"/target", admin.token, map[string]string{"email": future.email}, http.StatusOK)

	// Bağlanınca: satır silindi, liste yalnızca diğer grubu gösterir, açık zil yenilenir.
	if n := api.rowCount(surprise, futureID); n != 0 {
		t.Fatalf("hedef bağlandıktan sonra %d satır kaldı", n)
	}
	list := api.notifications(future)
	if len(list.Items) != 1 || list.Items[0].GroupID != other || list.UnreadCount != 1 {
		t.Fatalf("hedef bağlandıktan sonra liste: %+v", list)
	}
	select {
	case <-signal:
	default:
		t.Fatal("hedefin açık bildirim akışına yenileme sinyali gitmedi")
	}

	// Sonraki değişiklikler hedef için satır üretmez.
	api.mustOK(http.MethodPost, "/groups/"+surprise+"/ideas", member.token, map[string]any{
		"title": "Termos", "estimated_price": "500", "search_query": "termos",
	}, http.StatusCreated)
	api.mustOK(http.MethodPatch, "/groups/"+surprise+"/budget", admin.token, map[string]string{"total_budget": "2000"}, http.StatusOK)
	if n := api.rowCount(surprise, futureID); n != 0 {
		t.Fatalf("hedef için %d satır üretildi", n)
	}

	// Savunma derinliği: satır bir şekilde tabloda olsa bile (yarış, eski veri) hedef onu
	// listede ve sayaçta görmez, okundu yapamaz; "tümünü okundu yap" da ona dokunmaz.
	stale := insert()
	list = api.notifications(future)
	if len(list.Items) != 1 || list.Items[0].GroupID != other || list.UnreadCount != 1 {
		t.Fatalf("gizli satır listede/sayaçta göründü: %+v", list)
	}
	if strings.Contains(string(api.mustOK(http.MethodGet, "/notifications", future.token, nil, http.StatusOK).Data), "Gizli hediye") {
		t.Fatal("hedefe sürpriz bilgisi sızdı")
	}
	if code, _ := api.call(http.MethodPost, "/notifications/"+stale.String()+"/read", future.token, nil); code != http.StatusNotFound {
		t.Fatalf("gizli satır okundu yapılabildi: HTTP %d", code)
	}
	api.mustOK(http.MethodPost, "/notifications/read-all", future.token, nil, http.StatusOK)
	var row models.Notification
	if err := api.db.First(&row, "id = ?", stale).Error; err != nil || row.ReadAt != nil {
		t.Fatalf("gizli satır değişti: %v %+v", err, row.ReadAt)
	}
}

func TestNotificationReadAndOwnership(t *testing.T) {
	s := setupScenario(t)
	api := s.api
	api.mustOK(http.MethodPatch, "/groups/"+s.groupID+"/budget", s.admin.token, map[string]string{"total_budget": "5000"}, http.StatusOK)

	list := api.notifications(s.member)
	if len(list.Items) != 2 || list.UnreadCount != 2 {
		t.Fatalf("başlangıç: %+v", list)
	}
	first := list.Items[0].ID

	// Başkasının bildirimi: 404 (varlığı da sızmaz) ve değişmez. Oturumsuz 401, bozuk kimlik 400.
	for name, u := range map[string]user{"admin": s.admin, "target": s.target, "outsider": s.outside} {
		if code, _ := api.call(http.MethodPost, "/notifications/"+first+"/read", u.token, nil); code != http.StatusNotFound {
			t.Fatalf("%s başkasının bildirimini okundu yapabildi: HTTP %d", name, code)
		}
	}
	if code, _ := api.call(http.MethodPost, "/notifications/"+first+"/read", "", nil); code != http.StatusUnauthorized {
		t.Fatalf("oturumsuz: HTTP %d", code)
	}
	if code, _ := api.call(http.MethodPost, "/notifications/not-a-uuid/read", s.member.token, nil); code != http.StatusBadRequest {
		t.Fatalf("bozuk kimlik: HTTP %d", code)
	}
	if code, _ := api.call(http.MethodPost, "/notifications/"+uuid.NewString()+"/read", s.member.token, nil); code != http.StatusNotFound {
		t.Fatalf("olmayan bildirim: HTTP %d", code)
	}
	for _, path := range []string{"/notifications", "/notifications/stream"} {
		if code, _ := api.call(http.MethodGet, path, "", nil); code != http.StatusUnauthorized {
			t.Fatalf("oturumsuz %s: HTTP %d", path, code)
		}
	}
	if code, _ := api.call(http.MethodPost, "/notifications/read-all", "", nil); code != http.StatusUnauthorized {
		t.Fatalf("oturumsuz read-all: HTTP %d", code)
	}
	if got := api.notifications(s.member); got.UnreadCount != 2 {
		t.Fatalf("başarısız denemeler sayacı değiştirdi: %d", got.UnreadCount)
	}

	// Kendi bildirimi: okundu olur, tekrar çağırmak güvenlidir ve ilk okunma anını korur.
	api.mustOK(http.MethodPost, "/notifications/"+first+"/read", s.member.token, nil, http.StatusOK)
	got := api.notifications(s.member)
	if got.UnreadCount != 1 || got.Items[0].ReadAt == nil || got.Items[1].ReadAt != nil {
		t.Fatalf("okundu sonrası: %+v", got)
	}
	readAt := *got.Items[0].ReadAt
	api.mustOK(http.MethodPost, "/notifications/"+first+"/read", s.member.token, nil, http.StatusOK)
	if again := api.notifications(s.member); !again.Items[0].ReadAt.Equal(readAt) {
		t.Fatal("ikinci okundu çağrısı okunma anını değiştirdi")
	}

	// Tümünü okundu yap yalnızca çağıranı etkiler.
	adminUnread := api.notifications(s.admin).UnreadCount
	api.mustOK(http.MethodPost, "/notifications/read-all", s.member.token, nil, http.StatusOK)
	got = api.notifications(s.member)
	if got.UnreadCount != 0 || got.Items[1].ReadAt == nil {
		t.Fatalf("tümünü okundu sonrası: %+v", got)
	}
	if adminUnread == 0 || api.notifications(s.admin).UnreadCount != adminUnread {
		t.Fatal("tümünü okundu yap başka kullanıcıyı etkiledi")
	}
	api.mustOK(http.MethodPost, "/notifications/read-all", s.member.token, nil, http.StatusOK) // idempotent
}

func TestReminderScheduler(t *testing.T) {
	api := newTestAPI(t)
	admin, member, target := api.newUser("Admin"), api.newUser("Member"), api.newUser("Target")
	targetID := api.userID(target)

	// Tarihler gerçek "bugün"e göre kurulur (API tarihi doğrular); zamanlayıcının saati enjekte edilir.
	day := func(offset int) models.Date {
		n := time.Now().UTC()
		return models.Date{Time: time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, offset)}
	}
	newGroup := func(title string, date models.Date, withTarget bool) string {
		body := map[string]any{"title": title, "target_person_name": "Hedef", "total_budget": "1000", "event_date": date.String()}
		if withTarget {
			body["target_email"] = target.email
		}
		id := idOf(t, api.mustOK(http.MethodPost, "/groups", admin.token, body, http.StatusCreated))
		api.mustOK(http.MethodPost, "/groups/"+id+"/members", admin.token, map[string]string{"email": member.email}, http.StatusCreated)
		return id
	}

	var now time.Time
	scheduler := services.NewReminderScheduler(repository.NewNotificationRepository(api.db), api.userHub)
	scheduler.Now = func() time.Time { return now }
	// reminderRows bir grup için üretilmiş hatırlatma satırı sayısıdır (tüm kullanıcılar).
	reminderRows := func(groupID string) int64 {
		t.Helper()
		var count int64
		if err := api.db.Model(&models.Notification{}).Where("group_id = ? AND dedupe_key IS NOT NULL", groupID).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		return count
	}
	// runAt zamanlayıcıyı verilen anda çalıştırır ve o grup için yeni üretilen hatırlatma
	// sayısını döner. Gruplar aynı veritabanını paylaştığı için sayım grup bazındadır.
	runAt := func(groupID string, at time.Time) int {
		t.Helper()
		before := reminderRows(groupID)
		now = at
		if _, err := scheduler.RunOnce(); err != nil {
			t.Fatal(err)
		}
		return int(reminderRows(groupID) - before)
	}
	// run saati etkinlik gününe göre offset gün kaydırır (Türkiye saatiyle öğlen, 09:00 UTC).
	run := func(groupID string, eventDay models.Date, offset int) int {
		t.Helper()
		return runAt(groupID, eventDay.AddDate(0, 0, offset).Add(9*time.Hour))
	}
	// reminders kullanıcının bir gruptaki hatırlatmalarını (tür + days_left) yeniden eskiye döner.
	reminders := func(u user, groupID string) []string {
		t.Helper()
		out := []string{}
		for _, n := range api.notifications(u).Items {
			if n.GroupID != groupID {
				continue
			}
			switch n.Type {
			case models.NotificationEventReminder:
				out = append(out, "reminder:"+jsonNumber(n.Params["days_left"]))
			case models.NotificationWinnerOverdue:
				out = append(out, "overdue")
			}
		}
		return out
	}

	d := day(40)
	g := newGroup("Zamanında", d, true)

	steps := []struct {
		offset      int
		created     int
		admin, memb string
	}{
		{-5, 0, "", ""},                                           // henüz erken
		{-3, 2, "reminder:3", "reminder:3"},                       // 3 gün kala: tüm üyeler
		{-3, 0, "reminder:3", "reminder:3"},                       // aynı gün tekrar: tekilleştirme
		{-2, 0, "reminder:3", "reminder:3"},                       // ertesi gün: aynı hatırlatma yinelenmez
		{-1, 2, "reminder:1,reminder:3", "reminder:1,reminder:3"}, // 1 gün kala
		{-1, 0, "reminder:1,reminder:3", "reminder:1,reminder:3"},
		{0, 0, "reminder:1,reminder:3", "reminder:1,reminder:3"},         // etkinlik günü: yenisi yok
		{1, 1, "overdue,reminder:1,reminder:3", "reminder:1,reminder:3"}, // süre doldu: yalnızca yönetici
		{1, 0, "overdue,reminder:1,reminder:3", "reminder:1,reminder:3"}, // bir kez
		{9, 0, "overdue,reminder:1,reminder:3", "reminder:1,reminder:3"}, // günler sonra da yinelenmez
	}
	for i, st := range steps {
		if got := run(g, d, st.offset); got != st.created {
			t.Fatalf("adım %d (gün %+d): %d bildirim üretildi, beklenen %d", i, st.offset, got, st.created)
		}
		if got := strings.Join(reminders(admin, g), ","); got != st.admin {
			t.Fatalf("adım %d (gün %+d): admin %q, beklenen %q", i, st.offset, got, st.admin)
		}
		if got := strings.Join(reminders(member, g), ","); got != st.memb {
			t.Fatalf("adım %d (gün %+d): üye %q, beklenen %q", i, st.offset, got, st.memb)
		}
	}
	if n := api.rowCount(g, targetID); n != 0 {
		t.Fatalf("hedef kişiye %d hatırlatma üretildi", n)
	}
	if list := api.notifications(target); len(list.Items) != 0 {
		t.Fatalf("hedef kişi hatırlatma gördü: %+v", list)
	}

	// Kaçanları yakalama: API 3 gün kala kapalıydı; 2 gün kala açılınca yine hatırlatır.
	// Etkinlik günü açılırsa 1 günlük hatırlatma "bugün" olarak üretilir.
	late := day(60)
	gLate := newGroup("Geç uyanan", late, false)
	if got := run(gLate, late, -2); got != 2 {
		t.Fatalf("2 gün kala yakalama: %d", got)
	}
	if got := strings.Join(reminders(member, gLate), ","); got != "reminder:2" {
		t.Fatalf("yakalanan hatırlatma: %q", got)
	}
	if got := run(gLate, late, 0); got != 2 {
		t.Fatalf("etkinlik günü yakalama: %d", got)
	}
	if got := strings.Join(reminders(member, gLate), ","); got != "reminder:0,reminder:2" {
		t.Fatalf("etkinlik günü hatırlatması: %q", got)
	}

	// Kazanan seçilmiş grup: yaklaşan tarih hatırlatılır, "süre doldu" üretilmez.
	done := day(80)
	gDone := newGroup("Tamamlanan", done, false)
	idea := idOf(t, api.mustOK(http.MethodPost, "/groups/"+gDone+"/ideas", admin.token, map[string]any{
		"title": "Kitap", "estimated_price": "100", "search_query": "kitap",
	}, http.StatusCreated))
	api.mustOK(http.MethodPost, "/groups/"+gDone+"/winner", admin.token, map[string]any{"idea_ids": []string{idea}}, http.StatusOK)
	if got := run(gDone, done, -3); got != 2 {
		t.Fatalf("tamamlanan grupta 3 gün kala: %d", got)
	}
	if got := run(gDone, done, 2); got != 0 {
		t.Fatalf("tamamlanan grupta süre doldu bildirimi üretildi: %d", got)
	}
	// Grup yeniden açılırsa kazanan yine seçilmemiş olur; yöneticiye bir kez hatırlatılır.
	api.mustOK(http.MethodDelete, "/groups/"+gDone+"/winner", admin.token, nil, http.StatusOK)
	if got := run(gDone, done, 2); got != 1 {
		t.Fatalf("yeniden açılan grupta süre doldu: %d", got)
	}
	if got := run(gDone, done, 3); got != 0 {
		t.Fatalf("süre doldu yinelendi: %d", got)
	}

	// Tarih değişirse yeni tarih için yeniden hatırlatılır (anahtar tarihi içerir).
	moved := day(100)
	gMoved := newGroup("Ertelenen", moved, false)
	if got := run(gMoved, moved, -3); got != 2 {
		t.Fatalf("ilk tarih: %d", got)
	}
	newDate := models.Date{Time: moved.AddDate(0, 0, 20)}
	api.mustOK(http.MethodPatch, "/groups/"+gMoved+"/event-date", admin.token, map[string]any{"event_date": newDate.String()}, http.StatusOK)
	if got := run(gMoved, moved, -1); got != 0 {
		t.Fatalf("eski tarih için hatırlatma üretildi: %d", got)
	}
	if got := run(gMoved, newDate, -3); got != 2 {
		t.Fatalf("yeni tarih: %d", got)
	}

	// "Bugün" Türkiye saatine göredir: 22:30 UTC, TRT'de ertesi gündür.
	edge := day(140)
	gEdge := newGroup("Gece yarısı", edge, false)
	if got := runAt(gEdge, edge.AddDate(0, 0, -5).Add(22*time.Hour+30*time.Minute)); got != 0 { // TRT: 4 gün kala 01:30
		t.Fatalf("4 gün kala: %d", got)
	}
	if got := runAt(gEdge, edge.AddDate(0, 0, -4).Add(22*time.Hour+30*time.Minute)); got != 2 { // TRT: 3 gün kala 01:30
		t.Fatalf("TRT'ye göre 3 gün kala: %d", got)
	}
	if got := strings.Join(reminders(member, gEdge), ","); got != "reminder:3" {
		t.Fatalf("TRT hatırlatması: %q", got)
	}
}

func jsonNumber(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

// Run: başlangıçta hemen çalışır, aralıkla yineler ve context iptal edilince döner.
func TestReminderSchedulerRunStopsOnCancel(t *testing.T) {
	api := newTestAPI(t)
	admin := api.newUser("Admin")
	n := time.Now().UTC()
	date := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 20)
	groupID := idOf(t, api.mustOK(http.MethodPost, "/groups", admin.token, map[string]any{
		"title": "Zamanlayıcı", "target_person_name": "Hedef", "total_budget": "1000", "event_date": date.Format(models.DateLayout),
	}, http.StatusCreated))
	adminID := api.userID(admin)

	scheduler := services.NewReminderScheduler(repository.NewNotificationRepository(api.db), api.userHub)
	scheduler.Now = func() time.Time { return date.AddDate(0, 0, -1).Add(9 * time.Hour) }
	scheduler.Interval = 10 * time.Millisecond
	signal, cancelSub, err := api.userHub.Subscribe(adminID)
	if err != nil {
		t.Fatal(err)
	}
	defer cancelSub()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		scheduler.Run(ctx)
	}()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal("zamanlayıcı başlangıçta çalışmadı ya da akışa sinyal göndermedi")
	}
	time.Sleep(50 * time.Millisecond) // birkaç tur daha: yinelenen satır olmamalı
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("zamanlayıcı context iptalinde durmadı")
	}
	if n := api.rowCount(groupID, adminID); n != 1 {
		t.Fatalf("%d hatırlatma satırı var, beklenen 1", n)
	}
}

func TestNotificationStream(t *testing.T) {
	s := setupScenario(t)
	srv := httptest.NewServer(s.api.router)
	t.Cleanup(srv.Close)

	open := func(token string) <-chan string {
		t.Helper()
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/notifications/stream", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = res.Body.Close() })
		if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "text/event-stream" || res.Header.Get("X-Accel-Buffering") != "no" {
			t.Fatalf("akış açılamadı: HTTP %d %v", res.StatusCode, res.Header)
		}
		lines := make(chan string, 32)
		go func() {
			sc := bufio.NewScanner(res.Body)
			for sc.Scan() {
				lines <- sc.Text()
			}
		}()
		return lines
	}
	expect := func(lines <-chan string, want string) {
		t.Helper()
		deadline := time.After(2 * time.Second)
		for {
			select {
			case l := <-lines:
				if l == want {
					return
				}
			case <-deadline:
				t.Fatalf("%q gelmedi", want)
			}
		}
	}
	silent := func(name string, lines <-chan string) {
		t.Helper()
		deadline := time.After(200 * time.Millisecond)
		for {
			select {
			case l := <-lines:
				if strings.HasPrefix(l, "event:") || strings.HasPrefix(l, "data:") {
					t.Fatalf("%s akışına olay gitti: %q", name, l)
				}
			case <-deadline:
				return
			}
		}
	}

	admin, member, target := open(s.admin.token), open(s.member.token), open(s.target.token)
	for _, l := range []<-chan string{admin, member, target} {
		expect(l, ": connected")
	}

	// Üye fikir ekler: admin'in akışına veri taşımayan olay gider; ekleyen ve hedef almaz.
	s.api.mustOK(http.MethodPost, "/groups/"+s.groupID+"/ideas", s.member.token, map[string]any{
		"title": "Çadır", "estimated_price": "2000", "search_query": "çadır",
	}, http.StatusCreated)
	expect(admin, "event: changed")
	expect(admin, "data: {}")
	silent("işlemi yapan üye", member)
	silent("hedef kişi", target)

	// Okundu yapmak kullanıcının kendi akışına (diğer sekmeleri) sinyal gönderir.
	s.api.mustOK(http.MethodPost, "/notifications/read-all", s.admin.token, nil, http.StatusOK)
	expect(admin, "event: changed")
}

// Yeni bir değişiklik ucu eklendiğinde bildirim kararı bilinçli verilsin diye: /groups/:id/*
// ve /ideas/:id/* altındaki her değişiklik rotası burada "bildirir" ya da "bildirmez" olarak
// sınıflandırılmış olmalıdır. Yeni rota eklenince bu test, karar verilene kadar kırılır.
func TestEveryMutationRouteHasANotificationDecision(t *testing.T) {
	api := newTestAPI(t)
	decided := map[string]bool{ // true: bildirim üretir
		"POST /api/v1/groups/:id/members":            true,
		"POST /api/v1/groups/:id/ideas":              true,
		"POST /api/v1/groups/:id/ideas/generate-ai":  true,
		"PATCH /api/v1/groups/:id/budget":            true,
		"PATCH /api/v1/groups/:id/event-date":        true,
		"POST /api/v1/groups/:id/winner":             true,
		"DELETE /api/v1/groups/:id/winner":           true,
		"PATCH /api/v1/groups/:id/target":            false, // hedefin zili yalnızca yenilenir; satır üretilmez
		"DELETE /api/v1/groups/:id":                  false, // grup silinir; bildirimleri de CASCADE ile gider
		"POST /api/v1/groups/:id/interests":          false,
		"DELETE /api/v1/groups/:id/interests":        false,
		"POST /api/v1/groups/:id/ideas/preview-link": false, // veritabanına yazmaz
		"POST /api/v1/ideas/:id/vote":                false,
		"DELETE /api/v1/ideas/:id/vote":              false,
		"POST /api/v1/ideas/:id/reject":              false,
		"DELETE /api/v1/ideas/:id":                   false,
	}
	seen := map[string]bool{}
	for _, r := range api.router.Routes() {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			continue
		}
		if !strings.HasPrefix(r.Path, "/api/v1/groups/:id") && !strings.HasPrefix(r.Path, "/api/v1/ideas/:id") {
			continue
		}
		key := r.Method + " " + r.Path
		seen[key] = true
		if _, ok := decided[key]; !ok {
			t.Errorf("%s için bildirim kararı yok: handler'da controllers.RecordNotification çağır ya da bu tabloya false olarak ekle", key)
		}
	}
	for key := range decided {
		if !seen[key] {
			t.Errorf("%s artık router'da yok; tablodan kaldır", key)
		}
	}
}
