package main

// MULTI grup (birden çok alıcı, alıcı başına havuz) entegrasyon testleri: gerçek router,
// gerçek middleware zinciri ve gerçek PostgreSQL. Çalıştırma koşulları için
// antispoiler_integration_test.go başındaki nota bakın.
//
// En kritik kural havuz düzeyinde Anti-Spoiler'dır: bir havuzun alıcısı kendi havuzu hakkında
// hiçbir şey öğrenemez — ne doğrudan (403), ne liste/detay/bildirim yanıtlarındaki farklardan,
// ne de canlı akışın zamanlamasından.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/MSheker20/CovertGift/backend/pkg/gemini"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// multiScenario: Admin bir MULTI grup kurar; Alice, Bob ve Carol üyedir. Alice alıcıdır (havuzunu
// grup yöneticisi yönetir); Admin de alıcıdır ve havuzunu Bob yönetir. Bob, Alice'in havuzuna;
// Alice, Admin'in havuzuna birer fikir ekler.
type multiScenario struct {
	api                              *testAPI
	admin, alice, bob, carol         user
	adminID, aliceID, bobID, carolID uuid.UUID
	groupID                          string
	poolAlice, poolAdmin             string
	ideaAlice, ideaAdmin             string
}

func setupMulti(t *testing.T) *multiScenario {
	t.Helper()
	api := newTestAPI(t)
	s := &multiScenario{api: api, admin: api.newUser("Admin"), alice: api.newUser("Alice"), bob: api.newUser("Bob"), carol: api.newUser("Carol")}
	s.adminID, s.aliceID, s.bobID, s.carolID = api.userID(s.admin), api.userID(s.alice), api.userID(s.bob), api.userID(s.carol)
	s.groupID = idOf(t, api.mustOK(http.MethodPost, "/groups", s.admin.token, map[string]any{
		"title": "Ofis Yılbaşı", "mode": "MULTI", "description": "Herkese ayrı havuz",
	}, http.StatusCreated))
	for _, u := range []user{s.alice, s.bob, s.carol} {
		api.mustOK(http.MethodPost, s.g()+"/members", s.admin.token, map[string]string{"email": u.email}, http.StatusCreated)
	}
	s.poolAlice = s.addRecipient(s.aliceID, nil)
	s.poolAdmin = s.addRecipient(s.adminID, &s.bobID)
	s.ideaAlice = s.addIdea(s.bob, s.poolAlice, "Alice için fikir")
	s.ideaAdmin = s.addIdea(s.alice, s.poolAdmin, "Admin için fikir")
	return s
}

func (s *multiScenario) g() string { return "/groups/" + s.groupID }

func (s *multiScenario) pool(id string) string { return s.g() + "/pools/" + id }

func (s *multiScenario) addRecipient(userID uuid.UUID, manager *uuid.UUID) string {
	s.api.t.Helper()
	body := map[string]any{"user_id": userID}
	if manager != nil {
		body["manager_user_id"] = *manager
	}
	res := s.api.mustOK(http.MethodPost, s.g()+"/recipients", s.admin.token, body, http.StatusCreated)
	var out struct {
		Pool struct {
			ID string `json:"id"`
		} `json:"pool"`
	}
	if err := json.Unmarshal(res.Data, &out); err != nil || out.Pool.ID == "" {
		s.api.t.Fatalf("alıcı yanıtı okunamadı: %s", res.Data)
	}
	return out.Pool.ID
}

func (s *multiScenario) addIdea(u user, poolID, title string) string {
	s.api.t.Helper()
	return idOf(s.api.t, s.api.mustOK(http.MethodPost, s.pool(poolID)+"/ideas", u.token, map[string]any{
		"title": title, "estimated_price": "500", "search_query": title,
	}, http.StatusCreated))
}

// rawBody yanıt gövdesini bayt olarak döner (anlık görüntü karşılaştırması için).
func (a *testAPI) rawBody(method, path, token string) []byte {
	a.t.Helper()
	res := a.raw(method, path, token, nil, nil)
	if res.Code != http.StatusOK {
		a.t.Fatalf("%s %s: HTTP %d %s", method, path, res.Code, res.Body)
	}
	return res.Body.Bytes()
}

func expectSurprise(t *testing.T, label string, code int, res apiResponse) {
	t.Helper()
	if code != http.StatusForbidden || res.Success || res.Error != surpriseMessage {
		t.Fatalf("%s: alıcı engellenmedi: HTTP %d %+v", label, code, res)
	}
	if len(res.Data) > 0 && string(res.Data) != "null" {
		t.Fatalf("%s: 403 yanıtı veri sızdırdı: %s", label, res.Data)
	}
}

type poolCard struct {
	ID               string                   `json:"id"`
	Recipient        services.AccountSummary  `json:"recipient"`
	IsMine           bool                     `json:"is_mine"`
	Locked           bool                     `json:"locked"`
	Manager          *services.AccountSummary `json:"manager"`
	CanManage        bool                     `json:"can_manage"`
	Status           string                   `json:"status"`
	TotalBudget      string                   `json:"total_budget"`
	PerPersonBudget  *string                  `json:"per_person_budget"`
	NoContributors   bool                     `json:"no_contributors"`
	ContributorCount int                      `json:"contributor_count"`
	MyContribution   bool                     `json:"my_contribution"`
	IdeaCount        int                      `json:"idea_count"`
	PendingVotes     int                      `json:"pending_votes"`
	Contributors     []struct {
		UserID       string `json:"user_id"`
		Contributing bool   `json:"contributing"`
	} `json:"contributors"`
}

func (s *multiScenario) poolCards(u user) map[string]poolCard {
	s.api.t.Helper()
	res := s.api.mustOK(http.MethodGet, s.g()+"/pools", u.token, nil, http.StatusOK)
	var cards []poolCard
	if err := json.Unmarshal(res.Data, &cards); err != nil {
		s.api.t.Fatalf("havuz listesi okunamadı: %s", res.Data)
	}
	out := map[string]poolCard{}
	for _, c := range cards {
		out[c.ID] = c
	}
	return out
}

func (s *multiScenario) poolDetail(u user, poolID string) poolCard {
	s.api.t.Helper()
	res := s.api.mustOK(http.MethodGet, s.pool(poolID), u.token, nil, http.StatusOK)
	var card poolCard
	if err := json.Unmarshal(res.Data, &card); err != nil {
		s.api.t.Fatalf("havuz detayı okunamadı: %s", res.Data)
	}
	return card
}

// lockedKeys bir JSON nesnesindeki havuz kartının anahtarlarını döner.
func lockedCardKeys(t *testing.T, raw []byte, poolID string) []string {
	t.Helper()
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	var cards []map[string]json.RawMessage
	if err := json.Unmarshal(env.Data, &cards); err != nil {
		var detail struct {
			Pools []map[string]json.RawMessage `json:"pools"`
		}
		if err := json.Unmarshal(env.Data, &detail); err != nil {
			t.Fatalf("havuz kartları okunamadı: %s", env.Data)
		}
		cards = detail.Pools
	}
	for _, c := range cards {
		if string(c["id"]) == `"`+poolID+`"` {
			keys := make([]string, 0, len(c))
			for k := range c {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return keys
		}
	}
	t.Fatalf("kart %s bulunamadı: %s", poolID, env.Data)
	return nil
}

// (a) Havuzun alıcısı /groups/:id/pools/:poolId altındaki HER rotada, gövde doğrulamasından önce
// 403 + sürpriz mesajı alır. Tablo router.Routes()'tan denetlenir: yeni bir havuz rotası
// eklendiğinde tabloya girilene kadar test kırılır.
func TestPoolRecipientBlockedOnEveryPoolRoute(t *testing.T) {
	s := setupMulti(t)
	const prefix = "/api/v1/groups/:id/pools/:poolId"
	bodies := map[string]func(ideaID string) any{
		"GET ":                    func(string) any { return nil },
		"GET /ideas":              func(string) any { return nil },
		"POST /ideas":             func(string) any { return map[string]any{"title": "Spoiler fikri", "estimated_price": "10"} },
		"POST /ideas/generate-ai": func(string) any { return nil },
		"POST /interests":         func(string) any { return map[string]string{"interest": "Spoiler"} },
		"DELETE /interests":       func(string) any { return map[string]string{"interest": "Kamp"} },
		"PATCH /budget":           func(string) any { return map[string]string{"total_budget": "1"} },
		"PATCH /event-date":       func(string) any { return map[string]any{"event_date": nil} },
		"POST /winner":            func(id string) any { return map[string][]string{"idea_ids": {id}} },
		"DELETE /winner":          func(string) any { return nil },
		"PUT /contribution":       func(string) any { return map[string]bool{"contributing": false} },
	}
	seen := map[string]bool{}
	for _, r := range s.api.router.Routes() {
		if !strings.HasPrefix(r.Path, prefix) {
			continue
		}
		key := r.Method + " " + strings.TrimPrefix(r.Path, prefix)
		seen[key] = true
		if _, ok := bodies[key]; !ok {
			t.Errorf("%s%s Anti-Spoiler tablosunda yok: alıcının 403 aldığını doğrulamak için tabloya ekle", r.Method, r.Path)
		}
	}
	for key := range bodies {
		if !seen[key] {
			t.Errorf("%s artık router'da yok; tablodan kaldır", key)
		}
	}

	recipients := []struct {
		name         string
		u            user
		poolID, idea string
	}{
		{"alıcı üye", s.alice, s.poolAlice, s.ideaAlice},
		{"alıcı grup yöneticisi", s.admin, s.poolAdmin, s.ideaAdmin},
	}
	for key, body := range bodies {
		method, suffix, _ := strings.Cut(key, " ")
		for _, r := range recipients {
			// Geçerli gövde ve bozuk gövde: ikisi de 403 (PoolAccess gövdeyi hiç okumaz).
			for variant, b := range map[string]any{"geçerli": body(r.idea), "bozuk": "gövde-değil"} {
				t.Run(r.name+" "+key+" "+variant, func(t *testing.T) {
					code, res := s.api.call(method, s.pool(r.poolID)+suffix, r.u.token, b)
					expectSurprise(t, key, code, res)
				})
			}
		}
	}

	// /ideas/:id/* — kendi havuzundaki fikir, gövde doğrulamasından önce. Tablo router.Routes()'tan
	// denetlenir: yeni bir /ideas/:id rotası eklenince tabloya girilene kadar test kırılır.
	const ideaPrefix = "/api/v1/ideas/:id"
	ideaBodies := map[string][]any{
		"POST /vote":   {map[string]int{"vote_type": 1}, map[string]int{"vote_type": 5}, "gövde-değil"},
		"DELETE /vote": {nil, "gövde-değil"},
		"POST /reject": {nil, "gövde-değil"},
		"DELETE ":      {nil, "gövde-değil"},
	}
	ideaSeen := map[string]bool{}
	for _, r := range s.api.router.Routes() {
		if r.Path != ideaPrefix && !strings.HasPrefix(r.Path, ideaPrefix+"/") {
			continue
		}
		key := r.Method + " " + strings.TrimPrefix(r.Path, ideaPrefix)
		ideaSeen[key] = true
		if _, ok := ideaBodies[key]; !ok {
			t.Errorf("%s %s Anti-Spoiler fikir tablosunda yok: alıcının 403 aldığını doğrulamak için tabloya ekle", r.Method, r.Path)
		}
	}
	for key := range ideaBodies {
		if !ideaSeen[key] {
			t.Errorf("%s artık router'da yok; fikir tablosundan kaldır", key)
		}
	}
	for _, r := range recipients {
		for key, variants := range ideaBodies {
			method, suffix, _ := strings.Cut(key, " ")
			for _, body := range variants {
				code, res := s.api.call(method, "/ideas/"+r.idea+suffix, r.u.token, body)
				expectSurprise(t, r.name+" "+key, code, res)
			}
		}
	}

	// Denemeler hiçbir şeyi değiştirmedi; alıcılar başka havuzlarda normal katılır.
	card := s.poolDetail(s.carol, s.poolAlice)
	if card.TotalBudget != "0" || card.IdeaCount != 1 || card.Status != models.GroupStatusActive {
		t.Fatalf("alıcının denemeleri havuzu değiştirdi: %+v", card)
	}
	s.api.mustOK(http.MethodPost, "/ideas/"+s.ideaAlice+"/vote", s.admin.token, map[string]int{"vote_type": 1}, http.StatusOK)
	s.api.mustOK(http.MethodPost, "/ideas/"+s.ideaAdmin+"/vote", s.carol.token, map[string]int{"vote_type": 1}, http.StatusOK)
	s.api.mustOK(http.MethodPut, s.pool(s.poolAdmin)+"/contribution", s.alice.token, map[string]bool{"contributing": false}, http.StatusOK)

	// Üye olmayan: sürpriz mesajı olmadan 403. Olmayan havuz: 404. Bozuk havuz kimliği: 400.
	outsider := s.api.newUser("Outsider")
	if code, res := s.api.call(http.MethodGet, s.pool(s.poolAlice), outsider.token, nil); code != http.StatusForbidden || res.Error == surpriseMessage {
		t.Fatalf("üye olmayan: HTTP %d %+v", code, res)
	}
	if code, _ := s.api.call(http.MethodGet, s.pool(uuid.NewString()), s.bob.token, nil); code != http.StatusNotFound {
		t.Fatalf("olmayan havuz: HTTP %d", code)
	}
	if code, _ := s.api.call(http.MethodGet, s.pool("not-a-uuid"), s.bob.token, nil); code != http.StatusBadRequest {
		t.Fatalf("bozuk havuz kimliği: HTTP %d", code)
	}
}

// (h) SINGLE'a özgü rotalar MULTI grupta, MULTI'ye özgü rotalar SINGLE grupta 409 döner.
func TestGroupModeRoutesAreExclusive(t *testing.T) {
	s := setupMulti(t)
	wrongMode := services.ErrWrongGroupMode.Error()
	g := s.g()
	for _, r := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, g + "/ideas", nil},
		{http.MethodPost, g + "/ideas", map[string]any{"title": "Grup fikri"}},
		{http.MethodPost, g + "/ideas/generate-ai", nil},
		{http.MethodPatch, g + "/budget", map[string]string{"total_budget": "100"}},
		{http.MethodPost, g + "/interests", map[string]string{"interest": "Kamp"}},
		{http.MethodDelete, g + "/interests", map[string]string{"interest": "Kamp"}},
		{http.MethodPatch, g + "/event-date", map[string]any{"event_date": nil}},
		{http.MethodPost, g + "/winner", map[string][]string{"idea_ids": {s.ideaAlice}}},
		{http.MethodDelete, g + "/winner", nil},
		{http.MethodPatch, g + "/target", map[string]string{"email": s.api.newUser("Hedef").email}},
	} {
		code, res := s.api.call(r.method, r.path, s.admin.token, r.body)
		if code != http.StatusConflict || res.Error != wrongMode {
			t.Errorf("MULTI grupta %s %s: HTTP %d %+v", r.method, r.path, code, res)
		}
	}
	// Önizleme veritabanına yazmaz; iki türde de açık (izinli olmayan adres ağ isteği yapmaz).
	s.api.mustOK(http.MethodPost, g+"/ideas/preview-link", s.bob.token, map[string]string{"url": "http://127.0.0.1/x"}, http.StatusOK)

	// MULTI grup oluşturma: hedef/bütçe/tarih alanları 400; SINGLE'da target_person_name zorunlu.
	for name, body := range map[string]map[string]any{
		"hedef adı":     {"title": "Çoklu", "mode": "MULTI", "target_person_name": "Ali"},
		"hedef e-posta": {"title": "Çoklu", "mode": "MULTI", "target_email": s.bob.email},
		"bütçe":         {"title": "Çoklu", "mode": "MULTI", "total_budget": "100"},
		"tarih":         {"title": "Çoklu", "mode": "MULTI", "event_date": time.Now().AddDate(0, 1, 0).Format(models.DateLayout)},
		"ilgi alanı":    {"title": "Çoklu", "mode": "MULTI", "target_interests": "Kamp"},
		"geçersiz tür":  {"title": "Çoklu", "mode": "multi"},
		"SINGLE adsız":  {"title": "Tekli"},
	} {
		if code, res := s.api.call(http.MethodPost, "/groups", s.admin.token, body); code != http.StatusBadRequest || res.Success {
			t.Errorf("%s: HTTP %d %+v", name, code, res)
		}
	}

	// SINGLE grup: MULTI rotaları 409, havuz rotaları 404 (havuz yok); grup JSON'u mode taşır.
	single := s.api.mustOK(http.MethodPost, "/groups", s.admin.token, map[string]any{"title": "Tekli", "target_person_name": "Ali"}, http.StatusCreated)
	if !strings.Contains(string(single.Data), `"mode":"SINGLE"`) {
		t.Fatalf("SINGLE grup mode taşımıyor: %s", single.Data)
	}
	sg := "/groups/" + idOf(t, single)
	for _, r := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, sg + "/pools", nil},
		{http.MethodPost, sg + "/recipients", map[string]any{"user_id": s.adminID}},
		{http.MethodPatch, sg + "/recipients/" + s.adminID.String() + "/manager", map[string]any{"manager_user_id": nil}},
		{http.MethodDelete, sg + "/recipients/" + s.adminID.String(), nil},
	} {
		code, res := s.api.call(r.method, r.path, s.admin.token, r.body)
		if code != http.StatusConflict || res.Error != wrongMode {
			t.Errorf("SINGLE grupta %s %s: HTTP %d %+v", r.method, r.path, code, res)
		}
	}
	if code, _ := s.api.call(http.MethodGet, sg+"/pools/"+s.poolAlice, s.admin.token, nil); code != http.StatusNotFound {
		t.Fatalf("SINGLE grupta havuz rotası: HTTP %d", code)
	}
	if detail := s.api.mustOK(http.MethodGet, sg, s.admin.token, nil, http.StatusOK); strings.Contains(string(detail.Data), `"pools"`) {
		t.Fatalf("SINGLE grup detayında pools alanı var: %s", detail.Data)
	}
}

// (b) Alıcının GET /groups, GET /groups/:id, GET /groups/:id/pools ve GET /notifications
// yanıtları, kendi havuzundaki her türlü değişiklikten önce ve sonra bayt bayt aynıdır.
func TestPoolRecipientSnapshotInvariance(t *testing.T) {
	s := setupMulti(t)
	paths := []string{"/groups", s.g(), s.g() + "/pools", "/notifications"}
	snapshot := func(u user) map[string][]byte {
		out := map[string][]byte{}
		for _, p := range paths {
			out[p] = s.api.rawBody(http.MethodGet, p, u.token)
		}
		return out
	}
	// mutate havuzda, alıcı dışındaki üyelerle her tür değişikliği yapar.
	mutate := func(poolID string, manager, m1, m2 user, tag, budget, date string) []string {
		t.Helper()
		p := s.pool(poolID)
		call := func(method, path string, u user, body any, want int) apiResponse {
			t.Helper()
			return s.api.mustOK(method, path, u.token, body, want)
		}
		titles := []string{tag + " Bonsai", tag + " Kitap", tag + " Termos"}
		i1 := s.addIdea(m1, poolID, titles[0])
		i2 := s.addIdea(m2, poolID, titles[1])
		i3 := s.addIdea(manager, poolID, titles[2])
		call(http.MethodPost, "/ideas/"+i1+"/vote", m2, map[string]int{"vote_type": 1}, http.StatusOK)
		call(http.MethodPost, "/ideas/"+i2+"/vote", m1, map[string]int{"vote_type": -1}, http.StatusOK)
		call(http.MethodPost, "/ideas/"+i2+"/vote", manager, map[string]int{"vote_type": -1}, http.StatusOK) // çoğunluk vetosu
		call(http.MethodDelete, "/ideas/"+i1+"/vote", m2, nil, http.StatusOK)
		call(http.MethodPost, "/ideas/"+i1+"/reject", manager, nil, http.StatusOK)
		call(http.MethodPatch, p+"/budget", manager, map[string]string{"total_budget": budget}, http.StatusOK)
		call(http.MethodPatch, p+"/event-date", manager, map[string]any{"event_date": date}, http.StatusOK)
		call(http.MethodPost, p+"/interests", m1, map[string]string{"interest": tag + "Origami"}, http.StatusOK)
		call(http.MethodPost, p+"/interests", m2, map[string]string{"interest": tag + "Yoga"}, http.StatusOK)
		call(http.MethodDelete, p+"/interests", manager, map[string]string{"interest": tag + "Yoga"}, http.StatusOK)
		call(http.MethodPut, p+"/contribution", m1, map[string]bool{"contributing": false}, http.StatusOK)
		call(http.MethodPut, p+"/contribution", m2, map[string]bool{"contributing": false}, http.StatusOK)
		call(http.MethodPut, p+"/contribution", m1, map[string]bool{"contributing": true}, http.StatusOK)
		call(http.MethodPost, p+"/winner", manager, map[string][]string{"idea_ids": {i3}}, http.StatusOK)
		call(http.MethodDelete, p+"/winner", manager, nil, http.StatusOK)
		call(http.MethodPost, p+"/winner", manager, map[string][]string{"idea_ids": {i3}}, http.StatusOK)
		call(http.MethodDelete, p+"/winner", manager, nil, http.StatusOK)
		call(http.MethodDelete, "/ideas/"+i3, manager, nil, http.StatusOK)
		// AI anahtarı yok: başarısız istek de hiçbir şey değiştirmemeli.
		if code, _ := s.api.call(http.MethodPost, p+"/ideas/generate-ai", m1.token, nil); code != http.StatusServiceUnavailable {
			t.Fatalf("AI: HTTP %d", code)
		}
		// Yazmayan link önizlemesi (izinli olmayan adres ağ isteği yapmaz).
		call(http.MethodPost, s.g()+"/ideas/preview-link", m1, map[string]string{"url": "http://127.0.0.1/" + tag}, http.StatusOK)
		// Başka bir üyenin havuzu kurulup (fikirleriyle) kaldırılır: net etki yok; havuz silinince
		// pool_added bildirimleri de silinir, alıcının görünümü önceki haline döner.
		other := s.addRecipient(s.carolID, nil)
		s.addIdea(m1, other, tag+" Geçici")
		call(http.MethodDelete, s.g()+"/recipients/"+s.carolID.String()+"?confirm=true", s.admin, nil, http.StatusOK)
		return append(titles, tag+"Origami", tag+"Yoga", tag+" Geçici", budget, date)
	}

	cases := []struct {
		name            string
		recipient       user
		poolID          string
		manager, m1, m2 user
		tag, budget     string
		dateOffset      int
	}{
		{"alıcı üye", s.alice, s.poolAlice, s.admin, s.bob, s.carol, "GizliA", "4321", 45},
		{"alıcı grup yöneticisi", s.admin, s.poolAdmin, s.bob, s.alice, s.carol, "GizliB", "9876", 61},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := snapshot(tc.recipient)
			date := time.Now().AddDate(0, 0, tc.dateOffset).Format(models.DateLayout)
			secrets := mutate(tc.poolID, tc.manager, tc.m1, tc.m2, tc.tag, tc.budget, date)
			after := snapshot(tc.recipient)
			for _, p := range paths {
				if string(before[p]) != string(after[p]) {
					t.Errorf("%s yanıtı alıcının kendi havuzundaki değişikliklerle değişti:\nönce:  %s\nsonra: %s", p, before[p], after[p])
				}
				for _, secret := range secrets {
					if strings.Contains(string(after[p]), secret) {
						t.Errorf("%s yanıtı sır içeriyor (%q): %s", p, secret, after[p])
					}
				}
			}
			// Kilitli kartta yalnızca id, recipient, is_mine, locked var (yönetici, durum, tarih yok).
			for _, p := range []string{s.g(), s.g() + "/pools"} {
				if keys := strings.Join(lockedCardKeys(t, after[p], tc.poolID), ","); keys != "id,is_mine,locked,recipient" {
					t.Errorf("%s: kilitli kartın alanları %s", p, keys)
				}
			}
			// Diğer üyeler değişiklikleri görür (testin gerçekten bir şey değiştirdiğinin kanıtı).
			card := s.poolDetail(tc.m2, tc.poolID)
			// Kalan tek aktif fikir kurulumdaki fikirdir (biri veto, biri çoğunluk vetosu, biri silindi).
			if !strings.HasPrefix(card.TotalBudget, tc.budget) || card.IdeaCount != 1 || card.ContributorCount != 2 {
				t.Fatalf("değişiklikler diğer üyelere yansımadı: %+v", card)
			}
		})
	}
}

// (c) Grup akışı: alıcının kendi havuzundaki değişiklik hiçbir olay üretmez; başka bir havuzdaki
// değişiklik üretir. Görünen havuz kümesi değişince akış olaydan sonra kapanır.
func TestPoolEventsStreamHidesOwnPool(t *testing.T) {
	s := setupMulti(t)
	srv := httptest.NewServer(s.api.router)
	t.Cleanup(srv.Close)

	open := func(token string) <-chan string {
		t.Helper()
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1"+s.g()+"/events", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = res.Body.Close() })
		if res.StatusCode != http.StatusOK {
			t.Fatalf("akış açılamadı: HTTP %d", res.StatusCode)
		}
		lines := make(chan string, 64)
		go func() {
			defer close(lines)
			sc := bufio.NewScanner(res.Body)
			for sc.Scan() {
				lines <- sc.Text()
			}
		}()
		return lines
	}
	expect := func(name string, lines <-chan string, want string) {
		t.Helper()
		deadline := time.After(2 * time.Second)
		for {
			select {
			case l, ok := <-lines:
				if !ok {
					t.Fatalf("%s: akış %q gelmeden kapandı", name, want)
				}
				if l == want {
					return
				}
			case <-deadline:
				t.Fatalf("%s: %q gelmedi", name, want)
			}
		}
	}
	silent := func(name string, lines <-chan string) {
		t.Helper()
		deadline := time.After(300 * time.Millisecond)
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

	// drain önceki olayların kalan satırlarını (data:, birleşen sinyaller) tüketir.
	drain := func(lines <-chan string) {
		t.Helper()
		deadline := time.After(150 * time.Millisecond)
		for {
			select {
			case <-lines:
			case <-deadline:
				return
			}
		}
	}

	openStream := func(path, token string) <-chan string {
		t.Helper()
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1"+path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := http.DefaultClient.Do(req)
		if err != nil || res.StatusCode != http.StatusOK {
			t.Fatalf("akış açılamadı: %v %v", err, res)
		}
		t.Cleanup(func() { _ = res.Body.Close() })
		lines := make(chan string, 64)
		go func() {
			defer close(lines)
			sc := bufio.NewScanner(res.Body)
			for sc.Scan() {
				lines <- sc.Text()
			}
		}()
		return lines
	}

	alice, admin, carol := open(s.alice.token), open(s.admin.token), open(s.carol.token)
	// Alice'in bildirim zili akışı: kendi havuzundaki hiçbir değişiklik sinyal üretmez.
	aliceBell := openStream("/notifications/stream", s.alice.token)
	for _, l := range []<-chan string{alice, admin, carol, aliceBell} {
		expect("bağlantı", l, ": connected")
	}

	// Alice'in havuzu: Carol görür, Alice hiçbir şey almaz.
	s.addIdea(s.bob, s.poolAlice, "Sessiz fikir")
	expect("carol", carol, "event: changed")
	silent("alice (kendi havuzu: fikir)", alice)
	s.api.mustOK(http.MethodPatch, s.pool(s.poolAlice)+"/budget", s.admin.token, map[string]string{"total_budget": "700"}, http.StatusOK)
	s.api.mustOK(http.MethodPost, "/ideas/"+s.ideaAlice+"/vote", s.carol.token, map[string]int{"vote_type": -1}, http.StatusOK)
	s.api.mustOK(http.MethodPut, s.pool(s.poolAlice)+"/contribution", s.carol.token, map[string]bool{"contributing": false}, http.StatusOK)
	silent("alice (kendi havuzu: bütçe/oy/katkı)", alice)

	// Diğer tüm havuz değişiklikleri ve yazmayan istekler de Alice'e hiçbir olay göndermez.
	pa := s.pool(s.poolAlice)
	extra := s.addIdea(s.carol, s.poolAlice, "Silinecek fikir")
	winner := s.addIdea(s.carol, s.poolAlice, "Kazanacak fikir")
	date := time.Now().AddDate(0, 0, 20).Format(models.DateLayout)
	for _, step := range []struct {
		name         string
		method, path string
		u            user
		body         any
	}{
		{"preview-link", http.MethodPost, s.g() + "/ideas/preview-link", s.bob, map[string]string{"url": "http://127.0.0.1/x"}},
		{"oy geri alma", http.MethodDelete, "/ideas/" + s.ideaAlice + "/vote", s.carol, nil},
		{"veto", http.MethodPost, "/ideas/" + s.ideaAlice + "/reject", s.admin, nil},
		{"fikir silme", http.MethodDelete, "/ideas/" + extra, s.admin, nil},
		{"tarih", http.MethodPatch, pa + "/event-date", s.admin, map[string]any{"event_date": date}},
		{"ilgi alanı ekleme", http.MethodPost, pa + "/interests", s.bob, map[string]string{"interest": "Gizli hobi"}},
		{"ilgi alanı silme", http.MethodDelete, pa + "/interests", s.admin, map[string]string{"interest": "Gizli hobi"}},
		{"kazanan", http.MethodPost, pa + "/winner", s.admin, map[string][]string{"idea_ids": {winner}}},
		{"yeniden açma", http.MethodDelete, pa + "/winner", s.admin, nil},
		{"başarısız AI", http.MethodPost, pa + "/ideas/generate-ai", s.bob, nil},
	} {
		code, res := s.api.call(step.method, step.path, step.u.token, step.body)
		if code >= 400 && step.name != "başarısız AI" {
			t.Fatalf("%s: HTTP %d %+v", step.name, code, res)
		}
	}
	silent("alice (kendi havuzu: önizleme/veto/silme/tarih/ilgi/kazanan/AI)", alice)
	silent("alice zili (kendi havuzu)", aliceBell)
	expect("admin", admin, "event: changed")
	drain(admin)

	// Admin'in havuzu: Alice görür, Admin hiçbir şey almaz.
	s.api.mustOK(http.MethodPatch, s.pool(s.poolAdmin)+"/budget", s.bob.token, map[string]string{"total_budget": "900"}, http.StatusOK)
	expect("alice (başka havuz)", alice, "event: changed")
	silent("admin (kendi havuzu)", admin)
	drain(alice)

	// Grup düzeyindeki değişiklik herkese gider.
	s.api.mustOK(http.MethodPatch, s.g()+"/description", s.admin.token, map[string]string{"description": "Yeni açıklama"}, http.StatusOK)
	expect("alice (grup)", alice, "event: changed")

	// Yeni alıcı: Alice'in görebildiği havuz kümesi değişir; akış olaydan sonra kapanır (istemci
	// yeni kümeyle yeniden bağlanır).
	s.addRecipient(s.carolID, nil)
	expect("alice (yeni alıcı)", alice, "event: changed")
	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-alice:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("görünen havuz kümesi değişince akış kapanmadı")
		}
	}
}

// (d) Havuz bildirimleri ve hatırlatmaları pool_id taşır ve havuzun alıcısına hiç gitmez;
// "süre doldu" yalnızca etkin yöneticiye gider.
func TestPoolNotificationsAndReminders(t *testing.T) {
	s := setupMulti(t)
	api := s.api
	poolRows := func(userID uuid.UUID, poolID string) int64 {
		t.Helper()
		var n int64
		if err := api.db.Model(&models.Notification{}).Where("user_id = ? AND pool_id = ?", userID, poolID).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		return n
	}
	type poolNotification struct {
		Type    string         `json:"type"`
		GroupID string         `json:"group_id"`
		PoolID  string         `json:"pool_id"`
		Params  map[string]any `json:"params"`
	}
	// find kullanıcının verilen türdeki (poolID boş değilse o havuza ait) bildirimini döner.
	find := func(u user, kind, poolID string) *poolNotification {
		t.Helper()
		var list struct {
			Items []poolNotification `json:"items"`
		}
		if err := json.Unmarshal(api.mustOK(http.MethodGet, "/notifications", u.token, nil, http.StatusOK).Data, &list); err != nil {
			t.Fatal(err)
		}
		for _, n := range list.Items {
			if n.Type == kind && n.GroupID == s.groupID && (poolID == "" || n.PoolID == poolID) {
				item := n
				return &item
			}
		}
		return nil
	}
	listRaw := func(u user) string {
		return string(api.rawBody(http.MethodGet, "/notifications", u.token))
	}

	// Kurulum olayları: pool_added (alıcı ve yönetici hariç), idea_added (alıcı ve ekleyen hariç).
	for _, u := range []user{s.bob, s.carol} {
		if !strings.Contains(listRaw(u), `"pool_id":"`+s.poolAlice+`"`) {
			t.Fatalf("%s havuz bildirimi almadı: %s", u.email, listRaw(u))
		}
	}
	if n := find(s.carol, models.NotificationIdeaAdded, s.poolAlice); n == nil || n.Params["recipient_name"] != "Alice" || n.Params["idea_title"] != "Alice için fikir" {
		t.Fatalf("carol idea_added: %+v", n)
	}
	if n := find(s.bob, models.NotificationPoolAdded, s.poolAdmin); n == nil || n.Params["recipient_name"] != "Admin" {
		t.Fatalf("bob pool_added: %+v", n)
	}
	if poolRows(s.aliceID, s.poolAlice) != 0 || poolRows(s.adminID, s.poolAdmin) != 0 {
		t.Fatal("havuzun alıcısı için bildirim satırı üretildi")
	}
	if strings.Contains(listRaw(s.alice), s.poolAlice) || strings.Contains(listRaw(s.admin), s.poolAdmin) {
		t.Fatal("alıcının bildirim listesi kendi havuzunu içeriyor")
	}

	// Yönetici ataması: yalnızca yeni yönetici.
	api.mustOK(http.MethodPatch, s.g()+"/recipients/"+s.adminID.String()+"/manager", s.admin.token, map[string]any{"manager_user_id": s.carolID}, http.StatusOK)
	if n := find(s.carol, models.NotificationPoolManagerAssigned, s.poolAdmin); n == nil || n.Params["recipient_name"] != "Admin" {
		t.Fatalf("carol pool_manager_assigned: %+v", n)
	}
	if find(s.bob, models.NotificationPoolManagerAssigned, "") != nil || find(s.alice, models.NotificationPoolManagerAssigned, "") != nil {
		t.Fatal("yönetici ataması başkasına gitti")
	}
	// Aynı yönetici yeniden atanırsa bildirim yok.
	before := len(api.notifications(s.carol).Items)
	api.mustOK(http.MethodPatch, s.g()+"/recipients/"+s.adminID.String()+"/manager", s.admin.token, map[string]any{"manager_user_id": s.carolID}, http.StatusOK)
	if len(api.notifications(s.carol).Items) != before {
		t.Fatal("değişmeyen yönetici için bildirim üretildi")
	}

	// Bütçe ve tarih: alıcı hariç, pool_id ve recipient_name ile.
	api.mustOK(http.MethodPatch, s.pool(s.poolAlice)+"/budget", s.admin.token, map[string]string{"total_budget": "1500"}, http.StatusOK)
	if n := find(s.bob, models.NotificationBudgetChanged, s.poolAlice); n == nil || n.Params["total_budget"] != "1500.00" || n.Params["recipient_name"] != "Alice" {
		t.Fatalf("bob budget_changed: %+v", n)
	}
	// Katkı ve ilgi alanı bildirim üretmez.
	counts := func() int {
		return len(api.notifications(s.admin).Items) + len(api.notifications(s.bob).Items) + len(api.notifications(s.carol).Items) + len(api.notifications(s.alice).Items)
	}
	c0 := counts()
	api.mustOK(http.MethodPut, s.pool(s.poolAlice)+"/contribution", s.bob.token, map[string]bool{"contributing": false}, http.StatusOK)
	api.mustOK(http.MethodPost, s.pool(s.poolAlice)+"/interests", s.bob.token, map[string]string{"interest": "Kamp"}, http.StatusOK)
	if counts() != c0 {
		t.Fatal("katkı/ilgi alanı bildirim üretti")
	}
	if poolRows(s.aliceID, s.poolAlice) != 0 || poolRows(s.adminID, s.poolAdmin) != 0 {
		t.Fatal("havuzun alıcısı için bildirim satırı üretildi")
	}

	// Savunma derinliği: tabloda alıcıya ait bir havuz satırı olsa bile görünmez, okundu yapılamaz.
	poolUUID := uuid.MustParse(s.poolAlice)
	stale := models.Notification{UserID: s.aliceID, GroupID: uuid.MustParse(s.groupID), PoolID: &poolUUID, Type: models.NotificationIdeaAdded,
		Params: models.NotificationParams{"idea_title": "Gizli eski satır"}}
	if err := api.db.Create(&stale).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(listRaw(s.alice), "Gizli eski satır") {
		t.Fatal("gizli havuz satırı listede göründü")
	}
	if code, _ := api.call(http.MethodPost, "/notifications/"+stale.ID.String()+"/read", s.alice.token, nil); code != http.StatusNotFound {
		t.Fatalf("gizli havuz satırı okundu yapılabildi: HTTP %d", code)
	}

	// Hatırlatmalar.
	n := time.Now().UTC()
	event := models.Date{Time: time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 40)}
	api.mustOK(http.MethodPatch, s.pool(s.poolAlice)+"/event-date", s.admin.token, map[string]any{"event_date": event.String()}, http.StatusOK)
	api.mustOK(http.MethodPatch, s.pool(s.poolAdmin)+"/event-date", s.carol.token, map[string]any{"event_date": event.String()}, http.StatusOK)
	scheduler := services.NewReminderScheduler(repository.NewNotificationRepository(api.db), api.userHub)
	reminderUsers := func(poolID, kind string) []string {
		t.Helper()
		var rows []models.Notification
		if err := api.db.Where("pool_id = ? AND type = ? AND dedupe_key IS NOT NULL", poolID, kind).Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
		names := map[uuid.UUID]string{s.adminID: "admin", s.aliceID: "alice", s.bobID: "bob", s.carolID: "carol"}
		out := []string{}
		for _, r := range rows {
			if r.Params["recipient_name"] == nil || !strings.Contains(*r.DedupeKey, poolID) {
				t.Fatalf("hatırlatma havuz bilgisi taşımıyor: %+v", r)
			}
			out = append(out, names[r.UserID])
		}
		sort.Strings(out)
		return out
	}
	scheduler.Now = func() time.Time { return event.AddDate(0, 0, -3).Add(9 * time.Hour) }
	if _, err := scheduler.RunOnce(); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(reminderUsers(s.poolAlice, models.NotificationEventReminder), ","); got != "admin,bob,carol" {
		t.Fatalf("Alice havuzu hatırlatması: %s", got)
	}
	if got := strings.Join(reminderUsers(s.poolAdmin, models.NotificationEventReminder), ","); got != "alice,bob,carol" {
		t.Fatalf("Admin havuzu hatırlatması: %s", got)
	}
	scheduler.Now = func() time.Time { return event.AddDate(0, 0, 1).Add(9 * time.Hour) }
	if _, err := scheduler.RunOnce(); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(reminderUsers(s.poolAlice, models.NotificationWinnerOverdue), ","); got != "admin" {
		t.Fatalf("Alice havuzu süre doldu (etkin yönetici = grup yöneticisi): %s", got)
	}
	if got := strings.Join(reminderUsers(s.poolAdmin, models.NotificationWinnerOverdue), ","); got != "carol" {
		t.Fatalf("Admin havuzu süre doldu (etkin yönetici = Carol): %s", got)
	}
	if poolRows(s.aliceID, s.poolAlice) != 1 || poolRows(s.adminID, s.poolAdmin) != 0 { // 1 = yukarıdaki elle eklenen satır
		t.Fatal("havuzun alıcısına hatırlatma üretildi")
	}
}

func (a *testAPI) contributionRows(poolID string) map[uuid.UUID]bool {
	a.t.Helper()
	var rows []models.PoolContribution
	if err := a.db.Where("pool_id = ?", poolID).Find(&rows).Error; err != nil {
		a.t.Fatal(err)
	}
	out := map[uuid.UUID]bool{}
	for _, r := range rows {
		out[r.UserID] = r.Contributing
	}
	return out
}

// (e) Üye ekleme (/members) ve davet linkiyle katılma, alıcısı kendisi olmayan her havuza katkı
// satırı yazar; havuz kurulurken de alıcı dışındaki tüm üyeler yazılır.
func TestPoolContributionsOnEveryJoinPath(t *testing.T) {
	s := setupMulti(t)
	api := s.api
	for _, pool := range []string{s.poolAlice, s.poolAdmin} {
		rows := api.contributionRows(pool)
		if len(rows) != 3 {
			t.Fatalf("havuz kurulurken katkı satırları: %v", rows)
		}
	}
	if rows := api.contributionRows(s.poolAlice); !rows[s.adminID] || !rows[s.bobID] || !rows[s.carolID] {
		t.Fatalf("Alice havuzu katkıları: %v", rows)
	}

	dave, erin := api.newUser("Dave"), api.newUser("Erin")
	daveID, erinID := api.userID(dave), api.userID(erin)
	api.mustOK(http.MethodPost, s.g()+"/members", s.admin.token, map[string]string{"email": dave.email}, http.StatusCreated)
	invite := api.createInvite(s.admin, s.groupID, 7)
	api.mustOK(http.MethodPost, "/invites/"+invite.Token+"/accept", erin.token, nil, http.StatusOK)

	for _, pool := range []string{s.poolAlice, s.poolAdmin} {
		rows := api.contributionRows(pool)
		if !rows[daveID] || !rows[erinID] || len(rows) != 5 {
			t.Fatalf("katılanlar için katkı satırı yok: %v", rows)
		}
		card := s.poolDetail(s.carol, pool)
		if card.ContributorCount != 5 || len(card.Contributors) != 5 {
			t.Fatalf("katkı veren sayısı: %+v", card)
		}
	}
	// Sonradan kurulan havuz: alıcı (Dave) hariç herkes.
	poolDave := s.addRecipient(daveID, nil)
	if rows := api.contributionRows(poolDave); len(rows) != 5 || rows[daveID] {
		t.Fatalf("yeni havuz katkıları: %v", rows)
	}
	if _, ok := api.contributionRows(poolDave)[daveID]; ok {
		t.Fatal("alıcı kendi havuzuna katkı satırı aldı")
	}
}

// (f) Katkı tercihi: yalnızca kendi satırı, kişi başı tutar, tamamlanan havuzda 409, yeniden
// açılınca serbest.
func TestPoolContributionToggle(t *testing.T) {
	s := setupMulti(t)
	api := s.api
	p := s.pool(s.poolAlice)
	api.mustOK(http.MethodPatch, p+"/budget", s.admin.token, map[string]string{"total_budget": "300"}, http.StatusOK)
	card := s.poolDetail(s.bob, s.poolAlice)
	if card.ContributorCount != 3 || card.PerPersonBudget == nil || *card.PerPersonBudget != "100" || card.NoContributors || !card.MyContribution {
		t.Fatalf("başlangıç: %+v", card)
	}

	toggle := func(u user, on bool, want int) map[string]any {
		t.Helper()
		code, res := api.call(http.MethodPut, p+"/contribution", u.token, map[string]bool{"contributing": on})
		if code != want {
			t.Fatalf("katkı %v: HTTP %d, beklenen %d: %+v", on, code, want, res)
		}
		out := map[string]any{}
		_ = json.Unmarshal(res.Data, &out)
		return out
	}
	if got := toggle(s.bob, false, http.StatusOK); got["contributing"] != false || got["contributor_count"] != float64(2) || got["per_person_budget"] != "150" {
		t.Fatalf("bob kapattı: %v", got)
	}
	// Katkı vermeyen yine fikir ekler ve oy verir; hiçbir şey silinmez.
	idea := s.addIdea(s.bob, s.poolAlice, "Katkısız fikir")
	api.mustOK(http.MethodPost, "/ideas/"+s.ideaAlice+"/vote", s.bob.token, map[string]int{"vote_type": 1}, http.StatusOK)
	toggle(s.carol, false, http.StatusOK)
	if got := toggle(s.admin, false, http.StatusOK); got["no_contributors"] != true || got["per_person_budget"] != nil {
		t.Fatalf("katkı veren yok: %v", got)
	}
	if card := s.poolDetail(s.bob, s.poolAlice); !card.NoContributors || card.PerPersonBudget != nil || card.IdeaCount != 2 {
		t.Fatalf("katkı veren yokken kart: %+v", card)
	}
	if code, _ := api.call(http.MethodPut, p+"/contribution", s.bob.token, map[string]any{}); code != http.StatusBadRequest {
		t.Fatalf("eksik alan: HTTP %d", code)
	}

	api.mustOK(http.MethodPost, p+"/winner", s.admin.token, map[string][]string{"idea_ids": {idea}}, http.StatusOK)
	if code, res := api.call(http.MethodPut, p+"/contribution", s.bob.token, map[string]bool{"contributing": true}); code != http.StatusConflict || res.Error != services.ErrPoolCompleted.Error() {
		t.Fatalf("tamamlanan havuzda katkı: HTTP %d %+v", code, res)
	}
	// Tamamlanan havuzda fikir, oy, veto ve silme de kapalı.
	for _, r := range []struct {
		method, path string
		u            user
		body         any
	}{
		{http.MethodPost, p + "/ideas", s.bob, map[string]any{"title": "Geç kalan"}},
		{http.MethodPost, "/ideas/" + s.ideaAlice + "/vote", s.carol, map[string]int{"vote_type": 1}},
		{http.MethodDelete, "/ideas/" + s.ideaAlice + "/vote", s.bob, nil},
		{http.MethodPost, "/ideas/" + s.ideaAlice + "/reject", s.admin, nil},
		{http.MethodDelete, "/ideas/" + s.ideaAlice, s.admin, nil},
		{http.MethodPost, p + "/winner", s.admin, map[string][]string{"idea_ids": {s.ideaAlice}}},
	} {
		if code, _ := api.call(r.method, r.path, r.u.token, r.body); code != http.StatusConflict {
			t.Errorf("tamamlanan havuzda %s %s: HTTP %d", r.method, r.path, code)
		}
	}
	// Grubun durumu çağırana göre: Bob için tek görünen havuz tamamlandı değil (Admin havuzu açık).
	api.mustOK(http.MethodDelete, p+"/winner", s.admin.token, nil, http.StatusOK)
	if got := toggle(s.bob, true, http.StatusOK); got["contributing"] != true || got["contributor_count"] != float64(1) || got["per_person_budget"] != "300" {
		t.Fatalf("yeniden açılınca katkı: %v", got)
	}
	if code, _ := api.call(http.MethodDelete, p+"/winner", s.admin.token, nil); code != http.StatusConflict {
		t.Fatalf("açık havuzu yeniden açma: HTTP %d", code)
	}
}

// Etkin yönetici ve yetkiler; grup durumu çağırana göre hesaplanır.
func TestPoolManagerPowersAndViewerStatus(t *testing.T) {
	s := setupMulti(t)
	api := s.api
	pa, pd := s.pool(s.poolAlice), s.pool(s.poolAdmin)

	// Alice'in havuzu: yönetici grup yöneticisi; Bob'un değil.
	cards := s.poolCards(s.bob)
	if c := cards[s.poolAlice]; c.Manager == nil || c.Manager.ID != s.adminID || c.CanManage || c.Locked {
		t.Fatalf("bob için Alice havuzu: %+v", c)
	}
	if c := cards[s.poolAdmin]; c.Manager == nil || c.Manager.ID != s.bobID || !c.CanManage {
		t.Fatalf("bob için Admin havuzu: %+v", c)
	}
	if c := s.poolCards(s.admin)[s.poolAlice]; !c.CanManage {
		t.Fatalf("admin Alice havuzunu yönetemiyor: %+v", c)
	}
	// Yönetici olmayan: bütçe, tarih, kazanan, ilgi alanı silme, veto ve başkasının fikrini silme 403.
	for _, r := range []struct {
		method, path string
		u            user
		body         any
	}{
		{http.MethodPatch, pa + "/budget", s.bob, map[string]string{"total_budget": "1"}},
		{http.MethodPatch, pa + "/event-date", s.carol, map[string]any{"event_date": nil}},
		{http.MethodPost, pa + "/winner", s.bob, map[string][]string{"idea_ids": {s.ideaAlice}}},
		{http.MethodDelete, pa + "/interests", s.carol, map[string]string{"interest": "x"}},
		{http.MethodPost, "/ideas/" + s.ideaAlice + "/reject", s.carol, nil},
		{http.MethodDelete, "/ideas/" + s.ideaAlice, s.carol, nil},
		{http.MethodPatch, pd + "/budget", s.carol, map[string]string{"total_budget": "1"}},
		{http.MethodPost, "/ideas/" + s.ideaAdmin + "/reject", s.carol, nil},
	} {
		if code, _ := api.call(r.method, r.path, r.u.token, r.body); code != http.StatusForbidden {
			t.Errorf("yönetici olmayan %s %s: HTTP %d", r.method, r.path, code)
		}
	}
	// Etkin yönetici yapabilir; fikri ekleyen kendi fikrini silebilir.
	api.mustOK(http.MethodPost, "/ideas/"+s.ideaAdmin+"/reject", s.bob.token, nil, http.StatusOK)
	own := s.addIdea(s.carol, s.poolAdmin, "Carol'ın fikri")
	api.mustOK(http.MethodDelete, "/ideas/"+own, s.carol.token, nil, http.StatusOK)
	api.mustOK(http.MethodPost, pd+"/interests", s.carol.token, map[string]string{"interest": "Satranç"}, http.StatusOK)
	if code, _ := api.call(http.MethodPost, pd+"/interests", s.carol.token, map[string]string{"interest": "satranç"}); code != http.StatusConflict {
		t.Fatalf("tekrar ilgi alanı: HTTP %d", code)
	}
	api.mustOK(http.MethodDelete, pd+"/interests", s.bob.token, map[string]string{"interest": "Satranç"}, http.StatusOK)
	// Başka havuzun fikri kazanan seçilemez.
	if code, _ := api.call(http.MethodPost, pa+"/winner", s.admin.token, map[string][]string{"idea_ids": {s.ideaAdmin}}); code != http.StatusBadRequest {
		t.Fatalf("başka havuzun fikri: HTTP %d", code)
	}

	// Çoğunluk vetosu: seçmenler alıcı hariç üyeler (3); 2 eksi oy fikri eler.
	v := s.addIdea(s.carol, s.poolAlice, "Veto adayı")
	api.mustOK(http.MethodPost, "/ideas/"+v+"/vote", s.bob.token, map[string]int{"vote_type": -1}, http.StatusOK)
	if st := ideaStatus(t, api, s.bob, s.poolAlice, v); st != "ACTIVE" {
		t.Fatalf("1 eksi oyla elendi: %s", st)
	}
	api.mustOK(http.MethodPost, "/ideas/"+v+"/vote", s.admin.token, map[string]int{"vote_type": -1}, http.StatusOK)
	if st := ideaStatus(t, api, s.bob, s.poolAlice, v); st != "REJECTED" {
		t.Fatalf("2/3 eksi oyla elenmedi: %s", st)
	}

	// Durum çağırana göre: Alice'in havuzu tamamlanınca Bob için (Admin havuzu açık) ACTIVE;
	// Admin için (tek görünen havuz Alice'inki) COMPLETED; Alice için (tek görünen havuz Admin'inki) ACTIVE.
	api.mustOK(http.MethodPost, pa+"/winner", s.admin.token, map[string][]string{"idea_ids": {s.ideaAlice}}, http.StatusOK)
	status := func(u user) string {
		var groups []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Mode   string `json:"mode"`
		}
		_ = json.Unmarshal(api.mustOK(http.MethodGet, "/groups", u.token, nil, http.StatusOK).Data, &groups)
		for _, g := range groups {
			if g.ID == s.groupID {
				if g.Mode != models.GroupModeMulti {
					t.Fatalf("mode: %s", g.Mode)
				}
				return g.Status
			}
		}
		return ""
	}
	if got := status(s.bob) + "," + status(s.admin) + "," + status(s.alice); got != "ACTIVE,COMPLETED,ACTIVE" {
		t.Fatalf("çağırana göre durum: %s", got)
	}
}

func ideaStatus(t *testing.T, api *testAPI, u user, poolID, ideaID string) string {
	t.Helper()
	res := api.mustOK(http.MethodGet, "/groups/"+poolGroup(t, api, poolID)+"/pools/"+poolID+"/ideas", u.token, nil, http.StatusOK)
	var ideas []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		PoolID string `json:"pool_id"`
	}
	if err := json.Unmarshal(res.Data, &ideas); err != nil {
		t.Fatal(err)
	}
	for _, i := range ideas {
		if i.ID == ideaID {
			if i.PoolID != poolID {
				t.Fatalf("fikrin pool_id'si yanlış: %+v", i)
			}
			return i.Status
		}
	}
	return ""
}

func poolGroup(t *testing.T, api *testAPI, poolID string) string {
	t.Helper()
	var pool models.GiftPool
	if err := api.db.First(&pool, "id = ?", poolID).Error; err != nil {
		t.Fatal(err)
	}
	return pool.GroupID.String()
}

// (g) Alıcı ekleme/kaldırma kuralları.
func TestRecipientManagementRules(t *testing.T) {
	s := setupMulti(t)
	api := s.api
	r := s.g() + "/recipients"
	outsider := api.newUser("Outsider")
	outsiderID := api.userID(outsider)

	for _, c := range []struct {
		name string
		by   user
		body map[string]any
		want int
		err  error
	}{
		{"üye ekleyemez", s.bob, map[string]any{"user_id": s.carolID}, http.StatusForbidden, services.ErrUnauthorizedRecipients},
		{"zaten alıcı", s.admin, map[string]any{"user_id": s.aliceID}, http.StatusConflict, services.ErrAlreadyRecipient},
		{"üye değil", s.admin, map[string]any{"user_id": outsiderID}, http.StatusBadRequest, services.ErrRecipientNotMember},
		{"başka alıcıya yönetici", s.admin, map[string]any{"user_id": s.carolID, "manager_user_id": s.bobID}, http.StatusBadRequest, services.ErrManagerNotAllowed},
		{"eksik user_id", s.admin, map[string]any{}, http.StatusBadRequest, nil},
		// Yönetici olmayan, gövdesi bozuk olsa da önce 403 alır (gövde okunmaz).
		{"üye, bozuk gövde", s.bob, map[string]any{"user_id": "bozuk"}, http.StatusForbidden, services.ErrUnauthorizedRecipients},
	} {
		code, res := api.call(http.MethodPost, r, c.by.token, c.body)
		if code != c.want || (c.err != nil && res.Error != c.err.Error()) {
			t.Errorf("%s: HTTP %d %+v", c.name, code, res)
		}
	}

	// Yönetici rotaları: kendi havuzu için yönetici zorunlu, üye olmalı, alıcıyla aynı olamaz.
	m := r + "/" + s.adminID.String() + "/manager"
	for _, c := range []struct {
		name string
		body map[string]any
		want error
	}{
		{"null", map[string]any{"manager_user_id": nil}, services.ErrManagerRequired},
		{"kendisi", map[string]any{"manager_user_id": s.adminID}, services.ErrManagerIsRecipient},
		{"üye değil", map[string]any{"manager_user_id": outsiderID}, services.ErrManagerNotMember},
	} {
		if code, res := api.call(http.MethodPatch, m, s.admin.token, c.body); code != http.StatusBadRequest || res.Error != c.want.Error() {
			t.Errorf("yönetici %s: HTTP %d %+v", c.name, code, res)
		}
	}
	if code, res := api.call(http.MethodPatch, m, s.bob.token, "bozuk-gövde"); code != http.StatusForbidden || res.Error != services.ErrUnauthorizedRecipients.Error() {
		t.Errorf("üye, bozuk gövdeyle yönetici değiştirme: HTTP %d %+v", code, res)
	}
	if code, res := api.call(http.MethodPatch, r+"/"+s.aliceID.String()+"/manager", s.admin.token, map[string]any{"manager_user_id": s.bobID}); code != http.StatusBadRequest || res.Error != services.ErrManagerNotAllowed.Error() {
		t.Errorf("başka alıcının yöneticisi: HTTP %d %+v", code, res)
	}
	if code, _ := api.call(http.MethodPatch, r+"/"+s.carolID.String()+"/manager", s.admin.token, map[string]any{"manager_user_id": nil}); code != http.StatusNotFound {
		t.Errorf("alıcı olmayanın yöneticisi: HTTP %d", code)
	}
	res := api.mustOK(http.MethodPatch, m, s.admin.token, map[string]any{"manager_user_id": s.carolID}, http.StatusOK)
	if string(res.Data) != `{"pool_id":"`+s.poolAdmin+`"}` {
		t.Fatalf("yönetici yanıtı: %s", res.Data)
	}

	// Kaldırma: fikirsiz havuz onaysız silinir.
	poolCarol := s.addRecipient(s.carolID, nil)
	api.mustOK(http.MethodDelete, r+"/"+s.carolID.String(), s.admin.token, nil, http.StatusOK)
	if _, ok := s.poolCards(s.bob)[poolCarol]; ok {
		t.Fatal("havuz silinmedi")
	}
	// Fikirli havuz: onaysız 409, onayla fikirler, oylar, katkılar ve bildirimler de silinir.
	api.mustOK(http.MethodPost, "/ideas/"+s.ideaAlice+"/vote", s.carol.token, map[string]int{"vote_type": 1}, http.StatusOK)
	if code, res := api.call(http.MethodDelete, r+"/"+s.aliceID.String(), s.admin.token, nil); code != http.StatusConflict || res.Error != services.ErrPoolHasIdeas.Error() {
		t.Fatalf("fikirli havuz onaysız: HTTP %d %+v", code, res)
	}
	if code, _ := api.call(http.MethodDelete, r+"/"+s.aliceID.String()+"?confirm=true", s.bob.token, nil); code != http.StatusForbidden {
		t.Fatalf("üye alıcı kaldırdı: HTTP %d", code)
	}
	api.mustOK(http.MethodDelete, r+"/"+s.aliceID.String()+"?confirm=true", s.admin.token, nil, http.StatusOK)
	for table, query := range map[string]string{
		"gift_ideas": "pool_id = ?", "pool_contributions": "pool_id = ?", "notifications": "pool_id = ?",
	} {
		var n int64
		if err := api.db.Table(table).Where(query, s.poolAlice).Count(&n).Error; err != nil || n != 0 {
			t.Fatalf("%s havuzdan sonra kaldı: %d %v", table, n, err)
		}
	}
	var votes int64
	api.db.Model(&models.Vote{}).Where("gift_idea_id = ?", s.ideaAlice).Count(&votes)
	if votes != 0 {
		t.Fatalf("oylar silinmedi: %d", votes)
	}
	if code, _ := api.call(http.MethodDelete, r+"/"+s.aliceID.String(), s.admin.token, nil); code != http.StatusNotFound {
		t.Fatalf("alıcı olmayanı kaldırma: HTTP %d", code)
	}

	// Yöneticinin kendi havuzu: fikir olsun olmasın onaysız her zaman 409 (fikir varlığı sızmaz).
	api.mustOK(http.MethodDelete, "/ideas/"+s.ideaAdmin, s.alice.token, nil, http.StatusOK) // havuz artık fikirsiz
	if code, res := api.call(http.MethodDelete, r+"/"+s.adminID.String(), s.admin.token, nil); code != http.StatusConflict || res.Error != services.ErrPoolHasIdeas.Error() {
		t.Fatalf("yöneticinin fikirsiz kendi havuzu onaysız: HTTP %d %+v", code, res)
	}
	s.addIdea(s.bob, s.poolAdmin, "Gizli")
	code, withIdeas := api.call(http.MethodDelete, r+"/"+s.adminID.String(), s.admin.token, nil)
	_, without := api.call(http.MethodDelete, r+"/"+s.adminID.String(), s.admin.token, nil)
	if code != http.StatusConflict || withIdeas.Error != without.Error {
		t.Fatalf("yöneticinin fikirli kendi havuzu farklı yanıt verdi: HTTP %d %+v", code, withIdeas)
	}
	api.mustOK(http.MethodDelete, r+"/"+s.adminID.String()+"?confirm=true", s.admin.token, nil, http.StatusOK)

	// Yönetici kendini alıcı yaparsa yanıt kilitli karttır.
	api.mustOK(http.MethodPost, s.g()+"/members", s.admin.token, map[string]string{"email": outsider.email}, http.StatusCreated)
	if code, res := api.call(http.MethodPost, r, s.admin.token, map[string]any{"user_id": s.adminID}); code != http.StatusBadRequest || res.Error != services.ErrManagerRequired.Error() {
		t.Fatalf("yöneticisiz kendi havuzu: HTTP %d %+v", code, res)
	}
	added := api.mustOK(http.MethodPost, r, s.admin.token, map[string]any{"user_id": s.adminID, "manager_user_id": outsiderID}, http.StatusCreated)
	var locked map[string]map[string]any
	if err := json.Unmarshal(added.Data, &locked); err != nil || locked["pool"]["locked"] != true || len(locked["pool"]) != 4 {
		t.Fatalf("kendini alıcı yapan yöneticiye kilitli kart dönmedi: %s", added.Data)
	}

	// MULTI grubu silmek havuzları, katkıları ve havuz fikirlerini de siler.
	s.addIdea(s.carol, fmt.Sprint(locked["pool"]["id"]), "Son fikir")
	api.mustOK(http.MethodDelete, s.g(), s.admin.token, nil, http.StatusOK)
	for _, table := range []string{"gift_pools", "gift_ideas", "notifications"} {
		var n int64
		if err := api.db.Table(table).Where("group_id = ?", s.groupID).Count(&n).Error; err != nil || n != 0 {
			t.Fatalf("grup silinince %s kaldı: %d %v", table, n, err)
		}
	}
	var contributions int64
	api.db.Model(&models.PoolContribution{}).Count(&contributions)
	if contributions != 0 {
		t.Fatalf("grup silinince %d katkı satırı kaldı", contributions)
	}
}

// Hesap silme: alıcının havuzu silinir; yönetici silinirse havuz yöneticisiz kalır, grup yöneticisi
// yeniden atar.
func TestPoolAccountDeletion(t *testing.T) {
	s := setupMulti(t)
	api := s.api
	api.mustOK(http.MethodDelete, "/me", s.bob.token, nil, http.StatusOK)
	card := s.poolCards(s.alice)[s.poolAdmin]
	if card.Manager != nil || card.CanManage {
		t.Fatalf("silinen yönetici kartta kaldı: %+v", card)
	}
	if c := s.poolCards(s.carol)[s.poolAdmin]; c.CanManage {
		t.Fatal("yöneticisiz havuzu biri yönetebiliyor")
	}
	if code, _ := api.call(http.MethodPatch, s.pool(s.poolAdmin)+"/budget", s.carol.token, map[string]string{"total_budget": "1"}); code != http.StatusForbidden {
		t.Fatalf("yöneticisiz havuzda bütçe: HTTP %d", code)
	}
	api.mustOK(http.MethodPatch, s.g()+"/recipients/"+s.adminID.String()+"/manager", s.admin.token, map[string]any{"manager_user_id": s.carolID}, http.StatusOK)
	api.mustOK(http.MethodPatch, s.pool(s.poolAdmin)+"/budget", s.carol.token, map[string]string{"total_budget": "1"}, http.StatusOK)

	api.mustOK(http.MethodDelete, "/me", s.alice.token, nil, http.StatusOK)
	if _, ok := s.poolCards(s.carol)[s.poolAlice]; ok {
		t.Fatal("silinen alıcının havuzu kaldı")
	}
}

// (i) Eşzamanlı alıcı ekleme ve üye ekleme: her havuzda katkı satırları, alıcı dışındaki tüm
// üyelerle birebir aynıdır (grup satırı kilidi iki işlemi sıraya sokar).
func TestConcurrentRecipientAndMemberJoin(t *testing.T) {
	s := setupMulti(t)
	api := s.api
	const rounds = 6
	var wg sync.WaitGroup
	errs := make(chan string, rounds*2)
	do := func(method, path, token string, body any, want int) {
		defer wg.Done()
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, "/api/v1"+path, strings.NewReader(string(raw)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()
		api.router.ServeHTTP(res, req)
		if res.Code != want {
			errs <- fmt.Sprintf("%s %s: HTTP %d %s", method, path, res.Code, res.Body)
		}
	}
	for i := 0; i < rounds; i++ {
		member := api.newUser(fmt.Sprintf("Recipient%d", i))
		api.mustOK(http.MethodPost, s.g()+"/members", s.admin.token, map[string]string{"email": member.email}, http.StatusCreated)
		memberID := api.userID(member)
		joiner := api.newUser(fmt.Sprintf("Joiner%d", i))
		wg.Add(2)
		go do(http.MethodPost, s.g()+"/recipients", s.admin.token, map[string]any{"user_id": memberID}, http.StatusCreated)
		go do(http.MethodPost, s.g()+"/members", s.admin.token, map[string]string{"email": joiner.email}, http.StatusCreated)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}

	var pools []models.GiftPool
	if err := api.db.Find(&pools, "group_id = ?", s.groupID).Error; err != nil || len(pools) != rounds+2 {
		t.Fatalf("havuz sayısı: %d %v", len(pools), err)
	}
	var memberIDs []uuid.UUID
	if err := api.db.Model(&models.GroupMember{}).Where("group_id = ?", s.groupID).Pluck("user_id", &memberIDs).Error; err != nil {
		t.Fatal(err)
	}
	for _, p := range pools {
		rows := api.contributionRows(p.ID.String())
		if len(rows) != len(memberIDs)-1 {
			t.Errorf("havuz %s: %d katkı satırı, %d üye (alıcı hariç beklenen %d)", p.ID, len(rows), len(memberIDs), len(memberIDs)-1)
		}
		for _, id := range memberIDs {
			if _, ok := rows[id]; ok == (id == p.RecipientUserID) {
				t.Errorf("havuz %s: üye %s katkı satırı tutarsız (alıcı=%v)", p.ID, id, id == p.RecipientUserID)
			}
		}
	}
}

// Şema: yeni yabancı anahtarlar mevcut bir veritabanında da AutoMigrate ile oluşur.
func TestPoolSchemaConstraints(t *testing.T) {
	api := newTestAPI(t)
	m := api.db.Migrator()
	for _, c := range []struct {
		model any
		name  string
	}{
		{&models.GiftIdea{}, "Pool"}, {&models.Notification{}, "Pool"}, {&models.GiftPool{}, "Group"},
		{&models.GiftPool{}, "Recipient"}, {&models.GiftPool{}, "Manager"}, {&models.PoolContribution{}, "Pool"},
		{&models.PoolContribution{}, "User"},
	} {
		if !m.HasConstraint(c.model, c.name) {
			t.Errorf("%T.%s yabancı anahtarı yok", c.model, c.name)
		}
	}
	if !m.HasColumn(&models.Group{}, "mode") {
		t.Fatal("groups.mode yok")
	}
	creator := api.userID(api.newUser("Creator"))
	if err := api.db.Exec("INSERT INTO groups (title, target_person_name, creator_id, mode) VALUES ('x', 'y', ?, 'SINGLE')", creator).Error; err != nil {
		t.Fatalf("geçerli mode reddedildi: %v", err)
	}
	if err := api.db.Exec("INSERT INTO groups (title, target_person_name, creator_id, mode) VALUES ('x', 'y', ?, 'BOTH')", creator).Error; err == nil {
		t.Fatal("geçersiz mode kabul edildi")
	}
}

// fakeGenerator, Gemini yerine sabit, kaynaklı öneriler döner ve son istemi saklar.
type fakeGenerator struct {
	mu     sync.Mutex
	prompt string
}

func (g *fakeGenerator) GenerateGiftIdeas(_ context.Context, prompt string) ([]gemini.AIIdeaSuggestion, error) {
	g.mu.Lock()
	g.prompt = prompt
	g.mu.Unlock()
	item := func(title string) gemini.AIIdeaSuggestion {
		return gemini.AIIdeaSuggestion{Title: title, SearchQuery: title, Description: "Ofis için uygun hediye.",
			EstimatedPrice: decimal.NewFromInt(800), Sources: []gemini.Source{{Title: "trendyol.com", URL: "https://www.trendyol.com/x"}}}
	}
	return []gemini.AIIdeaSuggestion{item("AI Gizli Bitki Seti"), item("AI Gizli Kitap Ayracı")}, nil
}

// (4) Başarılı AI üretimi havuz kapsamında: fikirler pool_id taşır, ai_ideas_added bildirimi
// pool_id + recipient_name ile gider ve alıcıya hiçbir şey (satır, liste, akış) ulaşmaz.
func TestPoolAIGenerationNotifiesWithoutReachingRecipient(t *testing.T) {
	gen := &fakeGenerator{}
	api := newTestAPIWithAI(t, gen)
	s := &multiScenario{api: api, admin: api.newUser("Admin"), alice: api.newUser("Alice"), bob: api.newUser("Bob"), carol: api.newUser("Carol")}
	s.adminID, s.aliceID, s.bobID, s.carolID = api.userID(s.admin), api.userID(s.alice), api.userID(s.bob), api.userID(s.carol)
	s.groupID = idOf(t, api.mustOK(http.MethodPost, "/groups", s.admin.token, map[string]any{"title": "AI grubu", "mode": "MULTI"}, http.StatusCreated))
	for _, u := range []user{s.alice, s.bob, s.carol} {
		api.mustOK(http.MethodPost, s.g()+"/members", s.admin.token, map[string]string{"email": u.email}, http.StatusCreated)
	}
	s.poolAlice = s.addRecipient(s.aliceID, nil)
	api.mustOK(http.MethodPost, s.pool(s.poolAlice)+"/interests", s.bob.token, map[string]string{"interest": "Bitki"}, http.StatusOK)
	before := api.rawBody(http.MethodGet, "/notifications", s.alice.token)
	signal, cancel, err := api.userHub.Subscribe(s.aliceID)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	res := api.mustOK(http.MethodPost, s.pool(s.poolAlice)+"/ideas/generate-ai", s.bob.token, nil, http.StatusCreated)
	var ideas []struct {
		PoolID string `json:"pool_id"`
	}
	if err := json.Unmarshal(res.Data, &ideas); err != nil || len(ideas) != 2 || ideas[0].PoolID != s.poolAlice {
		t.Fatalf("AI fikirleri havuza bağlı değil: %s", res.Data)
	}
	if !strings.Contains(gen.prompt, "İsim: Alice") || !strings.Contains(gen.prompt, "Bitki") {
		t.Fatalf("istem havuz verisini kullanmıyor:\n%s", gen.prompt)
	}
	for _, u := range []user{s.admin, s.carol} {
		raw := string(api.rawBody(http.MethodGet, "/notifications", u.token))
		if !strings.Contains(raw, `"pool_id":"`+s.poolAlice+`","type":"ai_ideas_added"`) || !strings.Contains(raw, `"recipient_name":"Alice"`) {
			t.Fatalf("%s ai_ideas_added (pool_id ile) almadı: %s", u.email, raw)
		}
	}
	var rows int64
	api.db.Model(&models.Notification{}).Where("user_id = ? AND pool_id = ?", s.aliceID, s.poolAlice).Count(&rows)
	if rows != 0 {
		t.Fatalf("alıcı için %d bildirim satırı üretildi", rows)
	}
	if after := api.rawBody(http.MethodGet, "/notifications", s.alice.token); string(after) != string(before) {
		t.Fatalf("alıcının bildirimleri değişti:\n%s\n%s", before, after)
	}
	select {
	case <-signal:
		t.Fatal("alıcının bildirim akışına sinyal gitti")
	default:
	}
	// Alıcı AI'ı kendi havuzunda tetikleyemez (üretici hiç çağrılmaz).
	gen.prompt = ""
	code, r := api.call(http.MethodPost, s.pool(s.poolAlice)+"/ideas/generate-ai", s.alice.token, nil)
	expectSurprise(t, "alıcı AI", code, r)
	if gen.prompt != "" {
		t.Fatal("alıcının isteği üreticiyi çağırdı")
	}
}

// next_event_date: görünen açık havuzların en yakın YAKLAŞAN tarihi; yaklaşan yoksa en erken geçmiş
// tarih. Çağıranın kendi havuzu hiçbir zaman sayılmaz.
func TestNextEventDatePrefersUpcoming(t *testing.T) {
	s := setupMulti(t)
	api := s.api
	poolCarol := s.addRecipient(s.carolID, nil)
	trt := time.Now().In(time.FixedZone("TRT", 3*3600))
	day := func(offset int) string { return trt.AddDate(0, 0, offset).Format(models.DateLayout) }
	next := func(u user) string {
		t.Helper()
		var rows []struct {
			ID   string `json:"id"`
			Next string `json:"next_event_date"`
		}
		if err := json.Unmarshal(api.mustOK(http.MethodGet, "/groups", u.token, nil, http.StatusOK).Data, &rows); err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if r.ID == s.groupID {
				return r.Next
			}
		}
		return "?"
	}
	setDate := func(poolID string, manager user, date string) {
		t.Helper()
		api.mustOK(http.MethodPatch, s.pool(poolID)+"/event-date", manager.token, map[string]any{"event_date": date}, http.StatusOK)
	}
	setDate(s.poolAdmin, s.bob, day(-10)) // geçmiş
	setDate(poolCarol, s.admin, day(30))  // yaklaşan
	setDate(s.poolAlice, s.admin, day(2)) // Alice'in kendi havuzu: en yakın ama Alice için sayılmaz
	if got := next(s.alice); got != day(30) {
		t.Fatalf("alice: geçmiş + yaklaşan → yaklaşan beklenir, %s", got)
	}
	if got := next(s.bob); got != day(2) {
		t.Fatalf("bob: en yakın yaklaşan beklenir, %s", got)
	}
	setDate(poolCarol, s.admin, day(-3))
	if got := next(s.alice); got != day(-10) {
		t.Fatalf("alice: yalnızca geçmiş → en erken geçmiş beklenir, %s", got)
	}
	setDate(s.poolAlice, s.admin, day(1)) // kendi havuzundaki değişiklik Alice'in değerini etkilemez
	if got := next(s.alice); got != day(-10) {
		t.Fatalf("alice: kendi havuzu sayıldı, %s", got)
	}
}
