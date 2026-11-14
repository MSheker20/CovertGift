package main

// Davet linki entegrasyon testleri: gerçek router, middleware zinciri ve PostgreSQL.
// Çalıştırma talimatı için antispoiler_integration_test.go'ya bakın.

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/google/uuid"
)

type inviteCreated struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (a *testAPI) createInvite(u user, groupID string, days int) inviteCreated {
	a.t.Helper()
	res := a.mustOK(http.MethodPost, "/groups/"+groupID+"/invite", u.token, map[string]int{"expires_in_days": days}, http.StatusCreated)
	var created inviteCreated
	if err := json.Unmarshal(res.Data, &created); err != nil || created.Token == "" {
		a.t.Fatalf("davet yanıtı okunamadı: %s", res.Data)
	}
	return created
}

func (a *testAPI) memberRows(groupID string, userID uuid.UUID) int64 {
	a.t.Helper()
	var n int64
	if err := a.db.Model(&models.GroupMember{}).Where("group_id = ? AND user_id = ?", groupID, userID).Count(&n).Error; err != nil {
		a.t.Fatal(err)
	}
	return n
}

func TestInviteAdminLifecycle(t *testing.T) {
	s := setupScenario(t)
	g := "/groups/" + s.groupID + "/invite"

	var status struct {
		Active    bool       `json:"active"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	read := func() string {
		res := s.api.mustOK(http.MethodGet, g, s.admin.token, nil, http.StatusOK)
		status.Active, status.ExpiresAt = false, nil
		if err := json.Unmarshal(res.Data, &status); err != nil {
			t.Fatal(err)
		}
		return string(res.Data)
	}
	if read(); status.Active {
		t.Fatal("davet yokken active=true")
	}

	first := s.api.createInvite(s.admin, s.groupID, 7)
	if raw := read(); !status.Active || status.ExpiresAt == nil || strings.Contains(raw, first.Token) || strings.Contains(raw, "token") {
		t.Fatalf("durum yanıtı yanlış ya da token sızdırıyor: %s", raw)
	}
	if d := time.Until(first.ExpiresAt); d < 6*24*time.Hour || d > 7*24*time.Hour+time.Minute {
		t.Fatalf("7 günlük davetin süresi %v", d)
	}

	// Yenileme: eski link ölür, yenisi çalışır, aktif satır tek kalır.
	second := s.api.createInvite(s.admin, s.groupID, 30)
	if code, _ := s.api.call(http.MethodGet, "/invites/"+first.Token, s.outside.token, nil); code != http.StatusNotFound {
		t.Fatalf("yenilenen eski link hâlâ çalışıyor: HTTP %d", code)
	}
	if code, _ := s.api.call(http.MethodPost, "/invites/"+first.Token+"/accept", s.outside.token, nil); code != http.StatusNotFound {
		t.Fatalf("yenilenen eski link ile katılınabildi: HTTP %d", code)
	}
	s.api.mustOK(http.MethodGet, "/invites/"+second.Token, s.outside.token, nil, http.StatusOK)
	var active int64
	s.api.db.Model(&models.GroupInvite{}).Where("group_id = ? AND revoked_at IS NULL", s.groupID).Count(&active)
	if active != 1 {
		t.Fatalf("aktif davet sayısı %d, beklenen 1", active)
	}

	// İptal idempotent.
	s.api.mustOK(http.MethodDelete, g, s.admin.token, nil, http.StatusOK)
	s.api.mustOK(http.MethodDelete, g, s.admin.token, nil, http.StatusOK)
	if read(); status.Active {
		t.Fatal("iptalden sonra active=true")
	}
	if code, _ := s.api.call(http.MethodGet, "/invites/"+second.Token, s.outside.token, nil); code != http.StatusNotFound {
		t.Fatalf("iptal edilen link çalışıyor: HTTP %d", code)
	}
}

func TestInviteCreateValidatesDuration(t *testing.T) {
	s := setupScenario(t)
	g := "/groups/" + s.groupID + "/invite"
	for _, body := range []any{
		map[string]int{"expires_in_days": 0}, map[string]int{"expires_in_days": 3},
		map[string]int{"expires_in_days": 365}, map[string]int{"expires_in_days": -7},
		map[string]string{"expires_in_days": "7"}, map[string]string{},
	} {
		if code, res := s.api.call(http.MethodPost, g, s.admin.token, body); code != http.StatusBadRequest || res.Success {
			t.Errorf("%v: HTTP %d, beklenen 400", body, code)
		}
	}
	for _, days := range []int{1, 7, 30} {
		s.api.createInvite(s.admin, s.groupID, days)
	}
}

func TestInviteManagementIsAdminOnlyAndAntiSpoiler(t *testing.T) {
	s := setupScenario(t)
	g := "/groups/" + s.groupID + "/invite"
	created := s.api.createInvite(s.admin, s.groupID, 7)
	for _, r := range []struct{ method string }{{http.MethodGet}, {http.MethodPost}, {http.MethodDelete}} {
		if code, res := s.api.call(r.method, g, s.member.token, map[string]int{"expires_in_days": 7}); code != http.StatusForbidden || res.Success {
			t.Errorf("üye %s: HTTP %d, beklenen 403", r.method, code)
		}
		if code, _ := s.api.call(r.method, g, s.outside.token, map[string]int{"expires_in_days": 7}); code != http.StatusForbidden {
			t.Errorf("üye olmayan %s: HTTP %d, beklenen 403", r.method, code)
		}
		code, res := s.api.call(r.method, g, s.target.token, map[string]int{"expires_in_days": 7})
		if code != http.StatusForbidden || res.Error != surpriseMessage {
			t.Errorf("hedef %s: HTTP %d %q, beklenen sürpriz 403", r.method, code, res.Error)
		}
	}
	// Yetkisiz denemeler mevcut linki bozmamış olmalı.
	s.api.mustOK(http.MethodGet, "/invites/"+created.Token, s.outside.token, nil, http.StatusOK)
}

func TestInviteStoresOnlyTokenHash(t *testing.T) {
	s := setupScenario(t)
	created := s.api.createInvite(s.admin, s.groupID, 7)
	var rows []models.GroupInvite
	if err := s.api.db.Where("group_id = ?", s.groupID).Find(&rows).Error; err != nil || len(rows) != 1 {
		t.Fatalf("davet satırı okunamadı: %v %d", err, len(rows))
	}
	if rows[0].TokenHash == created.Token || len(rows[0].TokenHash) != 64 {
		t.Fatalf("veritabanında ham token ya da hex SHA-256 olmayan değer: %q", rows[0].TokenHash)
	}
	var matches int64
	s.api.db.Raw("SELECT COUNT(*) FROM group_invites WHERE token_hash = ?", created.Token).Scan(&matches)
	if matches != 0 {
		t.Fatal("ham token tabloda bulundu")
	}
	// Yalnızca özet bilinse bile (ham token değil) link çalışmaz.
	if code, _ := s.api.call(http.MethodGet, "/invites/"+rows[0].TokenHash, s.outside.token, nil); code != http.StatusNotFound {
		t.Fatalf("özet ile davet açıldı: HTTP %d", code)
	}
}

func TestInviteExpiredAndMalformedTokensAre404WithOneMessage(t *testing.T) {
	s := setupScenario(t)
	created := s.api.createInvite(s.admin, s.groupID, 1)
	s.api.db.Model(&models.GroupInvite{}).Where("group_id = ?", s.groupID).Update("expires_at", time.Now().Add(-time.Minute))

	var messages []string
	for _, token := range []string{created.Token, strings.Repeat("A", 43), "kisa", strings.Repeat("A", 200)} {
		for _, call := range []struct{ method, path string }{{http.MethodGet, "/invites/" + token}, {http.MethodPost, "/invites/" + token + "/accept"}} {
			code, res := s.api.call(call.method, call.path, s.outside.token, nil)
			if code != http.StatusNotFound || res.Success {
				t.Fatalf("%s %s: HTTP %d, beklenen 404", call.method, call.path, code)
			}
			messages = append(messages, res.Error)
		}
	}
	for _, m := range messages {
		if m != messages[0] || m == "" {
			t.Fatalf("404 mesajları farklı (hangi hatanın olduğu sızıyor): %q", messages)
		}
	}
	if s.api.memberRows(s.groupID, s.api.userID(s.outside)) != 0 {
		t.Fatal("süresi dolmuş link ile üye eklendi")
	}
}

func TestInviteRequiresAuthentication(t *testing.T) {
	s := setupScenario(t)
	created := s.api.createInvite(s.admin, s.groupID, 7)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		path := "/invites/" + created.Token
		if method == http.MethodPost {
			path += "/accept"
		}
		if code, _ := s.api.call(method, path, "", nil); code != http.StatusUnauthorized {
			t.Errorf("%s anonim: HTTP %d, beklenen 401", method, code)
		}
	}
}

func TestInviteTargetGets403WithoutAnyGroupData(t *testing.T) {
	s := setupScenario(t)
	created := s.api.createInvite(s.admin, s.groupID, 7)
	targetID := s.api.userID(s.target)
	for _, call := range []struct{ method, path string }{
		{http.MethodGet, "/invites/" + created.Token},
		{http.MethodPost, "/invites/" + created.Token + "/accept"},
	} {
		code, res := s.api.call(call.method, call.path, s.target.token, nil)
		if code != http.StatusForbidden || res.Success || res.Error != surpriseMessage {
			t.Fatalf("%s: HTTP %d %+v, beklenen sürpriz 403", call.method, code, res)
		}
		if len(res.Data) > 0 && string(res.Data) != "null" {
			t.Fatalf("403 veri sızdırdı: %s", res.Data)
		}
		body := res.Error + string(res.Data)
		for _, leak := range []string{s.groupID, "Doğum günü sürprizi", "Admin", "member_count", "inviter"} {
			if strings.Contains(body, leak) {
				t.Fatalf("403 yanıtı %q sızdırıyor: %s", leak, body)
			}
		}
	}
	if s.api.memberRows(s.groupID, targetID) != 0 {
		t.Fatal("hedef kişi davetle üye oldu")
	}
}

func TestInviteTargetLinkedAfterLinkCreationCannotJoin(t *testing.T) {
	// Hedefi sonradan bağlanan kullanıcı, önceden oluşturulmuş linkle de katılamaz.
	api := newTestAPI(t)
	admin, future := api.newUser("Admin"), api.newUser("Future")
	groupID := idOf(t, api.mustOK(http.MethodPost, "/groups", admin.token, map[string]any{
		"title": "Geç bağlanan", "target_person_name": "Hedef", "total_budget": "100",
	}, http.StatusCreated))
	created := api.createInvite(admin, groupID, 7)
	api.mustOK(http.MethodGet, "/invites/"+created.Token, future.token, nil, http.StatusOK)
	api.mustOK(http.MethodPatch, "/groups/"+groupID+"/target", admin.token, map[string]string{"email": future.email}, http.StatusOK)

	for _, call := range []struct{ method, path string }{
		{http.MethodGet, "/invites/" + created.Token}, {http.MethodPost, "/invites/" + created.Token + "/accept"},
	} {
		if code, res := api.call(call.method, call.path, future.token, nil); code != http.StatusForbidden || res.Error != surpriseMessage {
			t.Fatalf("%s: HTTP %d %q", call.method, code, res.Error)
		}
	}
	if api.memberRows(groupID, api.userID(future)) != 0 {
		t.Fatal("sonradan bağlanan hedef üye oldu")
	}
}

func TestInvitePreviewAndAcceptByNonMember(t *testing.T) {
	s := setupScenario(t)
	created := s.api.createInvite(s.admin, s.groupID, 7)
	outsiderID := s.api.userID(s.outside)

	res := s.api.mustOK(http.MethodGet, "/invites/"+created.Token, s.outside.token, nil, http.StatusOK)
	var preview struct {
		GroupID       *string `json:"group_id"`
		GroupTitle    string  `json:"group_title"`
		InviterName   string  `json:"inviter_name"`
		MemberCount   int     `json:"member_count"`
		AlreadyMember bool    `json:"already_member"`
	}
	if err := json.Unmarshal(res.Data, &preview); err != nil {
		t.Fatal(err)
	}
	if preview.GroupID != nil || preview.GroupTitle != "Doğum günü sürprizi" || preview.InviterName != "Admin" || preview.MemberCount != 2 || preview.AlreadyMember {
		t.Fatalf("önizleme yanlış: %+v", preview)
	}
	if strings.Contains(string(res.Data), "group_id") {
		t.Fatalf("üye olmayana önizleme grup kimliğini verdi: %s", res.Data)
	}
	if s.api.memberRows(s.groupID, outsiderID) != 0 {
		t.Fatal("önizleme üyelik oluşturdu")
	}

	// Grup akışına changed sinyali gider.
	signal, cancel, err := s.api.hub.Subscribe(uuid.MustParse(s.groupID))
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	accept := s.api.mustOK(http.MethodPost, "/invites/"+created.Token+"/accept", s.outside.token, nil, http.StatusOK)
	var joined struct {
		GroupID string `json:"group_id"`
	}
	if err := json.Unmarshal(accept.Data, &joined); err != nil || joined.GroupID != s.groupID {
		t.Fatalf("accept yanıtı yanlış: %s", accept.Data)
	}
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal("katılımdan sonra grup akışına sinyal gitmedi")
	}

	if n := s.api.memberRows(s.groupID, outsiderID); n != 1 {
		t.Fatalf("üye satırı sayısı %d, beklenen 1", n)
	}
	var member models.GroupMember
	s.api.db.Where("group_id = ? AND user_id = ?", s.groupID, outsiderID).First(&member)
	if member.Role != "member" {
		t.Fatalf("rol %q, beklenen member", member.Role)
	}
	// Katılan kişi artık grubu görür.
	if !listedGroupIDs(t, s.api.mustOK(http.MethodGet, "/groups", s.outside.token, nil, http.StatusOK))[s.groupID] {
		t.Fatal("katılan kişi grubu listede göremedi")
	}
	// Bildirimler: diğer üyelere member_joined; katılan kişi added_to_group ALMAZ; hedef hiçbir şey almaz.
	if got := s.api.notifications(s.admin).Items[0]; got.Type != "member_joined" || got.Params["member_name"] != "Outsider" {
		t.Fatalf("admin bildirimi yanlış: %+v", got)
	}
	if got := s.api.notifications(s.member).Items[0]; got.Type != "member_joined" {
		t.Fatalf("üye bildirimi yanlış: %+v", got)
	}
	if n := s.api.rowCount(s.groupID, outsiderID); n != 0 {
		t.Fatalf("katılan kişiye %d bildirim üretildi", n)
	}
	if n := s.api.rowCount(s.groupID, s.api.userID(s.target)); n != 0 {
		t.Fatalf("hedefe %d bildirim üretildi", n)
	}
}

func TestInviteAcceptIsIdempotent(t *testing.T) {
	s := setupScenario(t)
	created := s.api.createInvite(s.admin, s.groupID, 7)
	outsiderID := s.api.userID(s.outside)
	s.api.mustOK(http.MethodPost, "/invites/"+created.Token+"/accept", s.outside.token, nil, http.StatusOK)
	before := s.api.rowCount(s.groupID, s.api.userID(s.admin))

	s.api.mustOK(http.MethodPost, "/invites/"+created.Token+"/accept", s.outside.token, nil, http.StatusOK)
	if n := s.api.memberRows(s.groupID, outsiderID); n != 1 {
		t.Fatalf("üye satırı sayısı %d, beklenen 1", n)
	}
	if after := s.api.rowCount(s.groupID, s.api.userID(s.admin)); after != before {
		t.Fatalf("ikinci accept bildirim üretti: %d -> %d", before, after)
	}

	// Zaten üye olan (admin/üye) için önizleme already_member döner, accept rolü değiştirmez.
	res := s.api.mustOK(http.MethodGet, "/invites/"+created.Token, s.member.token, nil, http.StatusOK)
	if !strings.Contains(string(res.Data), `"already_member":true`) || !strings.Contains(string(res.Data), `"group_id":"`+s.groupID+`"`) {
		t.Fatalf("already_member true değil: %s", res.Data)
	}
	s.api.mustOK(http.MethodPost, "/invites/"+created.Token+"/accept", s.admin.token, nil, http.StatusOK)
	var role string
	s.api.db.Model(&models.GroupMember{}).Where("group_id = ? AND user_id = ?", s.groupID, s.api.userID(s.admin)).Select("role").Scan(&role)
	if role != "admin" {
		t.Fatalf("admin davetle rolünü kaybetti: %q", role)
	}
}

func TestInviteConcurrentAcceptCreatesOneMember(t *testing.T) {
	s := setupScenario(t)
	created := s.api.createInvite(s.admin, s.groupID, 7)
	outsiderID := s.api.userID(s.outside)
	var wg sync.WaitGroup
	codes := make([]int, 6)
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i], _ = s.api.call(http.MethodPost, "/invites/"+created.Token+"/accept", s.outside.token, nil)
		}()
	}
	wg.Wait()
	for _, c := range codes {
		if c != http.StatusOK {
			t.Fatalf("eşzamanlı accept kodları: %v", codes)
		}
	}
	if n := s.api.memberRows(s.groupID, outsiderID); n != 1 {
		t.Fatalf("üye satırı sayısı %d, beklenen 1", n)
	}
}

func TestInviteCompletedGroupCanBeJoined(t *testing.T) {
	s := setupScenario(t)
	s.api.mustOK(http.MethodPost, "/groups/"+s.groupID+"/winner", s.admin.token, map[string][]string{"idea_ids": {s.ideaID}}, http.StatusOK)
	created := s.api.createInvite(s.admin, s.groupID, 7)
	s.api.mustOK(http.MethodPost, "/invites/"+created.Token+"/accept", s.outside.token, nil, http.StatusOK)
	if s.api.memberRows(s.groupID, s.api.userID(s.outside)) != 1 {
		t.Fatal("tamamlanmış gruba katılınamadı")
	}
}

func TestInviteDiesWithGroup(t *testing.T) {
	s := setupScenario(t)
	created := s.api.createInvite(s.admin, s.groupID, 7)
	s.api.mustOK(http.MethodDelete, "/groups/"+s.groupID, s.admin.token, nil, http.StatusOK)
	if code, _ := s.api.call(http.MethodGet, "/invites/"+created.Token, s.outside.token, nil); code != http.StatusNotFound {
		t.Fatalf("silinen grubun linki çalışıyor: HTTP %d", code)
	}
}

func TestInviteAcceptWithDeletedAccountIs401(t *testing.T) {
	s := setupScenario(t)
	created := s.api.createInvite(s.admin, s.groupID, 7)
	ghost := s.api.newUser("Ghost")
	s.api.mustOK(http.MethodDelete, "/me", ghost.token, nil, http.StatusOK)
	code, res := s.api.call(http.MethodPost, "/invites/"+created.Token+"/accept", ghost.token, nil)
	if code != http.StatusUnauthorized || res.Success {
		t.Fatalf("silinmiş hesap için HTTP %d, beklenen 401", code)
	}
}

// Davetle katılma ile e-postayla üye ekleme aynı kullanıcı için yarışırsa 500 dönmemeli ve
// tek üye satırı kalmalıdır.
func TestInviteAcceptRacingAddMemberByEmailNeverReturns500(t *testing.T) {
	s := setupScenario(t)
	created := s.api.createInvite(s.admin, s.groupID, 7)
	for i := 0; i < 12; i++ {
		joiner := s.api.newUser("Racer")
		joinerID := s.api.userID(joiner)
		var wg sync.WaitGroup
		var addCode, acceptCode int
		start := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			addCode, _ = s.api.call(http.MethodPost, "/groups/"+s.groupID+"/members", s.admin.token, map[string]string{"email": joiner.email})
		}()
		go func() {
			defer wg.Done()
			<-start
			acceptCode, _ = s.api.call(http.MethodPost, "/invites/"+created.Token+"/accept", joiner.token, nil)
		}()
		close(start)
		wg.Wait()
		if (addCode != http.StatusCreated && addCode != http.StatusConflict) || acceptCode != http.StatusOK {
			t.Fatalf("iterasyon %d: ekleme HTTP %d, katılma HTTP %d", i, addCode, acceptCode)
		}
		if n := s.api.memberRows(s.groupID, joinerID); n != 1 {
			t.Fatalf("iterasyon %d: üye satırı %d", i, n)
		}
	}
}

// Değişmez: kullanıcı aynı grupta hem üye hem hedef kişi olamaz; davetle katılma ile hedef
// bağlama yarışırsa biri kazanır, diğeri 403/409 alır, 500 dönmez.
func TestInviteAcceptRacingTargetLinkNeverMakesTargetAMember(t *testing.T) {
	api := newTestAPI(t)
	admin := api.newUser("Admin")
	for i := 0; i < 20; i++ {
		candidate := api.newUser("Candidate")
		candidateID := api.userID(candidate)
		groupID := idOf(t, api.mustOK(http.MethodPost, "/groups", admin.token, map[string]any{
			"title": "Yarış", "target_person_name": "Aday", "total_budget": "10",
		}, http.StatusCreated))
		created := api.createInvite(admin, groupID, 7)

		var wg sync.WaitGroup
		var acceptCode, linkCode int
		start := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			acceptCode, _ = api.call(http.MethodPost, "/invites/"+created.Token+"/accept", candidate.token, nil)
		}()
		go func() {
			defer wg.Done()
			<-start
			linkCode, _ = api.call(http.MethodPatch, "/groups/"+groupID+"/target", admin.token, map[string]string{"email": candidate.email})
		}()
		close(start)
		wg.Wait()

		var group models.Group
		if err := api.db.First(&group, "id = ?", groupID).Error; err != nil {
			t.Fatal(err)
		}
		isTarget := group.TargetUserID != nil && *group.TargetUserID == candidateID
		isMember := api.memberRows(groupID, candidateID) > 0
		if isTarget && isMember {
			t.Fatalf("iterasyon %d: kullanıcı hem üye hem hedef (accept %d, link %d)", i, acceptCode, linkCode)
		}
		switch {
		case isMember && (acceptCode != http.StatusOK || linkCode != http.StatusConflict):
			t.Fatalf("iterasyon %d: üye oldu ama accept %d, link %d", i, acceptCode, linkCode)
		case isTarget && (linkCode != http.StatusOK || acceptCode != http.StatusForbidden):
			t.Fatalf("iterasyon %d: hedef oldu ama accept %d, link %d", i, acceptCode, linkCode)
		case !isMember && !isTarget:
			t.Fatalf("iterasyon %d: ikisi de olmadı (accept %d, link %d)", i, acceptCode, linkCode)
		}
	}
}
