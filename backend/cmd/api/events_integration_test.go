package main

// SSE entegrasyon testi: gerçek router üzerinden akış açılır, başka bir üyenin oyu
// "changed" olayı üretir; hedef kişi ve üye olmayan akış açamaz.

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGroupEventsStream(t *testing.T) {
	s := setupScenario(t)
	srv := httptest.NewServer(s.api.router)
	// Cleanup'lar ters sırayla çalışır: önce akış bağlamları iptal edilir, en son sunucu kapanır.
	// defer kullanılsaydı sunucu, açık akışın 5 dakikalık ömrünü beklerdi.
	t.Cleanup(srv.Close)
	url := srv.URL + "/api/v1/groups/" + s.groupID + "/events"

	open := func(token string) *http.Response {
		t.Helper()
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = res.Body.Close() })
		return res
	}

	for name, token := range map[string]string{"target": s.target.token, "outsider": s.outside.token} {
		if res := open(token); res.StatusCode != http.StatusForbidden {
			t.Fatalf("%s akış açabildi: HTTP %d", name, res.StatusCode)
		}
	}
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if res, err := http.DefaultClient.Do(req); err != nil || res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("oturumsuz akış: %v %v", err, res)
	}

	res := open(s.admin.token)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("admin akış açamadı: HTTP %d", res.StatusCode)
	}
	lines := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()
	expect := func(want string) {
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
	expect(": connected")

	// /ideas/:id altında bir değişiklik (oy) ve /groups/:id altında bir değişiklik (bütçe).
	s.api.mustOK(http.MethodPost, "/ideas/"+s.ideaID+"/vote", s.member.token, map[string]int{"vote_type": 1}, http.StatusOK)
	expect("event: changed")
	s.api.mustOK(http.MethodPatch, "/groups/"+s.groupID+"/budget", s.admin.token, map[string]string{"total_budget": "4000"}, http.StatusOK)
	expect("event: changed")

	// Başarısız istek olay üretmez.
	s.api.call(http.MethodPost, "/ideas/"+s.ideaID+"/vote", s.member.token, map[string]int{"vote_type": 5})
	select {
	case l := <-lines:
		if l == "event: changed" {
			t.Fatal("başarısız istek olay üretti")
		}
	case <-time.After(200 * time.Millisecond):
	}
}
