package main

// Havuz kilit sırası (groups satırı → gift_pools satırı → diğerleri) için eşzamanlılık testleri:
// fikir ekleme, oy, veto, kazanan seçimi ve katkı; alıcı kaldırma, yönetici değiştirme ve grup
// silme ile aynı anda koşar. Hiçbir istek 500 (ör. PostgreSQL deadlock) almamalıdır.
//
//	TEST_DATABASE_URL=... go test -p 1 -race -count=5 -run Concurrent ./cmd/api/

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// hammer isteği t.Fatal çağırmadan gönderir (goroutine'lerden güvenli) ve durum kodunu döner.
func (a *testAPI) hammer(method, path, token string, body any) (int, string) {
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, "/api/v1"+path, strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	a.router.ServeHTTP(res, req)
	return res.Code, res.Body.String()
}

type outcomes struct {
	mu   sync.Mutex
	seen map[string]map[int]int
	bad  []string
}

func (o *outcomes) record(op string, code int, body string, allowed ...int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.seen == nil {
		o.seen = map[string]map[int]int{}
	}
	if o.seen[op] == nil {
		o.seen[op] = map[int]int{}
	}
	o.seen[op][code]++
	for _, a := range allowed {
		if code == a {
			return
		}
	}
	o.bad = append(o.bad, fmt.Sprintf("%s: HTTP %d %s", op, code, body))
}

func TestConcurrentPoolIdeasVsRecipientChanges(t *testing.T) {
	s := setupMulti(t)
	api := s.api
	const rounds, writers = 6, 4
	var o outcomes
	for round := 0; round < rounds; round++ {
		carolPool := s.addRecipient(s.carolID, nil)
		idea := s.addIdea(s.bob, carolPool, fmt.Sprintf("Tur %d", round))
		var wg sync.WaitGroup
		start := make(chan struct{})
		run := func(f func()) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				f()
			}()
		}
		for w := 0; w < writers; w++ {
			w := w
			run(func() {
				code, body := api.hammer(http.MethodPost, s.pool(carolPool)+"/ideas", s.bob.token, map[string]any{"title": fmt.Sprintf("Yarış %d-%d", round, w), "estimated_price": "10"})
				o.record("add idea (removed pool)", code, body, http.StatusCreated, http.StatusNotFound)
			})
			run(func() {
				code, body := api.hammer(http.MethodPost, s.pool(s.poolAdmin)+"/ideas", s.alice.token, map[string]any{"title": fmt.Sprintf("Yönetici havuzu %d-%d", round, w), "estimated_price": "10"})
				o.record("add idea (manager change)", code, body, http.StatusCreated)
			})
		}
		run(func() {
			code, body := api.hammer(http.MethodPost, "/ideas/"+idea+"/vote", s.alice.token, map[string]int{"vote_type": -1})
			o.record("vote (removed pool)", code, body, http.StatusOK, http.StatusNotFound)
		})
		run(func() {
			code, body := api.hammer(http.MethodPost, "/ideas/"+s.ideaAdmin+"/vote", s.carol.token, map[string]int{"vote_type": 1})
			o.record("vote (manager change)", code, body, http.StatusOK, http.StatusConflict)
		})
		run(func() {
			code, body := api.hammer(http.MethodPut, s.pool(carolPool)+"/contribution", s.alice.token, map[string]bool{"contributing": round%2 == 0})
			o.record("contribution (removed pool)", code, body, http.StatusOK, http.StatusNotFound)
		})
		run(func() {
			code, body := api.hammer(http.MethodDelete, s.g()+"/recipients/"+s.carolID.String()+"?confirm=true", s.admin.token, nil)
			o.record("remove recipient", code, body, http.StatusOK)
		})
		run(func() {
			manager := s.bobID
			if round%2 == 0 {
				manager = s.carolID
			}
			code, body := api.hammer(http.MethodPatch, s.g()+"/recipients/"+s.adminID.String()+"/manager", s.admin.token, map[string]any{"manager_user_id": manager})
			o.record("set manager", code, body, http.StatusOK)
		})
		close(start)
		wg.Wait()
		if len(o.bad) > 0 {
			t.Fatalf("tur %d:\n%s", round, strings.Join(o.bad, "\n"))
		}
	}
	t.Logf("sonuçlar: %v", o.seen)
}

// Grup silme; fikir ekleme, oy, veto, kazanan ve katkı ile yarışır (grup → havuz kilit sırası).
func TestConcurrentGroupDeleteVsPoolWrites(t *testing.T) {
	api := newTestAPI(t)
	var o outcomes
	for round := 0; round < 4; round++ {
		s := &multiScenario{api: api, admin: api.newUser("Admin"), alice: api.newUser("Alice"), bob: api.newUser("Bob"), carol: api.newUser("Carol")}
		s.adminID, s.aliceID, s.bobID, s.carolID = api.userID(s.admin), api.userID(s.alice), api.userID(s.bob), api.userID(s.carol)
		s.groupID = idOf(t, api.mustOK(http.MethodPost, "/groups", s.admin.token, map[string]any{"title": "Silinecek", "mode": "MULTI"}, http.StatusCreated))
		for _, u := range []user{s.alice, s.bob, s.carol} {
			api.mustOK(http.MethodPost, s.g()+"/members", s.admin.token, map[string]string{"email": u.email}, http.StatusCreated)
		}
		s.poolAlice = s.addRecipient(s.aliceID, nil)
		ideas := []string{s.addIdea(s.bob, s.poolAlice, "Fikir A"), s.addIdea(s.carol, s.poolAlice, "Fikir B")}
		// SINGLE grup da aynı yarışla denenir (grup satırı FOR SHARE ↔ FOR UPDATE).
		single := idOf(t, api.mustOK(http.MethodPost, "/groups", s.admin.token, map[string]any{"title": "Tekli", "target_person_name": "Ali"}, http.StatusCreated))
		api.mustOK(http.MethodPost, "/groups/"+single+"/members", s.admin.token, map[string]string{"email": s.bob.email}, http.StatusCreated)
		singleIdea := idOf(t, api.mustOK(http.MethodPost, "/groups/"+single+"/ideas", s.bob.token, map[string]any{"title": "Tekli fikir"}, http.StatusCreated))

		var wg sync.WaitGroup
		start := make(chan struct{})
		run := func(f func()) {
			wg.Add(1)
			go func() { defer wg.Done(); <-start; f() }()
		}
		// Grup silindiyse: GroupAccess / fikir yok 404, üyelik satırı gitmişse 403.
		gone := []int{http.StatusNotFound, http.StatusForbidden}
		for i := 0; i < 3; i++ {
			run(func() {
				code, body := api.hammer(http.MethodPost, s.pool(s.poolAlice)+"/ideas", s.carol.token, map[string]any{"title": uuid.NewString()[:8]})
				o.record("pool add idea", code, body, append([]int{http.StatusCreated, http.StatusConflict}, gone...)...) // 409: kazanan seçildi
			})
			run(func() {
				code, body := api.hammer(http.MethodPost, "/groups/"+single+"/ideas", s.bob.token, map[string]any{"title": uuid.NewString()[:8]})
				o.record("single add idea", code, body, append([]int{http.StatusCreated}, gone...)...)
			})
		}
		for _, u := range []user{s.bob, s.carol, s.admin} {
			u := u
			run(func() {
				code, body := api.hammer(http.MethodPost, "/ideas/"+ideas[0]+"/vote", u.token, map[string]int{"vote_type": -1})
				o.record("pool vote", code, body, append([]int{http.StatusOK, http.StatusConflict}, gone...)...)
			})
		}
		run(func() {
			code, body := api.hammer(http.MethodPost, "/ideas/"+singleIdea+"/vote", s.admin.token, map[string]int{"vote_type": -1})
			o.record("single vote", code, body, append([]int{http.StatusOK}, gone...)...)
		})
		run(func() {
			code, body := api.hammer(http.MethodPost, "/ideas/"+ideas[1]+"/reject", s.admin.token, nil)
			o.record("pool reject", code, body, append([]int{http.StatusOK, http.StatusConflict}, gone...)...)
		})
		run(func() {
			code, body := api.hammer(http.MethodPost, s.pool(s.poolAlice)+"/winner", s.admin.token, map[string][]string{"idea_ids": {ideas[1]}})
			o.record("pool winner", code, body, append([]int{http.StatusOK, http.StatusConflict}, gone...)...)
		})
		run(func() {
			code, body := api.hammer(http.MethodPut, s.pool(s.poolAlice)+"/contribution", s.bob.token, map[string]bool{"contributing": false})
			o.record("pool contribution", code, body, append([]int{http.StatusOK, http.StatusConflict}, gone...)...)
		})
		run(func() {
			code, body := api.hammer(http.MethodDelete, s.g(), s.admin.token, nil)
			o.record("delete multi group", code, body, http.StatusOK)
		})
		run(func() {
			code, body := api.hammer(http.MethodDelete, "/groups/"+single, s.admin.token, nil)
			o.record("delete single group", code, body, http.StatusOK)
		})
		close(start)
		wg.Wait()
	}
	for _, b := range o.bad {
		t.Error(b)
	}
	t.Logf("sonuçlar: %v", o.seen)
}

// Hesap silme, kullanıcının dokunduğu grupları ve havuzları diğer yazma işlemleriyle aynı sırayla
// (grup -> havuz) kilitler. Silinen kişi burada bir havuzun yöneticisi, iki havuzda fikir sahibi
// ve bir havuzun alıcısıdır; eşzamanlı oy, fikir, kazanan ve katkı işlemleri 500 dönmemelidir.
func TestConcurrentAccountDeleteVsPoolWrites(t *testing.T) {
	api := newTestAPI(t)
	var o outcomes
	for round := 0; round < 4; round++ {
		s := &multiScenario{api: api, admin: api.newUser("Admin"), alice: api.newUser("Alice"), bob: api.newUser("Bob"), carol: api.newUser("Carol")}
		s.adminID, s.aliceID, s.bobID, s.carolID = api.userID(s.admin), api.userID(s.alice), api.userID(s.bob), api.userID(s.carol)
		s.groupID = idOf(t, api.mustOK(http.MethodPost, "/groups", s.admin.token, map[string]any{"title": "Hesap silme", "mode": "MULTI"}, http.StatusCreated))
		for _, u := range []user{s.alice, s.bob, s.carol} {
			api.mustOK(http.MethodPost, s.g()+"/members", s.admin.token, map[string]string{"email": u.email}, http.StatusCreated)
		}
		s.poolAlice = s.addRecipient(s.aliceID, nil)
		poolAdmin := s.addRecipient(s.adminID, &s.bobID) // Bob yöneticinin havuzunun sorumlusu
		poolBob := s.addRecipient(s.bobID, nil)
		bobIdea := s.addIdea(s.bob, s.poolAlice, "Bob'un fikri")
		bobIdeaAdmin := s.addIdea(s.bob, poolAdmin, "Bob'un diğer fikri")
		carolIdea := s.addIdea(s.carol, s.poolAlice, "Carol'un fikri")
		inBobPool := s.addIdea(s.carol, poolBob, "Bob için")

		var wg sync.WaitGroup
		start := make(chan struct{})
		run := func(f func()) {
			wg.Add(1)
			go func() { defer wg.Done(); <-start; f() }()
		}
		// Fikir ya da havuz Bob'la birlikte silindiyse 404; işlem silmeden önce yetiştiyse 200/201/409.
		for _, u := range []user{s.carol, s.admin} {
			u := u
			run(func() {
				code, body := api.hammer(http.MethodPost, "/ideas/"+bobIdea+"/vote", u.token, map[string]int{"vote_type": 1})
				o.record("vote on deleted user's idea", code, body, http.StatusOK, http.StatusNotFound, http.StatusConflict)
			})
			run(func() {
				code, body := api.hammer(http.MethodPost, "/ideas/"+carolIdea+"/vote", u.token, map[string]int{"vote_type": -1})
				o.record("vote in shared pool", code, body, http.StatusOK, http.StatusConflict)
			})
		}
		run(func() {
			code, body := api.hammer(http.MethodPost, "/ideas/"+bobIdeaAdmin+"/vote", s.carol.token, map[string]int{"vote_type": 1})
			o.record("vote in managed pool", code, body, http.StatusOK, http.StatusNotFound, http.StatusConflict)
		})
		run(func() {
			code, body := api.hammer(http.MethodPost, "/ideas/"+inBobPool+"/vote", s.alice.token, map[string]int{"vote_type": 1})
			o.record("vote in deleted user's pool", code, body, http.StatusOK, http.StatusNotFound, http.StatusConflict)
		})
		for i := 0; i < 2; i++ {
			run(func() {
				code, body := api.hammer(http.MethodPost, s.pool(s.poolAlice)+"/ideas", s.carol.token, map[string]any{"title": uuid.NewString()[:8]})
				o.record("add idea in shared pool", code, body, http.StatusCreated, http.StatusConflict)
			})
			run(func() {
				code, body := api.hammer(http.MethodPost, s.pool(poolBob)+"/ideas", s.alice.token, map[string]any{"title": uuid.NewString()[:8]})
				o.record("add idea in deleted user's pool", code, body, http.StatusCreated, http.StatusNotFound)
			})
			run(func() {
				code, body := api.hammer(http.MethodPost, s.pool(poolAdmin)+"/ideas", s.carol.token, map[string]any{"title": uuid.NewString()[:8]})
				o.record("add idea in managed pool", code, body, http.StatusCreated)
			})
		}
		run(func() {
			code, body := api.hammer(http.MethodPost, s.pool(s.poolAlice)+"/winner", s.admin.token, map[string][]string{"idea_ids": {carolIdea}})
			o.record("winner in shared pool", code, body, http.StatusOK, http.StatusConflict)
		})
		run(func() {
			code, body := api.hammer(http.MethodPut, s.pool(poolBob)+"/contribution", s.carol.token, map[string]bool{"contributing": false})
			o.record("contribution in deleted user's pool", code, body, http.StatusOK, http.StatusNotFound)
		})
		run(func() {
			code, body := api.hammer(http.MethodDelete, "/me", s.bob.token, nil)
			o.record("delete account", code, body, http.StatusOK)
		})
		close(start)
		wg.Wait()

		// Silme sonrası tutarlılık: Bob'un havuzu yok, yöneticinin havuzu sorumlusuz kaldı.
		cards := s.poolCards(s.carol)
		if _, ok := cards[poolBob]; ok {
			t.Errorf("tur %d: silinen kullanıcının havuzu hâlâ listede", round)
		}
		if _, ok := cards[poolAdmin]; !ok {
			t.Errorf("tur %d: yöneticinin havuzu kayboldu", round)
		}
	}
	for _, b := range o.bad {
		t.Error(b)
	}
	t.Logf("sonuçlar: %v", o.seen)
}
