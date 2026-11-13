package main

// Grup kapak fotoğrafı entegrasyon testleri: gerçek router, middleware zinciri ve PostgreSQL.
// Çalıştırma koşulları için antispoiler_integration_test.go başındaki nota bakın.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/middleware"
	"github.com/MSheker20/CovertGift/backend/pkg/imageproc"
)

// testPhoto dokulu bir görsel üretir; seed farklı içerik (dolayısıyla farklı sürüm) verir.
func testPhoto(w, h, seed int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			n := uint8((x*7 + y*13 + seed*31) % 29)
			img.SetRGBA(x, y, color.RGBA{R: uint8(x*255/w) + n, G: uint8(y*255/h) + n, B: uint8(seed*40) + n, A: 255})
		}
	}
	return img
}

func testJPEG(t *testing.T, w, h, seed int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, testPhoto(w, h, seed), &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func testPNG(t *testing.T, w, h, seed int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, testPhoto(w, h, seed)); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// multipartBody tek dosyalı bir multipart gövdesi kurar.
func multipartBody(t *testing.T, field, filename, contentType string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="`+field+`"; filename="`+filename+`"`)
	header.Set("Content-Type", contentType)
	part, err := w.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, w.FormDataContentType()
}

// raw yanıtı ayrıştırmadan döner (görsel uçları JSON dönmez).
func (a *testAPI) raw(method, path, token string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	a.t.Helper()
	if body == nil {
		body = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, "/api/v1"+path, body)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res := httptest.NewRecorder()
	a.router.ServeHTTP(res, req)
	return res
}

// upload görseli `file` alanında, verilen dosya adı ve Content-Type ile yükler.
func (a *testAPI) upload(groupID, token, field, filename, contentType string, data []byte) (int, apiResponse) {
	a.t.Helper()
	body, formType := multipartBody(a.t, field, filename, contentType, data)
	res := a.raw(http.MethodPut, "/groups/"+groupID+"/cover", token, body, map[string]string{"Content-Type": formType})
	var parsed apiResponse
	if err := json.Unmarshal(res.Body.Bytes(), &parsed); err != nil {
		a.t.Fatalf("PUT cover: yanıt JSON değil (HTTP %d): %s", res.Code, res.Body)
	}
	return res.Code, parsed
}

func (a *testAPI) uploadCover(groupID, token string, data []byte) (int, apiResponse) {
	a.t.Helper()
	return a.upload(groupID, token, "file", "kapak.jpg", "image/jpeg", data)
}

// mustUploadCover yükler ve dönen sürümü verir.
func (a *testAPI) mustUploadCover(groupID, token string, data []byte) string {
	a.t.Helper()
	code, res := a.uploadCover(groupID, token, data)
	if code != http.StatusOK || !res.Success {
		a.t.Fatalf("kapak yüklenemedi: HTTP %d %+v", code, res)
	}
	var out struct {
		CoverVersion string `json:"cover_version"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
	}
	if err := json.Unmarshal(res.Data, &out); err != nil || out.CoverVersion == "" {
		a.t.Fatalf("yükleme yanıtında cover_version yok: %s", res.Data)
	}
	if out.Width*3 != out.Height*4 || out.Width > imageproc.CoverMaxWidth {
		a.t.Fatalf("kapak 4:3 ve en çok 1600 geniş olmalı: %dx%d", out.Width, out.Height)
	}
	return out.CoverVersion
}

type coverGroupView struct {
	CoverVersion   string `json:"cover_version"`
	CoverAdminOnly bool   `json:"cover_admin_only"`
}

// coverState grup detayındaki kapak alanlarını döner.
func (a *testAPI) coverState(groupID, token string) coverGroupView {
	a.t.Helper()
	res := a.mustOK(http.MethodGet, "/groups/"+groupID, token, nil, http.StatusOK)
	var d struct {
		Group coverGroupView `json:"group"`
	}
	if err := json.Unmarshal(res.Data, &d); err != nil {
		a.t.Fatal(err)
	}
	return d.Group
}

func (a *testAPI) getCover(groupID, token string, headers map[string]string) *httptest.ResponseRecorder {
	a.t.Helper()
	return a.raw(http.MethodGet, "/groups/"+groupID+"/cover", token, nil, headers)
}

func (a *testAPI) coverRows(groupID string) int64 {
	a.t.Helper()
	var n int64
	if err := a.db.Table("group_covers").Where("group_id = ?", groupID).Count(&n).Error; err != nil {
		a.t.Fatal(err)
	}
	return n
}

// Hedef kişi kapak uçlarının hiçbirine (görüntüleme dahil) erişemez.
func TestCoverAntiSpoilerTargetBlocked(t *testing.T) {
	s := setupScenario(t)
	version := s.api.mustUploadCover(s.groupID, s.admin.token, testJPEG(t, 1200, 900, 1))
	stored := s.api.getCover(s.groupID, s.admin.token, nil).Body.Bytes()

	assertBlocked := func(name string, res *httptest.ResponseRecorder) {
		t.Helper()
		var parsed apiResponse
		if err := json.Unmarshal(res.Body.Bytes(), &parsed); err != nil {
			t.Fatalf("%s: yanıt JSON değil (HTTP %d, %d bayt)", name, res.Code, res.Body.Len())
		}
		if res.Code != http.StatusForbidden || parsed.Success || parsed.Error != surpriseMessage {
			t.Fatalf("%s: hedef kişi engellenmedi: HTTP %d %+v", name, res.Code, parsed)
		}
		if len(parsed.Data) > 0 && string(parsed.Data) != "null" {
			t.Fatalf("%s: 403 yanıtı veri sızdırdı: %s", name, parsed.Data)
		}
		if ct := res.Header().Get("Content-Type"); strings.HasPrefix(ct, "image/") {
			t.Fatalf("%s: 403 yanıtı görsel türünde döndü: %s", name, ct)
		}
		if res.Header().Get("ETag") != "" {
			t.Fatalf("%s: 403 yanıtı ETag sızdırdı", name)
		}
		if bytes.Contains(res.Body.Bytes(), stored[:64]) || strings.Contains(res.Body.String(), version) {
			t.Fatalf("%s: 403 yanıtı kapak verisi içeriyor", name)
		}
	}

	g := "/groups/" + s.groupID
	assertBlocked("GET cover", s.api.getCover(s.groupID, s.target.token, nil))
	// Doğru ETag'i bilse bile 304 alamaz (kapağın varlığı/sürümü doğrulanamaz).
	assertBlocked("GET cover If-None-Match", s.api.getCover(s.groupID, s.target.token, map[string]string{"If-None-Match": `"` + version + `"`}))
	body, formType := multipartBody(t, "file", "kapak.jpg", "image/jpeg", testJPEG(t, 800, 600, 2))
	assertBlocked("PUT cover", s.api.raw(http.MethodPut, g+"/cover", s.target.token, body, map[string]string{"Content-Type": formType}))
	assertBlocked("DELETE cover", s.api.raw(http.MethodDelete, g+"/cover", s.target.token, nil, nil))
	assertBlocked("PATCH cover-policy", s.api.raw(http.MethodPatch, g+"/cover-policy", s.target.token,
		strings.NewReader(`{"cover_admin_only":true}`), map[string]string{"Content-Type": "application/json"}))

	// Hedefin denemeleri hiçbir şeyi değiştirmedi.
	if state := s.api.coverState(s.groupID, s.admin.token); state.CoverVersion != version || state.CoverAdminOnly {
		t.Fatalf("hedefin isteklerinden sonra kapak durumu değişti: %+v", state)
	}
	if got := s.api.getCover(s.groupID, s.member.token, nil); got.Code != http.StatusOK || !bytes.Equal(got.Body.Bytes(), stored) {
		t.Fatalf("hedefin isteklerinden sonra kapak değişti: HTTP %d", got.Code)
	}
	// Hedefin grup listesi kapağa dair hiçbir iz taşımaz.
	if list := s.api.mustOK(http.MethodGet, "/groups", s.target.token, nil, http.StatusOK); strings.Contains(string(list.Data), version) {
		t.Fatal("hedefin grup listesi kapak sürümünü içeriyor")
	}
}

// Hedef sonradan bağlanırsa, önceden yüklenmiş kapağa da erişemez.
func TestCoverAntiSpoilerTargetLinkedLater(t *testing.T) {
	api := newTestAPI(t)
	admin, target := api.newUser("Admin"), api.newUser("Target")
	groupID := idOf(t, api.mustOK(http.MethodPost, "/groups", admin.token, map[string]any{
		"title": "Hedefsiz grup", "target_person_name": "Henüz kayıtsız", "total_budget": "500",
	}, http.StatusCreated))
	version := api.mustUploadCover(groupID, admin.token, testJPEG(t, 800, 600, 1))

	// Bağlanmadan önce: yabancı biri olarak 403, ama sürpriz mesajı yok.
	if res := api.getCover(groupID, target.token, nil); res.Code != http.StatusForbidden || strings.Contains(res.Body.String(), surpriseMessage) {
		t.Fatalf("bağlanmadan önce: HTTP %d %s", res.Code, res.Body)
	}
	api.mustOK(http.MethodPatch, "/groups/"+groupID+"/target", admin.token, map[string]string{"email": target.email}, http.StatusOK)

	for _, headers := range []map[string]string{nil, {"If-None-Match": `"` + version + `"`}} {
		res := api.getCover(groupID, target.token, headers)
		if res.Code != http.StatusForbidden || !strings.Contains(res.Body.String(), surpriseMessage) {
			t.Fatalf("sonradan bağlanan hedef kapağa erişti: HTTP %d", res.Code)
		}
	}
	if code, res := api.uploadCover(groupID, target.token, testJPEG(t, 800, 600, 2)); code != http.StatusForbidden || res.Error != surpriseMessage {
		t.Fatalf("sonradan bağlanan hedef kapak yükleyebildi: HTTP %d", code)
	}
}

func TestCoverPermissions(t *testing.T) {
	s := setupScenario(t)
	g := "/groups/" + s.groupID
	policy := func(token string, adminOnly bool) (int, apiResponse) {
		return s.api.call(http.MethodPatch, g+"/cover-policy", token, map[string]bool{"cover_admin_only": adminOnly})
	}

	// Varsayılan: kısıt kapalı, üye ekleyebilir, değiştirebilir ve kaldırabilir.
	if state := s.api.coverState(s.groupID, s.member.token); state.CoverAdminOnly || state.CoverVersion != "" {
		t.Fatalf("yeni grupta kapak durumu: %+v", state)
	}
	first := s.api.mustUploadCover(s.groupID, s.member.token, testJPEG(t, 800, 600, 1))
	second := s.api.mustUploadCover(s.groupID, s.member.token, testJPEG(t, 800, 600, 2))
	if first == second {
		t.Fatal("farklı görsel aynı sürümü üretti")
	}
	s.api.mustOK(http.MethodDelete, g+"/cover", s.member.token, nil, http.StatusOK)

	// Politika ucunu yalnızca admin çağırır.
	if code, res := policy(s.member.token, true); code != http.StatusForbidden || res.Error == surpriseMessage {
		t.Fatalf("üye kapak iznini değiştirebildi: HTTP %d %+v", code, res)
	}
	if s.api.coverState(s.groupID, s.admin.token).CoverAdminOnly {
		t.Fatal("üyenin reddedilen isteği politikayı değiştirdi")
	}
	for _, bad := range []any{map[string]any{}, map[string]any{"cover_admin_only": "evet"}, map[string]any{"cover_admin_only": nil}} {
		if code, _ := s.api.call(http.MethodPatch, g+"/cover-policy", s.admin.token, bad); code != http.StatusBadRequest {
			t.Fatalf("geçersiz politika gövdesi %v: HTTP %d", bad, code)
		}
	}

	// Kısıt açık: üye değiştiremez/kaldıramaz ama görebilir; admin her zaman yapar.
	adminVersion := s.api.mustUploadCover(s.groupID, s.admin.token, testJPEG(t, 800, 600, 3))
	if code, res := policy(s.admin.token, true); code != http.StatusOK || !strings.Contains(string(res.Data), "true") {
		t.Fatalf("admin kısıtı açamadı: HTTP %d %+v", code, res)
	}
	if !s.api.coverState(s.groupID, s.member.token).CoverAdminOnly {
		t.Fatal("grup detayı cover_admin_only=true göstermiyor")
	}
	if code, res := s.api.uploadCover(s.groupID, s.member.token, testJPEG(t, 800, 600, 4)); code != http.StatusForbidden || res.Error == surpriseMessage {
		t.Fatalf("kısıt açıkken üye kapak yükleyebildi: HTTP %d %+v", code, res)
	}
	if code, _ := s.api.call(http.MethodDelete, g+"/cover", s.member.token, nil); code != http.StatusForbidden {
		t.Fatalf("kısıt açıkken üye kapağı kaldırabildi: HTTP %d", code)
	}
	if state := s.api.coverState(s.groupID, s.admin.token); state.CoverVersion != adminVersion {
		t.Fatalf("üyenin reddedilen istekleri kapağı değiştirdi: %+v", state)
	}
	if res := s.api.getCover(s.groupID, s.member.token, nil); res.Code != http.StatusOK {
		t.Fatalf("kısıt açıkken üye kapağı göremedi: HTTP %d", res.Code)
	}
	if v := s.api.mustUploadCover(s.groupID, s.admin.token, testJPEG(t, 800, 600, 5)); v == adminVersion {
		t.Fatal("admin kısıt açıkken kapağı değiştiremedi")
	}
	s.api.mustOK(http.MethodDelete, g+"/cover", s.admin.token, nil, http.StatusOK)
	s.api.mustUploadCover(s.groupID, s.admin.token, testJPEG(t, 800, 600, 6))

	// Kısıt kapanınca üye yeniden değiştirebilir.
	if code, _ := policy(s.admin.token, false); code != http.StatusOK {
		t.Fatalf("admin kısıtı kapatamadı: HTTP %d", code)
	}
	s.api.mustUploadCover(s.groupID, s.member.token, testJPEG(t, 800, 600, 7))
	s.api.mustOK(http.MethodDelete, g+"/cover", s.member.token, nil, http.StatusOK)
}

func TestCoverOutsiderAndUnauthenticated(t *testing.T) {
	s := setupScenario(t)
	s.api.mustUploadCover(s.groupID, s.admin.token, testJPEG(t, 800, 600, 1))
	g := "/groups/" + s.groupID
	jsonHeader := map[string]string{"Content-Type": "application/json"}

	requests := func(token, group string) map[string]*httptest.ResponseRecorder {
		body, formType := multipartBody(t, "file", "kapak.jpg", "image/jpeg", testJPEG(t, 800, 600, 2))
		return map[string]*httptest.ResponseRecorder{
			"GET cover":          s.api.raw(http.MethodGet, group+"/cover", token, nil, nil),
			"PUT cover":          s.api.raw(http.MethodPut, group+"/cover", token, body, map[string]string{"Content-Type": formType}),
			"DELETE cover":       s.api.raw(http.MethodDelete, group+"/cover", token, nil, nil),
			"PATCH cover-policy": s.api.raw(http.MethodPatch, group+"/cover-policy", token, strings.NewReader(`{"cover_admin_only":true}`), jsonHeader),
		}
	}
	for name, res := range requests(s.outside.token, g) {
		if res.Code != http.StatusForbidden || strings.Contains(res.Body.String(), surpriseMessage) || strings.HasPrefix(res.Header().Get("Content-Type"), "image/") {
			t.Errorf("üye olmayan %s: HTTP %d %s", name, res.Code, res.Body)
		}
	}
	for name, res := range requests("", g) {
		if res.Code != http.StatusUnauthorized {
			t.Errorf("oturumsuz %s: HTTP %d", name, res.Code)
		}
	}
	for name, res := range requests(s.admin.token, "/groups/00000000-0000-4000-8000-000000000000") {
		if res.Code != http.StatusNotFound {
			t.Errorf("olmayan grup %s: HTTP %d", name, res.Code)
		}
	}
	if s.api.coverRows(s.groupID) != 1 || s.api.coverState(s.groupID, s.admin.token).CoverAdminOnly {
		t.Fatal("yetkisiz istekler kapağı ya da politikayı değiştirdi")
	}
}

func TestCoverVersionETagAndDelete(t *testing.T) {
	s := setupScenario(t)
	g := "/groups/" + s.groupID

	// Kapak yokken: 404, liste ve detayda sürüm yok.
	if res := s.api.getCover(s.groupID, s.admin.token, nil); res.Code != http.StatusNotFound {
		t.Fatalf("kapak yokken GET: HTTP %d", res.Code)
	}
	if list := s.api.mustOK(http.MethodGet, "/groups", s.admin.token, nil, http.StatusOK); strings.Contains(string(list.Data), "cover_version") {
		t.Fatalf("kapak yokken liste cover_version içeriyor: %s", list.Data)
	}

	v1 := s.api.mustUploadCover(s.groupID, s.admin.token, testJPEG(t, 3000, 2000, 1))
	if got := s.api.coverState(s.groupID, s.member.token).CoverVersion; got != v1 {
		t.Fatalf("detaydaki sürüm %q, yüklemedeki %q", got, v1)
	}

	// Görsel: JPEG, 4:3, en çok 1600x1200, ETag = sürüm, özel önbellek.
	res := s.api.getCover(s.groupID, s.member.token, nil)
	if res.Code != http.StatusOK || res.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("GET cover: HTTP %d, tür %q", res.Code, res.Header().Get("Content-Type"))
	}
	etag := res.Header().Get("ETag")
	if etag != `"`+v1+`"` {
		t.Fatalf("ETag %q, beklenen %q", etag, `"`+v1+`"`)
	}
	cc := res.Header().Get("Cache-Control")
	if !strings.Contains(cc, "private") || !strings.Contains(cc, "max-age") || strings.Contains(cc, "public") {
		t.Fatalf("Cache-Control %q", cc)
	}
	stored := res.Body.Bytes()
	cfg, format, err := image.DecodeConfig(bytes.NewReader(stored))
	if err != nil || format != "jpeg" || cfg.Width != 1600 || cfg.Height != 1200 {
		t.Fatalf("saklanan kapak: %v %s %dx%d", err, format, cfg.Width, cfg.Height)
	}

	// If-None-Match → 304, gövdesiz. Eski/başka ETag → 200.
	for _, header := range []string{etag, "W/" + etag, `"baska", ` + etag, "*"} {
		res := s.api.getCover(s.groupID, s.member.token, map[string]string{"If-None-Match": header})
		if res.Code != http.StatusNotModified || res.Body.Len() != 0 || res.Header().Get("ETag") != etag {
			t.Fatalf("If-None-Match %q: HTTP %d, %d bayt", header, res.Code, res.Body.Len())
		}
	}
	if res := s.api.getCover(s.groupID, s.member.token, map[string]string{"If-None-Match": `"eski"`}); res.Code != http.StatusOK || !bytes.Equal(res.Body.Bytes(), stored) {
		t.Fatalf("eski ETag ile: HTTP %d", res.Code)
	}

	// Liste ve detay sürümü taşır ama baytları asla taşımaz.
	if len(stored) < 20<<10 {
		t.Fatalf("test görseli çok küçük sıkıştı (%d bayt); bayt sızıntısı testi anlamsız olur", len(stored))
	}
	list := s.api.mustOK(http.MethodGet, "/groups", s.admin.token, nil, http.StatusOK)
	detail := s.api.mustOK(http.MethodGet, g, s.admin.token, nil, http.StatusOK)
	for name, payload := range map[string][]byte{"liste": list.Data, "detay": detail.Data} {
		if !strings.Contains(string(payload), `"cover_version":"`+v1+`"`) {
			t.Errorf("%s cover_version taşımıyor", name)
		}
		if len(payload) > 8<<10 {
			t.Errorf("%s yanıtı %d bayt; kapak baytları sızmış olabilir", name, len(payload))
		}
		for _, leak := range []string{`"data"`, `"content_type"`, `"etag"`, "/9j/"} { // /9j/ = base64 JPEG başlangıcı
			if strings.Contains(string(payload), leak) {
				t.Errorf("%s yanıtı kapak alanı içeriyor: %s", name, leak)
			}
		}
	}

	// Değiştirince sürüm ve içerik değişir; eski ETag artık 200 döner.
	v2 := s.api.mustUploadCover(s.groupID, s.member.token, testPNG(t, 640, 480, 2))
	if v2 == v1 {
		t.Fatal("kapak değişti ama cover_version aynı kaldı")
	}
	if got := s.api.coverState(s.groupID, s.admin.token).CoverVersion; got != v2 {
		t.Fatalf("detay yeni sürümü göstermiyor: %q", got)
	}
	if res := s.api.getCover(s.groupID, s.admin.token, map[string]string{"If-None-Match": etag}); res.Code != http.StatusOK || res.Header().Get("ETag") != `"`+v2+`"` {
		t.Fatalf("değişen kapak eski ETag ile 304 döndü: HTTP %d", res.Code)
	}
	if s.api.coverRows(s.groupID) != 1 {
		t.Fatal("grup başına tek kapak satırı olmalı")
	}

	// Aynı görsel yeniden yüklenirse sürüm aynı kalır (içerik özeti).
	if again := s.api.mustUploadCover(s.groupID, s.admin.token, testPNG(t, 640, 480, 2)); again != v2 {
		t.Fatalf("aynı görsel farklı sürüm üretti: %q / %q", again, v2)
	}

	// Silince: 404, sürüm boş, satır yok; ikinci silme de başarılı.
	s.api.mustOK(http.MethodDelete, g+"/cover", s.admin.token, nil, http.StatusOK)
	if res := s.api.getCover(s.groupID, s.admin.token, map[string]string{"If-None-Match": `"` + v2 + `"`}); res.Code != http.StatusNotFound {
		t.Fatalf("silinen kapak: HTTP %d", res.Code)
	}
	if state := s.api.coverState(s.groupID, s.admin.token); state.CoverVersion != "" {
		t.Fatalf("silindikten sonra sürüm: %q", state.CoverVersion)
	}
	if s.api.coverRows(s.groupID) != 0 {
		t.Fatal("silinen kapağın satırı duruyor")
	}
	s.api.mustOK(http.MethodDelete, g+"/cover", s.admin.token, nil, http.StatusOK)
}

// jpegWithGPS JPEG'e yön (6) ve sahte konum taşıyan bir EXIF bölümü ekler.
func jpegWithGPS(jpg []byte) []byte {
	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, 6, 0, 0, 0, 0, 0, 0}
	tiff = append(tiff, []byte("GPSLatitude=41.0082;GPSLongitude=28.9784")...)
	payload := append([]byte("Exif\x00\x00"), tiff...)
	segment := binary.BigEndian.AppendUint16([]byte{0xFF, 0xE1}, uint16(len(payload)+2))
	segment = append(segment, payload...)
	out := append([]byte{}, jpg[:2]...)
	out = append(out, segment...)
	return append(out, jpg[2:]...)
}

func TestCoverStoredImageIsReencoded(t *testing.T) {
	s := setupScenario(t)
	input := jpegWithGPS(testJPEG(t, 800, 600, 1)) // 800x600 depolanan, yön 6 → 600x800 gösterilen
	s.api.mustUploadCover(s.groupID, s.admin.token, input)
	stored := s.api.getCover(s.groupID, s.admin.token, nil).Body.Bytes()
	for _, leak := range []string{"Exif", "GPSLatitude", "41.0082"} {
		if bytes.Contains(stored, []byte(leak)) {
			t.Fatalf("saklanan kapak metadata içeriyor: %s", leak)
		}
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(stored))
	if err != nil || cfg.Width != 600 || cfg.Height != 450 {
		t.Fatalf("yön uygulanıp 4:3 kırpılmalıydı (600x450): %v %dx%d", err, cfg.Width, cfg.Height)
	}
	// Veritabanındaki satır yalnızca işlenmiş görseli tutar.
	var row struct {
		ContentType string
		Width       int
		Height      int
		Size        int
	}
	if err := s.api.db.Raw("SELECT content_type, width, height, octet_length(data) AS size FROM group_covers WHERE group_id = ?", s.groupID).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ContentType != "image/jpeg" || row.Width != 600 || row.Height != 450 || row.Size != len(stored) {
		t.Fatalf("group_covers satırı: %+v (GET %d bayt)", row, len(stored))
	}
}

func TestCoverRejectsBadUploads(t *testing.T) {
	s := setupScenario(t)
	g := "/groups/" + s.groupID
	good := s.api.mustUploadCover(s.groupID, s.admin.token, testJPEG(t, 800, 600, 1))
	valid := testJPEG(t, 800, 600, 2)

	// Başlığında 20000x20000 yazan küçük PNG (decompression bomb).
	bomb := testPNG(t, 64, 64, 1)
	binary.BigEndian.PutUint32(bomb[16:20], 20000)
	binary.BigEndian.PutUint32(bomb[20:24], 20000)

	cases := []struct {
		name, field, filename, contentType string
		data                               []byte
		want                               int
	}{
		{"bozuk jpeg", "file", "a.jpg", "image/jpeg", valid[:len(valid)/3], http.StatusBadRequest},
		{"png imzalı çöp", "file", "a.png", "image/png", append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, bytes.Repeat([]byte{7}, 128)...), http.StatusBadRequest},
		{"çözünürlük bombası", "file", "a.png", "image/png", bomb, http.StatusBadRequest},
		{"çok küçük görsel", "file", "a.png", "image/png", testPNG(t, 10, 10, 1), http.StatusBadRequest},
		{"jpeg diye gönderilen metin", "file", "a.jpg", "image/jpeg", []byte("ben görsel değilim"), http.StatusUnsupportedMediaType},
		{"gif", "file", "a.gif", "image/gif", []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\xff\xff\xff\x00\x00\x00;"), http.StatusUnsupportedMediaType},
		{"jpeg diye gönderilen svg", "file", "a.jpg", "image/jpeg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), http.StatusUnsupportedMediaType},
		{"heic", "file", "a.heic", "image/heic", append([]byte{0, 0, 0, 0x18}, []byte("ftypheic\x00\x00\x00\x00mif1heic")...), http.StatusUnsupportedMediaType},
		{"boş dosya", "file", "a.jpg", "image/jpeg", nil, http.StatusUnsupportedMediaType},
		{"yanlış alan adı", "image", "a.jpg", "image/jpeg", valid, http.StatusBadRequest},
		{"10 MB üstü dosya", "file", "a.jpg", "image/jpeg", append(append([]byte{}, valid...), make([]byte, imageproc.MaxInputBytes)...), http.StatusRequestEntityTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, res := s.api.upload(s.groupID, s.admin.token, tc.field, tc.filename, tc.contentType, tc.data)
			if code != tc.want || res.Success || res.Error == "" {
				t.Fatalf("HTTP %d, beklenen %d: %+v", code, tc.want, res)
			}
		})
	}

	t.Run("gövde sınırını aşan istek okunmadan 413", func(t *testing.T) {
		body, formType := multipartBody(t, "file", "a.jpg", "image/jpeg", make([]byte, middleware.CoverUploadMaxBodyBytes+1))
		res := s.api.raw(http.MethodPut, g+"/cover", s.admin.token, body, map[string]string{"Content-Type": formType})
		if res.Code != http.StatusRequestEntityTooLarge || !strings.Contains(res.Body.String(), "10 MB") {
			t.Fatalf("HTTP %d %s", res.Code, res.Body)
		}
	})
	t.Run("uzunluğu bildirilmeyen büyük gövde 413", func(t *testing.T) {
		body, formType := multipartBody(t, "file", "a.jpg", "image/jpeg", make([]byte, middleware.CoverUploadMaxBodyBytes+1))
		req := httptest.NewRequest(http.MethodPut, "/api/v1"+g+"/cover", io.NopCloser(body))
		req.ContentLength = -1
		req.Header.Set("Content-Type", formType)
		req.Header.Set("Authorization", "Bearer "+s.admin.token)
		res := httptest.NewRecorder()
		s.api.router.ServeHTTP(res, req)
		if res.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("HTTP %d %s", res.Code, res.Body)
		}
	})
	t.Run("multipart olmayan gövde 400", func(t *testing.T) {
		if code, _ := s.api.call(http.MethodPut, g+"/cover", s.admin.token, map[string]string{"file": "x"}); code != http.StatusBadRequest {
			t.Fatalf("HTTP %d", code)
		}
	})
	t.Run("tür başlığa değil bayta göre: jpeg diye gönderilen png kabul edilir", func(t *testing.T) {
		code, res := s.api.upload(s.groupID, s.member.token, "file", "a.jpg", "image/jpeg", testPNG(t, 640, 480, 3))
		if code != http.StatusOK {
			t.Fatalf("HTTP %d %+v", code, res)
		}
		s.api.mustUploadCover(s.groupID, s.admin.token, testJPEG(t, 800, 600, 1)) // eski kapağı geri koy
	})
	t.Run("büyük kapak sınırı diğer uçlara uygulanmaz", func(t *testing.T) {
		big := `{"title":"` + strings.Repeat("x", int(middleware.DefaultMaxBodyBytes)) + `"}`
		for _, r := range []struct{ method, path string }{
			{http.MethodPost, g + "/ideas"},
			{http.MethodPatch, g + "/cover-policy"},
			{http.MethodPost, g + "/cover"}, // yalnızca PUT geniş sınıra sahiptir
		} {
			res := s.api.raw(r.method, r.path, s.admin.token, strings.NewReader(big), map[string]string{"Content-Type": "application/json"})
			if res.Code != http.StatusRequestEntityTooLarge {
				t.Errorf("%s %s: HTTP %d, beklenen 413", r.method, r.path, res.Code)
			}
		}
	})

	// Reddedilen yüklemeler mevcut kapağı bozmadı.
	if got := s.api.coverState(s.groupID, s.admin.token).CoverVersion; got != good {
		t.Fatalf("başarısız yüklemelerden sonra kapak değişti: %q / %q", got, good)
	}
}

func TestCoverIsDeletedWithGroupAndCreator(t *testing.T) {
	s := setupScenario(t)
	s.api.mustUploadCover(s.groupID, s.member.token, testJPEG(t, 800, 600, 1))
	if s.api.coverRows(s.groupID) != 1 {
		t.Fatal("kapak satırı oluşmadı")
	}
	s.api.mustOK(http.MethodDelete, "/groups/"+s.groupID, s.admin.token, nil, http.StatusOK)
	if s.api.coverRows(s.groupID) != 0 {
		t.Fatal("grup silindi ama kapağı kaldı (CASCADE çalışmadı)")
	}

	// Kapağı yükleyen üye hesabını silerse kapak kalır (updated_by boşalır); grubu kuran
	// hesabını silerse grup ve kapağı birlikte gider.
	groupID := idOf(t, s.api.mustOK(http.MethodPost, "/groups", s.admin.token, map[string]any{
		"title": "İkinci grup", "target_person_name": "Hedef", "total_budget": "100",
	}, http.StatusCreated))
	s.api.mustOK(http.MethodPost, "/groups/"+groupID+"/members", s.admin.token, map[string]string{"email": s.member.email}, http.StatusCreated)
	s.api.mustUploadCover(groupID, s.member.token, testJPEG(t, 800, 600, 2))
	s.api.mustOK(http.MethodDelete, "/me", s.member.token, nil, http.StatusOK)
	var updatedBy *string
	if err := s.api.db.Raw("SELECT updated_by::text FROM group_covers WHERE group_id = ?", groupID).Scan(&updatedBy).Error; err != nil {
		t.Fatal(err)
	}
	if s.api.coverRows(groupID) != 1 || updatedBy != nil {
		t.Fatalf("yükleyen silinince kapak kalmalı ve updated_by boşalmalıydı: satır=%d updated_by=%v", s.api.coverRows(groupID), updatedBy)
	}
	s.api.mustOK(http.MethodDelete, "/me", s.admin.token, nil, http.StatusOK)
	if s.api.coverRows(groupID) != 0 {
		t.Fatal("kurucu hesabını sildi ama grubun kapağı kaldı")
	}
}

// Kapak değişiklikleri grup akışına sinyal gönderir ama bildirim satırı üretmez.
func TestCoverChangesPublishGroupEventWithoutNotification(t *testing.T) {
	s := setupScenario(t)
	srv := httptest.NewServer(s.api.router)
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/groups/"+s.groupID+"/events", nil)
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
	expectChanged := func(what string) {
		t.Helper()
		deadline := time.After(2 * time.Second)
		for {
			select {
			case line := <-lines:
				if line == "event: changed" {
					return
				}
			case <-deadline:
				t.Fatalf("%s sonrası 'changed' olayı gelmedi", what)
			}
		}
	}
	expectSilence := func(what string) {
		t.Helper()
		deadline := time.After(300 * time.Millisecond)
		for {
			select {
			case line := <-lines:
				if line == "event: changed" {
					t.Fatalf("%s 'changed' olayı üretti", what)
				}
			case <-deadline:
				return
			}
		}
	}
	notifications := func() int64 {
		var n int64
		if err := s.api.db.Table("notifications").Where("group_id = ?", s.groupID).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := notifications()
	g := "/groups/" + s.groupID

	s.api.mustUploadCover(s.groupID, s.admin.token, testJPEG(t, 800, 600, 1))
	expectChanged("kapak yükleme")
	s.api.mustOK(http.MethodPatch, g+"/cover-policy", s.admin.token, map[string]bool{"cover_admin_only": true}, http.StatusOK)
	expectChanged("politika değişikliği")

	// Başarısız istekler ve görüntüleme olay üretmez.
	if code, _ := s.api.uploadCover(s.groupID, s.member.token, testJPEG(t, 800, 600, 2)); code != http.StatusForbidden {
		t.Fatalf("HTTP %d", code)
	}
	if code, _ := s.api.uploadCover(s.groupID, s.admin.token, []byte("görsel değil")); code != http.StatusUnsupportedMediaType {
		t.Fatalf("HTTP %d", code)
	}
	s.api.getCover(s.groupID, s.member.token, nil)
	expectSilence("başarısız yükleme / görüntüleme")

	s.api.mustOK(http.MethodDelete, g+"/cover", s.admin.token, nil, http.StatusOK)
	expectChanged("kapak silme")

	if after := notifications(); after != before {
		t.Fatalf("kapak işlemleri bildirim üretti: %d → %d", before, after)
	}
}
