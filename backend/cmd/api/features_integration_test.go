package main

// Oyu geri alma, kazanan seçimi/grubu kapatma ve hediye tarihi entegrasyon testleri.
// Çalıştırma koşulları için antispoiler_integration_test.go başındaki nota bakın.

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

type ideaView struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Score  int    `json:"score"`
	MyVote int    `json:"my_vote"`
}

func (s *scenario) ideas(t *testing.T, token string) map[string]ideaView {
	t.Helper()
	res := s.api.mustOK(http.MethodGet, "/groups/"+s.groupID+"/ideas", token, nil, http.StatusOK)
	var list []ideaView
	if err := json.Unmarshal(res.Data, &list); err != nil {
		t.Fatal(err)
	}
	out := map[string]ideaView{}
	for _, i := range list {
		out[i.ID] = i
	}
	return out
}

type groupView struct {
	Status      string  `json:"status"`
	CompletedAt *string `json:"completed_at"`
	EventDate   *string `json:"event_date"`
}

func (s *scenario) group(t *testing.T) groupView {
	t.Helper()
	res := s.api.mustOK(http.MethodGet, "/groups/"+s.groupID, s.admin.token, nil, http.StatusOK)
	var d struct {
		Group groupView `json:"group"`
	}
	if err := json.Unmarshal(res.Data, &d); err != nil {
		t.Fatal(err)
	}
	return d.Group
}

func expectStatus(t *testing.T, s *scenario, method, path, token string, body any, want int) {
	t.Helper()
	if code, res := s.api.call(method, path, token, body); code != want {
		t.Fatalf("%s %s: HTTP %d, want %d: %+v", method, path, code, want, res)
	}
}

func TestUnvote(t *testing.T) {
	s := setupScenario(t)
	vote := "/ideas/" + s.ideaID + "/vote"
	s.api.mustOK(http.MethodPost, vote, s.admin.token, map[string]int{"vote_type": 1}, http.StatusOK)
	if i := s.ideas(t, s.admin.token)[s.ideaID]; i.Score != 1 || i.MyVote != 1 {
		t.Fatalf("oy kaydedilmedi: %+v", i)
	}
	s.api.mustOK(http.MethodDelete, vote, s.admin.token, nil, http.StatusOK)
	if i := s.ideas(t, s.admin.token)[s.ideaID]; i.Score != 0 || i.MyVote != 0 {
		t.Fatalf("oy kaldırılmadı: %+v", i)
	}
	// Oy yokken tekrar kaldırmak da başarılıdır.
	s.api.mustOK(http.MethodDelete, vote, s.admin.token, nil, http.StatusOK)
	// Başkasının oyu etkilenmez.
	s.api.mustOK(http.MethodPost, vote, s.member.token, map[string]int{"vote_type": -1}, http.StatusOK)
	s.api.mustOK(http.MethodDelete, vote, s.admin.token, nil, http.StatusOK)
	if i := s.ideas(t, s.member.token)[s.ideaID]; i.Score != -1 || i.MyVote != -1 {
		t.Fatalf("başka üyenin oyu silindi: %+v", i)
	}
	expectStatus(t, s, http.MethodDelete, vote, s.outside.token, nil, http.StatusForbidden)
}

func TestSelectWinnerClosesGroupAndReopen(t *testing.T) {
	s := setupScenario(t)
	g := "/groups/" + s.groupID
	second := idOf(t, s.api.mustOK(http.MethodPost, g+"/ideas", s.member.token, map[string]any{"title": "Termos", "estimated_price": "500"}, http.StatusCreated))
	rejected := idOf(t, s.api.mustOK(http.MethodPost, g+"/ideas", s.member.token, map[string]any{"title": "Anahtarlık", "estimated_price": "50"}, http.StatusCreated))
	s.api.mustOK(http.MethodPost, "/ideas/"+rejected+"/reject", s.admin.token, nil, http.StatusOK)

	// Başka grubun fikri seçilemez.
	other := idOf(t, s.api.mustOK(http.MethodPost, "/groups", s.admin.token, map[string]any{"title": "Başka grup", "target_person_name": "Xavi"}, http.StatusCreated))
	otherIdea := idOf(t, s.api.mustOK(http.MethodPost, "/groups/"+other+"/ideas", s.admin.token, map[string]any{"title": "Başka fikir"}, http.StatusCreated))

	expectStatus(t, s, http.MethodPost, g+"/winner", s.member.token, map[string][]string{"idea_ids": {s.ideaID}}, http.StatusForbidden)
	expectStatus(t, s, http.MethodPost, g+"/winner", s.admin.token, map[string][]string{"idea_ids": {otherIdea}}, http.StatusBadRequest)
	expectStatus(t, s, http.MethodPost, g+"/winner", s.admin.token, map[string][]string{"idea_ids": {rejected}}, http.StatusConflict)
	expectStatus(t, s, http.MethodPost, g+"/winner", s.admin.token, map[string][]string{"idea_ids": {}}, http.StatusBadRequest)
	if st := s.group(t).Status; st != "ACTIVE" {
		t.Fatalf("başarısız denemeler grubu değiştirdi: %s", st)
	}

	s.api.mustOK(http.MethodPost, g+"/winner", s.admin.token, map[string][]string{"idea_ids": {s.ideaID}}, http.StatusOK)
	if gv := s.group(t); gv.Status != "COMPLETED" || gv.CompletedAt == nil {
		t.Fatalf("grup tamamlanmadı: %+v", gv)
	}
	ideas := s.ideas(t, s.member.token)
	if ideas[s.ideaID].Status != "SELECTED" || ideas[second].Status != "ACTIVE" {
		t.Fatalf("kazanan durumu yanlış: %+v", ideas)
	}

	// Tamamlanan grupta fikir ve oy işlemleri kapalı.
	for _, r := range []struct {
		method, path, token string
		body                any
	}{
		{http.MethodPost, "/ideas/" + second + "/vote", s.member.token, map[string]int{"vote_type": 1}},
		{http.MethodDelete, "/ideas/" + second + "/vote", s.member.token, nil},
		{http.MethodPost, g + "/ideas", s.member.token, map[string]any{"title": "Geç kalan fikir"}},
		{http.MethodPost, g + "/ideas/generate-ai", s.member.token, nil},
		{http.MethodPost, "/ideas/" + second + "/reject", s.admin.token, nil},
		{http.MethodDelete, "/ideas/" + second, s.admin.token, nil},
		{http.MethodPost, g + "/winner", s.admin.token, map[string][]string{"idea_ids": {second}}},
	} {
		expectStatus(t, s, r.method, r.path, r.token, r.body, http.StatusConflict)
	}
	// Hedef kişi kapanmış grupta da sürpriz mesajı alır (Anti-Spoiler önce gelir).
	if code, res := s.api.call(http.MethodPost, "/ideas/"+second+"/vote", s.target.token, map[string]int{"vote_type": 1}); code != http.StatusForbidden || res.Error != surpriseMessage {
		t.Fatalf("hedef kapalı grupta sürpriz mesajı almadı: HTTP %d %+v", code, res)
	}

	expectStatus(t, s, http.MethodDelete, g+"/winner", s.member.token, nil, http.StatusForbidden)
	s.api.mustOK(http.MethodDelete, g+"/winner", s.admin.token, nil, http.StatusOK)
	if gv := s.group(t); gv.Status != "ACTIVE" || gv.CompletedAt != nil {
		t.Fatalf("grup yeniden açılmadı: %+v", gv)
	}
	if st := s.ideas(t, s.member.token)[s.ideaID].Status; st != "ACTIVE" {
		t.Fatalf("kazanan fikir aktife dönmedi: %s", st)
	}
	expectStatus(t, s, http.MethodDelete, g+"/winner", s.admin.token, nil, http.StatusConflict)
	s.api.mustOK(http.MethodPost, "/ideas/"+second+"/vote", s.member.token, map[string]int{"vote_type": 1}, http.StatusOK)
}

func TestEventDate(t *testing.T) {
	s := setupScenario(t)
	g := "/groups/" + s.groupID
	soon := time.Now().AddDate(0, 0, 10).Format("2006-01-02")

	created := s.api.mustOK(http.MethodPost, "/groups", s.admin.token, map[string]any{"title": "Tarihli", "target_person_name": "Xavi", "event_date": soon}, http.StatusCreated)
	var cg groupView
	_ = json.Unmarshal(created.Data, &cg)
	if cg.EventDate == nil || *cg.EventDate != soon {
		t.Fatalf("oluşturmada tarih kaydedilmedi: %+v", cg)
	}
	for _, bad := range []any{"31.12.2026", "2026-13-01", time.Now().AddDate(6, 0, 0).Format("2006-01-02"), time.Now().AddDate(-2, 0, 0).Format("2006-01-02")} {
		expectStatus(t, s, http.MethodPost, "/groups", s.admin.token, map[string]any{"title": "Kötü", "target_person_name": "Xavi", "event_date": bad}, http.StatusBadRequest)
		expectStatus(t, s, http.MethodPatch, g+"/event-date", s.admin.token, map[string]any{"event_date": bad}, http.StatusBadRequest)
	}

	if gv := s.group(t); gv.EventDate != nil {
		t.Fatalf("tarih beklenmedik şekilde dolu: %v", *gv.EventDate)
	}
	expectStatus(t, s, http.MethodPatch, g+"/event-date", s.member.token, map[string]any{"event_date": soon}, http.StatusForbidden)
	s.api.mustOK(http.MethodPatch, g+"/event-date", s.admin.token, map[string]any{"event_date": soon}, http.StatusOK)
	if gv := s.group(t); gv.EventDate == nil || *gv.EventDate != soon {
		t.Fatalf("tarih güncellenmedi: %+v", gv)
	}
	// Geçmiş tarih (1 yıl içinde) kabul edilir.
	past := time.Now().AddDate(0, -1, 0).Format("2006-01-02")
	s.api.mustOK(http.MethodPatch, g+"/event-date", s.admin.token, map[string]any{"event_date": past}, http.StatusOK)
	s.api.mustOK(http.MethodPatch, g+"/event-date", s.admin.token, map[string]any{"event_date": nil}, http.StatusOK)
	if gv := s.group(t); gv.EventDate != nil {
		t.Fatalf("tarih kaldırılmadı: %v", *gv.EventDate)
	}
}

func TestSelectMultipleWinners(t *testing.T) {
	s := setupScenario(t)
	g := "/groups/" + s.groupID
	second := idOf(t, s.api.mustOK(http.MethodPost, g+"/ideas", s.member.token, map[string]any{"title": "Termos", "estimated_price": "500"}, http.StatusCreated))
	third := idOf(t, s.api.mustOK(http.MethodPost, g+"/ideas", s.member.token, map[string]any{"title": "Kahve seti", "estimated_price": "700"}, http.StatusCreated))
	other := idOf(t, s.api.mustOK(http.MethodPost, "/groups", s.admin.token, map[string]any{"title": "Başka grup", "target_person_name": "Xavi"}, http.StatusCreated))
	otherIdea := idOf(t, s.api.mustOK(http.MethodPost, "/groups/"+other+"/ideas", s.admin.token, map[string]any{"title": "Başka fikir"}, http.StatusCreated))

	// Listede başka grubun fikri varsa hiçbiri seçilmez.
	expectStatus(t, s, http.MethodPost, g+"/winner", s.admin.token, map[string][]string{"idea_ids": {s.ideaID, otherIdea}}, http.StatusBadRequest)
	tooMany := make([]string, 21)
	for i := range tooMany {
		tooMany[i] = s.ideaID
	}
	// 21 kimlik bağlama kuralında reddedilir (tekrarlar sonra ayıklanır).
	expectStatus(t, s, http.MethodPost, g+"/winner", s.admin.token, map[string][]string{"idea_ids": tooMany}, http.StatusBadRequest)
	if st := s.group(t).Status; st != "ACTIVE" {
		t.Fatalf("başarısız deneme grubu değiştirdi: %s", st)
	}
	for id, st := range s.ideas(t, s.member.token) {
		if st.Status != "ACTIVE" {
			t.Fatalf("başarısız deneme fikri değiştirdi: %s %s", id, st.Status)
		}
	}

	// Tekrarlanan kimlik bir kez sayılır; iki kazanan seçilir, üçüncü aktif kalır.
	s.api.mustOK(http.MethodPost, g+"/winner", s.admin.token, map[string][]string{"idea_ids": {s.ideaID, second, s.ideaID}}, http.StatusOK)
	ideas := s.ideas(t, s.member.token)
	if ideas[s.ideaID].Status != "SELECTED" || ideas[second].Status != "SELECTED" || ideas[third].Status != "ACTIVE" {
		t.Fatalf("kazanan durumları yanlış: %+v", ideas)
	}
	if st := s.group(t).Status; st != "COMPLETED" {
		t.Fatalf("grup tamamlanmadı: %s", st)
	}

	// Yeniden açınca tüm kazananlar aktife döner.
	s.api.mustOK(http.MethodDelete, g+"/winner", s.admin.token, nil, http.StatusOK)
	for id, st := range s.ideas(t, s.member.token) {
		if st.Status != "ACTIVE" {
			t.Fatalf("yeniden açınca fikir aktife dönmedi: %s %s", id, st.Status)
		}
	}
}

type listCounts struct {
	MemberCount  int `json:"member_count"`
	IdeaCount    int `json:"idea_count"`
	PendingVotes int `json:"pending_votes"`
}

func (s *scenario) listCounts(t *testing.T, token string) map[string]listCounts {
	t.Helper()
	res := s.api.mustOK(http.MethodGet, "/groups", token, nil, http.StatusOK)
	var list []struct {
		ID string `json:"id"`
		listCounts
	}
	if err := json.Unmarshal(res.Data, &list); err != nil {
		t.Fatal(err)
	}
	out := map[string]listCounts{}
	for _, g := range list {
		out[g.ID] = g.listCounts
	}
	return out
}

func TestGroupListCounts(t *testing.T) {
	s := setupScenario(t)
	g := "/groups/" + s.groupID
	empty := idOf(t, s.api.mustOK(http.MethodPost, "/groups", s.admin.token, map[string]any{"title": "Boş grup", "target_person_name": "Xavi"}, http.StatusCreated))

	expect := func(token, group string, want listCounts) {
		t.Helper()
		if got, ok := s.listCounts(t, token)[group]; !ok || got != want {
			t.Fatalf("grup %s sayaçları %+v (listede: %v), want %+v", group, got, ok, want)
		}
	}

	// Üye fikri ekledi: admin'i bekleyen bir oy var, üyenin kendi fikri sayılmaz.
	expect(s.admin.token, s.groupID, listCounts{MemberCount: 2, IdeaCount: 1, PendingVotes: 1})
	expect(s.member.token, s.groupID, listCounts{MemberCount: 2, IdeaCount: 1, PendingVotes: 0})
	expect(s.admin.token, empty, listCounts{MemberCount: 1, IdeaCount: 0, PendingVotes: 0})

	// Reddedilen fikir ne fikir sayısına ne bekleyen oylara girer.
	rejected := idOf(t, s.api.mustOK(http.MethodPost, g+"/ideas", s.member.token, map[string]any{"title": "Anahtarlık"}, http.StatusCreated))
	s.api.mustOK(http.MethodPost, "/ideas/"+rejected+"/reject", s.admin.token, nil, http.StatusOK)
	second := idOf(t, s.api.mustOK(http.MethodPost, g+"/ideas", s.member.token, map[string]any{"title": "Termos"}, http.StatusCreated))
	expect(s.admin.token, s.groupID, listCounts{MemberCount: 2, IdeaCount: 2, PendingVotes: 2})

	// Oy verilen fikir (beğeni ya da beğenmeme) bekleyenlerden düşer; başkasının oyu etkilemez.
	s.api.mustOK(http.MethodPost, "/ideas/"+s.ideaID+"/vote", s.admin.token, map[string]int{"vote_type": -1}, http.StatusOK)
	expect(s.admin.token, s.groupID, listCounts{MemberCount: 2, IdeaCount: 2, PendingVotes: 1})
	s.api.mustOK(http.MethodPost, "/ideas/"+second+"/vote", s.member.token, map[string]int{"vote_type": 1}, http.StatusOK)
	expect(s.admin.token, s.groupID, listCounts{MemberCount: 2, IdeaCount: 2, PendingVotes: 1})

	// Tamamlanan grupta oylama kapalıdır; bekleyen oy kalmaz.
	s.api.mustOK(http.MethodPost, g+"/winner", s.admin.token, map[string][]string{"idea_ids": {second}}, http.StatusOK)
	expect(s.admin.token, s.groupID, listCounts{MemberCount: 2, IdeaCount: 2, PendingVotes: 0})

	// Anti-Spoiler: hedef kişinin listesinde grup yine yok.
	if _, ok := s.listCounts(t, s.target.token)[s.groupID]; ok {
		t.Fatal("hedef kişi grubu listede gördü")
	}
}
