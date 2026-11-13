package main

// Grup açıklaması (PATCH /groups/:id/description) ve grup detayındaki hedef hesap özeti
// (target_account) entegrasyon testleri. Çalıştırma koşulları için
// antispoiler_integration_test.go başındaki nota bakın.

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type descriptionDetail struct {
	Group struct {
		Description *string `json:"description"`
	} `json:"group"`
	TargetAccount *map[string]any `json:"target_account"`
}

func readDetail(t *testing.T, api *testAPI, groupID, token string) (descriptionDetail, string) {
	t.Helper()
	res := api.mustOK(http.MethodGet, "/groups/"+groupID, token, nil, http.StatusOK)
	var d descriptionDetail
	if err := json.Unmarshal(res.Data, &d); err != nil {
		t.Fatal(err)
	}
	return d, string(res.Data)
}

func descOf(d descriptionDetail) string {
	if d.Group.Description == nil {
		return ""
	}
	return *d.Group.Description
}

func TestGroupDescriptionUpdate(t *testing.T) {
	s := setupScenario(t)
	g := "/groups/" + s.groupID
	patch := func(token string, body any) (int, apiResponse) {
		return s.api.call(http.MethodPatch, g+"/description", token, body)
	}

	// Yönetici: metin kırpılır, yanıt güncel gruptur.
	res := s.api.mustOK(http.MethodPatch, g+"/description", s.admin.token,
		map[string]string{"description": "  \n Kampı seven Hedef için ortak hediye.\nFikirleri hafta sonuna kadar ekleyelim.  \t"}, http.StatusOK)
	var updated struct {
		ID          string `json:"id"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(res.Data, &updated); err != nil {
		t.Fatal(err)
	}
	want := "Kampı seven Hedef için ortak hediye.\nFikirleri hafta sonuna kadar ekleyelim."
	if updated.ID != s.groupID || updated.Description != want {
		t.Fatalf("yanıt güncel grubu taşımıyor: %s", res.Data)
	}
	if strings.Contains(string(res.Data), s.target.email) || strings.Contains(string(res.Data), "target_user\"") {
		t.Fatalf("açıklama yanıtı hedef hesabı sızdırdı: %s", res.Data)
	}

	// Detay ve liste aynı açıklamayı döner (üye de görür).
	for name, u := range map[string]user{"admin": s.admin, "member": s.member} {
		if d, _ := readDetail(t, s.api, s.groupID, u.token); descOf(d) != want {
			t.Fatalf("%s detayda açıklamayı göremedi: %q", name, descOf(d))
		}
	}
	list := s.api.mustOK(http.MethodGet, "/groups", s.member.token, nil, http.StatusOK)
	var items []struct {
		ID          string `json:"id"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(list.Data, &items); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range items {
		if it.ID == s.groupID {
			found = it.Description == want
		}
	}
	if !found {
		t.Fatalf("liste açıklamayı döndürmedi: %s", list.Data)
	}

	// Üye düzenleyemez (403, sürpriz mesajı değil); hedef kişi sürpriz mesajıyla 403 alır.
	if code, r := patch(s.member.token, map[string]string{"description": "Üyenin metni"}); code != http.StatusForbidden || r.Error == surpriseMessage {
		t.Fatalf("üye açıklamayı değiştirebildi ya da yanlış hata aldı: HTTP %d %+v", code, r)
	}
	if code, r := patch(s.target.token, map[string]string{"description": "Hedefin metni"}); code != http.StatusForbidden || r.Error != surpriseMessage {
		t.Fatalf("hedef kişi engellenmedi: HTTP %d %+v", code, r)
	}
	if code, _ := patch(s.outside.token, map[string]string{"description": "Yabancının metni"}); code != http.StatusForbidden {
		t.Fatalf("üye olmayan açıklamayı değiştirebildi: HTTP %d", code)
	}
	if code, _ := patch("", map[string]string{"description": "Oturumsuz"}); code != http.StatusUnauthorized {
		t.Fatalf("oturumsuz istek: HTTP %d", code)
	}

	// Uzunluk: rune sayılır (Türkçe harfler 2 bayt olsa da 1000 karakter geçerli).
	if code, r := patch(s.admin.token, map[string]string{"description": strings.Repeat("ş", 1001)}); code != http.StatusBadRequest || r.Success {
		t.Fatalf("1001 karakter kabul edildi: HTTP %d", code)
	}
	// Eksik alan ve yanlış tür 400.
	for _, bad := range []any{map[string]any{}, map[string]any{"description": nil}, map[string]any{"description": 5}} {
		if code, _ := patch(s.admin.token, bad); code != http.StatusBadRequest {
			t.Fatalf("%v: HTTP %d, beklenen 400", bad, code)
		}
	}
	// Başarısız denemeler açıklamayı değiştirmedi.
	if d, _ := readDetail(t, s.api, s.groupID, s.admin.token); descOf(d) != want {
		t.Fatalf("başarısız istekler açıklamayı değiştirdi: %q", descOf(d))
	}

	// Kırpıldıktan sonra tam 1000 karakter geçerlidir.
	long := strings.Repeat("ğ", 1000)
	s.api.mustOK(http.MethodPatch, g+"/description", s.admin.token, map[string]string{"description": "  " + long + "\n"}, http.StatusOK)
	if d, _ := readDetail(t, s.api, s.groupID, s.member.token); descOf(d) != long {
		t.Fatal("1000 karakterlik açıklama saklanmadı")
	}

	// Boş ya da yalnızca boşluk açıklamayı siler; alan yanıtta yer almaz.
	for _, empty := range []string{"   \n\t ", ""} {
		s.api.mustOK(http.MethodPatch, g+"/description", s.admin.token, map[string]string{"description": "geçici"}, http.StatusOK)
		s.api.mustOK(http.MethodPatch, g+"/description", s.admin.token, map[string]string{"description": empty}, http.StatusOK)
		d, raw := readDetail(t, s.api, s.groupID, s.admin.token)
		if d.Group.Description != nil {
			t.Fatalf("%q açıklamayı silmedi: %s", empty, raw)
		}
	}
}

func TestGroupDescriptionOnCreate(t *testing.T) {
	api := newTestAPI(t)
	admin := api.newUser("Admin")
	base := map[string]any{"title": "Oluşturma", "target_person_name": "Kişi", "total_budget": "100"}
	with := func(desc string) map[string]any {
		body := map[string]any{"description": desc}
		for k, v := range base {
			body[k] = v
		}
		return body
	}
	if code, _ := api.call(http.MethodPost, "/groups", admin.token, with(strings.Repeat("a", 1001))); code != http.StatusBadRequest {
		t.Fatalf("oluşturmada 1001 karakter: HTTP %d, beklenen 400", code)
	}
	res := api.mustOK(http.MethodPost, "/groups", admin.token, with("  Kısa açıklama \n"), http.StatusCreated)
	if d, _ := readDetail(t, api, idOf(t, res), admin.token); descOf(d) != "Kısa açıklama" {
		t.Fatalf("oluşturmada açıklama kırpılmadı: %q", descOf(d))
	}
	res = api.mustOK(http.MethodPost, "/groups", admin.token, with("   "), http.StatusCreated)
	if d, _ := readDetail(t, api, idOf(t, res), admin.token); d.Group.Description != nil {
		t.Fatal("yalnızca boşluk olan açıklama saklandı")
	}
}

func TestGroupDescriptionPublishesEventWithoutNotification(t *testing.T) {
	s := setupScenario(t)
	srv := httptest.NewServer(s.api.router)
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/groups/"+s.groupID+"/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+s.member.token)
	stream, err := http.DefaultClient.Do(req)
	if err != nil || stream.StatusCode != http.StatusOK {
		t.Fatalf("akış açılamadı: %v", err)
	}
	t.Cleanup(func() { _ = stream.Body.Close() })
	lines := make(chan string, 32)
	go func() {
		sc := bufio.NewScanner(stream.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()
	wait := func(d time.Duration) bool {
		deadline := time.After(d)
		for {
			select {
			case line := <-lines:
				if line == "event: changed" {
					return true
				}
			case <-deadline:
				return false
			}
		}
	}
	count := func() int64 {
		var n int64
		if err := s.api.db.Table("notifications").Where("group_id = ?", s.groupID).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := count()

	s.api.mustOK(http.MethodPatch, "/groups/"+s.groupID+"/description", s.admin.token, map[string]string{"description": "Yeni açıklama"}, http.StatusOK)
	if !wait(2 * time.Second) {
		t.Fatal("açıklama değişikliği 'changed' olayı üretmedi")
	}
	// Başarısız istek olay üretmez.
	if code, _ := s.api.call(http.MethodPatch, "/groups/"+s.groupID+"/description", s.member.token, map[string]string{"description": "x"}); code != http.StatusForbidden {
		t.Fatalf("HTTP %d", code)
	}
	if wait(300 * time.Millisecond) {
		t.Fatal("başarısız istek 'changed' olayı üretti")
	}
	if after := count(); after != before {
		t.Fatalf("açıklama değişikliği bildirim üretti: %d -> %d", before, after)
	}
}

// Grup detayı bağlı hedef hesabın yalnızca kimliğini ve adını döner; e-posta hiçbir yanıtta
// yer almaz. Liste ucu hedef hesap bilgisini hiç taşımaz; hedef kişi hiçbir yoldan erişemez.
func TestTargetAccountSummaryOnlyForMembers(t *testing.T) {
	api := newTestAPI(t)
	admin, member, target := api.newUser("Admin"), api.newUser("Member"), api.newUser("Hedefcan")
	var targetID string
	if err := api.db.Table("users").Select("id").Where("email = ?", target.email).Scan(&targetID).Error; err != nil || targetID == "" {
		t.Fatalf("hedef kimliği okunamadı: %v", err)
	}

	// Hedefsiz grup: target_account yok.
	created := api.mustOK(http.MethodPost, "/groups", admin.token, map[string]any{
		"title": "Sonradan bağlanacak", "target_person_name": "Hatice", "total_budget": "500",
	}, http.StatusCreated)
	gid := idOf(t, created)
	g := "/groups/" + gid
	api.mustOK(http.MethodPost, g+"/members", admin.token, map[string]string{"email": member.email}, http.StatusCreated)
	if d, raw := readDetail(t, api, gid, member.token); d.TargetAccount != nil || strings.Contains(raw, "target_account") {
		t.Fatalf("hedefsiz grupta target_account döndü: %s", raw)
	}

	// Bağlama yanıtı hedefin e-postasını ya da hesap nesnesini taşımaz.
	link := api.mustOK(http.MethodPatch, g+"/target", admin.token, map[string]string{"email": target.email}, http.StatusOK)
	if strings.Contains(string(link.Data), target.email) || strings.Contains(string(link.Data), "Hedefcan") {
		t.Fatalf("bağlama yanıtı hedef hesabı sızdırdı: %s", link.Data)
	}

	for name, u := range map[string]user{"admin": admin, "member": member} {
		res := api.mustOK(http.MethodGet, g, u.token, nil, http.StatusOK)
		var d struct {
			Group         map[string]json.RawMessage `json:"group"`
			TargetAccount map[string]json.RawMessage `json:"target_account"`
		}
		if err := json.Unmarshal(res.Data, &d); err != nil {
			t.Fatal(err)
		}
		if len(d.TargetAccount) != 2 || string(d.TargetAccount["id"]) != `"`+targetID+`"` || string(d.TargetAccount["name"]) != `"Hedefcan"` {
			t.Fatalf("%s: target_account yalnızca id + ad olmalı: %s", name, res.Data)
		}
		if _, ok := d.Group["target_user"]; ok {
			t.Fatalf("%s: grup nesnesi target_user taşıyor: %s", name, res.Data)
		}
		raw := string(res.Data)
		if strings.Contains(raw, target.email) || strings.Contains(raw, "password") {
			t.Fatalf("%s: detay hedefin e-postasını ya da şifre alanını sızdırdı: %s", name, raw)
		}
	}

	// Liste ucu hedef hesap adını hiç taşımaz.
	for name, u := range map[string]user{"admin": admin, "member": member} {
		raw := string(api.mustOK(http.MethodGet, "/groups", u.token, nil, http.StatusOK).Data)
		if strings.Contains(raw, "target_account") || strings.Contains(raw, "Hedefcan") || strings.Contains(raw, target.email) || strings.Contains(raw, "target_user\"") {
			t.Fatalf("%s: grup listesi hedef hesabı taşıyor: %s", name, raw)
		}
	}

	// Hedef kişi: detay ve açıklama ucu 403 + sürpriz mesajı, veri yok; listede grup yok.
	for _, r := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, g, nil},
		{http.MethodPatch, g + "/description", map[string]string{"description": "x"}},
	} {
		code, res := api.call(r.method, r.path, target.token, r.body)
		if code != http.StatusForbidden || res.Error != surpriseMessage || (len(res.Data) > 0 && string(res.Data) != "null") {
			t.Fatalf("%s %s: hedef engellenmedi ya da veri sızdı: HTTP %d %+v", r.method, r.path, code, res)
		}
	}
	listRaw := string(api.mustOK(http.MethodGet, "/groups", target.token, nil, http.StatusOK).Data)
	if strings.Contains(listRaw, gid) || strings.Contains(listRaw, "Hatice") {
		t.Fatalf("hedefin listesi grubu taşıyor: %s", listRaw)
	}
	// Hedefin bildirimlerinde de bu gruptan iz yok.
	notes := string(api.mustOK(http.MethodGet, "/notifications", target.token, nil, http.StatusOK).Data)
	if strings.Contains(notes, gid) {
		t.Fatalf("hedefin bildirimleri grubu taşıyor: %s", notes)
	}
}
