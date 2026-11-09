package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MSheker20/CovertGift/backend/internal/middleware"
)

// Gövde limiti ve giriş doğrulaması gerçek router üzerinden denenir.
// Çalıştırma koşulları için antispoiler_integration_test.go başındaki nota bakın.
func TestRequestLimitsAndValidation(t *testing.T) {
	s := setupScenario(t)
	g := "/groups/" + s.groupID

	t.Run("oversized body is rejected with 413", func(t *testing.T) {
		big := `{"title":"` + strings.Repeat("x", int(middleware.DefaultMaxBodyBytes)) + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1"+g+"/ideas", bytes.NewReader([]byte(big)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+s.member.token)
		res := httptest.NewRecorder()
		s.api.router.ServeHTTP(res, req)
		if res.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("HTTP %d, want 413", res.Code)
		}
	})

	cases := []struct {
		name, method, path, token string
		body                      any
	}{
		{"negative budget on create", http.MethodPost, "/groups", s.admin.token, map[string]any{"title": "Grup", "target_person_name": "Hedef", "total_budget": "-10"}},
		{"budget overflowing numeric(10,2)", http.MethodPost, "/groups", s.admin.token, map[string]any{"title": "Grup", "target_person_name": "Hedef", "total_budget": "1000000000"}},
		{"negative idea price", http.MethodPost, g + "/ideas", s.member.token, map[string]any{"title": "Termos", "estimated_price": "-1"}},
		{"idea price with 3 decimals", http.MethodPost, g + "/ideas", s.member.token, map[string]any{"title": "Termos", "estimated_price": "1.999"}},
		{"search_query over 255", http.MethodPost, g + "/ideas", s.member.token, map[string]any{"title": "Termos", "search_query": strings.Repeat("q", 256)}},
		{"javascript product_url", http.MethodPost, g + "/ideas", s.member.token, map[string]any{"title": "Termos", "product_url": "javascript:alert(1)"}},
		{"image_url over 500", http.MethodPost, g + "/ideas", s.member.token, map[string]any{"title": "Termos", "image_url": "https://cdn.example.com/" + strings.Repeat("i", 500)}},
		{"vote 0", http.MethodPost, "/ideas/" + s.ideaID + "/vote", s.member.token, map[string]int{"vote_type": 0}},
		{"vote 2", http.MethodPost, "/ideas/" + s.ideaID + "/vote", s.member.token, map[string]int{"vote_type": 2}},
		{"member email over 150", http.MethodPost, g + "/members", s.admin.token, map[string]string{"email": strings.Repeat("a", 145) + "@x.com"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, res := s.api.call(tc.method, tc.path, tc.token, tc.body)
			if code != http.StatusBadRequest || res.Success {
				t.Fatalf("HTTP %d, want 400: %+v", code, res)
			}
		})
	}

	t.Run("valid idea with http product_url still accepted", func(t *testing.T) {
		s.api.mustOK(http.MethodPost, g+"/ideas", s.member.token, map[string]any{
			"title": "Termos", "estimated_price": "1450.50", "product_url": "https://www.trendyol.com/x-p-1", "image_url": "https://cdn.dsmcdn.com/a.jpg",
		}, http.StatusCreated)
	})
}
