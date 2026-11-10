package main

// Profil güncelleme (PATCH /me) ve şifre değiştirme (PUT /me/password) entegrasyon testleri.
// Çalıştırma koşulları için antispoiler_integration_test.go başındaki nota bakın.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type profileUpdate struct {
	User struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		Email        string `json:"email"`
		PasswordHash string `json:"password_hash"`
	} `json:"user"`
	Token string `json:"token"`
}

func decodeProfile(t *testing.T, res apiResponse) profileUpdate {
	t.Helper()
	var p profileUpdate
	if err := json.Unmarshal(res.Data, &p); err != nil {
		t.Fatalf("profil yanıtı okunamadı: %s", res.Data)
	}
	if strings.Contains(string(res.Data), "password") {
		t.Fatalf("yanıtta şifre alanı var: %s", res.Data)
	}
	return p
}

func TestUpdateProfile(t *testing.T) {
	api := newTestAPI(t)
	can := api.newUser("Can")
	ela := api.newUser("Ela")

	t.Run("name only keeps the token", func(t *testing.T) {
		p := decodeProfile(t, api.mustOK(http.MethodPatch, "/me", can.token, map[string]string{"name": "Can Yılmaz", "email": can.email}, http.StatusOK))
		if p.User.Name != "Can Yılmaz" || p.User.Email != can.email || p.Token != "" {
			t.Fatalf("beklenmeyen yanıt: %+v", p)
		}
	})

	t.Run("new email returns a fresh token and allows login with it", func(t *testing.T) {
		newEmail := "can-yeni-" + can.email
		p := decodeProfile(t, api.mustOK(http.MethodPatch, "/me", can.token, map[string]string{"name": "Can Yılmaz", "email": newEmail}, http.StatusOK))
		if p.User.Email != newEmail || p.Token == "" {
			t.Fatalf("yeni e-posta veya token yok: %+v", p)
		}
		// Yeni token da eski token da geçerli (oturumlar iptal edilmez).
		for _, tok := range []string{p.Token, can.token} {
			var me struct {
				Email string `json:"email"`
			}
			_ = json.Unmarshal(api.mustOK(http.MethodGet, "/me", tok, nil, http.StatusOK).Data, &me)
			if me.Email != newEmail {
				t.Fatalf("/me eski e-postayı döndü: %s", me.Email)
			}
		}
		api.mustOK(http.MethodPost, "/auth/login", "", map[string]string{"email": newEmail, "password": testPassword}, http.StatusOK)
		if code, _ := api.call(http.MethodPost, "/auth/login", "", map[string]string{"email": can.email, "password": testPassword}); code != http.StatusUnauthorized {
			t.Fatalf("eski e-postayla giriş: HTTP %d, want 401", code)
		}
		can.email = newEmail
	})

	t.Run("changing the case of own email is allowed", func(t *testing.T) {
		upper := strings.ToUpper(can.email[:1]) + can.email[1:]
		api.mustOK(http.MethodPatch, "/me", can.token, map[string]string{"name": "Can Yılmaz", "email": upper}, http.StatusOK)
		can.email = upper
	})

	t.Run("another user's email is 409, case-insensitively", func(t *testing.T) {
		for _, email := range []string{ela.email, strings.ToUpper(ela.email)} {
			if code, res := api.call(http.MethodPatch, "/me", can.token, map[string]string{"name": "Can", "email": email}); code != http.StatusConflict || res.Success {
				t.Fatalf("%s: HTTP %d, want 409: %+v", email, code, res)
			}
		}
	})

	invalid := []struct {
		name string
		body any
	}{
		{"missing name", map[string]string{"email": "x@test.local"}},
		{"missing email", map[string]string{"name": "Can"}},
		{"short name", map[string]string{"name": "C", "email": "x@test.local"}},
		{"long name", map[string]string{"name": strings.Repeat("c", 101), "email": "x@test.local"}},
		{"bad email", map[string]string{"name": "Can", "email": "not-an-email"}},
		{"long email", map[string]string{"name": "Can", "email": strings.Repeat("a", 145) + "@x.com"}},
	}
	for _, tc := range invalid {
		t.Run("400 "+tc.name, func(t *testing.T) {
			if code, res := api.call(http.MethodPatch, "/me", can.token, tc.body); code != http.StatusBadRequest || res.Success {
				t.Fatalf("HTTP %d, want 400: %+v", code, res)
			}
		})
	}

	t.Run("no token is 401", func(t *testing.T) {
		if code, _ := api.call(http.MethodPatch, "/me", "", map[string]string{"name": "Can", "email": "x@test.local"}); code != http.StatusUnauthorized {
			t.Fatalf("HTTP %d, want 401", code)
		}
	})

	// Başarısız denemeler profili değiştirmedi.
	var me struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	_ = json.Unmarshal(api.mustOK(http.MethodGet, "/me", can.token, nil, http.StatusOK).Data, &me)
	if me.Name != "Can Yılmaz" || me.Email != can.email {
		t.Fatalf("başarısız denemeler profili değiştirdi: %+v", me)
	}
}

func TestChangePassword(t *testing.T) {
	api := newTestAPI(t)
	can := api.newUser("Can")
	const newPassword = "YeniSifre456!"

	cases := []struct {
		name string
		body any
	}{
		{"wrong current password", map[string]string{"current_password": "yanlis-sifre", "new_password": newPassword}},
		{"same as current", map[string]string{"current_password": testPassword, "new_password": testPassword}},
		{"new password too short", map[string]string{"current_password": testPassword, "new_password": "12345"}},
		{"new password too long", map[string]string{"current_password": testPassword, "new_password": strings.Repeat("p", 101)}},
		{"missing current", map[string]string{"new_password": newPassword}},
	}
	for _, tc := range cases {
		t.Run("400 "+tc.name, func(t *testing.T) {
			if code, res := api.call(http.MethodPut, "/me/password", can.token, tc.body); code != http.StatusBadRequest || res.Success {
				t.Fatalf("HTTP %d, want 400: %+v", code, res)
			}
		})
	}
	// Yanlış şifre denemesi oturumu kapatmamalı (istemci 401'de çıkış yapar).
	api.mustOK(http.MethodGet, "/me", can.token, nil, http.StatusOK)

	t.Run("no token is 401", func(t *testing.T) {
		if code, _ := api.call(http.MethodPut, "/me/password", "", map[string]string{"current_password": testPassword, "new_password": newPassword}); code != http.StatusUnauthorized {
			t.Fatalf("HTTP %d, want 401", code)
		}
	})

	api.mustOK(http.MethodPut, "/me/password", can.token, map[string]string{"current_password": testPassword, "new_password": newPassword}, http.StatusOK)

	api.mustOK(http.MethodPost, "/auth/login", "", map[string]string{"email": can.email, "password": newPassword}, http.StatusOK)
	if code, _ := api.call(http.MethodPost, "/auth/login", "", map[string]string{"email": can.email, "password": testPassword}); code != http.StatusUnauthorized {
		t.Fatalf("eski şifreyle giriş: HTTP %d, want 401", code)
	}
	// Mevcut oturum açık kalır.
	api.mustOK(http.MethodGet, "/me", can.token, nil, http.StatusOK)
}
