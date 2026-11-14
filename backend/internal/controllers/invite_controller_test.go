package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type stubInviteService struct {
	services.InviteService
	err      error
	days     int
	accepted *services.InviteAccepted
}

func (s *stubInviteService) Create(_, _ uuid.UUID, days int) (*services.InviteCreated, error) {
	s.days = days
	if s.err != nil {
		return nil, s.err
	}
	return &services.InviteCreated{Token: "tok"}, nil
}
func (s *stubInviteService) Revoke(_, _ uuid.UUID) error { return s.err }
func (s *stubInviteService) Preview(string, uuid.UUID) (*services.InvitePreview, error) {
	return nil, s.err
}
func (s *stubInviteService) Accept(string, uuid.UUID) (*services.InviteAccepted, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.accepted, nil
}

func serveInvite(svc services.InviteService, method, path, body string) (int, Response, *gin.Context) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	ctl := NewInviteController(svc)
	var captured *gin.Context
	with := func(h gin.HandlerFunc) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Set("userID", uuid.New())
			h(c)
			captured = c
		}
	}
	router.POST("/groups/:id/invite", with(ctl.Create))
	router.DELETE("/groups/:id/invite", with(ctl.Revoke))
	router.GET("/invites/:token", with(ctl.Preview))
	router.POST("/invites/:token/accept", with(ctl.Accept))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	var res Response
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	return rec.Code, res, captured
}

func TestInviteControllerErrorMapping(t *testing.T) {
	gid := uuid.NewString()
	for _, tc := range []struct {
		name   string
		err    error
		method string
		path   string
		body   string
		want   int
	}{
		{"invalid expiry", services.ErrInvalidInviteExpiry, "POST", "/groups/" + gid + "/invite", `{"expires_in_days":3}`, 400},
		{"missing expiry", nil, "POST", "/groups/" + gid + "/invite", `{}`, 400},
		{"not admin", services.ErrUnauthorizedInvite, "POST", "/groups/" + gid + "/invite", `{"expires_in_days":7}`, 403},
		{"revoke not admin", services.ErrUnauthorizedInvite, "DELETE", "/groups/" + gid + "/invite", "", 403},
		{"preview unknown", services.ErrInviteNotFound, "GET", "/invites/x", "", 404},
		{"preview target", services.ErrAntiSpoilerTriggered, "GET", "/invites/x", "", 403},
		{"accept unknown", services.ErrInviteNotFound, "POST", "/invites/x/accept", "", 404},
		{"accept target", services.ErrAntiSpoilerTriggered, "POST", "/invites/x/accept", "", 403},
		{"accept internal", errors.New("db down"), "POST", "/invites/x/accept", "", 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, res, _ := serveInvite(&stubInviteService{err: tc.err}, tc.method, tc.path, tc.body)
			if code != tc.want || res.Success {
				t.Fatalf("HTTP %d %+v, beklenen %d", code, res, tc.want)
			}
			if code == 403 && tc.err == services.ErrAntiSpoilerTriggered && res.Error != services.ErrAntiSpoilerTriggered.Error() {
				t.Fatalf("sürpriz mesajı değil: %q", res.Error)
			}
			if code == 500 && strings.Contains(res.Error, "db down") {
				t.Fatal("iç hata istemciye sızdı")
			}
		})
	}
}

func TestInviteCreateReturns201WithToken(t *testing.T) {
	svc := &stubInviteService{}
	code, res, _ := serveInvite(svc, "POST", "/groups/"+uuid.NewString()+"/invite", `{"expires_in_days":30}`)
	if code != http.StatusCreated || !res.Success || svc.days != 30 {
		t.Fatalf("HTTP %d %+v days=%d", code, res, svc.days)
	}
}

func TestInviteAcceptRecordsNotificationOnlyWhenJoined(t *testing.T) {
	gid := uuid.New()
	for _, joined := range []bool{true, false} {
		svc := &stubInviteService{accepted: &services.InviteAccepted{GroupID: gid, Joined: joined, MemberName: "Yeni"}}
		code, _, c := serveInvite(svc, "POST", "/invites/x/accept", "")
		if code != http.StatusOK {
			t.Fatalf("HTTP %d", code)
		}
		events := RecordedNotifications(c)
		_, changed := RecordedGroupChanged(c)
		if joined && (len(events) != 1 || events[0].Type != "member_joined" || !changed) {
			t.Fatalf("yeni üyede bildirim/yayın yok: %+v %v", events, changed)
		}
		if !joined && (len(events) != 0 || changed) {
			t.Fatalf("idempotent katılımda bildirim/yayın var: %+v %v", events, changed)
		}
	}
}
