package main

// Profil fotoğrafı entegrasyon testleri. TEST_DATABASE_URL gerekir (antispoiler_integration_test.go).

import (
	"bytes"
	"encoding/json"
	"image"
	_ "image/jpeg"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MSheker20/CovertGift/backend/pkg/imageproc"
)

func (a *testAPI) uploadAvatarAs(token, filename, contentType string, data []byte) (int, apiResponse) {
	a.t.Helper()
	body, formType := multipartBody(a.t, "file", filename, contentType, data)
	res := a.raw(http.MethodPut, "/me/avatar", token, body, map[string]string{"Content-Type": formType})
	var parsed apiResponse
	if err := json.Unmarshal(res.Body.Bytes(), &parsed); err != nil {
		a.t.Fatalf("PUT /me/avatar: yanıt JSON değil (HTTP %d): %s", res.Code, res.Body)
	}
	return res.Code, parsed
}

// mustUploadAvatar yükler ve dönen sürümü verir.
func (a *testAPI) mustUploadAvatar(token string, data []byte) string {
	a.t.Helper()
	code, res := a.uploadAvatarAs(token, "pp.jpg", "image/jpeg", data)
	if code != http.StatusOK || !res.Success {
		a.t.Fatalf("profil fotoğrafı yüklenemedi: HTTP %d %+v", code, res)
	}
	var out struct {
		AvatarVersion string `json:"avatar_version"`
		Width         int    `json:"width"`
		Height        int    `json:"height"`
	}
	if err := json.Unmarshal(res.Data, &out); err != nil || out.AvatarVersion == "" {
		a.t.Fatalf("yanıtta avatar_version yok: %s", res.Data)
	}
	if out.Width != out.Height || out.Width > imageproc.AvatarMaxSize {
		a.t.Fatalf("fotoğraf kare ve en çok %d olmalı: %dx%d", imageproc.AvatarMaxSize, out.Width, out.Height)
	}
	return out.AvatarVersion
}

func (a *testAPI) getAvatar(ownerID, token string, headers map[string]string) *httptest.ResponseRecorder {
	a.t.Helper()
	return a.raw(http.MethodGet, "/users/"+ownerID+"/avatar", token, nil, headers)
}

func (a *testAPI) meAvatarVersion(u user) string {
	a.t.Helper()
	var me struct {
		AvatarVersion string `json:"avatar_version"`
	}
	if err := json.Unmarshal(a.mustOK(http.MethodGet, "/me", u.token, nil, http.StatusOK).Data, &me); err != nil {
		a.t.Fatal(err)
	}
	return me.AvatarVersion
}

// Yükleme, /me'de sürüm, sahibin ve grup arkadaşının alması, ETag/304, değiştirme ve silme.
func TestAvatarUploadGetETagAndDelete(t *testing.T) {
	s := setupMulti(t)
	api := s.api
	aliceID := s.aliceID.String()

	if res := api.getAvatar(aliceID, s.bob.token, nil); res.Code != http.StatusNotFound {
		t.Fatalf("fotoğraf yokken 404 beklenir: %d", res.Code)
	}
	v1 := api.mustUploadAvatar(s.alice.token, testJPEG(t, 800, 600, 1))
	if got := api.meAvatarVersion(s.alice); got != v1 {
		t.Fatalf("/me avatar_version %q, beklenen %q", got, v1)
	}

	for name, u := range map[string]user{"sahibi": s.alice, "grup arkadaşı": s.bob} {
		res := api.getAvatar(aliceID, u.token, nil)
		if res.Code != http.StatusOK || res.Header().Get("Content-Type") != "image/jpeg" {
			t.Fatalf("%s fotoğrafı alamadı: %d %s", name, res.Code, res.Header().Get("Content-Type"))
		}
		img, _, err := image.Decode(bytes.NewReader(res.Body.Bytes()))
		if err != nil || img.Bounds().Dx() != img.Bounds().Dy() {
			t.Fatalf("%s: dönen görsel kare JPEG değil: %v %v", name, err, img.Bounds())
		}
		if res.Header().Get("ETag") != `"`+v1+`"` || !strings.Contains(res.Header().Get("Cache-Control"), "private") {
			t.Fatalf("%s: önbellek başlıkları yanlış: %v", name, res.Header())
		}
	}
	if res := api.getAvatar(aliceID, s.bob.token, map[string]string{"If-None-Match": `"` + v1 + `"`}); res.Code != http.StatusNotModified || res.Body.Len() != 0 {
		t.Fatalf("eşleşen ETag için gövdesiz 304 beklenir: %d (%d bayt)", res.Code, res.Body.Len())
	}

	v2 := api.mustUploadAvatar(s.alice.token, testPNG(t, 300, 500, 2))
	if v2 == v1 {
		t.Fatal("yeni fotoğraf yeni sürüm almalı")
	}
	if res := api.getAvatar(aliceID, s.bob.token, map[string]string{"If-None-Match": `"` + v1 + `"`}); res.Code != http.StatusOK {
		t.Fatalf("eski ETag ile yeni görsel dönmeli: %d", res.Code)
	}

	api.mustOK(http.MethodDelete, "/me/avatar", s.alice.token, nil, http.StatusOK)
	api.mustOK(http.MethodDelete, "/me/avatar", s.alice.token, nil, http.StatusOK) // idempotent
	if got := api.meAvatarVersion(s.alice); got != "" {
		t.Fatalf("silince avatar_version boşalmalı: %q", got)
	}
	if res := api.getAvatar(aliceID, s.bob.token, nil); res.Code != http.StatusNotFound {
		t.Fatalf("silinen fotoğraf için 404 beklenir: %d", res.Code)
	}
}

// Ortak grubu olmayan kişi fotoğrafı göremez; yanıt "fotoğraf yok"tan ayırt edilemez (404).
// SINGLE grubun hedef kişisi üye olmadığı için üyelerle grup paylaşmaz.
func TestAvatarVisibleOnlyToPeopleSharingAGroup(t *testing.T) {
	s := setupScenario(t)
	api := s.api
	targetID := api.userID(s.target).String()
	adminID := api.userID(s.admin).String()
	api.mustUploadAvatar(s.target.token, testJPEG(t, 400, 400, 3))
	api.mustUploadAvatar(s.admin.token, testJPEG(t, 400, 400, 4))

	for name, tc := range map[string]struct {
		owner string
		u     user
	}{
		"dışarıdaki kişi yöneticinin fotoğrafını": {adminID, s.outside},
		"üye hedef kişinin fotoğrafını":           {targetID, s.member},
		"yönetici hedef kişinin fotoğrafını":      {targetID, s.admin},
		"hedef kişi yöneticinin fotoğrafını":      {adminID, s.target},
	} {
		res := api.getAvatar(tc.owner, tc.u.token, nil)
		var body apiResponse
		_ = json.Unmarshal(res.Body.Bytes(), &body)
		if res.Code != http.StatusNotFound || body.Error != "profil fotoğrafı bulunamadı" {
			t.Errorf("%s görmemeli (yok gibi 404): %d %q", name, res.Code, body.Error)
		}
	}
	if res := api.getAvatar(adminID, s.member.token, nil); res.Code != http.StatusOK {
		t.Fatalf("üye yöneticinin fotoğrafını görmeli: %d", res.Code)
	}
	if res := api.getAvatar(adminID, "", nil); res.Code != http.StatusUnauthorized {
		t.Fatalf("oturumsuz istek 401 almalı: %d", res.Code)
	}
	if res := api.getAvatar("not-a-uuid", s.admin.token, nil); res.Code != http.StatusBadRequest {
		t.Fatalf("geçersiz kimlik 400 almalı: %d", res.Code)
	}

	// Grup detayında hedef kişinin özeti fotoğraf sürümü taşımaz (üyeler göremez).
	detail := string(api.mustOK(http.MethodGet, "/groups/"+s.groupID, s.admin.token, nil, http.StatusOK).Data)
	var targetSummary struct {
		TargetUser *struct {
			AvatarVersion string `json:"avatar_version"`
		} `json:"target_account"`
	}
	if err := json.Unmarshal([]byte(detail), &targetSummary); err != nil {
		t.Fatal(err)
	}
	if targetSummary.TargetUser != nil && targetSummary.TargetUser.AvatarVersion != "" {
		t.Fatal("hedef kişinin fotoğraf sürümü üyelere gösterilmemeli")
	}
}

// avatar_version üye listesine, havuz alıcısına, sorumluya ve katılımcılara yansır.
func TestAvatarVersionInMultiResponses(t *testing.T) {
	s := setupMulti(t)
	api := s.api
	v := api.mustUploadAvatar(s.bob.token, testJPEG(t, 600, 600, 5))
	aliceV := api.mustUploadAvatar(s.alice.token, testJPEG(t, 600, 600, 6))

	// Bob, Admin'in havuzunun sorumlusu; Alice bir havuzun alıcısı. Carol'un görünümüyle bakılır.
	var detail struct {
		Members []struct {
			UserID string `json:"user_id"`
			User   struct {
				AvatarVersion string `json:"avatar_version"`
			} `json:"user"`
		} `json:"members"`
		Pools []struct {
			ID        string `json:"id"`
			Recipient struct {
				ID            string `json:"id"`
				AvatarVersion string `json:"avatar_version"`
			} `json:"recipient"`
			Manager *struct {
				ID            string `json:"id"`
				AvatarVersion string `json:"avatar_version"`
			} `json:"manager"`
		} `json:"pools"`
	}
	if err := json.Unmarshal(api.mustOK(http.MethodGet, s.g(), s.carol.token, nil, http.StatusOK).Data, &detail); err != nil {
		t.Fatal(err)
	}
	foundMember, foundRecipient, foundManager := false, false, false
	for _, m := range detail.Members {
		if m.UserID == s.bobID.String() && m.User.AvatarVersion == v {
			foundMember = true
		}
	}
	for _, p := range detail.Pools {
		if p.Recipient.ID == s.aliceID.String() && p.Recipient.AvatarVersion == aliceV {
			foundRecipient = true
		}
		if p.Manager != nil && p.Manager.ID == s.bobID.String() && p.Manager.AvatarVersion == v {
			foundManager = true
		}
	}
	if !foundMember || !foundRecipient || !foundManager {
		t.Fatalf("avatar_version eksik: üye %v, alıcı %v, sorumlu %v (%+v)", foundMember, foundRecipient, foundManager, detail)
	}

	var pool struct {
		Contributors []struct {
			UserID        string `json:"user_id"`
			AvatarVersion string `json:"avatar_version"`
		} `json:"contributors"`
	}
	if err := json.Unmarshal(api.mustOK(http.MethodGet, s.pool(s.poolAlice), s.carol.token, nil, http.StatusOK).Data, &pool); err != nil {
		t.Fatal(err)
	}
	ok := false
	for _, c := range pool.Contributors {
		if c.UserID == s.bobID.String() && c.AvatarVersion == v {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("katılımcılarda Bob'un avatar_version'ı yok: %+v", pool.Contributors)
	}
}

// Hatalı yüklemeler: tür, boyut, küçük görsel, dosyasız istek, gövde sınırı.
func TestAvatarRejectsBadUploads(t *testing.T) {
	api := newTestAPI(t)
	u := api.newUser("Foto")
	cases := []struct {
		name     string
		filename string
		ctype    string
		data     []byte
		want     int
	}{
		{"GIF", "a.gif", "image/gif", []byte("GIF89a\x01\x00\x01\x00"), http.StatusUnsupportedMediaType},
		{"bozuk JPEG", "a.jpg", "image/jpeg", append([]byte{0xFF, 0xD8, 0xFF}, bytes.Repeat([]byte{1}, 50)...), http.StatusBadRequest},
		{"çok küçük", "a.jpg", "image/jpeg", testJPEG(t, 100, 300, 7), http.StatusBadRequest},
		{"2 MB üstü", "a.jpg", "image/jpeg", bytes.Repeat([]byte{0xFF}, imageproc.MaxInputBytes+10), http.StatusRequestEntityTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, res := api.uploadAvatarAs(u.token, tc.filename, tc.ctype, tc.data)
			if code != tc.want || res.Success {
				t.Fatalf("HTTP %d, beklenen %d: %+v", code, tc.want, res)
			}
		})
	}
	res := api.raw(http.MethodPut, "/me/avatar", u.token, strings.NewReader("{}"), map[string]string{"Content-Type": "application/json"})
	if res.Code != http.StatusBadRequest {
		t.Fatalf("dosyasız istek 400 almalı: %d", res.Code)
	}
	if got := api.meAvatarVersion(u); got != "" {
		t.Fatalf("başarısız yüklemeler sürüm yazmamalı: %q", got)
	}
}

// Hesap silinince fotoğraf da silinir.
func TestAvatarIsDeletedWithAccount(t *testing.T) {
	api := newTestAPI(t)
	u := api.newUser("Silinen")
	id := api.userID(u)
	api.mustUploadAvatar(u.token, testJPEG(t, 300, 300, 8))
	api.mustOK(http.MethodDelete, "/me", u.token, nil, http.StatusOK)
	var n int64
	if err := api.db.Table("user_avatars").Where("user_id = ?", id).Count(&n).Error; err != nil || n != 0 {
		t.Fatalf("hesapla birlikte fotoğraf silinmeli: %v %d", err, n)
	}
}
