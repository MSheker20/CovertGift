package controllers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// Bu testlerde db nil verilir: reddedilen her istek veritabanına dokunmadan dönmelidir,
// aksi halde nil işaretçi paniği testi düşürür.
func TestResetDBGuardsRejectBeforeTouchingDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name       string
		appEnv     string
		allowReset bool
		body       string
		want       int
	}{
		{"production always closed", "production", true, `{"confirm":"RESET"}`, http.StatusForbidden},
		{"flag off by default", "development", false, `{"confirm":"RESET"}`, http.StatusForbidden},
		{"missing body", "development", true, ``, http.StatusBadRequest},
		{"empty object", "development", true, `{}`, http.StatusBadRequest},
		{"wrong phrase", "development", true, `{"confirm":"yes"}`, http.StatusBadRequest},
		{"lowercase phrase", "development", true, `{"confirm":"reset"}`, http.StatusBadRequest},
		{"malformed json", "development", true, `{"confirm":`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/dev/reset-db", NewDevController(nil, tc.appEnv, tc.allowReset).ResetDB)
			req := httptest.NewRequest(http.MethodPost, "/dev/reset-db", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != tc.want {
				t.Fatalf("got HTTP %d, want %d: %s", res.Code, tc.want, res.Body)
			}
		})
	}
}
