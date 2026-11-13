package main

// Anti-Spoiler entegrasyon testleri: gerçek router, gerçek middleware zinciri ve gerçek
// PostgreSQL ile uçtan uca çalışır.
//
// Yalnızca TEST_DATABASE_URL tanımlıysa çalışır; aksi halde atlanır. Testler başlamadan
// tüm tabloları boşalttığı için veritabanı adı "_test" ile bitmek ZORUNDADIR. Geçici test
// veritabanı için:
//
//	docker compose --profile test up -d --wait test-db
//	TEST_DATABASE_URL=postgres://covertgift:covertgift_test@localhost:55432/covertgift_test?sslmode=disable go test ./cmd/api/

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/MSheker20/CovertGift/backend/internal/config"
	"github.com/MSheker20/CovertGift/backend/internal/realtime"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const testPassword = "Password123!"

var surpriseMessage = services.ErrAntiSpoilerTriggered.Error()

type apiResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

type testAPI struct {
	t      *testing.T
	router *gin.Engine
	db     *gorm.DB
	// userHub bildirim akışlarının hub'ıdır; testler sinyali buradan dinler.
	userHub *realtime.Hub
}

// testDatabaseURL TEST_DATABASE_URL'yi döndürür; tanımlı değilse testi atlar, veritabanı
// adı "_test" ile bitmiyorsa testi durdurur (gerçek veriyi korumak için).
func testDatabaseURL(t *testing.T) string {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if raw == "" {
		t.Skip("TEST_DATABASE_URL tanımlı değil; Anti-Spoiler entegrasyon testleri atlandı")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("TEST_DATABASE_URL ayrıştırılamadı: %v", err)
	}
	if name := strings.TrimPrefix(u.Path, "/"); !strings.HasSuffix(name, "_test") {
		t.Fatalf("TEST_DATABASE_URL veritabanı adı %q; tablolar silineceği için adı _test ile bitmelidir", name)
	}
	return raw
}

func newTestAPI(t *testing.T) *testAPI {
	t.Helper()
	dsn := testDatabaseURL(t)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("test veritabanına bağlanılamadı: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := migrate(db); err != nil {
		t.Fatalf("migrasyon: %v", err)
	}
	if err := db.Exec("TRUNCATE TABLE votes, gift_ideas, group_members, groups, users RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatalf("test tabloları boşaltılamadı: %v", err)
	}
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{JWTSecret: "integration-test-secret", AppEnv: "test"}
	userHub := realtime.NewHub(100)
	return &testAPI{t: t, router: newRouter(db, cfg, nil, realtime.NewHub(100), userHub), db: db, userHub: userHub}
}

func (a *testAPI) call(method, path, token string, body any) (int, apiResponse) {
	a.t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			a.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, "/api/v1"+path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res := httptest.NewRecorder()
	a.router.ServeHTTP(res, req)
	var parsed apiResponse
	if err := json.Unmarshal(res.Body.Bytes(), &parsed); err != nil {
		a.t.Fatalf("%s %s: yanıt JSON değil (HTTP %d): %s", method, path, res.Code, res.Body)
	}
	return res.Code, parsed
}

func (a *testAPI) mustOK(method, path, token string, body any, want int) apiResponse {
	a.t.Helper()
	code, res := a.call(method, path, token, body)
	if code != want || !res.Success {
		a.t.Fatalf("%s %s: HTTP %d, beklenen %d: %+v", method, path, code, want, res)
	}
	return res
}

type user struct {
	email string
	token string
}

func (a *testAPI) newUser(name string) user {
	a.t.Helper()
	email := strings.ToLower(name) + "-" + uuid.NewString()[:8] + "@test.local"
	a.mustOK(http.MethodPost, "/auth/register", "", map[string]string{"name": name, "email": email, "password": testPassword}, http.StatusCreated)
	res := a.mustOK(http.MethodPost, "/auth/login", "", map[string]string{"email": email, "password": testPassword}, http.StatusOK)
	var login struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(res.Data, &login); err != nil || login.Token == "" {
		a.t.Fatalf("giriş token'ı alınamadı: %s", res.Data)
	}
	return user{email: email, token: login.Token}
}

func idOf(t *testing.T, res apiResponse) string {
	t.Helper()
	var obj struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(res.Data, &obj); err != nil || obj.ID == "" {
		t.Fatalf("yanıtta id yok: %s", res.Data)
	}
	return obj.ID
}

func listedGroupIDs(t *testing.T, res apiResponse) map[string]bool {
	t.Helper()
	var groups []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(res.Data, &groups); err != nil {
		t.Fatalf("grup listesi okunamadı: %s", res.Data)
	}
	ids := map[string]bool{}
	for _, g := range groups {
		ids[g.ID] = true
	}
	return ids
}

// scenario: admin, hedefi e-postayla bağlı bir grup kurar, üye ekler ve üye bir fikir ekler.
type scenario struct {
	api                            *testAPI
	admin, member, target, outside user
	groupID, ideaID                string
}

func setupScenario(t *testing.T) *scenario {
	t.Helper()
	api := newTestAPI(t)
	s := &scenario{api: api, admin: api.newUser("Admin"), member: api.newUser("Member"), target: api.newUser("Target"), outside: api.newUser("Outsider")}
	group := api.mustOK(http.MethodPost, "/groups", s.admin.token, map[string]any{
		"title": "Doğum günü sürprizi", "target_person_name": "Hedef", "target_email": s.target.email,
		"target_interests": "Kamp", "total_budget": "3000",
	}, http.StatusCreated)
	s.groupID = idOf(t, group)
	api.mustOK(http.MethodPost, "/groups/"+s.groupID+"/members", s.admin.token, map[string]string{"email": s.member.email}, http.StatusCreated)
	idea := api.mustOK(http.MethodPost, "/groups/"+s.groupID+"/ideas", s.member.token, map[string]any{
		"title": "Kamp sandalyesi", "estimated_price": "900", "search_query": "kamp sandalyesi",
	}, http.StatusCreated)
	s.ideaID = idOf(t, idea)
	return s
}

func TestAntiSpoilerGroupListHidesTargetGroup(t *testing.T) {
	s := setupScenario(t)
	for name, u := range map[string]user{"admin": s.admin, "member": s.member} {
		if ids := listedGroupIDs(t, s.api.mustOK(http.MethodGet, "/groups", u.token, nil, http.StatusOK)); !ids[s.groupID] {
			t.Errorf("%s kendi grubunu listede göremedi", name)
		}
	}
	res := s.api.mustOK(http.MethodGet, "/groups", s.target.token, nil, http.StatusOK)
	if listedGroupIDs(t, res)[s.groupID] {
		t.Fatal("hedef kişi kendisi için kurulan grubu listede gördü")
	}
	if strings.Contains(string(res.Data), "Doğum günü sürprizi") {
		t.Fatal("hedef kişinin grup listesi sürpriz grubun başlığını içeriyor")
	}
}

func TestAntiSpoilerTargetBlockedOnEveryGroupRoute(t *testing.T) {
	s := setupScenario(t)
	g := "/groups/" + s.groupID
	routes := []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, g, nil},
		{http.MethodDelete, g, nil},
		{http.MethodPost, g + "/members", map[string]string{"email": s.outside.email}},
		{http.MethodPost, g + "/interests", map[string]string{"interest": "Satranç"}},
		{http.MethodDelete, g + "/interests", map[string]string{"interest": "Kamp"}},
		{http.MethodPatch, g + "/budget", map[string]string{"total_budget": "1"}},
		{http.MethodPatch, g + "/target", map[string]string{"email": s.outside.email}},
		{http.MethodGet, g + "/ideas", nil},
		{http.MethodPost, g + "/ideas", map[string]any{"title": "Spoiler fikri", "estimated_price": "10"}},
		{http.MethodPost, g + "/ideas/preview-link", map[string]string{"url": "https://www.trendyol.com/x-p-1"}},
		{http.MethodPost, g + "/ideas/generate-ai", nil},
		{http.MethodPost, "/ideas/" + s.ideaID + "/vote", map[string]int{"vote_type": 1}},
		{http.MethodPost, "/ideas/" + s.ideaID + "/reject", nil},
		{http.MethodDelete, "/ideas/" + s.ideaID, nil},
		{http.MethodDelete, "/ideas/" + s.ideaID + "/vote", nil},
		{http.MethodPatch, g + "/event-date", map[string]any{"event_date": "2027-01-01"}},
		{http.MethodPost, g + "/winner", map[string][]string{"idea_ids": {s.ideaID}}},
		{http.MethodDelete, g + "/winner", nil},
		{http.MethodGet, g + "/cover", nil},
		{http.MethodPut, g + "/cover", nil},
		{http.MethodDelete, g + "/cover", nil},
		{http.MethodPatch, g + "/cover-policy", map[string]bool{"cover_admin_only": true}},
	}
	for _, r := range routes {
		t.Run(r.method+" "+strings.Replace(strings.Replace(r.path, s.groupID, ":id", 1), s.ideaID, ":idea", 1), func(t *testing.T) {
			code, res := s.api.call(r.method, r.path, s.target.token, r.body)
			if code != http.StatusForbidden || res.Success || res.Error != surpriseMessage {
				t.Fatalf("hedef kişi engellenmedi: HTTP %d %+v", code, res)
			}
			if len(res.Data) > 0 && string(res.Data) != "null" {
				t.Fatalf("403 yanıtı veri sızdırdı: %s", res.Data)
			}
		})
	}

	// Hedefin denemeleri hiçbir şeyi değiştirmemiş olmalı: grup, fikir ve bütçe yerinde.
	detail := s.api.mustOK(http.MethodGet, g, s.admin.token, nil, http.StatusOK)
	if !strings.Contains(string(detail.Data), "3000") {
		t.Fatalf("hedefin isteğinden sonra bütçe değişmiş: %s", detail.Data)
	}
	ideas := s.api.mustOK(http.MethodGet, g+"/ideas", s.member.token, nil, http.StatusOK)
	if !strings.Contains(string(ideas.Data), s.ideaID) || strings.Contains(string(ideas.Data), "Spoiler fikri") {
		t.Fatalf("hedefin isteğinden sonra hediye havuzu değişmiş: %s", ideas.Data)
	}
}

func TestAntiSpoilerOutsiderIsForbiddenWithoutSurpriseMessage(t *testing.T) {
	s := setupScenario(t)
	for _, r := range []struct{ method, path string }{
		{http.MethodGet, "/groups/" + s.groupID},
		{http.MethodGet, "/groups/" + s.groupID + "/ideas"},
	} {
		code, res := s.api.call(r.method, r.path, s.outside.token, nil)
		if code != http.StatusForbidden || res.Success {
			t.Fatalf("%s %s: üye olmayan erişebildi: HTTP %d %+v", r.method, r.path, code, res)
		}
		if res.Error == surpriseMessage {
			t.Fatalf("üye olmayana sürpriz mesajı döndü; bu, grubun onun için kurulduğunu ima eder")
		}
	}
	code, _ := s.api.call(http.MethodPost, "/ideas/"+s.ideaID+"/vote", s.outside.token, map[string]int{"vote_type": 1})
	if code != http.StatusForbidden {
		t.Fatalf("üye olmayan oy verebildi: HTTP %d", code)
	}
}

func TestAntiSpoilerTargetCannotBeAddedAsMember(t *testing.T) {
	s := setupScenario(t)
	code, res := s.api.call(http.MethodPost, "/groups/"+s.groupID+"/members", s.admin.token, map[string]string{"email": s.target.email})
	if code != http.StatusBadRequest || res.Success {
		t.Fatalf("hedef kişi üye olarak eklenebildi: HTTP %d %+v", code, res)
	}
	code, res = s.api.call(http.MethodGet, "/groups/"+s.groupID, s.target.token, nil)
	if code != http.StatusForbidden || res.Error != surpriseMessage {
		t.Fatalf("başarısız üye ekleme denemesinden sonra hedef erişebildi: HTTP %d", code)
	}
}

func TestAntiSpoilerTargetLinkedLater(t *testing.T) {
	api := newTestAPI(t)
	admin, member, target := api.newUser("Admin"), api.newUser("Member"), api.newUser("Target")
	group := api.mustOK(http.MethodPost, "/groups", admin.token, map[string]any{
		"title": "Hedefsiz grup", "target_person_name": "Henüz kayıtsız", "total_budget": "500",
	}, http.StatusCreated)
	g := "/groups/" + idOf(t, group)
	api.mustOK(http.MethodPost, g+"/members", admin.token, map[string]string{"email": member.email}, http.StatusCreated)

	// Grubun üyesi hedef olarak bağlanamaz (sürprizi zaten görmüştür).
	if code, _ := api.call(http.MethodPatch, g+"/target", admin.token, map[string]string{"email": member.email}); code != http.StatusConflict {
		t.Fatalf("üye hedef olarak bağlanabildi: HTTP %d", code)
	}
	// Yalnızca admin bağlayabilir.
	if code, _ := api.call(http.MethodPatch, g+"/target", member.token, map[string]string{"email": target.email}); code != http.StatusForbidden {
		t.Fatalf("admin olmayan hedef bağlayabildi: HTTP %d", code)
	}

	api.mustOK(http.MethodPatch, g+"/target", admin.token, map[string]string{"email": target.email}, http.StatusOK)

	code, res := api.call(http.MethodGet, g, target.token, nil)
	if code != http.StatusForbidden || res.Error != surpriseMessage {
		t.Fatalf("sonradan bağlanan hedef engellenmedi: HTTP %d %+v", code, res)
	}
	if listedGroupIDs(t, api.mustOK(http.MethodGet, "/groups", target.token, nil, http.StatusOK))[strings.TrimPrefix(g, "/groups/")] {
		t.Fatal("sonradan bağlanan hedef grubu listede gördü")
	}
	// Hedef bir kez bağlanır; değiştirilemez.
	other := api.newUser("Other")
	if code, _ := api.call(http.MethodPatch, g+"/target", admin.token, map[string]string{"email": other.email}); code != http.StatusConflict {
		t.Fatalf("bağlı hedef değiştirilebildi: HTTP %d", code)
	}
}
