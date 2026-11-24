package main

// Arkadaşlık ve grup daveti entegrasyon testleri: gerçek router, middleware zinciri ve PostgreSQL.
// Çalıştırma talimatı için antispoiler_integration_test.go'ya bakın.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/google/uuid"
)

func (a *testAPI) userIDStr(u user) string {
	a.t.Helper()
	return a.userID(u).String()
}

type friendEntry struct {
	FriendshipID string `json:"friendship_id"`
	ID           string `json:"id"`
	Name         string `json:"name"`
}

type friendList struct {
	Friends  []friendEntry `json:"friends"`
	Incoming []friendEntry `json:"incoming"`
	Outgoing []friendEntry `json:"outgoing"`
}

func (a *testAPI) friends(u user) friendList {
	a.t.Helper()
	var l friendList
	if err := json.Unmarshal(a.mustOK(http.MethodGet, "/friends", u.token, nil, http.StatusOK).Data, &l); err != nil {
		a.t.Fatal(err)
	}
	return l
}

// befriend a ve b'yi istek + kabul ile arkadaş yapar.
func (a *testAPI) befriend(x, y user) {
	a.t.Helper()
	a.mustOK(http.MethodPost, "/friends/requests", x.token, map[string]string{"email": y.email}, http.StatusCreated)
	in := a.friends(y).Incoming
	if len(in) != 1 {
		a.t.Fatalf("gelen istek sayısı %d", len(in))
	}
	a.mustOK(http.MethodPost, "/friends/requests/"+in[0].FriendshipID+"/accept", y.token, nil, http.StatusOK)
}

func TestFriendRequestLifecycle(t *testing.T) {
	api := newTestAPI(t)
	ali, ayse, can := api.newUser("Ali"), api.newUser("Ayse"), api.newUser("Can")

	// Kendine, kayıtsız e-postaya ve geçersiz gövdeye istek atılamaz.
	if code, _ := api.call(http.MethodPost, "/friends/requests", ali.token, map[string]string{"email": ali.email}); code != http.StatusBadRequest {
		t.Fatalf("kendine istek: HTTP %d", code)
	}
	if code, _ := api.call(http.MethodPost, "/friends/requests", ali.token, map[string]string{"email": "yok-" + uuid.NewString()[:6] + "@test.local"}); code != http.StatusNotFound {
		t.Fatalf("kayıtsız e-posta: HTTP %d", code)
	}

	api.mustOK(http.MethodPost, "/friends/requests", ali.token, map[string]string{"email": ayse.email}, http.StatusCreated)
	if code, _ := api.call(http.MethodPost, "/friends/requests", ali.token, map[string]string{"email": ayse.email}); code != http.StatusConflict {
		t.Fatalf("tekrar istek: HTTP %d", code)
	}
	if l := api.friends(ali); len(l.Outgoing) != 1 || len(l.Friends) != 0 {
		t.Fatalf("Ali'nin listesi yanlış: %+v", l)
	}
	in := api.friends(ayse).Incoming
	if len(in) != 1 {
		t.Fatalf("Ayşe'nin gelen isteği yok")
	}
	// İsteği gönderen kendi isteğini kabul edemez; üçüncü kişi de dokunamaz.
	if code, _ := api.call(http.MethodPost, "/friends/requests/"+in[0].FriendshipID+"/accept", ali.token, nil); code != http.StatusNotFound {
		t.Fatalf("gönderen kabul etti: HTTP %d", code)
	}
	if code, _ := api.call(http.MethodDelete, "/friends/requests/"+in[0].FriendshipID, can.token, nil); code != http.StatusNotFound {
		t.Fatalf("üçüncü kişi isteği sildi: HTTP %d", code)
	}
	api.mustOK(http.MethodPost, "/friends/requests/"+in[0].FriendshipID+"/accept", ayse.token, nil, http.StatusOK)
	if l := api.friends(ali); len(l.Friends) != 1 || l.Friends[0].Name != "Ayse" {
		t.Fatalf("kabul sonrası Ali'nin arkadaşları: %+v", l)
	}
	if code, _ := api.call(http.MethodPost, "/friends/requests", ayse.token, map[string]string{"email": ali.email}); code != http.StatusConflict {
		t.Fatalf("arkadaşa istek: HTTP %d", code)
	}

	// Karşılıklı istek: Can → Ali varken Ali → Can doğrudan kabul edilir.
	api.mustOK(http.MethodPost, "/friends/requests", can.token, map[string]string{"email": ali.email}, http.StatusCreated)
	res := api.mustOK(http.MethodPost, "/friends/requests", ali.token, map[string]string{"email": can.email}, http.StatusCreated)
	var out struct{ Status string }
	_ = json.Unmarshal(res.Data, &out)
	if out.Status != models.FriendshipAccepted {
		t.Fatalf("karşılıklı istek kabul edilmedi: %s", res.Data)
	}

	// Arkadaşlıktan çıkarma.
	canID := api.userIDStr(can)
	api.mustOK(http.MethodDelete, "/friends/users/"+canID, ali.token, nil, http.StatusOK)
	if code, _ := api.call(http.MethodDelete, "/friends/users/"+canID, ali.token, nil); code != http.StatusNotFound {
		t.Fatalf("ikinci çıkarma: HTTP %d", code)
	}
	if l := api.friends(can); len(l.Friends) != 0 {
		t.Fatalf("Can'in listesinde hâlâ Ali var")
	}
}

func TestFriendLink(t *testing.T) {
	api := newTestAPI(t)
	ali, ayse := api.newUser("Ali"), api.newUser("Ayse")

	var link struct{ Code string }
	_ = json.Unmarshal(api.mustOK(http.MethodGet, "/friends/link", ali.token, nil, http.StatusOK).Data, &link)
	var again struct{ Code string }
	_ = json.Unmarshal(api.mustOK(http.MethodGet, "/friends/link", ali.token, nil, http.StatusOK).Data, &again)
	if link.Code == "" || link.Code != again.Code {
		t.Fatalf("link kodu sabit değil: %q %q", link.Code, again.Code)
	}

	var preview struct {
		Name          string `json:"name"`
		IsSelf        bool   `json:"is_self"`
		AlreadyFriend bool   `json:"already_friend"`
	}
	_ = json.Unmarshal(api.mustOK(http.MethodGet, "/friend-links/"+link.Code, ayse.token, nil, http.StatusOK).Data, &preview)
	if preview.Name != "Ali" || preview.IsSelf || preview.AlreadyFriend {
		t.Fatalf("önizleme yanlış: %+v", preview)
	}
	if code, _ := api.call(http.MethodPost, "/friend-links/"+link.Code+"/accept", ali.token, nil); code != http.StatusBadRequest {
		t.Fatalf("kendi linki: HTTP %d", code)
	}
	api.mustOK(http.MethodPost, "/friend-links/"+link.Code+"/accept", ayse.token, nil, http.StatusOK)
	api.mustOK(http.MethodPost, "/friend-links/"+link.Code+"/accept", ayse.token, nil, http.StatusOK) // idempotent
	if l := api.friends(ali); len(l.Friends) != 1 {
		t.Fatalf("link sonrası arkadaş yok: %+v", l)
	}

	// Sıfırlama eski kodu öldürür; bozuk kod da aynı 404'ü alır.
	api.mustOK(http.MethodPost, "/friends/link/reset", ali.token, nil, http.StatusOK)
	for _, code := range []string{link.Code, "kisa", "AAAAAAAAAAAAAAAAAAAAAA"} {
		if got, _ := api.call(http.MethodGet, "/friend-links/"+code, ayse.token, nil); got != http.StatusNotFound {
			t.Fatalf("kod %q: HTTP %d", code, got)
		}
	}
}

type myInvite struct {
	ID         string `json:"id"`
	GroupID    string `json:"group_id"`
	GroupTitle string `json:"group_title"`
}

// addMember: inviter, u'yu e-postayla gruba davet eder (groupPath "/groups/<id>"), u daveti kabul eder.
// E-postayla ekleme artık doğrudan üyelik değil davettir; üyeliğe ihtiyaç duyan testler bunu kullanır.
func (a *testAPI) addMember(groupPath, inviterToken string, u user) {
	a.t.Helper()
	a.mustOK(http.MethodPost, groupPath+"/members", inviterToken, map[string]string{"email": u.email}, http.StatusCreated)
	groupID := strings.TrimPrefix(groupPath, "/groups/")
	for _, inv := range a.myInvites(u) {
		if inv.GroupID == groupID {
			a.mustOK(http.MethodPost, "/me/group-invites/"+inv.ID+"/accept", u.token, nil, http.StatusOK)
			return
		}
	}
	a.t.Fatalf("%s için %s grubunda davet yok", u.email, groupID)
}

func (a *testAPI) myInvites(u user) []myInvite {
	a.t.Helper()
	var rows []myInvite
	if err := json.Unmarshal(a.mustOK(http.MethodGet, "/me/group-invites", u.token, nil, http.StatusOK).Data, &rows); err != nil {
		a.t.Fatal(err)
	}
	return rows
}

func (a *testAPI) notificationTypes(u user, groupID string) []string {
	a.t.Helper()
	var list struct {
		Items []struct {
			GroupID string `json:"group_id"`
			Type    string `json:"type"`
		} `json:"items"`
	}
	_ = json.Unmarshal(a.mustOK(http.MethodGet, "/notifications", u.token, nil, http.StatusOK).Data, &list)
	types := []string{}
	for _, it := range list.Items {
		if it.GroupID == groupID {
			types = append(types, it.Type)
		}
	}
	return types
}

func TestQuickGroupWithFriendInvites(t *testing.T) {
	api := newTestAPI(t)
	admin, ayse, can, deniz, stranger := api.newUser("Admin"), api.newUser("Ayse"), api.newUser("Can"), api.newUser("Deniz"), api.newUser("Stranger")
	api.befriend(admin, ayse)
	api.befriend(admin, can)
	api.befriend(admin, deniz)
	ayseID, canID, denizID, strangerID := api.userIDStr(ayse), api.userIDStr(can), api.userIDStr(deniz), api.userIDStr(stranger)

	// Arkadaş olmayan davet edilemez; hedef kişi aynı anda davetli olamaz.
	body := func(members []string, target string) map[string]any {
		b := map[string]any{"title": "Deniz'e sürpriz", "target_person_name": "Deniz", "total_budget": "1000", "member_ids": members}
		if target != "" {
			b["target_user_id"] = target
		}
		return b
	}
	if code, _ := api.call(http.MethodPost, "/groups", admin.token, body([]string{strangerID}, "")); code != http.StatusBadRequest {
		t.Fatalf("yabancı davet: HTTP %d", code)
	}
	if code, _ := api.call(http.MethodPost, "/groups", admin.token, body([]string{denizID}, denizID)); code != http.StatusBadRequest {
		t.Fatalf("hedef davet: HTTP %d", code)
	}

	groupID := idOf(t, api.mustOK(http.MethodPost, "/groups", admin.token, body([]string{ayseID, canID}, denizID), http.StatusCreated))

	// Davetliler henüz üye değil; davet listesinde ve zilde görünür. Hedef hiçbir şey görmez.
	if code, _ := api.call(http.MethodGet, "/groups/"+groupID, ayse.token, nil); code != http.StatusForbidden {
		t.Fatalf("davetli üye gibi erişti: HTTP %d", code)
	}
	if rows := api.myInvites(ayse); len(rows) != 1 || rows[0].GroupID != groupID {
		t.Fatalf("Ayşe'nin davetleri: %+v", rows)
	}
	if types := api.notificationTypes(ayse, groupID); len(types) != 1 || types[0] != models.NotificationGroupInvite {
		t.Fatalf("Ayşe'nin bildirimleri: %v", types)
	}
	if rows := api.myInvites(deniz); len(rows) != 0 {
		t.Fatalf("hedef davet görüyor: %+v", rows)
	}
	var pending []struct {
		UserID string `json:"user_id"`
	}
	_ = json.Unmarshal(api.mustOK(http.MethodGet, "/groups/"+groupID+"/member-invites", admin.token, nil, http.StatusOK).Data, &pending)
	if len(pending) != 2 {
		t.Fatalf("bekleyen davet sayısı %d", len(pending))
	}
	if code, _ := api.call(http.MethodGet, "/groups/"+groupID+"/member-invites", deniz.token, nil); code != http.StatusForbidden {
		t.Fatalf("hedef davet listesini gördü: HTTP %d", code)
	}

	// Kabul: üye olur, davet ve davet bildirimi silinir, admin "katıldı" bildirimi alır.
	inv := api.myInvites(ayse)[0]
	api.mustOK(http.MethodPost, "/me/group-invites/"+inv.ID+"/accept", ayse.token, nil, http.StatusOK)
	api.mustOK(http.MethodGet, "/groups/"+groupID, ayse.token, nil, http.StatusOK)
	if types := api.notificationTypes(ayse, groupID); len(types) != 0 {
		t.Fatalf("kabul sonrası davet bildirimi kaldı: %v", types)
	}
	if types := api.notificationTypes(admin, groupID); len(types) != 1 || types[0] != models.NotificationMemberJoined {
		t.Fatalf("admin bildirimleri: %v", types)
	}
	if code, _ := api.call(http.MethodPost, "/me/group-invites/"+inv.ID+"/accept", ayse.token, nil); code != http.StatusNotFound {
		t.Fatalf("ikinci kabul: HTTP %d", code)
	}

	// Ret: davet kalkar, üye olmaz; admin tekrar davet edebilir; üye admin değilse davet edemez.
	canInv := api.myInvites(can)[0]
	if code, _ := api.call(http.MethodPost, "/me/group-invites/"+canInv.ID+"/accept", ayse.token, nil); code != http.StatusNotFound {
		t.Fatalf("başkasının davetini kabul: HTTP %d", code)
	}
	api.mustOK(http.MethodDelete, "/me/group-invites/"+canInv.ID, can.token, nil, http.StatusOK)
	if api.memberRows(groupID, uuid.MustParse(canID)) != 0 {
		t.Fatal("reddeden üye oldu")
	}
	if code, _ := api.call(http.MethodPost, "/groups/"+groupID+"/member-invites", ayse.token, map[string]any{"user_ids": []string{canID}}); code != http.StatusForbidden {
		t.Fatalf("üye davet etti: HTTP %d", code)
	}
	api.mustOK(http.MethodPost, "/groups/"+groupID+"/member-invites", admin.token, map[string]any{"user_ids": []string{canID}}, http.StatusCreated)
	api.mustOK(http.MethodDelete, "/groups/"+groupID+"/member-invites/"+canID, admin.token, nil, http.StatusOK)
	if rows := api.myInvites(can); len(rows) != 0 {
		t.Fatalf("iptal edilen davet duruyor: %+v", rows)
	}
	if types := api.notificationTypes(can, groupID); len(types) != 0 {
		t.Fatalf("iptal sonrası davet bildirimi kaldı: %v", types)
	}
}

func TestMemberInviteRemovedWhenInviteeBecomesTarget(t *testing.T) {
	api := newTestAPI(t)
	admin, ayse := api.newUser("Admin"), api.newUser("Ayse")
	api.befriend(admin, ayse)
	ayseID := api.userIDStr(ayse)
	groupID := idOf(t, api.mustOK(http.MethodPost, "/groups", admin.token, map[string]any{
		"title": "Sürpriz", "target_person_name": "Ayşe", "total_budget": "1000", "member_ids": []string{ayseID},
	}, http.StatusCreated))
	inv := api.myInvites(ayse)
	if len(inv) != 1 {
		t.Fatalf("davet yok")
	}

	// Davetli sonradan hedef olarak bağlanır: daveti ve bildirimi aynı transaction'da silinir.
	api.mustOK(http.MethodPatch, "/groups/"+groupID+"/target", admin.token, map[string]string{"email": ayse.email}, http.StatusOK)
	if rows := api.myInvites(ayse); len(rows) != 0 {
		t.Fatalf("hedef daveti hâlâ görüyor: %+v", rows)
	}
	if code, res := api.call(http.MethodPost, "/me/group-invites/"+inv[0].ID+"/accept", ayse.token, nil); code != http.StatusNotFound || res.Success {
		t.Fatalf("hedef davetle katılabildi: HTTP %d", code)
	}
	if types := api.notificationTypes(ayse, groupID); len(types) != 0 {
		t.Fatalf("hedefte bildirim kaldı: %v", types)
	}
	if code, _ := api.call(http.MethodPost, "/groups/"+groupID+"/member-invites", admin.token, map[string]any{"user_ids": []string{ayseID}}); code != http.StatusBadRequest {
		t.Fatalf("hedef yeniden davet edildi: HTTP %d", code)
	}
}

// Bir hedefin daveti (ör. eski veri) satırda kalsa bile listelenmez ve kabul 403 döner.
func TestTargetInviteRowIsIgnored(t *testing.T) {
	s := setupScenario(t)
	targetID := s.api.userID(s.target)
	adminID := s.api.userID(s.admin)
	row := models.GroupMemberInvite{GroupID: uuid.MustParse(s.groupID), InviteeID: targetID, InviterID: adminID}
	if err := s.api.db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if rows := s.api.myInvites(s.target); len(rows) != 0 {
		t.Fatalf("hedef davet görüyor: %+v", rows)
	}
	code, res := s.api.call(http.MethodPost, "/me/group-invites/"+row.ID.String()+"/accept", s.target.token, nil)
	if code != http.StatusForbidden || res.Error != surpriseMessage {
		t.Fatalf("hedef kabul: HTTP %d %q", code, res.Error)
	}
	if s.api.memberRows(s.groupID, targetID) != 0 {
		t.Fatal("hedef üye oldu")
	}
}

// E-postayla ekleme artık davettir: arkadaşlık şartı yok, her üye gönderebilir, kişi kabul edince üye olur.
func TestEmailAddSendsInvite(t *testing.T) {
	s := setupScenario(t)
	g := "/groups/" + s.groupID
	stranger := s.api.newUser("Stranger")
	strangerID := s.api.userID(stranger)

	res := s.api.mustOK(http.MethodPost, g+"/members", s.member.token, map[string]string{"email": stranger.email}, http.StatusCreated)
	var out struct {
		UserID string `json:"user_id"`
		Status string `json:"status"`
	}
	_ = json.Unmarshal(res.Data, &out)
	if out.Status != "INVITED" || out.UserID != strangerID.String() {
		t.Fatalf("yanıt: %s", res.Data)
	}
	if s.api.memberRows(s.groupID, strangerID) != 0 {
		t.Fatal("e-postayla eklenen kişi onay vermeden üye oldu")
	}
	if code, _ := s.api.call(http.MethodGet, g, stranger.token, nil); code != http.StatusForbidden {
		t.Fatalf("davetli grubu gördü: HTTP %d", code)
	}
	if code, _ := s.api.call(http.MethodPost, g+"/members", s.admin.token, map[string]string{"email": stranger.email}); code != http.StatusConflict {
		t.Fatalf("ikinci davet: HTTP %d", code)
	}
	if code, _ := s.api.call(http.MethodPost, g+"/members", s.admin.token, map[string]string{"email": s.member.email}); code != http.StatusConflict {
		t.Fatalf("mevcut üyeye davet: HTTP %d", code)
	}

	// Davet linkiyle katılırsa bekleyen e-posta daveti ve bildirimi temizlenir.
	link := s.api.createInvite(s.admin, s.groupID, 7)
	s.api.mustOK(http.MethodPost, "/invites/"+link.Token+"/accept", stranger.token, nil, http.StatusOK)
	if rows := s.api.myInvites(stranger); len(rows) != 0 {
		t.Fatalf("linkle katıldıktan sonra davet kaldı: %+v", rows)
	}
	if types := s.api.notificationTypes(stranger, s.groupID); len(types) != 0 {
		t.Fatalf("linkle katıldıktan sonra davet bildirimi kaldı: %v", types)
	}
}
